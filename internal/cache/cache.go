// internal/cache/cache.go
package cache

import (
	"sync"
	"time"

	// Adjust import path based on your go.mod module path
	"github.com/ShawnEdgell/modio-api-go/internal/modio" // To use the ModioMod struct
)

// Store holds the in-memory cache for mods and maps.
type Store struct {
	mu          sync.RWMutex // Read-Write mutex for concurrent access
	Maps        []modio.Mod  // Slice to hold map data
	Scripts     []modio.Mod  // Slice to hold script mod data
	LastUpdated time.Time
}

// NewStore creates and returns a new Cache Store.
func NewStore() *Store {
	return &Store{}
}

// Update replaces the cached data with new data.
// It's designed to be called by the scheduler after fetching fresh data.
func (s *Store) Update(maps []modio.Mod, scripts []modio.Mod) {
	s.mu.Lock() // Acquire a write lock
	defer s.mu.Unlock()

	s.Maps = maps
	s.Scripts = scripts
	s.LastUpdated = time.Now().UTC() // Store update time in UTC
}

// GetMaps returns a copy of the cached maps and the last updated time.
// Returning a copy prevents external modification of the cache.
func (s *Store) GetMaps() ([]modio.Mod, time.Time) {
	s.mu.RLock() // Acquire a read lock
	defer s.mu.RUnlock()

	// Return a copy to prevent modification of the underlying slice by callers
	// For simple structs like ModioMod, a shallow copy is usually fine here.
	// If ModioMod had pointers that could be mutated, a deep copy might be needed.
	mapsCopy := make([]modio.Mod, len(s.Maps))
	copy(mapsCopy, s.Maps)

	return mapsCopy, s.LastUpdated
}

// GetScripts returns a copy of the cached script mods and the last updated time.
func (s *Store) GetScripts() ([]modio.Mod, time.Time) {
	s.mu.RLock() // Acquire a read lock
	defer s.mu.RUnlock()

	scriptsCopy := make([]modio.Mod, len(s.Scripts))
	copy(scriptsCopy, s.Scripts)

	return scriptsCopy, s.LastUpdated
}