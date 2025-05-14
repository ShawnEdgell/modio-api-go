// internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type AppConfig struct {
	ServerPort               string
	ModioAPIKey              string
	ModioGameID              string
	ModioAPIDomain           string
	CacheRefreshInterval     time.Duration
	LightweightCheckInterval time.Duration

	// --- New Redis Config ---
	RedisAddr     string
	RedisPassword string // Leave empty if no password
	RedisDB       int    // Default is 0
}

func Load() *AppConfig {
	cfg := &AppConfig{
		ServerPort:               getEnv("PORT", "8000"),
		ModioAPIKey:              os.Getenv("MODIO_API_KEY"), // Critical: No default
		ModioGameID:              getEnv("MODIO_GAME_ID", "629"), // SkaterXL Game ID
		ModioAPIDomain:           getEnv("MODIO_API_DOMAIN", "api.mod.io"), // Official domain
		CacheRefreshInterval:     getEnvAsDurationHours("CACHE_REFRESH_INTERVAL_HOURS", 6*time.Hour),
		LightweightCheckInterval: getEnvAsDurationMinutes("LIGHTWEIGHT_CHECK_INTERVAL_MINUTES", 15*time.Minute), // Check more frequently

		// --- Load Redis Config ---
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""), // Default to no password
		RedisDB:       getEnvAsInt("REDIS_DB", 0),   // Default to DB 0
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

func getEnvAsDurationHours(key string, fallback time.Duration) time.Duration { // Renamed from getEnvAsDuration
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

func getEnvAsInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if strValue != "" {
		if intVal, err := strconv.Atoi(strValue); err == nil {
			return intVal
		}
		log.Printf("Warning: Invalid integer format for %s: %s. Using default.", key, strValue)
	}
	return fallback
}