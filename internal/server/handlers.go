// internal/server/handlers.go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	// Adjust import paths
	"github.com/ShawnEdgell/modio-api-go/internal/cache"
	"github.com/ShawnEdgell/modio-api-go/internal/modio" // For modio.Mod type
)

// APIResponse is a generic structure for your API responses
type APIResponse struct {
	ItemType    string      `json:"itemType"`
	LastUpdated time.Time   `json:"lastUpdated"`
	Count       int         `json:"count"`
	Items       []modio.Mod `json:"items"`
}

// MapsHandler creates an http.HandlerFunc for serving cached map data.
func MapsHandler(cacheStore *cache.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			slog.Warn("Method not allowed for /maps", "method", r.Method)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		maps, lastUpdated := cacheStore.GetMaps()
		response := APIResponse{
			ItemType:    "maps",
			LastUpdated: lastUpdated,
			Count:       len(maps),
			Items:       maps,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode maps response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// ScriptsHandler creates an http.HandlerFunc for serving cached script mod data.
func ScriptsHandler(cacheStore *cache.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			slog.Warn("Method not allowed for /scripts", "method", r.Method)
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		scripts, lastUpdated := cacheStore.GetScripts()
		response := APIResponse{
			ItemType:    "scripts",
			LastUpdated: lastUpdated,
			Count:       len(scripts),
			Items:       scripts,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode scripts response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// HealthCheckHandler is a simple handler for health checks.
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}