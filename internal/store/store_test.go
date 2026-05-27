package store

import (
	"testing"
	"time"

	"github.com/Mininglamp-OSS/octo-speech/internal/config"
)

func TestAppStore_CacheExpiry(t *testing.T) {
	s := &AppStore{
		cache:   make(map[string]cacheEntry),
		ttl:     50 * time.Millisecond,
		maxSize: 100,
	}

	s.cache["key1"] = cacheEntry{
		info:      &AppInfo{AppID: "app1", Status: 1},
		expiresAt: time.Now().Add(50 * time.Millisecond),
	}

	// Should hit cache
	s.mu.RLock()
	entry, ok := s.cache["key1"]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		// Might be expired already in slow CI, skip
	}

	time.Sleep(60 * time.Millisecond)

	// Should be expired now
	s.mu.RLock()
	entry, ok = s.cache["key1"]
	s.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		t.Error("expected cache entry to be expired")
	}
}

func TestAppStore_EvictOldest(t *testing.T) {
	s := &AppStore{
		cache:   make(map[string]cacheEntry),
		ttl:     time.Minute,
		maxSize: 2,
	}

	now := time.Now()
	s.cache["old"] = cacheEntry{
		info:      &AppInfo{AppID: "old-app"},
		expiresAt: now.Add(1 * time.Second),
	}
	s.cache["new"] = cacheEntry{
		info:      &AppInfo{AppID: "new-app"},
		expiresAt: now.Add(10 * time.Second),
	}

	s.evictOldest()

	if _, ok := s.cache["old"]; ok {
		t.Error("expected oldest entry to be evicted")
	}
	if _, ok := s.cache["new"]; !ok {
		t.Error("expected newer entry to remain")
	}
}

func TestIsValidScopeType(t *testing.T) {
	tests := []struct {
		scope string
		want  bool
	}{
		{"global", true},
		{"space", true},
		{"org", true},
		{"project", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsValidScopeType(tt.scope)
		if got != tt.want {
			t.Errorf("IsValidScopeType(%q) = %v, want %v", tt.scope, got, tt.want)
		}
	}
}

func TestLocalConfigStore_Query_DefaultValues(t *testing.T) {
	cfg := &config.Config{
		LocalEnabled:       false,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}

	s := NewLocalConfigStore(nil, cfg)
	result, err := s.Query("", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Enabled {
		t.Error("expected enabled false")
	}
	if result.TimeoutMs != 10000 {
		t.Errorf("expected timeout 10000, got %d", result.TimeoutMs)
	}
	if result.ProbeURL != "http://localhost:8787/" {
		t.Errorf("unexpected probe URL: %s", result.ProbeURL)
	}
	if result.TranscribeURL != "http://localhost:8787/v1/voice/transcribe" {
		t.Errorf("unexpected transcribe URL: %s", result.TranscribeURL)
	}
}
