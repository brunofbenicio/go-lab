// Command server runs the webhook receiver API.
//
// Configuration comes from environment variables:
//
//	WEBHOOK_SECRET       shared HMAC secret, at least 16 bytes (required)
//	PORT                 TCP port to listen on (default 8080)
//	MAX_BODY_BYTES       largest accepted request body (default 65536)
//	TIMESTAMP_TOLERANCE  accepted distance between a delivery's timestamp and
//	                     the server clock, as a Go duration (default 5m)
//	IDEMPOTENCY_TTL      how long delivery IDs are remembered (default 24h);
//	                     must be at least twice TIMESTAMP_TOLERANCE
//
// The secret is never logged. Generate one with, for example,
// `openssl rand -hex 32`.
//
// Accepted events are logged; replace logProcessor to do real work with them.
//
// The server shuts down gracefully on SIGINT and SIGTERM: it stops accepting
// connections and waits for in-flight requests to finish.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	webhookreceiver "github.com/brunofbenicio/go-lab/apis/webhook-receiver"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/docs"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/httpapi"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/idempotency"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/replay"
)

const (
	defaultPort = "8080"
	// shutdownGrace is how long in-flight requests get to finish on shutdown.
	shutdownGrace = 10 * time.Second

	minSecretLength = 16
	maxBodyLimit    = 10 << 20 // upper bound for MAX_BODY_BYTES
	maxTolerance    = time.Hour
	maxDedupeTTL    = 30 * 24 * time.Hour
	defaultDedupe   = 24 * time.Hour
)

// config holds the server settings read from the environment.
type config struct {
	Port         string
	Secret       []byte
	MaxBodyBytes int64
	Tolerance    time.Duration
	DedupeTTL    time.Duration
}

// LogValue keeps the secret out of logs even if a config is logged by mistake.
func (c config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("port", c.Port),
		slog.Int64("max_body_bytes", c.MaxBodyBytes),
		slog.Duration("timestamp_tolerance", c.Tolerance),
		slog.Duration("idempotency_ttl", c.DedupeTTL),
		slog.Bool("secret_set", len(c.Secret) > 0),
	)
}

// configFromEnv reads and validates the configuration using getenv, which
// makes it testable without touching the real environment.
func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{
		Port:         defaultPort,
		MaxBodyBytes: httpapi.DefaultMaxBodyBytes,
		Tolerance:    replay.DefaultTolerance,
		DedupeTTL:    defaultDedupe,
	}

	secret := getenv("WEBHOOK_SECRET")
	if secret == "" {
		return config{}, errors.New("WEBHOOK_SECRET is required: set it to a random string of at least 16 bytes")
	}
	if len(secret) < minSecretLength {
		return config{}, fmt.Errorf("WEBHOOK_SECRET is too short: need at least %d bytes, got %d", minSecretLength, len(secret))
	}
	cfg.Secret = []byte(secret)

	if port := getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 0 || n > 65535 {
			return config{}, fmt.Errorf("PORT must be a number between 0 and 65535, got %q", port)
		}
		cfg.Port = port
	}
	if v := getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > maxBodyLimit {
			return config{}, fmt.Errorf("MAX_BODY_BYTES must be a number between 1 and %d, got %q", maxBodyLimit, v)
		}
		cfg.MaxBodyBytes = n
	}
	if v := getenv("TIMESTAMP_TOLERANCE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 || d > maxTolerance {
			return config{}, fmt.Errorf("TIMESTAMP_TOLERANCE must be a duration between 1ns and %s, such as 5m, got %q", maxTolerance, v)
		}
		cfg.Tolerance = d
	}
	if v := getenv("IDEMPOTENCY_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 || d > maxDedupeTTL {
			return config{}, fmt.Errorf("IDEMPOTENCY_TTL must be a duration between 1ns and %s, such as 24h, got %q", maxDedupeTTL, v)
		}
		cfg.DedupeTTL = d
	}

	// A delivery accepted now can still be replayed until its timestamp leaves
	// the tolerance window, which may take up to twice the tolerance (when the
	// timestamp is dated in the future). The duplicate memory must outlast that,
	// or the replay would slip through once the delivery ID is forgotten.
	if cfg.DedupeTTL < 2*cfg.Tolerance {
		return config{}, fmt.Errorf("IDEMPOTENCY_TTL (%s) must be at least twice TIMESTAMP_TOLERANCE (%s)", cfg.DedupeTTL, cfg.Tolerance)
	}
	return cfg, nil
}

// deps lets tests replace the clock and the event processor. The zero value
// selects the production behavior.
type deps struct {
	now       func() time.Time
	processor httpapi.Processor
}

// logProcessor is the default Processor: it records that an event arrived. The
// payload itself is not logged because it may contain personal data.
func logProcessor(logger *slog.Logger) httpapi.Processor {
	return httpapi.ProcessorFunc(func(ctx context.Context, e httpapi.Event) error {
		logger.InfoContext(ctx, "webhook received", "delivery", e.ID, "type", e.Type, "data_bytes", len(e.Data))
		return nil
	})
}

// newHandler wires the signature check, the replay and duplicate protections,
// the API and the documentation into a single http.Handler.
func newHandler(cfg config, logger *slog.Logger, d deps) (http.Handler, error) {
	now := d.now
	if now == nil {
		now = time.Now
	}
	processor := d.processor
	if processor == nil {
		processor = logProcessor(logger)
	}

	dedupe, err := idempotency.New(cfg.DedupeTTL, idempotency.WithClock(now))
	if err != nil {
		return nil, fmt.Errorf("create duplicate detector: %w", err)
	}
	timestamps, err := replay.NewChecker(cfg.Tolerance, replay.WithClock(now))
	if err != nil {
		return nil, fmt.Errorf("create timestamp checker: %w", err)
	}
	api, err := httpapi.New(httpapi.Config{
		Secret:     cfg.Secret,
		Dedupe:     dedupe,
		Timestamps: timestamps,
		Processor:  processor,
	}, httpapi.WithMaxBodyBytes(cfg.MaxBodyBytes), httpapi.WithClock(now), httpapi.WithLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("create api: %w", err)
	}
	documentation, err := docs.New("Webhook Receiver API", webhookreceiver.OpenAPI())
	if err != nil {
		return nil, fmt.Errorf("create documentation: %w", err)
	}

	mux := http.NewServeMux()
	api.Register(mux)
	documentation.Register(mux)
	return mux, nil
}

// serve runs srv on ln until ctx is cancelled, then shuts it down gracefully,
// giving in-flight requests up to grace to finish. It returns nil after a
// clean shutdown.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, grace time.Duration) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// Serve only returns on its own when something went wrong.
		return err
	case <-ctx.Done():
	}

	// The shutdown deadline must not inherit the cancellation of ctx.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close() // give up on stragglers
		<-serveErr
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// run starts the server and blocks until ctx is cancelled. The zero deps value
// selects the production behavior.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, d deps) error {
	cfg, err := configFromEnv(getenv)
	if err != nil {
		return err
	}
	handler, err := newHandler(cfg, logger, d)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("server listening", "addr", ln.Addr().String(), "docs", "/docs", "config", cfg)
	if err := serve(ctx, srv, ln, shutdownGrace); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	err := run(ctx, os.Getenv, logger, deps{})
	stop()
	if err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
