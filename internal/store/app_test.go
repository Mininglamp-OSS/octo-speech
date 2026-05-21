package store

import (
	"testing"
	"time"
)

func newTestAppStore() *AppStore {
	return &AppStore{
		db:      nil,
		cache:   make(map[string]cacheEntry),
		ttl:     5 * time.Minute,
		maxSize: 100,
	}
}

func seedCache(s *AppStore, keys ...string) {
	s.mu.Lock()
	for _, k := range keys {
		s.cache[k] = cacheEntry{
			info:      &AppInfo{AppID: "app_test", Status: 1},
			expiresAt: time.Now().Add(5 * time.Minute),
		}
	}
	s.mu.Unlock()
}

func cacheLen(s *AppStore) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}

func TestInvalidateCache(t *testing.T) {
	s := newTestAppStore()
	seedCache(s, "hash1", "hash2", "hash3")

	if cacheLen(s) != 3 {
		t.Fatalf("expected 3 cache entries, got %d", cacheLen(s))
	}

	s.invalidateCache()

	if cacheLen(s) != 0 {
		t.Errorf("expected 0 cache entries after invalidation, got %d", cacheLen(s))
	}
}

func TestInvalidateCache_Empty(t *testing.T) {
	s := newTestAppStore()

	s.invalidateCache()

	if cacheLen(s) != 0 {
		t.Errorf("expected 0 cache entries, got %d", cacheLen(s))
	}
}

func TestInvalidateCache_PreservesNewEntries(t *testing.T) {
	s := newTestAppStore()
	seedCache(s, "old1", "old2")

	s.invalidateCache()

	if cacheLen(s) != 0 {
		t.Fatalf("expected 0 after invalidation, got %d", cacheLen(s))
	}

	seedCache(s, "new1")
	if cacheLen(s) != 1 {
		t.Errorf("expected 1 entry after re-seeding, got %d", cacheLen(s))
	}
}
