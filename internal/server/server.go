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

	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/repository"
)

func Run(cfg *config.AppConfig, modRepo *repository.ModRepository) error { 
	router := NewRouter(modRepo) 

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

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

	slog.Info("HTTP server starting", "port", cfg.ServerPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %w", ":"+cfg.ServerPort, err)
	}

	slog.Info("Server stopped.")
	return nil
}
