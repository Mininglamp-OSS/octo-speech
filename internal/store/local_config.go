package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Mininglamp-OSS/octo-speech/internal/config"
)


type LocalASRConfig struct {
	Enabled       bool
	TimeoutMs     int
	ProbeURL      string
	TranscribeURL string
}

type LocalConfigStore struct {
	db  *sql.DB
	cfg *config.Config
}

func NewLocalConfigStore(db *sql.DB, cfg *config.Config) *LocalConfigStore {
	return &LocalConfigStore{db: db, cfg: cfg}
}

func (s *LocalConfigStore) Upsert(appID, subjectID, scopeType, scopeID string, enabled bool, timeoutMs *int, probeURL, transcribeURL *string) error {
	cols := []string{"app_id", "subject_id", "scope_type", "scope_id", "enabled"}
	args := []interface{}{appID, subjectID, scopeType, scopeID, enabled}
	updates := []string{"enabled=VALUES(enabled)"}

	if timeoutMs != nil {
		cols = append(cols, "timeout_ms")
		args = append(args, *timeoutMs)
		updates = append(updates, "timeout_ms=VALUES(timeout_ms)")
	}
	if probeURL != nil {
		cols = append(cols, "probe_url")
		args = append(args, *probeURL)
		updates = append(updates, "probe_url=VALUES(probe_url)")
	}
	if transcribeURL != nil {
		cols = append(cols, "transcribe_url")
		args = append(args, *transcribeURL)
		updates = append(updates, "transcribe_url=VALUES(transcribe_url)")
	}

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		"INSERT INTO local_asr_config (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(updates, ", "),
	)

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *LocalConfigStore) Delete(appID, subjectID, scopeType, scopeID string) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM local_asr_config
		 WHERE app_id = ? AND subject_id = ? AND scope_type = ? AND scope_id = ?`,
		appID, subjectID, scopeType, scopeID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *LocalConfigStore) Query(appID, subjectID, scopeType, scopeID string) (*LocalASRConfig, error) {
	result := &LocalASRConfig{
		Enabled:       s.cfg.LocalEnabled,
		TimeoutMs:     s.cfg.LocalTimeoutMs,
		ProbeURL:      s.cfg.LocalProbeURL,
		TranscribeURL: s.cfg.LocalTranscribeURL,
	}

	if appID == "" || subjectID == "" || scopeType == "" || scopeID == "" {
		return result, nil
	}

	var enabled sql.NullInt64
	var timeoutMs sql.NullInt64
	var probeURL sql.NullString
	var transcribeURL sql.NullString

	err := s.db.QueryRow(
		`SELECT enabled, timeout_ms, probe_url, transcribe_url
		 FROM local_asr_config
		 WHERE app_id = ? AND subject_id = ? AND scope_type = ? AND scope_id = ?`,
		appID, subjectID, scopeType, scopeID,
	).Scan(&enabled, &timeoutMs, &probeURL, &transcribeURL)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, fmt.Errorf("query local config: %w", err)
	}

	if enabled.Valid {
		result.Enabled = enabled.Int64 == 1
	}
	if timeoutMs.Valid {
		result.TimeoutMs = int(timeoutMs.Int64)
	}
	if probeURL.Valid {
		result.ProbeURL = probeURL.String
	}
	if transcribeURL.Valid {
		result.TranscribeURL = transcribeURL.String
	}

	return result, nil
}
