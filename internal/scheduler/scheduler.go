package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/config"
	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/ShawnEdgell/modio-api-go/internal/repository" // Ensure this path is correct
)

const (
	modEventsPageLimit       = 100
	mapPageCountSafeguard    = 25
	scriptPageCountSafeguard = 15
)

type Scheduler struct {
	modioClient *modio.Client
	modRepo     *repository.ModRepository
	cfg         *config.AppConfig
	stopChan    chan struct{}
	updateMu    sync.Mutex
}

func NewScheduler(client *modio.Client, repo *repository.ModRepository, cfg *config.AppConfig) *Scheduler {
	return &Scheduler{
		modioClient: client,
		modRepo:     repo,
		cfg:         cfg,
		stopChan:    make(chan struct{}),
	}
}

func (s *Scheduler) processRecentChangesViaEvents(ctx context.Context, triggeredBy string) {
	if !s.updateMu.TryLock() {
		slog.Info("Scheduler: Event processing or full sync already in progress, skipping.", "triggered_by", triggeredBy)
		return
	}
	slog.Info("Scheduler: Starting event processing cycle.", "triggered_by", triggeredBy)
	defer s.updateMu.Unlock()

	lastSyncEventTs, err := s.modRepo.GetSchedulerLastSyncEventTimestamp(ctx)
	if err != nil {
		slog.Error("Scheduler (Events): Failed to get last sync event timestamp from repository. Aborting event processing.", "error", err)
		return
	}
	if lastSyncEventTs == 0 {
		slog.Info("Scheduler (Events): No last sync event timestamp found. Initial full sync recommended or seed timestamp.")
	}

	slog.Info("Scheduler (Events): Fetching new mod events from Mod.io", "since_timestamp", lastSyncEventTs)

	var allEventsToProcess []modio.ModioEvent
	currentOffset := 0
	maxEventsToProcessInOneCycle := 1000
	totalEventsFetchedThisCycle := 0

	for {
		eventsResponse, err := s.modioClient.FetchModEvents(ctx, lastSyncEventTs, currentOffset, modEventsPageLimit)
		if err != nil {
			slog.Error("Scheduler (Events): Failed to fetch mod events page from Mod.io", "offset", currentOffset, "error", err)
			break
		}

		if eventsResponse == nil || len(eventsResponse.Data) == 0 {
			slog.Info("Scheduler (Events): No more new events found.", "total_fetched_this_page", 0)
			break
		}
		slog.Info("Scheduler (Events): Fetched events page.", "count", len(eventsResponse.Data), "offset", currentOffset, "total_available", eventsResponse.ResultTotal)
		allEventsToProcess = append(allEventsToProcess, eventsResponse.Data...)
		totalEventsFetchedThisCycle += len(eventsResponse.Data)

		if len(eventsResponse.Data) < modEventsPageLimit || totalEventsFetchedThisCycle >= eventsResponse.ResultTotal || totalEventsFetchedThisCycle >= maxEventsToProcessInOneCycle {
			break
		}
		currentOffset += len(eventsResponse.Data)
		
		select {
		case <-ctx.Done():
			slog.Info("Scheduler (Events): Context cancelled during event pagination.")
			return
		case <-s.stopChan:
			slog.Info("Scheduler (Events): Stop signal received during event pagination.")
			return
		default:
		}
	}

	if len(allEventsToProcess) == 0 {
		slog.Info("Scheduler (Events): No new events to process.")
		if err := s.modRepo.SetLastOverallWriteTimestamp(ctx, time.Now().UTC()); err != nil {
			slog.Error("Scheduler (Events): Failed to update last overall write timestamp after no events", "error", err)
		}
		return
	}

	slog.Info("Scheduler (Events): Processing events.", "count", len(allEventsToProcess))
	pipe := s.modRepo.Client().Pipeline() // Corrected: Use Client() method to get *redis.Client, then Pipeline()
	var latestEventTsProcessedInBatch int64 = lastSyncEventTs

	for _, event := range allEventsToProcess {
		select {
		case <-ctx.Done():
			slog.Info("Scheduler (Events): Context cancelled during event processing loop.")
			return
		case <-s.stopChan:
			slog.Info("Scheduler (Events): Stop signal received during event processing loop.")
			return
		default:
		}

		slog.Debug("Scheduler (Events): Processing event", "event_id", event.ID, "mod_id", event.ModID, "type", event.EventType, "date_added", event.DateAdded)
		modTypeTag := ""

		oldModData, err := s.modRepo.GetModByID(ctx, event.ModID)
		if err != nil {
			slog.Error("Scheduler (Events): Failed to get old mod data from repository for event processing", "mod_id", event.ModID, "event_type", event.EventType, "error", err)
		}
		if oldModData != nil {
			isMap := false
			isScript := false
			for _, tag := range oldModData.Tags {
				if tag.Name == modio.MapTag {
					isMap = true
					break
				}
				if tag.Name == modio.ScriptModTag {
					isScript = true
					break
				}
			}
			if isMap {
				modTypeTag = modio.MapTag
			} else if isScript {
				modTypeTag = modio.ScriptModTag
			}
		}

		switch event.EventType {
		case "MOD_DELETED", "MOD_UNAVAILABLE":
			if oldModData != nil {
				s.modRepo.AddRemoveModCommandsFromPipeline(ctx, pipe, oldModData, modTypeTag)
				slog.Info("Scheduler (Events): Mod marked for deletion from repository", "mod_id", event.ModID, "event_type", event.EventType)
			} else {
				slog.Warn("Scheduler (Events): Mod to be deleted/unavailable not found in repository, or type unknown. Full sync will reconcile.", "mod_id", event.ModID)
				pipe.Del(ctx, repository.ModKeyPrefix+strconv.Itoa(event.ModID)) // Corrected: Use exported ModKeyPrefix
			}
		case "MOD_AVAILABLE", "MOD_EDITED", "MODFILE_CHANGED":
			newModData, err := s.modioClient.GetModDetails(ctx, event.ModID)
			if err != nil {
				slog.Error("Scheduler (Events): Failed to fetch updated mod details from Mod.io", "mod_id", event.ModID, "event_type", event.EventType, "error", err)
				continue
			}
			if newModData == nil {
				slog.Warn("Scheduler (Events): Mod details not found on Mod.io after update event, possibly became unavailable immediately.", "mod_id", event.ModID, "event_type", event.EventType)
				if oldModData != nil {
					s.modRepo.AddRemoveModCommandsFromPipeline(ctx, pipe, oldModData, modTypeTag)
				} else {
					pipe.Del(ctx, repository.ModKeyPrefix+strconv.Itoa(event.ModID)) // Corrected: Use exported ModKeyPrefix
				}
				continue
			}
			
			isMapNew := false
			isScriptNew := false
			for _, tag := range newModData.Tags {
				if tag.Name == modio.MapTag {
					isMapNew = true; break
				}
				if tag.Name == modio.ScriptModTag {
					isScriptNew = true; break
				}
			}
			if isMapNew { modTypeTag = modio.MapTag } else if isScriptNew { modTypeTag = modio.ScriptModTag } else {
				// If type cannot be determined from new tags, try to use old type if available
				if modTypeTag == "" && oldModData != nil {
					// modTypeTag would have been set from oldModData's tags
				} else if modTypeTag == "" {
					slog.Warn("Scheduler (Events): Could not determine mod type for new/updated mod, tag indexing may be incomplete", "mod_id", newModData.ID)
					// Default to a generic or no type for indexing if necessary, or skip type-specific indexes
				}
			}


			if oldModData != nil {
				s.modRepo.RemoveOrphanedTagIndexEntries(ctx, pipe, oldModData, newModData, modTypeTag)
			}
			err = s.modRepo.AddModCommandsToPipeline(ctx, pipe, newModData, modTypeTag)
			if err != nil {
				slog.Error("Scheduler (Events): Error adding save commands to pipeline for mod", "mod_id", newModData.ID, "error", err)
			} else {
				slog.Info("Scheduler (Events): Mod marked for save/update in repository", "mod_id", newModData.ID, "event_type", event.EventType)
			}
		default:
			slog.Debug("Scheduler (Events): Ignoring event type", "type", event.EventType, "mod_id", event.ModID)
		}

		if event.DateAdded > latestEventTsProcessedInBatch {
			latestEventTsProcessedInBatch = event.DateAdded
		}
	}

	if pipe.Len() > 0 { // Only execute if there are commands
		if _, err := pipe.Exec(ctx); err != nil {
			slog.Error("Scheduler (Events): Failed to execute Redis pipeline for event processing", "error", err)
			return
		}
	}


	if latestEventTsProcessedInBatch > lastSyncEventTs {
		if err := s.modRepo.SetSchedulerLastSyncEventTimestamp(ctx, latestEventTsProcessedInBatch); err != nil {
			slog.Error("Scheduler (Events): Failed to update last sync event timestamp in repository", "error", err)
		} else {
			slog.Info("Scheduler (Events): Successfully updated last sync event timestamp.", "timestamp", latestEventTsProcessedInBatch)
		}
	}
	if err := s.modRepo.SetLastOverallWriteTimestamp(ctx, time.Now().UTC()); err != nil {
		slog.Error("Scheduler (Events): Failed to update last overall write timestamp", "error", err)
	}
	slog.Info("Scheduler (Events): Event processing cycle finished.")
}

