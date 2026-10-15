// Command server runs the todo API.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/httpapi"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("todo-api listening on %s (log level %s)", srv.Addr, cfg.LogLevel)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
