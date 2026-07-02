package admin

import (
	"strings"
	"testing"
)

func TestGenerateAppID(t *testing.T) {
	id, err := GenerateAppID()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "app_") {
		t.Errorf("expected prefix 'app_', got %q", id)
	}
	if len(id) != 20 {
		t.Errorf("expected length 20, got %d", len(id))
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("expected prefix 'sk-', got %q", key)
	}
	if len(key) != 32 {
		t.Errorf("expected length 32, got %d", len(key))
	}
}

func TestGenerateAppID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := GenerateAppID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatal(err)
		}
		if seen[key] {
			t.Fatalf("duplicate key: %s", key)
		}
		seen[key] = true
	}
}

func TestGenerateAppID_Base62Chars(t *testing.T) {
	for i := 0; i < 50; i++ {
		id, err := GenerateAppID()
		if err != nil {
			t.Fatal(err)
		}
		suffix := id[4:]
		for _, c := range suffix {
			if !strings.ContainsRune(base62, c) {
				t.Errorf("invalid character %c in ID %s", c, id)
			}
		}
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	hash := hashAPIKey(key)

	if len(hash) != 64 {
		t.Errorf("expected SHA-256 hex length 64, got %d", len(hash))
	}

	hash2 := hashAPIKey(key)
	if hash != hash2 {
		t.Error("expected deterministic hash")
	}

	hash3 := hashAPIKey("sk-differentKey12345678901234567")
	if hash == hash3 {
		t.Error("expected different hashes for different keys")
	}
}
