// internal/server/router.go
package server

import (
	"net/http"

	// Adjust import paths
	"github.com/ShawnEdgell/modio-api-go/internal/cache"
)

// NewRouter creates and configures a new HTTP router (ServeMux).
func NewRouter(cacheStore *cache.Store) *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/v1/skaterxl/maps", MapsHandler(cacheStore))
	mux.HandleFunc("/api/v1/skaterxl/scripts", ScriptsHandler(cacheStore)) // Or /mods if you prefer

	// Health check route
	mux.HandleFunc("/health", HealthCheckHandler())

	// Simple root redirect or message
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" { // Basic 404 for anything else not matched
            http.NotFound(w, r)
            return
        }
        http.Redirect(w, r, "https://www.skatebit.app", http.StatusMovedPermanently)
        // Or just a message:
        // fmt.Fprintf(w, "Mod.io Cache API for SkaterXL is running. See /api/v1/skaterxl/maps or /api/v1/skaterxl/scripts")
    })


	return mux
}