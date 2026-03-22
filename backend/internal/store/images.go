package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ImageFilter holds optional query filters for ListImages.
type ImageFilter struct {
	Registry string
	Status   string
	Search   string
	GroupID  string
}

// CreateImage inserts a new ImageRef.
func (d *DB) CreateImage(img ImageRef) (*ImageRef, error) {
	if img.ID == "" {
		img.ID = uuid.NewString()
	}
	img.AddedAt = time.Now().UTC()
	if img.LastStatus == "" {
		img.LastStatus = "unpinned"
	}

	_, err := d.sql.Exec(
		`INSERT INTO image_refs
		 (id, registry, repo, tag, digest, size, group_id, added_at, last_status, expires_in_days, pinned)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.ID, img.Registry, img.Repo, img.Tag, img.Digest, img.Size,
		img.GroupID, img.AddedAt, img.LastStatus, img.ExpiresInDays,
		boolToInt(img.Pinned),
	)
	if err != nil {
		return nil, fmt.Errorf("create image: %w", err)
	}
	return &img, nil
}

// GetImage returns an image by ID.
func (d *DB) GetImage(id string) (*ImageRef, error) {
	row := d.sql.QueryRow(
		`SELECT id, registry, repo, tag, digest, size, group_id,
		        added_at, last_keepalive_at, last_status, last_error, expires_in_days, pinned
		 FROM image_refs WHERE id = ?`, id,
	)
	return scanImage(row)
}

// ListImages returns images with optional filters.
func (d *DB) ListImages(f ImageFilter) ([]ImageRef, error) {
	q := `SELECT id, registry, repo, tag, digest, size, group_id,
		         added_at, last_keepalive_at, last_status, last_error, expires_in_days, pinned
		  FROM image_refs`

	var conds []string
	var args []any

	if f.Registry != "" && f.Registry != "all" {
		conds = append(conds, "registry = ?")
		args = append(args, f.Registry)
	}
	if f.Status != "" {
		conds = append(conds, "last_status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		conds = append(conds, "(repo LIKE ? OR tag LIKE ?)")
		args = append(args, "%"+f.Search+"%", "%"+f.Search+"%")
	}
	if f.GroupID != "" {
		conds = append(conds, "group_id = ?")
		args = append(args, f.GroupID)
	}

	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY added_at DESC"

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var imgs []ImageRef
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, *img)
	}
	return imgs, rows.Err()
}

// SetPinned updates the pinned flag for an image.
func (d *DB) SetPinned(id string, pinned bool) error {
	_, err := d.sql.Exec(
		`UPDATE image_refs SET pinned = ? WHERE id = ?`,
		boolToInt(pinned), id,
	)
	if err != nil {
		return fmt.Errorf("set pinned: %w", err)
	}
	return nil
}

// UpdateKeepaliveStatus records the result of a keepalive attempt on an image.
func (d *DB) UpdateKeepaliveStatus(id, status string, errMsg *string) error {
	now := time.Now().UTC()
	_, err := d.sql.Exec(
		`UPDATE image_refs SET last_keepalive_at = ?, last_status = ?, last_error = ? WHERE id = ?`,
		now, status, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("update keepalive status: %w", err)
	}
	return nil
}

// UpdateExpiresIn sets the expires_in_days field.
func (d *DB) UpdateExpiresIn(id string, days int) error {
	_, err := d.sql.Exec(`UPDATE image_refs SET expires_in_days = ? WHERE id = ?`, days, id)
	return err
}

// SetImageRegistry updates the registry field for an image.
func (d *DB) SetImageRegistry(id, registryName string) error {
	_, err := d.sql.Exec(`UPDATE image_refs SET registry = ? WHERE id = ?`, registryName, id)
	if err != nil {
		return fmt.Errorf("set image registry: %w", err)
	}
	return nil
}

// SetImageGroup updates the group assignment for an image.
// Pass nil to remove from any group.
func (d *DB) SetImageGroup(id string, groupID *string) error {
	_, err := d.sql.Exec(`UPDATE image_refs SET group_id = ? WHERE id = ?`, groupID, id)
	if err != nil {
		return fmt.Errorf("set image group: %w", err)
	}
	return nil
}

// DeleteImage removes an image and its logs (cascade).
func (d *DB) DeleteImage(id string) error {
	_, err := d.sql.Exec(`DELETE FROM image_refs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}

type imageScanner interface {
	Scan(dest ...any) error
}

func scanImage(s imageScanner) (*ImageRef, error) {
	var img ImageRef
	var pinned int
	var groupID sql.NullString
	var lastKeepaliveAt sql.NullTime
	var lastError sql.NullString

	err := s.Scan(
		&img.ID, &img.Registry, &img.Repo, &img.Tag, &img.Digest, &img.Size,
		&groupID, &img.AddedAt, &lastKeepaliveAt, &img.LastStatus, &lastError,
		&img.ExpiresInDays, &pinned,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan image: %w", err)
	}

	img.Pinned = pinned == 1
	if groupID.Valid {
		img.GroupID = &groupID.String
	}
	if lastKeepaliveAt.Valid {
		t := lastKeepaliveAt.Time
		img.LastKeepaliveAt = &t
	}
	if lastError.Valid {
		img.LastError = &lastError.String
	}
	return &img, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
