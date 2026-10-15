// Command server runs the todo API.
package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/app"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/server"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	if len(cfg.APIKeys) == 0 {
		logger.Warn("TODO_API_KEYS is not set: write operations are open to everyone")
	}

	srv := &http.Server{
		Handler:           app.NewHandler(cfg, todo.NewMemoryStore(), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		logger.Error("cannot listen", "addr", cfg.Addr(), "error", err)
		os.Exit(1)
	}

	// SIGINT (Ctrl+C) and SIGTERM (what orchestrators send) start a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("todo-api listening", "addr", ln.Addr().String(), "log_level", cfg.LogLevel)
	if err := server.Run(ctx, srv, ln, cfg.ShutdownTimeout); err != nil {
		logger.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
	logger.Info("todo-api stopped cleanly")
}
