// Package server runs an http.Server with graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Run serves srv on ln until ctx is cancelled (typically by SIGINT or SIGTERM)
// and then shuts down gracefully: the listener closes at once, in-flight
// requests get up to shutdownTimeout to finish, and whatever is still running
// after that is cut off.
//
// It returns nil after a clean shutdown, the serve error if the server fails on
// its own, or an error wrapping context.DeadlineExceeded if requests did not
// finish in time.
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	// ctx is already cancelled, so derive the shutdown deadline from a context
	// that keeps its values but not its cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close() // drop the connections that did not finish
		<-serveErr
		return fmt.Errorf("server: graceful shutdown: %w", err)
	}
	<-serveErr // Serve returns http.ErrServerClosed once Shutdown completes
	return nil
}