func (s *Scheduler) runFullSynchronization(ctx context.Context, triggeredBy string) {
	if !s.updateMu.TryLock() {
		slog.Info("Scheduler: Full sync or event processing already in progress, skipping.", "triggered_by", triggeredBy)
		return
	}
	slog.Info("Scheduler (Full Sync): Starting full data synchronization.", "triggered_by", triggeredBy)
	defer s.updateMu.Unlock()

	processType := func(itemTypeTag string, pageSafeguard int) (int64, error) { // Return max timestamp for this type
		slog.Info("Scheduler (Full Sync): Fetching all items from Mod.io.", "type", itemTypeTag)
		modsFromAPI, err := s.modioClient.FetchAllItems(ctx, itemTypeTag, pageSafeguard)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch all %s from Mod.io: %w", itemTypeTag, err)
		}
		slog.Info("Scheduler (Full Sync): Successfully fetched items from Mod.io.", "type", itemTypeTag, "count", len(modsFromAPI))

		modType := repository.GetModTypeFromTag(itemTypeTag) // Corrected: Use exported GetModTypeFromTag
		idsInRepo, err := s.modRepo.GetAllModIDsByType(ctx, modType)
		if err != nil {
			return 0, fmt.Errorf("failed to get all %s IDs from repository: %w", modType, err)
		}
		slog.Debug("Scheduler (Full Sync): Current IDs in repository.", "type", modType, "count", len(idsInRepo))

		apiModIDs := make(map[string]bool)
		for i := range modsFromAPI {
			mod := &modsFromAPI[i]
			apiModIDs[strconv.Itoa(mod.ID)] = true
		}

		pipe := s.modRepo.Client().Pipeline() // Corrected: Use Client() method
		var maxModUpdateTimestampForThisType int64 = 0

		for _, idInRepoStr := range idsInRepo {
			if !apiModIDs[idInRepoStr] {
				modID, _ := strconv.Atoi(idInRepoStr)
				slog.Debug("Scheduler (Full Sync): Mod found in repository but not in API fetch, marking for deletion.", "type", modType, "mod_id", modID)
				oldModData, err := s.modRepo.GetModByID(ctx, modID)
				if err != nil {
					slog.Error("Scheduler (Full Sync): Failed to get old mod data for deletion.", "mod_id", modID, "error", err)
					pipe.Del(ctx, repository.ModKeyPrefix+idInRepoStr) // Corrected: Use exported ModKeyPrefix
					continue
				}
				if oldModData != nil {
					s.modRepo.AddRemoveModCommandsFromPipeline(ctx, pipe, oldModData, itemTypeTag)
				} else {
					pipe.Del(ctx, repository.ModKeyPrefix+idInRepoStr) // Corrected: Use exported ModKeyPrefix
				}
			}
		}

		for i := range modsFromAPI {
			mod := &modsFromAPI[i] // Iterate by index to get addressable mod for pipeline
			if mod.DateUpdated > maxModUpdateTimestampForThisType {
				maxModUpdateTimestampForThisType = mod.DateUpdated
			}
			
			oldModData, _ := s.modRepo.GetModByID(ctx, mod.ID)
			if oldModData != nil {
				s.modRepo.RemoveOrphanedTagIndexEntries(ctx, pipe, oldModData, mod, itemTypeTag)
			}
			
			err := s.modRepo.AddModCommandsToPipeline(ctx, pipe, mod, itemTypeTag)
			if err != nil {
				slog.Error("Scheduler (Full Sync): Failed to add save commands for mod to pipeline.", "mod_id", mod.ID, "error", err)
			}
		}
		
		if pipe.Len() > 0 {
			slog.Info("Scheduler (Full Sync): Executing Redis pipeline for type.", "type", itemTypeTag, "commands_in_pipe", pipe.Len())
			if _, err := pipe.Exec(ctx); err != nil {
				return 0, fmt.Errorf("failed to execute Redis pipeline for %s: %w", itemTypeTag, err)
			}
		}
		slog.Info("Scheduler (Full Sync): Successfully synchronized type.", "type", itemTypeTag)
		return maxModUpdateTimestampForThisType, nil
	}

	var overallMaxModUpdateTimestamp int64 = 0
	
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute) // Increased timeout for full sync
	defer cancel()

	maxTsMaps, errMaps := processType(modio.MapTag, mapPageCountSafeguard)
	if errMaps != nil {
		slog.Error("Scheduler (Full Sync): Error processing maps.", "error", errMaps)
	} else {
		if maxTsMaps > overallMaxModUpdateTimestamp {
			overallMaxModUpdateTimestamp = maxTsMaps
		}
	}

	maxTsScripts, errScripts := processType(modio.ScriptModTag, scriptPageCountSafeguard)
	if errScripts != nil {
		slog.Error("Scheduler (Full Sync): Error processing scripts.", "error", errScripts)
	} else {
		if maxTsScripts > overallMaxModUpdateTimestamp {
			overallMaxModUpdateTimestamp = maxTsScripts
		}
	}

	if errMaps == nil && errScripts == nil {
		slog.Info("Scheduler (Full Sync): Both maps and scripts processed. Updating timestamps.")
		if overallMaxModUpdateTimestamp > 0 {
			if err := s.modRepo.SetSchedulerLastSyncEventTimestamp(ctxWithTimeout, overallMaxModUpdateTimestamp); err != nil {
				slog.Error("Scheduler (Full Sync): Failed to update last sync event timestamp after full sync.", "error", err)
			} else {
				slog.Info("Scheduler (Full Sync): Updated last sync event timestamp after full sync.", "timestamp", overallMaxModUpdateTimestamp)
			}
		}
	} else {
		slog.Warn("Scheduler (Full Sync): One or more types failed to process during full sync. Timestamps might not be fully updated.")
	}
	
	if err := s.modRepo.SetLastOverallWriteTimestamp(ctxWithTimeout, time.Now().UTC()); err != nil {
		slog.Error("Scheduler (Full Sync): Failed to update last overall write timestamp.", "error", err)
	}

	slog.Info("Scheduler (Full Sync): Full data synchronization cycle finished.")
}

