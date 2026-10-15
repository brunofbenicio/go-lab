// Package app assembles the complete HTTP handler of the todo API: the router
// wrapped in the middleware stack, configured from config.Config.
package app

import (
	"log/slog"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/config"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/httpapi"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/middleware"
	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// NewHandler returns the router wrapped, outermost first, in:
//
//  1. RequestID  - every later layer and log line can see the id
//  2. Logging    - sees the final status, including the 500 produced by Recover
//  3. Recover    - turns panics into a clean 500
//  4. CORS       - answers preflights before they cost rate limit tokens
//  5. RateLimit  - throttles before credentials are checked, slowing key guessing
//  6. APIKey     - protects write operations
func NewHandler(cfg config.Config, store todo.Store, logger *slog.Logger) http.Handler {
	corsOpts := middleware.DefaultCORS(cfg.CORSOrigins...)

	return middleware.Chain(
		httpapi.NewRouter(store),
		middleware.RequestID(),
		middleware.Logging(logger),
		middleware.Recover(logger),
		middleware.CORS(corsOpts),
		middleware.RateLimit(middleware.RateLimitOptions{
			RatePerSecond: cfg.RateLimitRPS,
			Burst:         cfg.RateLimitBurst,
		}),
		middleware.RequireAPIKey(cfg.APIKeys...),
	)
}
