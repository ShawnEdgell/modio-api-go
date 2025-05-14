package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/ShawnEdgell/modio-api-go/internal/modio"
	"github.com/redis/go-redis/v9"
)

const (
	// Exported for use by other packages if necessary (like scheduler for direct DEL on fallback)
	ModKeyPrefix                           = "mod:" // Capitalized
	modTypeSetKeyPrefix                    = "mods:type:"
	modTitleSortedSetKeyPrefix             = "mod_titles:"
	modDateUpdatedSortedSetKeyPrefix       = "mods_by_dateupdated:"
	modTagSetKeyPrefix                     = "tag:"
	systemLastOverallWriteTimestampKey     = "modapi:system:last_overall_write_ts"
	schedulerLastSyncEventTimestampKey = "modapi:scheduler:last_sync_event_ts"
)

// GetModTypeFromTag is now exported
func GetModTypeFromTag(itemTypeTag string) string {
	if strings.EqualFold(itemTypeTag, modio.MapTag) {
		return "map"
	} else if strings.EqualFold(itemTypeTag, modio.ScriptModTag) {
		return "script"
	}
	slog.Warn("Unknown itemTypeTag for mod type conversion in repository", "tag", itemTypeTag)
	return strings.ToLower(strings.TrimSpace(itemTypeTag))
}

func normalizeStringForIndex(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type ModRepository struct {
	rdb *redis.Client
}

func NewModRepository(rdb *redis.Client) *ModRepository {
	if rdb == nil {
		slog.Error("Redis client is nil in NewModRepository. Application may not function correctly.")
	}
	return &ModRepository{rdb: rdb}
}

// Client returns the underlying Redis client.
// This allows other packages (like the scheduler) to create pipelines.
func (r *ModRepository) Client() *redis.Client {
	return r.rdb
}

func (r *ModRepository) AddModCommandsToPipeline(ctx context.Context, pipe redis.Pipeliner, mod *modio.Mod, itemTypeTag string) error {
	modType := GetModTypeFromTag(itemTypeTag) // Use exported version
	modIDStr := strconv.Itoa(mod.ID)

	modKey := ModKeyPrefix + modIDStr // Use exported version
	modJSON, err := json.Marshal(mod)
	if err != nil {
		slog.Error("Failed to marshal mod to JSON for pipeline", "mod_id", mod.ID, "error", err)
		return fmt.Errorf("failed to marshal mod %d: %w", mod.ID, err)
	}
	pipe.Set(ctx, modKey, modJSON, 0)

	pipe.SAdd(ctx, modTypeSetKeyPrefix+modType, modIDStr)

	normalizedTitle := normalizeStringForIndex(mod.Name)
	autocompleteMember := fmt.Sprintf("%s:%s", normalizedTitle, modIDStr)
	pipe.ZAdd(ctx, modTitleSortedSetKeyPrefix+modType, redis.Z{Score: 0, Member: autocompleteMember})

	pipe.ZAdd(ctx, modDateUpdatedSortedSetKeyPrefix+modType, redis.Z{Score: float64(mod.DateUpdated), Member: modIDStr})

	for _, tag := range mod.Tags {
		normalizedTagName := normalizeStringForIndex(tag.Name)
		tagSetKey := fmt.Sprintf("%s%s:%s", modTagSetKeyPrefix, normalizedTagName, modType)
		pipe.SAdd(ctx, tagSetKey, modIDStr)
	}
	slog.Debug("Added commands to pipeline to save/update mod", "mod_id", mod.ID, "mod_name", mod.Name)
	return nil
}

func (r *ModRepository) AddRemoveModCommandsFromPipeline(ctx context.Context, pipe redis.Pipeliner, mod *modio.Mod, itemTypeTag string) {
	modType := GetModTypeFromTag(itemTypeTag) // Use exported version
	modIDStr := strconv.Itoa(mod.ID)

	pipe.Del(ctx, ModKeyPrefix+modIDStr) // Use exported version
	pipe.SRem(ctx, modTypeSetKeyPrefix+modType, modIDStr)

	normalizedTitle := normalizeStringForIndex(mod.Name)
	autocompleteMember := fmt.Sprintf("%s:%s", normalizedTitle, modIDStr)
	pipe.ZRem(ctx, modTitleSortedSetKeyPrefix+modType, autocompleteMember)

	pipe.ZRem(ctx, modDateUpdatedSortedSetKeyPrefix+modType, modIDStr)

	for _, tag := range mod.Tags {
		normalizedTagName := normalizeStringForIndex(tag.Name)
		tagSetKey := fmt.Sprintf("%s%s:%s", modTagSetKeyPrefix, normalizedTagName, modType)
		pipe.SRem(ctx, tagSetKey, modIDStr)
	}
	slog.Debug("Added commands to pipeline for removing mod", "mod_id", mod.ID)
}

func (r *ModRepository) GetModByID(ctx context.Context, modID int) (*modio.Mod, error) {
	modKey := ModKeyPrefix + strconv.Itoa(modID) // Use exported version
	slog.Debug("Fetching mod by ID from Redis", "key", modKey)

	modJSON, err := r.rdb.Get(ctx, modKey).Result()
	if err == redis.Nil {
		slog.Debug("Mod not found in Redis", "mod_id", modID, "key", modKey)
		return nil, nil
	}
	if err != nil {
		slog.Error("Failed to get mod from Redis", "mod_id", modID, "key", modKey, "error", err)
		return nil, err
	}

	var mod modio.Mod
	if err := json.Unmarshal([]byte(modJSON), &mod); err != nil {
		slog.Error("Failed to unmarshal mod JSON from Redis", "mod_id", modID, "key", modKey, "error", err)
		return nil, err
	}
	return &mod, nil
}

func (r *ModRepository) GetModsByIDs(ctx context.Context, modIDs []string) ([]*modio.Mod, error) {
	if len(modIDs) == 0 {
		return []*modio.Mod{}, nil
	}
	keys := make([]string, len(modIDs))
	for i, idStr := range modIDs {
		keys[i] = ModKeyPrefix + idStr // Use exported version
	}

	slog.Debug("Fetching multiple mods by IDs from Redis", "count", len(keys))
	results, err := r.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		slog.Error("Failed to MGET mods from Redis", "error", err)
		return nil, err
	}

	mods := make([]*modio.Mod, 0, len(results))
	for i, res := range results {
		if res == nil {
			slog.Debug("Mod ID not found during MGET", "id_queried", modIDs[i])
			continue
		}
		modJSON, ok := res.(string)
		if !ok {
			slog.Error("Unexpected type from MGET result for mod ID", "id_queried", modIDs[i], "type", fmt.Sprintf("%T", res))
			continue
		}
		var mod modio.Mod
		if err := json.Unmarshal([]byte(modJSON), &mod); err != nil {
			slog.Error("Failed to unmarshal mod JSON from MGET result", "id_queried", modIDs[i], "error", err)
			continue
		}
		mods = append(mods, &mod)
	}
	return mods, nil
}

