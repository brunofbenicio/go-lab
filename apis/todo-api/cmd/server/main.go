// Command server runs the todo API.
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/httpapi"
)

func main() {
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("todo-api listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
