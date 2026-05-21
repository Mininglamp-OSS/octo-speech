package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrAppNotFound = errors.New("app not found")

type AppInfo struct {
	AppID  string
	Status int
}

type AppRecord struct {
	ID        int64
	AppID     string
	AppName   string
	Status    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type cacheEntry struct {
	info      *AppInfo
	expiresAt time.Time
}

type AppStore struct {
	db      *sql.DB
	mu      sync.RWMutex
	cache   map[string]cacheEntry
	ttl     time.Duration
	maxSize int
}

func NewAppStore(db *sql.DB, ttlSeconds int) *AppStore {
	return &AppStore{
		db:      db,
		cache:   make(map[string]cacheEntry),
		ttl:     time.Duration(ttlSeconds) * time.Second,
		maxSize: 10000,
	}
}

func (s *AppStore) Authenticate(ctx context.Context, apiKey string) (*AppInfo, error) {
	h := sha256.Sum256([]byte(apiKey))
	apiKeyHash := hex.EncodeToString(h[:])

	s.mu.RLock()
	if entry, ok := s.cache[apiKeyHash]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.info, nil
	}
	s.mu.RUnlock()

	var appID string
	var status int
	err := s.db.QueryRowContext(ctx,
		"SELECT app_id, status FROM app_registry WHERE api_key = ?",
		apiKeyHash,
	).Scan(&appID, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	info := &AppInfo{AppID: appID, Status: status}

	s.mu.Lock()
	if len(s.cache) >= s.maxSize {
		s.evictOldest()
	}
	s.cache[apiKeyHash] = cacheEntry{
		info:      info,
		expiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()

	return info, nil
}

func (s *AppStore) Create(ctx context.Context, appID, appName, apiKeyHash string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO app_registry (app_id, app_name, api_key) VALUES (?, ?, ?)",
		appID, appName, apiKeyHash,
	)
	return err
}

func (s *AppStore) List(ctx context.Context, status *int, keyword string) ([]AppRecord, error) {
	query := "SELECT id, app_id, app_name, status, created_at, updated_at FROM app_registry WHERE 1=1"
	var args []interface{}

	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}
	if keyword != "" {
		query += " AND (app_name LIKE ? OR app_id LIKE ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AppRecord
	for rows.Next() {
		var r AppRecord
		if err := rows.Scan(&r.ID, &r.AppID, &r.AppName, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *AppStore) GetByAppID(ctx context.Context, appID string) (*AppRecord, error) {
	var r AppRecord
	err := s.db.QueryRowContext(ctx,
		"SELECT id, app_id, app_name, status, created_at, updated_at FROM app_registry WHERE app_id = ?",
		appID,
	).Scan(&r.ID, &r.AppID, &r.AppName, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (s *AppStore) invalidateCache() {
	s.mu.Lock()
	s.cache = make(map[string]cacheEntry)
	s.mu.Unlock()
}

func (s *AppStore) UpdateStatus(ctx context.Context, appID string, status int) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE app_registry SET status = ? WHERE app_id = ?",
		status, appID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrAppNotFound
	}
	s.invalidateCache()
	return nil
}

func (s *AppStore) UpdateAPIKey(ctx context.Context, appID, newKeyHash string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE app_registry SET api_key = ? WHERE app_id = ?",
		newKeyHash, appID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrAppNotFound
	}
	s.invalidateCache()
	return nil
}

func (s *AppStore) Delete(ctx context.Context, appID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM local_asr_config WHERE app_id = ?", appID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM vocabulary_profile WHERE app_id = ?", appID)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM app_registry WHERE app_id = ?", appID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrAppNotFound
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}

func (s *AppStore) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range s.cache {
		if first || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(s.cache, oldestKey)
	}
}