func (r *ModRepository) GetAllModIDsByType(ctx context.Context, modType string) ([]string, error) {
	typeSetKey := modTypeSetKeyPrefix + normalizeStringForIndex(modType)
	slog.Debug("Fetching all mod IDs by type from Redis Set", "key", typeSetKey)
	ids, err := r.rdb.SMembers(ctx, typeSetKey).Result()
	if err != nil {
		slog.Error("Failed to get mod IDs from type set in Redis", "key", typeSetKey, "error", err)
		return nil, err
	}
	return ids, nil
}

func (r *ModRepository) GetModsByType(ctx context.Context, modTypeTag string) ([]modio.Mod, time.Time, error) {
	modType := GetModTypeFromTag(modTypeTag) // Use exported version
	ids, err := r.GetAllModIDsByType(ctx, modType)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get mod IDs for type %s: %w", modType, err)
	}

	modPointers, err := r.GetModsByIDs(ctx, ids)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get mods by IDs for type %s: %w", modType, err)
	}

	mods := make([]modio.Mod, 0, len(modPointers))
	for _, modPtr := range modPointers {
		if modPtr != nil {
			mods = append(mods, *modPtr)
		}
	}

	lastWriteTime, err := r.GetLastOverallWriteTimestamp(ctx)
	if err != nil {
		slog.Warn("Could not get last overall write timestamp for GetModsByType", "modType", modType, "error", err)
	}
	return mods, lastWriteTime, nil
}

