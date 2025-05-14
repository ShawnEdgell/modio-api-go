package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/ShawnEdgell/modio-api-go/internal/repository"
	"github.com/ShawnEdgell/modio-api-go/internal/scheduler"
	"github.com/ShawnEdgell/modio-api-go/internal/server"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func initRedis(cfg *config.AppConfig) (*redis.Client, error) {
	slog.Info("Initializing Redis client", "address", cfg.RedisAddr, "db", cfg.RedisDB)
	rdbInstance := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := rdbInstance.Ping(ctx).Result(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		return nil, err
	}
	slog.Info("Successfully connected to Redis")
	return rdbInstance, nil
}

func main() {
	loggerHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // Use LevelInfo for production, LevelDebug for development
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(loggerHandler))

	if os.Getenv("APP_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			slog.Info("No .env file found or error loading, relying on system environment variables or defaults.")
		}
	}

	appConfig := config.Load()

	modioClient, err := modio.NewClient(appConfig)
	if err != nil {
		slog.Error("Failed to create Mod.io client", "error", err)
		os.Exit(1)
	}

	rdb, err = initRedis(appConfig)
	if err != nil {
		slog.Error("Failed to initialize Redis", "error", err)
		os.Exit(1)
	}

	slog.Info("Initializing Mod Repository")
	modRepo := repository.NewModRepository(rdb)

	slog.Info("Initializing data scheduler")
	dataScheduler := scheduler.NewScheduler(modioClient, modRepo, appConfig)
	dataScheduler.Start()

	stopOsSignal := make(chan os.Signal, 1)
	signal.Notify(stopOsSignal, syscall.SIGINT, syscall.SIGTERM)

	serverErrChan := make(chan error, 1)
	go func() {
		slog.Info("Starting HTTP server", "port", appConfig.ServerPort)
		if err := server.Run(appConfig, modRepo); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			serverErrChan <- err
		} else if err == http.ErrServerClosed {
			slog.Info("HTTP server closed")
			close(serverErrChan)
		} else {
			slog.Info("HTTP server stopped gracefully")
			close(serverErrChan)
		}
	}()

	var serverErr error
	select {
	case sig := <-stopOsSignal:
		slog.Info("OS signal received, initiating shutdown", "signal", sig.String())
		// server.Run has its own signal handler that should initiate srv.Shutdown()
		// Main will proceed to shut down other components.
	case err, ok := <-serverErrChan:
		if ok { // Error received from server.Run
			slog.Error("Server exited prematurely", "error", err)
			serverErr = err
		} else { // serverErrChan was closed
			slog.Info("Server goroutine completed its shutdown")
		}
	}

	slog.Info("Starting graceful shutdown sequence")

	slog.Info("Stopping scheduler")
	if dataScheduler != nil {
		dataScheduler.Stop()
	}
	slog.Info("Scheduler stopped")

	// The server.Run function handles its own http.Server.Shutdown.
	// We wait for the serverErrChan to know its status or rely on OS signal propagation.

	slog.Info("Closing Redis connection")
	if rdb != nil {
		if err := rdb.Close(); err != nil {
			slog.Error("Failed to close Redis connection", "error", err)
		} else {
			slog.Info("Redis connection closed")
		}
	}

	if serverErr != nil {
		slog.Error("Application exited due to server error", "error", serverErr)
		os.Exit(1)
	}
	slog.Info("Application shut down gracefully")
}