func (s *Scheduler) Start() {
	slog.Info("Starting Mod.io data scheduler...",
		"event_processing_interval", s.cfg.LightweightCheckInterval.String(),
		"full_sync_interval", s.cfg.CacheRefreshInterval.String(),
	)
	
	baseCtx, cancelAll := context.WithCancel(context.Background()) 
	// Store cancelAll if you want to trigger a shutdown of these goroutines from Stop more directly
	// For now, stopChan handles ticker goroutine, and updateMu prevents new long tasks.

	go func() {
		slog.Info("Scheduler: Performing initial full data synchronization.")
		// Use a specific context for this initial task that can be shorter if needed
		initialSyncCtx, initialSyncCancel := context.WithTimeout(baseCtx, 15*time.Minute) // Timeout for initial sync
		defer initialSyncCancel()
		s.runFullSynchronization(initialSyncCtx, "initial_startup")
	}()

	eventProcessingTicker := time.NewTicker(s.cfg.LightweightCheckInterval)
	fullSyncTicker := time.NewTicker(s.cfg.CacheRefreshInterval)

	go func() {
		defer slog.Info("Scheduler: Ticker goroutine stopped.")
		defer eventProcessingTicker.Stop()
		defer fullSyncTicker.Stop()

		for {
			select {
			case <-eventProcessingTicker.C:
				slog.Info("Scheduler: Event processing tick received.")
				// Use a specific context for each event processing cycle
				eventCtx, eventCancel := context.WithTimeout(baseCtx, 5*time.Minute) // Timeout for one event cycle
				s.processRecentChangesViaEvents(eventCtx, "scheduled_event_processing")
				eventCancel()
			case <-fullSyncTicker.C:
				slog.Info("Scheduler: Full synchronization tick received.")
				// Use a specific context for each full sync cycle
				fullSyncCtx, fullSyncCancel := context.WithTimeout(baseCtx, 30*time.Minute) // Timeout for one full sync cycle
				s.runFullSynchronization(fullSyncCtx, "scheduled_full_sync")
				fullSyncCancel()
			case <-s.stopChan:
				slog.Info("Scheduler: Stop signal received, cancelling base context and exiting ticker goroutine.")
				cancelAll() // Cancel baseCtx to signal running tasks
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	slog.Info("Scheduler: Attempting to stop...")
	if s.stopChan == nil {
		slog.Warn("Scheduler: stopChan is nil.")
		return
	}
	select {
	case <-s.stopChan:
		slog.Warn("Scheduler: Stop channel already closed.")
	default:
		close(s.stopChan) // This signals the main ticker goroutine to stop
		slog.Info("Scheduler: Stop signal sent to ticker goroutine.")
	}
	// Note: updateMu will prevent new long operations from starting.
	// Ongoing operations (event processing or full sync) will complete or timeout based on their own contexts.
}
