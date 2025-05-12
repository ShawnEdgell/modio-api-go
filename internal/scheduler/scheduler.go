// internal/scheduler/scheduler.go
package scheduler

import (
	"log/slog"
	"sync" // Import sync package for Mutex
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/cache"
	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/modio"
)

const (
	mapPageCountSafeguard    = 25
	scriptPageCountSafeguard = 15
	mapTag                   = "Map"
	scriptModTag             = "Script"
	lightweightCheckPageSize = 5 // How many items to fetch for the first page check
)

type Scheduler struct {
	modioClient  *modio.Client
	cacheStore   *cache.Store
	cfg          *config.AppConfig
	stopChan     chan struct{}
	updateMu     sync.Mutex // Mutex to prevent concurrent full updates
	isUpdating   bool       // Flag to indicate if a full update is in progress
}

func NewScheduler(client *modio.Client, store *cache.Store, cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		modioClient: client,
		cacheStore:  store,
		cfg:         cfg,
		stopChan:    make(chan struct{}),
	}
}

// runUpdate performs a full data refresh
func (s *Scheduler) runUpdate(triggeredBy string) {
	if !s.updateMu.TryLock() {
		slog.Info("Scheduler: Full update already in progress, skipping new trigger.", "triggered_by", triggeredBy)
		return
	}
	s.isUpdating = true
	defer func() {
		s.isUpdating = false
		s.updateMu.Unlock()
	}()

	slog.Info("Scheduler: Starting data refresh cycle.", "triggered_by", triggeredBy)

	var fetchedMaps, fetchedScripts []modio.Mod
	var errMaps, errScripts error

	slog.Info("Scheduler: Fetching maps...", "safeguard_max_pages", mapPageCountSafeguard)
	fetchedMaps, errMaps = s.modioClient.FetchAllItems(mapTag, mapPageCountSafeguard)
	if errMaps != nil {
		slog.Error("Scheduler: Failed to fetch maps from Mod.io", "error", errMaps)
	} else {
		slog.Info("Scheduler: Successfully fetched maps", "count", len(fetchedMaps))
	}

	slog.Info("Scheduler: Fetching script mods...", "safeguard_max_pages", scriptPageCountSafeguard)
	fetchedScripts, errScripts = s.modioClient.FetchAllItems(scriptModTag, scriptPageCountSafeguard)
	if errScripts != nil {
		slog.Error("Scheduler: Failed to fetch script mods from Mod.io", "error", errScripts)
	} else {
		slog.Info("Scheduler: Successfully fetched script mods", "count", len(fetchedScripts))
	}

	currentMaps, _ := s.cacheStore.GetMaps()
	currentScripts, _ := s.cacheStore.GetScripts()
	mapsToStore := currentMaps
	scriptsToStore := currentScripts

	if errMaps == nil {
		slog.Info("Scheduler: Staging fetched maps for cache update.")
		mapsToStore = fetchedMaps
	} else {
		slog.Warn("Scheduler: Maps fetch failed, keeping existing maps in cache.")
	}

	if errScripts == nil {
		slog.Info("Scheduler: Staging fetched scripts for cache update.")
		scriptsToStore = fetchedScripts
	} else {
		slog.Warn("Scheduler: Scripts fetch failed, keeping existing scripts in cache.")
	}

	s.cacheStore.Update(mapsToStore, scriptsToStore)
	slog.Info("Scheduler: Data refresh cycle complete.", "triggered_by", triggeredBy, "last_updated", s.cacheStore.LastUpdated.Format(time.RFC3339))
}

// runLightweightCheck fetches the first page of items and triggers a full update if changes are detected.
func (s *Scheduler) runLightweightCheck() {
	if s.isUpdating { // Don't start a lightweight check if a full update is already running
		slog.Info("Scheduler: Full update in progress, skipping lightweight check.")
		return
	}
	slog.Info("Scheduler: Performing lightweight check for new Mod.io items...")

	needsMapUpdate := false
	needsScriptUpdate := false

	// Check Maps
	currentMaps, _ := s.cacheStore.GetMaps()
	newTopMapsPage, err := s.modioClient.FetchAllItems(mapTag, 1) // Fetch only first page (FetchAllItems will stop after 1 page if maxPagesToFetch is 1)
                                                                // Or more directly: s.modioClient.fetchPage(mapTag, lightweightCheckPageSize, 0)
	if err != nil {
		slog.Error("Scheduler (Lightweight): Failed to fetch first page of maps", "error", err)
	} else if len(newTopMapsPage) > 0 {
		if len(currentMaps) == 0 || // If cache is empty
			currentMaps[0].ID != newTopMapsPage[0].ID || // If top item ID changed
			currentMaps[0].DateUpdated < newTopMapsPage[0].DateUpdated { // If top item is newer
			slog.Info("Scheduler (Lightweight): Change detected in maps. Triggering full map update.")
			needsMapUpdate = true
		}
	}

	// Check Scripts
	currentScripts, _ := s.cacheStore.GetScripts()
	newTopScriptsPage, err := s.modioClient.FetchAllItems(scriptModTag, 1) // Fetch only first page
	if err != nil {
		slog.Error("Scheduler (Lightweight): Failed to fetch first page of scripts", "error", err)
	} else if len(newTopScriptsPage) > 0 {
		if len(currentScripts) == 0 || // If cache is empty
			currentScripts[0].ID != newTopScriptsPage[0].ID || // If top item ID changed
			currentScripts[0].DateUpdated < newTopScriptsPage[0].DateUpdated { // If top item is newer
			slog.Info("Scheduler (Lightweight): Change detected in scripts. Triggering full script update.")
			needsScriptUpdate = true
		}
	}

	if needsMapUpdate || needsScriptUpdate {
		// Call runUpdate in a new goroutine to avoid blocking the lightweight check ticker,
		// and runUpdate now has its own lock to prevent concurrent full updates.
		go s.runUpdate("triggered_by_lightweight_check")
	} else {
		slog.Info("Scheduler (Lightweight): No significant changes detected on first pages.")
	}
}

func (s *Scheduler) Start() {
	slog.Info("Starting Mod.io data scheduler...",
		"full_refresh_interval", s.cfg.CacheRefreshInterval.String(),
		"lightweight_check_interval", s.cfg.LightweightCheckInterval.String(),
	)

	// Perform an initial full update immediately
	go func() {
		slog.Info("Scheduler: Performing initial data load.")
		s.runUpdate("initial_startup")
	}()

	fullRefreshTicker := time.NewTicker(s.cfg.CacheRefreshInterval)
	lightweightCheckTicker := time.NewTicker(s.cfg.LightweightCheckInterval)

	go func() {
		defer fullRefreshTicker.Stop()
		defer lightweightCheckTicker.Stop()
		for {
			select {
			case <-fullRefreshTicker.C:
				slog.Info("Scheduler: Full refresh tick received.")
				s.runUpdate("scheduled_full_refresh")
			case <-lightweightCheckTicker.C:
				slog.Info("Scheduler: Lightweight check tick received.")
				s.runLightweightCheck()
			case <-s.stopChan:
				slog.Info("Scheduler: Stop signal received, stopping all tickers.")
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	slog.Info("Scheduler: Attempting to stop scheduler...")
	select {
	case <-s.stopChan:
		slog.Warn("Scheduler: Stop channel already closed or stop initiated.")
	default:
		close(s.stopChan)
		slog.Info("Scheduler: Stop signal sent.")
	}
}