func (r *ModRepository) GetLastOverallWriteTimestamp(ctx context.Context) (time.Time, error) {
	val, err := r.rdb.Get(ctx, systemLastOverallWriteTimestampKey).Result()
	if err == redis.Nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, val)
}

func (r *ModRepository) SetLastOverallWriteTimestamp(ctx context.Context, t time.Time) error {
	slog.Debug("Setting last overall write timestamp in Redis", "timestamp", t.Format(time.RFC3339Nano))
	return r.rdb.Set(ctx, systemLastOverallWriteTimestampKey, t.Format(time.RFC3339Nano), 0).Err()
}

func (r *ModRepository) GetSchedulerLastSyncEventTimestamp(ctx context.Context) (int64, error) {
	val, err := r.rdb.Get(ctx, schedulerLastSyncEventTimestampKey).Result()
	if err == redis.Nil {
		slog.Info("Scheduler's last sync event timestamp not found in Redis.", "key", schedulerLastSyncEventTimestampKey)
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ts, convErr := strconv.ParseInt(val, 10, 64)
	if convErr != nil {
		return 0, convErr
	}
	return ts, nil
}

func (r *ModRepository) SetSchedulerLastSyncEventTimestamp(ctx context.Context, ts int64) error {
	slog.Debug("Setting scheduler's last sync event timestamp in Redis", "timestamp", ts)
	return r.rdb.Set(ctx, schedulerLastSyncEventTimestampKey, ts, 0).Err()
}

func (r *ModRepository) SearchTitlesByPrefix(ctx context.Context, modTypeTag string, prefix string, count int) ([]string, error) {
	modType := GetModTypeFromTag(modTypeTag) // Use exported version
	titleSortedSetKey := modTitleSortedSetKeyPrefix + modType
	normalizedPrefix := normalizeStringForIndex(prefix)

	if normalizedPrefix == "" {
		return []string{}, nil
	}

	results, err := r.rdb.ZRangeByLex(ctx, titleSortedSetKey, &redis.ZRangeBy{
		Min:    "[" + normalizedPrefix,
		Max:    "[" + normalizedPrefix + "\xff",
		Offset: 0,
		Count:  int64(count),
	}).Result()

	if err != nil {
		slog.Error("Error searching titles by prefix in Redis", "key", titleSortedSetKey, "prefix", normalizedPrefix, "error", err)
		return nil, err
	}
	slog.Debug("Autocomplete search results", "key", titleSortedSetKey, "prefix", normalizedPrefix, "count_results", len(results))
	return results, nil
}

func (r *ModRepository) RemoveOrphanedTagIndexEntries(ctx context.Context, pipe redis.Pipeliner, oldMod *modio.Mod, newMod *modio.Mod, itemTypeTag string) {
	modType := GetModTypeFromTag(itemTypeTag) // Use exported version
	modIDStr := strconv.Itoa(oldMod.ID)

	oldTags := make(map[string]bool)
	for _, tag := range oldMod.Tags {
		oldTags[normalizeStringForIndex(tag.Name)] = true
	}

	newTags := make(map[string]bool)
	if newMod != nil {
		for _, tag := range newMod.Tags {
			newTags[normalizeStringForIndex(tag.Name)] = true
		}
	}

	for oldTagName := range oldTags {
		if !newTags[oldTagName] {
			tagSetKey := fmt.Sprintf("%s%s:%s", modTagSetKeyPrefix, oldTagName, modType)
			pipe.SRem(ctx, tagSetKey, modIDStr)
			slog.Debug("Adding command to remove mod from orphaned tag set", "mod_id", modIDStr, "tag", oldTagName, "type", modType)
		}
	}
}

func (r *ModRepository) GetModIDsByTag(ctx context.Context, modTypeTag string, tagName string) ([]string, error) {
	modType := GetModTypeFromTag(modTypeTag) // Use exported version
	normalizedTagName := normalizeStringForIndex(tagName)
	tagSetKey := fmt.Sprintf("%s%s:%s", modTagSetKeyPrefix, normalizedTagName, modType)

	slog.Debug("Fetching mod IDs by tag from Redis", "key", tagSetKey)
	ids, err := r.rdb.SMembers(ctx, tagSetKey).Result()
	if err != nil {
		slog.Error("Failed to get mod IDs by tag from Redis", "key", tagSetKey, "error", err)
		return nil, err
	}
	return ids, nil
}
