// Command server runs the JWT authentication API.
//
// Configuration comes from environment variables:
//
//	JWT_SECRET  HMAC secret used to sign tokens, at least 32 bytes (required)
//	JWT_ISSUER  value of the iss claim (default "jwt-auth")
//	TOKEN_TTL   lifetime of access tokens, as a Go duration (default 15m)
//	PORT        TCP port to listen on (default 8080)
//
// The secret is never logged. Generate one with, for example,
// `openssl rand -base64 48`.
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

	jwtauth "github.com/brunofbenicio/go-lab/apis/jwt-auth"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/docs"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/httpapi"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/jwt"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/password"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/users"
)

const (
	defaultPort   = "8080"
	defaultIssuer = "jwt-auth"
	maxTokenTTL   = 24 * time.Hour
	// shutdownGrace is how long in-flight requests get to finish on shutdown.
	shutdownGrace = 10 * time.Second
)

// config holds the server settings read from the environment.
type config struct {
	Port     string
	Secret   []byte
	Issuer   string
	TokenTTL time.Duration
}

// LogValue keeps the secret out of logs even if a config is logged by mistake.
func (c config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("port", c.Port),
		slog.String("issuer", c.Issuer),
		slog.Duration("token_ttl", c.TokenTTL),
		slog.Bool("secret_set", len(c.Secret) > 0),
	)
}

// configFromEnv reads and validates the configuration using getenv, which
// makes it testable without touching the real environment.
func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{
		Port:     defaultPort,
		Issuer:   defaultIssuer,
		TokenTTL: httpapi.DefaultTokenTTL,
	}

	secret := getenv("JWT_SECRET")
	if secret == "" {
		return config{}, errors.New("JWT_SECRET is required: set it to a random string of at least 32 bytes")
	}
	if len(secret) < jwt.MinSecretLength {
		return config{}, fmt.Errorf("JWT_SECRET is too short: need at least %d bytes, got %d", jwt.MinSecretLength, len(secret))
	}
	cfg.Secret = []byte(secret)

	if port := getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 0 || n > 65535 {
			return config{}, fmt.Errorf("PORT must be a number between 0 and 65535, got %q", port)
		}
		cfg.Port = port
	}
	if issuer := getenv("JWT_ISSUER"); issuer != "" {
		cfg.Issuer = issuer
	}
	if ttl := getenv("TOKEN_TTL"); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil || d <= 0 || d > maxTokenTTL {
			return config{}, fmt.Errorf("TOKEN_TTL must be a duration between 1ns and %s, such as 15m or 1h, got %q", maxTokenTTL, ttl)
		}
		cfg.TokenTTL = d
	}
	return cfg, nil
}

// deps lets tests replace the sources of time, identifiers and password cost.
// The zero value selects the production behavior.
type deps struct {
	now        func() time.Time
	newID      func() (string, error)
	hasherOpts []password.Option
}

// newHandler wires the token signer, the password hasher, the user store, the
// API and the documentation into a single http.Handler.
func newHandler(cfg config, logger *slog.Logger, d deps) (http.Handler, error) {
	jwtOpts := []jwt.Option{jwt.WithIssuer(cfg.Issuer)}
	apiOpts := []httpapi.Option{httpapi.WithTokenTTL(cfg.TokenTTL), httpapi.WithLogger(logger)}
	if d.now != nil {
		jwtOpts = append(jwtOpts, jwt.WithClock(d.now))
		apiOpts = append(apiOpts, httpapi.WithClock(d.now))
	}
	if d.newID != nil {
		apiOpts = append(apiOpts, httpapi.WithIDGenerator(d.newID))
	}

	signer, err := jwt.NewHS256(cfg.Secret, jwtOpts...)
	if err != nil {
		return nil, fmt.Errorf("create token signer: %w", err)
	}
	hasher, err := password.NewHasher(d.hasherOpts...)
	if err != nil {
		return nil, fmt.Errorf("create password hasher: %w", err)
	}
	api, err := httpapi.New(httpapi.Dependencies{
		Users:  users.NewMemory(),
		Hasher: hasher,
		Tokens: signer,
	}, apiOpts...)
	if err != nil {
		return nil, fmt.Errorf("create api: %w", err)
	}
	documentation, err := docs.New("JWT Auth API", jwtauth.OpenAPI())
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
