// main.go
package main

import (
	// For graceful shutdown
	"log/slog"
	"net/http"
	"os"
	"os/signal" // For graceful shutdown
	"syscall"   // For graceful shutdown
	"time"

	"github.com/joho/godotenv" // For loading .env file for local development

	// Adjust import paths according to your go.mod module name
	"github.com/ShawnEdgell/modio-api-go/internal/cache"
	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/ShawnEdgell/modio-api-go/internal/scheduler"
	"github.com/ShawnEdgell/modio-api-go/internal/server"
)

func main() {
	// --- Logger Setup ---
	// (You can keep this as is, or adjust Level for production vs. dev)
	var loggerHandler slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Consider slog.LevelInfo for production
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
			}
			return a
		},
	})
	logger := slog.New(loggerHandler)
	slog.SetDefault(logger)

	// --- Load .env for local development ---
	// In production on VPS, Docker Compose will set environment variables from the VPS's .env file.
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found or error loading, relying on system environment variables or defaults.")
	}

	// --- Load Configuration ---
	appConfig := config.Load() // This uses the version that Fatals if MODIO_API_KEY is missing

	// --- Initialize Mod.io Client ---
	modioClient, err := modio.NewClient(appConfig)
	if err != nil {
		slog.Error("Failed to create Mod.io client. Ensure MODIO_API_KEY is set.", "error", err)
		os.Exit(1) // Exit if client can't be created (API key is essential)
	}

	// --- Initialize Cache ---
	slog.Info("Initializing in-memory cache store...")
	cacheStore := cache.NewStore()

	// --- Initialize and Start Scheduler ---
	// The scheduler will perform an initial data load and then periodic updates.
	slog.Info("Initializing data scheduler...")
	dataScheduler := scheduler.NewScheduler(modioClient, cacheStore, appConfig)
	dataScheduler.Start() // Runs data fetching in background goroutines

	// --- Create a channel to listen for OS signals for graceful shutdown ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// --- Start HTTP Server in a Goroutine ---
	// This allows the shutdown logic below to run.
	var serverErr error
	go func() {
		slog.Info("Starting HTTP server for Mod.io API Cache...", "port", appConfig.ServerPort)
		if err := server.Run(appConfig, cacheStore); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			serverErr = err // Capture error to potentially exit main goroutine
			stop <- syscall.SIGINT // Trigger shutdown on server error
		}
	}()

	// --- Wait for a shutdown signal ---
	<-stop // Block here until a signal is received

	slog.Info("Shutdown signal received. Cleaning up...")

	// --- Gracefully stop the scheduler ---
	// (Implement dataScheduler.Stop() if you added that to your scheduler for the ticker)
	// For now, the scheduler's goroutine will exit when the main app exits.
	// If you added the stopChan to the scheduler: dataScheduler.Stop()

	// The server.Run function's graceful shutdown (which we need to add to server.go)
	// will be handled there. If not, you'd call srv.Shutdown(ctx) here.
	// For now, this main will exit, and Docker will restart the container based on restart policy.
	// A more robust graceful shutdown for the HTTP server itself would be handled within server.Run or here.

	if serverErr != nil {
		slog.Error("Exiting due to server error.", "error", serverErr)
		os.Exit(1)
	}
	slog.Info("Application shut down gracefully.")
}