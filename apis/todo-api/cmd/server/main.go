// Command server runs the todo API.
package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/app"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
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
		Addr:              cfg.Addr(),
		Handler:           app.NewHandler(cfg, todo.NewMemoryStore(), logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("todo-api listening", "addr", srv.Addr, "log_level", cfg.LogLevel)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
