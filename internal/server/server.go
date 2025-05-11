// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Adjust import paths
	"github.com/ShawnEdgell/modio-api-go/internal/cache"
	"github.com/ShawnEdgell/modio-api-go/internal/config"
)

// Run starts the HTTP server.
func Run(cfg *config.AppConfig, cacheStore *cache.Store) error {
	router := NewRouter(cacheStore) // Get the router with handlers

	// Apply logging middleware (optional, from your http-go example)
	// You would define loggingMiddleware similar to how it was in your http-go project,
	// perhaps in this package or a shared middleware package.
	// For now, let's keep it simple and not add it here to reduce complexity,
	// but you can easily integrate it.
	// loggedRouter := loggingMiddleware(router) 

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router, // Use router here, or loggedRouter if you add middleware
		// Good practice: set timeouts to avoid Slowloris attacks.
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Goroutine for graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("Could not gracefully shutdown the server", "error", err)
		}
	}()

	slog.Info("HTTP server starting", "port", cfg.ServerPort, "timestamp", time.Now().Format(time.RFC3339Nano))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %w", ":"+cfg.ServerPort, err)
	}

	slog.Info("Server stopped.")
	return nil
}