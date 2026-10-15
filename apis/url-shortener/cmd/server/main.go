// Command server runs the URL shortener API.
//
// Configuration comes from environment variables:
//
//	PORT      TCP port to listen on (default 8080)
//	BASE_URL  public origin used to build short URLs, for example
//	          https://sho.example.com (default: derived from each request)
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
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	urlshortener "github.com/brunofbenicio/go-lab/apis/url-shortener"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/docs"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/httpapi"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/shortener"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/store"
)

const (
	defaultPort = "8080"
	// shutdownGrace is how long in-flight requests get to finish on shutdown.
	shutdownGrace = 10 * time.Second
)

// config holds the server settings read from the environment.
type config struct {
	Port    string
	BaseURL string
}

// configFromEnv reads and validates the configuration using getenv, which
// makes it testable without touching the real environment.
func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{Port: defaultPort, BaseURL: getenv("BASE_URL")}

	if port := getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 0 || n > 65535 {
			return config{}, fmt.Errorf("PORT must be a number between 0 and 65535, got %q", port)
		}
		cfg.Port = port
	}

	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return config{}, fmt.Errorf("BASE_URL must be an absolute http(s) URL, got %q", cfg.BaseURL)
		}
	}
	return cfg, nil
}

// newHandler wires the store, the service, the API and the documentation into
// a single http.Handler. Extra shortener options (clock, random source, ...)
// let tests make the service deterministic.
func newHandler(cfg config, logger *slog.Logger, opts ...shortener.Option) (http.Handler, error) {
	svc, err := shortener.New(store.NewMemory(), opts...)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}
	documentation, err := docs.New("URL Shortener API", urlshortener.OpenAPI())
	if err != nil {
		return nil, fmt.Errorf("create documentation: %w", err)
	}

	mux := http.NewServeMux()
	httpapi.New(svc, httpapi.WithBaseURL(cfg.BaseURL), httpapi.WithLogger(logger)).Register(mux)
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

// run starts the server and blocks until ctx is cancelled.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	cfg, err := configFromEnv(getenv)
	if err != nil {
		return err
	}
	handler, err := newHandler(cfg, logger)
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

	logger.Info("server listening", "addr", ln.Addr().String(), "docs", "/docs")
	if err := serve(ctx, srv, ln, shutdownGrace); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	err := run(ctx, os.Getenv, logger)
	stop()
	if err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
