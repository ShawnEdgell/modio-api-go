package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/ShawnEdgell/modio-api-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogchi "github.com/samber/slog-chi"
)

func NewRouter(modRepo *repository.ModRepository) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	// Replace chi's default logger with slog-chi
	// It will use the slog.Default() logger configured in your main.go
	r.Use(slogchi.New(slog.Default()))
	r.Use(middleware.Recoverer) // Recoverer should generally be after the logger
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/api/v1/skaterxl/maps", MapsHandler(modRepo))
	r.Get("/api/v1/skaterxl/scripts", ScriptsHandler(modRepo))

	r.Get("/api/v1/skaterxl/maps/autocomplete", AutocompleteHandler(modRepo, modio.MapTag))
	r.Get("/api/v1/skaterxl/scripts/autocomplete", AutocompleteHandler(modRepo, modio.ScriptModTag))

	r.Get("/health", HealthCheckHandler(modRepo))

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "https://www.skatebit.app", http.StatusMovedPermanently)
	})

	return r
}
