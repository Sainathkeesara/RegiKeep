package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateGroup inserts a new group.
func (d *DB) CreateGroup(name, interval, strategy string) (*Group, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := d.sql.Exec(
		`INSERT INTO groups (id, name, interval, strategy, enabled, created_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		id, name, interval, strategy, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return &Group{
		ID:        id,
		Name:      name,
		Interval:  interval,
		Strategy:  strategy,
		Enabled:   true,
		CreatedAt: now,
	}, nil
}

// GetGroup returns a group by ID.
func (d *DB) GetGroup(id string) (*Group, error) {
	row := d.sql.QueryRow(
		`SELECT id, name, interval, strategy, enabled, created_at FROM groups WHERE id = ?`, id,
	)
	return scanGroup(row)
}

// GetGroupByName returns a group by name.
func (d *DB) GetGroupByName(name string) (*Group, error) {
	row := d.sql.QueryRow(
		`SELECT id, name, interval, strategy, enabled, created_at FROM groups WHERE name = ?`, name,
	)
	return scanGroup(row)
}

// ListGroups returns all groups.
func (d *DB) ListGroups() ([]Group, error) {
	rows, err := d.sql.Query(
		`SELECT id, name, interval, strategy, enabled, created_at FROM groups ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *g)
	}
	return groups, rows.Err()
}

// UpdateGroup updates mutable fields of a group.
func (d *DB) UpdateGroup(id, name, interval, strategy string) error {
	_, err := d.sql.Exec(
		`UPDATE groups SET name = ?, interval = ?, strategy = ? WHERE id = ?`,
		name, interval, strategy, id,
	)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

// SetGroupEnabled enables or disables a group.
func (d *DB) SetGroupEnabled(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := d.sql.Exec(`UPDATE groups SET enabled = ? WHERE id = ?`, v, id)
	if err != nil {
		return fmt.Errorf("set group enabled: %w", err)
	}
	return nil
}

// DeleteGroup removes a group. Images in the group have their group_id set to NULL.
func (d *DB) DeleteGroup(id string) error {
	_, err := d.sql.Exec(`DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

type groupScanner interface {
	Scan(dest ...any) error
}

func scanGroup(s groupScanner) (*Group, error) {
	var g Group
	var enabled int
	err := s.Scan(&g.ID, &g.Name, &g.Interval, &g.Strategy, &enabled, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan group: %w", err)
	}
	g.Enabled = enabled == 1
	return &g, nil
}
