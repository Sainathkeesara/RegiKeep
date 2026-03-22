package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WriteKeepaliveLog records a keepalive attempt.
func (d *DB) WriteKeepaliveLog(imageRefID, status string, durationMs int64, errMsg *string) error {
	_, err := d.sql.Exec(
		`INSERT INTO keepalive_logs (id, image_ref_id, ran_at, status, duration_ms, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), imageRefID, time.Now().UTC(), status, durationMs, errMsg,
	)
	if err != nil {
		return fmt.Errorf("write keepalive log: %w", err)
	}
	return nil
}

// ListKeepaliveLogs returns the last 7 days of logs for an image.
func (d *DB) ListKeepaliveLogs(imageRefID string) ([]KeepaliveLog, error) {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	rows, err := d.sql.Query(
		`SELECT id, image_ref_id, ran_at, status, duration_ms, error
		 FROM keepalive_logs
		 WHERE image_ref_id = ? AND ran_at >= ?
		 ORDER BY ran_at DESC`,
		imageRefID, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("list keepalive logs: %w", err)
	}
	defer rows.Close()

	var logs []KeepaliveLog
	for rows.Next() {
		l, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, *l)
	}
	return logs, rows.Err()
}

// CountRecentFailures counts consecutive failures for an image within the last N logs.
func (d *DB) CountRecentFailures(imageRefID string, n int) (int, error) {
	rows, err := d.sql.Query(
		`SELECT status FROM keepalive_logs
		 WHERE image_ref_id = ?
		 ORDER BY ran_at DESC LIMIT ?`,
		imageRefID, n,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return 0, err
		}
		if status == "failure" {
			count++
		} else {
			break // stop at first non-failure
		}
	}
	return count, rows.Err()
}

type logScanner interface {
	Scan(dest ...any) error
}

func scanLog(s logScanner) (*KeepaliveLog, error) {
	var l KeepaliveLog
	var errStr sql.NullString
	err := s.Scan(&l.ID, &l.ImageRefID, &l.RanAt, &l.Status, &l.DurationMs, &errStr)
	if err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}
	if errStr.Valid {
		l.Error = &errStr.String
	}
	return &l, nil
}
