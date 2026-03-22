package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateArchiveManifest records a new archive operation.
func (d *DB) CreateArchiveManifest(m ArchiveManifest) (*ArchiveManifest, error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.ArchivedAt = time.Now().UTC()
	if m.RestoreStatus == "" {
		m.RestoreStatus = "idle"
	}

	_, err := d.sql.Exec(
		`INSERT INTO archive_manifests
		 (id, image_ref_id, repo, tag, digest, bucket, key, layers_count, original_bytes, compressed_bytes,
		  archived_at, restore_status, storage_backend)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ImageRefID, m.Repo, m.Tag, m.Digest, m.Bucket, m.Key, m.LayersCount, m.OriginalBytes,
		m.CompressedBytes, m.ArchivedAt, m.RestoreStatus, m.StorageBackend,
	)
	if err != nil {
		return nil, fmt.Errorf("create archive manifest: %w", err)
	}
	return &m, nil
}

// GetArchiveManifest returns an archive by ID.
func (d *DB) GetArchiveManifest(id string) (*ArchiveManifest, error) {
	row := d.sql.QueryRow(
		`SELECT id, image_ref_id, repo, tag, digest, bucket, key, layers_count, original_bytes, compressed_bytes,
		        archived_at, restored_at, restore_status, storage_backend
		 FROM archive_manifests WHERE id = ?`, id,
	)
	return scanArchive(row)
}

// GetArchiveByImageRef returns the most recent archive for an image ref.
func (d *DB) GetArchiveByImageRef(imageRefID string) (*ArchiveManifest, error) {
	row := d.sql.QueryRow(
		`SELECT id, image_ref_id, repo, tag, digest, bucket, key, layers_count, original_bytes, compressed_bytes,
		        archived_at, restored_at, restore_status, storage_backend
		 FROM archive_manifests WHERE image_ref_id = ?
		 ORDER BY archived_at DESC LIMIT 1`, imageRefID,
	)
	return scanArchive(row)
}

// ListArchives returns all archive manifests.
func (d *DB) ListArchives() ([]ArchiveManifest, error) {
	rows, err := d.sql.Query(
		`SELECT id, image_ref_id, repo, tag, digest, bucket, key, layers_count, original_bytes, compressed_bytes,
		        archived_at, restored_at, restore_status, storage_backend
		 FROM archive_manifests ORDER BY archived_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list archives: %w", err)
	}
	defer rows.Close()

	var out []ArchiveManifest
	for rows.Next() {
		a, err := scanArchive(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// SetRestoreStatus updates the restore status and optionally the restored_at timestamp.
func (d *DB) SetRestoreStatus(id, status string) error {
	var restoredAt *time.Time
	if status == "restored" {
		t := time.Now().UTC()
		restoredAt = &t
	}
	_, err := d.sql.Exec(
		`UPDATE archive_manifests SET restore_status = ?, restored_at = ? WHERE id = ?`,
		status, restoredAt, id,
	)
	if err != nil {
		return fmt.Errorf("set restore status: %w", err)
	}
	return nil
}

// ArchiveStats computes aggregate storage statistics.
type ArchiveStats struct {
	TotalOriginalBytes   int64
	TotalCompressedBytes int64
}

// GetArchiveStats returns aggregate storage totals.
func (d *DB) GetArchiveStats() (ArchiveStats, error) {
	var s ArchiveStats
	row := d.sql.QueryRow(
		`SELECT COALESCE(SUM(original_bytes),0), COALESCE(SUM(compressed_bytes),0)
		 FROM archive_manifests`,
	)
	err := row.Scan(&s.TotalOriginalBytes, &s.TotalCompressedBytes)
	if err != nil {
		return s, fmt.Errorf("get archive stats: %w", err)
	}
	return s, nil
}

// DeleteArchive removes an archive manifest by ID.
func (d *DB) DeleteArchive(id string) error {
	res, err := d.sql.Exec(`DELETE FROM archive_manifests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete archive: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("archive %q not found", id)
	}
	return nil
}

type archiveScanner interface {
	Scan(dest ...any) error
}

func scanArchive(s archiveScanner) (*ArchiveManifest, error) {
	var a ArchiveManifest
	var restoredAt sql.NullTime

	var imageRefID sql.NullString
	err := s.Scan(
		&a.ID, &imageRefID, &a.Repo, &a.Tag, &a.Digest,
		&a.Bucket, &a.Key, &a.LayersCount,
		&a.OriginalBytes, &a.CompressedBytes, &a.ArchivedAt, &restoredAt,
		&a.RestoreStatus, &a.StorageBackend,
	)
	if imageRefID.Valid {
		a.ImageRefID = imageRefID.String
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan archive: %w", err)
	}
	if restoredAt.Valid {
		t := restoredAt.Time
		a.RestoredAt = &t
	}
	return &a, nil
}
