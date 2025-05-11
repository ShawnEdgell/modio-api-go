// internal/config/config.go
package config

import (
	"log" // Using standard log for fatal errors before slog might be set up
	"os"
	"strconv"
	"time"
)

// AppConfig holds all configuration for the application.
type AppConfig struct {
	ServerPort           string
	ModioAPIKey          string
	ModioGameID          string
	ModioAPIDomain       string
	CacheRefreshInterval time.Duration
}

// Load loads application configuration from environment variables.
func Load() *AppConfig {
	cfg := &AppConfig{
		ServerPort:         getEnv("PORT", "8000"),
		ModioAPIKey:        os.Getenv("MODIO_API_KEY"), // Read directly, no fallback here
		ModioGameID:        getEnv("MODIO_GAME_ID", "629"),
		ModioAPIDomain:     getEnv("MODIO_API_DOMAIN", "g-9677.modapi.io"),
		CacheRefreshInterval: getEnvAsDuration("CACHE_REFRESH_INTERVAL_HOURS", 6*time.Hour),
	}

	if cfg.ModioAPIKey == "" {
		log.Fatal("FATAL ERROR: MODIO_API_KEY environment variable is not set. Application cannot start.")
	}

	return cfg
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsDuration retrieves an environment variable (as hours) and parses it as time.Duration.
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	strValue := getEnv(key, "")
	if strValue != "" {
		if hours, err := strconv.Atoi(strValue); err == nil {
			if hours > 0 { // Basic validation
				return time.Duration(hours) * time.Hour
			}
			log.Printf("Warning: Invalid non-positive value for %s: %s. Using default.", key, strValue)
		} else {
			log.Printf("Warning: Invalid integer format for %s: %s. Using default.", key, strValue)
		}
	}
	return fallback
}