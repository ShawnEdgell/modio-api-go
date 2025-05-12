// internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type AppConfig struct {
	ServerPort                 string
	ModioAPIKey                string
	ModioGameID                string
	ModioAPIDomain             string
	CacheRefreshInterval       time.Duration
	LightweightCheckInterval time.Duration // New field
}

func Load() *AppConfig {
	cfg := &AppConfig{
		ServerPort:               getEnv("PORT", "8000"),
		ModioAPIKey:              os.Getenv("MODIO_API_KEY"),
		ModioGameID:              getEnv("MODIO_GAME_ID", "629"),
		ModioAPIDomain:           getEnv("MODIO_API_DOMAIN", "g-9677.modapi.io"),
		CacheRefreshInterval:     getEnvAsDuration("CACHE_REFRESH_INTERVAL_HOURS", 6*time.Hour),
		LightweightCheckInterval: getEnvAsDurationMinutes("LIGHTWEIGHT_CHECK_INTERVAL_MINUTES", 30*time.Minute), // New: default to 30 mins
	}

	if cfg.ModioAPIKey == "" {
		log.Fatal("FATAL ERROR: MODIO_API_KEY environment variable is not set. Application cannot start.")
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Renamed for clarity for hours
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	strValue := getEnv(key, "")
	if strValue != "" {
		if hours, err := strconv.Atoi(strValue); err == nil {
			if hours > 0 {
				return time.Duration(hours) * time.Hour
			}
			log.Printf("Warning: Invalid non-positive value for %s (hours): %s. Using default.", key, strValue)
		} else {
			log.Printf("Warning: Invalid integer format for %s (hours): %s. Using default.", key, strValue)
		}
	}
	return fallback
}

// New helper function for minutes
func getEnvAsDurationMinutes(key string, fallback time.Duration) time.Duration {
	strValue := getEnv(key, "")
	if strValue != "" {
		if minutes, err := strconv.Atoi(strValue); err == nil {
			if minutes > 0 {
				return time.Duration(minutes) * time.Minute
			}
			log.Printf("Warning: Invalid non-positive value for %s (minutes): %s. Using default.", key, strValue)
		} else {
			log.Printf("Warning: Invalid integer format for %s (minutes): %s. Using default.", key, strValue)
		}
	}
	return fallback
}