package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/ShawnEdgell/modio-api-go/internal/repository"
	// For health check ping
)

type APIResponse struct {
	ItemType    string      `json:"itemType"`
	LastUpdated time.Time   `json:"lastUpdated"`
	Count       int         `json:"count"`
	Items       []modio.Mod `json:"items"`
}

type AutocompleteSuggestion struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("Failed to encode JSON response", "error", err)
			// Can't write http.Error here as headers/status might be sent
		}
	}
}

func MapsHandler(modRepo *repository.ModRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		maps, lastUpdated, err := modRepo.GetModsByType(r.Context(), modio.MapTag)
		if err != nil {
			slog.Error("Failed to get maps from repository", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		response := APIResponse{
			ItemType:    "maps",
			LastUpdated: lastUpdated,
			Count:       len(maps),
			Items:       maps,
		}
		writeJSONResponse(w, http.StatusOK, response)
	}
}

func ScriptsHandler(modRepo *repository.ModRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		scripts, lastUpdated, err := modRepo.GetModsByType(r.Context(), modio.ScriptModTag)
		if err != nil {
			slog.Error("Failed to get scripts from repository", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		response := APIResponse{
			ItemType:    "scripts",
			LastUpdated: lastUpdated,
			Count:       len(scripts),
			Items:       scripts,
		}
		writeJSONResponse(w, http.StatusOK, response)
	}
}

func AutocompleteHandler(modRepo *repository.ModRepository, itemTypeTag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
		if prefix == "" {
			http.Error(w, "Missing or empty 'prefix' query parameter", http.StatusBadRequest)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := 10 // Default limit
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 { // Max limit 50
				limit = l
			}
		}

		results, err := modRepo.SearchTitlesByPrefix(r.Context(), itemTypeTag, prefix, limit)
		if err != nil {
			slog.Error("Failed to get autocomplete suggestions", "prefix", prefix, "type", itemTypeTag, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		suggestions := make([]AutocompleteSuggestion, 0, len(results))
		for _, res := range results {
			parts := strings.SplitN(res, ":", 2) // Split "normalizedtitle:id"
			if len(parts) == 2 {
				id, err := strconv.Atoi(parts[1])
				if err == nil {
					// For the title, ideally, you'd fetch the original title from mod:<id>
					// as the indexed title is normalized (lowercase).
					// For simplicity here, we're using the normalized part.
					// A more complete solution would MGET the actual mod objects or store original title in ZSET member.
					suggestions = append(suggestions, AutocompleteSuggestion{ID: id, Title: parts[0]})
				}
			}
		}
		writeJSONResponse(w, http.StatusOK, suggestions)
	}
}

func HealthCheckHandler(modRepo *repository.ModRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check Redis connection as part of health
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		redisClient := modRepo.Client() // Get the underlying client
		if err := redisClient.Ping(ctx).Err(); err != nil {
			slog.Error("Health check failed: Redis ping error", "error", err)
			status := map[string]string{"status": "unhealthy", "reason": "redis_connection_error"}
			writeJSONResponse(w, http.StatusServiceUnavailable, status)
			return
		}

		status := map[string]string{"status": "ok", "redis": "connected"}
		writeJSONResponse(w, http.StatusOK, status)
	}
}
