// internal/scheduler/scheduler.go
package scheduler

import (
	"log/slog"
	"time"

	// Adjust import paths based on your go.mod module path
	"github.com/ShawnEdgell/modio-api-go/internal/cache"
	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/modio"
)

const (
	// These could also be moved to config if you want them more dynamic,
	// but for now, matching your TypeScript logic.
	mapPageCount    = 12
	scriptPageCount = 5
	mapTag          = "Map"    // Consistent with your modio.Client logic if it expects these exact strings
	scriptModTag    = "Script" // Consistent with your modio.Client logic
)

// Scheduler manages periodic updates of the data cache.
type Scheduler struct {
	modioClient *modio.Client
	cacheStore  *cache.Store
	cfg         *config.AppConfig
	stopChan    chan struct{} // Channel to signal stopping the scheduler
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(client *modio.Client, store *cache.Store, cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		modioClient: client,
		cacheStore:  store,
		cfg:         cfg,
		stopChan:    make(chan struct{}),
	}
}

// runUpdate performs a single cycle of fetching data from Mod.io and updating the cache.
func (s *Scheduler) runUpdate() {
	slog.Info("Scheduler: Starting data refresh cycle from Mod.io...")

	var allMaps, allScripts []modio.Mod
	var errMaps, errScripts error

	// Fetch maps
	slog.Info("Scheduler: Fetching maps...")
	allMaps, errMaps = s.modioClient.FetchAllItems(mapTag, mapPageCount)
	if errMaps != nil {
		slog.Error("Scheduler: Failed to fetch maps from Mod.io", "error", errMaps)
		// Decide on strategy: continue with potentially stale map data, or don't update maps?
		// For now, if there's an error, we won't update this part of the cache.
		// You could also load previously persisted data here if you implement file caching.
		allMaps = nil // Ensure we don't update with partial/error data if that's the policy
	} else {
		slog.Info("Scheduler: Successfully fetched maps", "count", len(allMaps))
	}

	// Fetch script mods
	slog.Info("Scheduler: Fetching script mods...")
	allScripts, errScripts = s.modioClient.FetchAllItems(scriptModTag, scriptPageCount)
	if errScripts != nil {
		slog.Error("Scheduler: Failed to fetch script mods from Mod.io", "error", errScripts)
		allScripts = nil
	} else {
		slog.Info("Scheduler: Successfully fetched script mods", "count", len(allScripts))
	}

	// Only update the cache if at least one fetch was successful, or decide on a different strategy.
	// For this example, we update with whatever was fetched (nil if errored).
	// Your cache.Update method should ideally handle nil slices gracefully if needed,
	// or this function should ensure it only passes valid data.
	// Let's assume cache.Update can handle being passed nil for one type if the other succeeded.
	// Or better, only update with non-nil slices.

	currentMaps, _ := s.cacheStore.GetMaps()
	currentScripts, _ := s.cacheStore.GetScripts()

	if errMaps == nil { // If map fetch succeeded
		slog.Info("Scheduler: Updating maps in cache.")
		currentMaps = allMaps
	} else {
		slog.Warn("Scheduler: Maps fetch failed, keeping existing maps in cache (if any).")
	}

	if errScripts == nil { // If script fetch succeeded
		slog.Info("Scheduler: Updating scripts in cache.")
		currentScripts = allScripts
	} else {
		slog.Warn("Scheduler: Scripts fetch failed, keeping existing scripts in cache (if any).")
	}
	
	s.cacheStore.Update(currentMaps, currentScripts) // Update with new or existing data
	slog.Info("Scheduler: Data refresh cycle complete.", "last_updated", s.cacheStore.LastUpdated)
}

// Start begins the cache update scheduler.
// It performs an initial update and then updates periodically.
func (s *Scheduler) Start() {
	slog.Info("Starting Mod.io data scheduler...", "refresh_interval", s.cfg.CacheRefreshInterval.String())

	// Perform an initial update immediately in a goroutine so it doesn't block startup.
	go s.runUpdate()

	// Start the ticker for periodic updates.
	ticker := time.NewTicker(s.cfg.CacheRefreshInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				s.runUpdate()
			case <-s.stopChan: // Listen for a stop signal
				ticker.Stop()
				slog.Info("Scheduler: Stopped.")
				return
			}
		}
	}()
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	slog.Info("Scheduler: Sending stop signal...")
	close(s.stopChan) // Close the channel to signal the goroutine to stop
}