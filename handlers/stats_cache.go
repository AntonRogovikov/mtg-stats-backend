package handlers

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type statsCacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

type statsCacheStore struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]statsCacheEntry
}

func newStatsCacheStore() *statsCacheStore {
	return &statsCacheStore{
		ttl:     time.Duration(envIntSeconds("STATS_CACHE_TTL_SECONDS", 15)) * time.Second,
		entries: make(map[string]statsCacheEntry),
	}
}

func (s *statsCacheStore) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return nil, false
	}
	return entry.payload, true
}

func (s *statsCacheStore) SetJSON(key string, v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.entries[key] = statsCacheEntry{
		payload:   payload,
		expiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return nil
}

func (s *statsCacheStore) InvalidateAll() {
	s.mu.Lock()
	clear(s.entries)
	s.mu.Unlock()
}

func envIntSeconds(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

var statsResponseCache = newStatsCacheStore()

func invalidateStatsCache() {
	statsResponseCache.InvalidateAll()
}
