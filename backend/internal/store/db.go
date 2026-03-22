package store

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DB wraps the SQLite connection and provides all store operations.
type DB struct {
	sql *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies the schema.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite is single-writer; cap pool to 1 writer, allow concurrent readers.
	db.SetMaxOpenConns(1)

	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &DB{sql: db}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

// SQL returns the raw *sql.DB for use in store methods.
func (d *DB) SQL() *sql.DB {
	return d.sql
}

func applySchema(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return applyMigrations(db)
}

// applyMigrations runs idempotent ALTER statements for schema evolution.
func applyMigrations(db *sql.DB) error {
	// v0.2: add registry_type column
	_, _ = db.Exec(`ALTER TABLE registries ADD COLUMN registry_type TEXT NOT NULL DEFAULT ''`)
	// v0.3: add credential columns
	_, _ = db.Exec(`ALTER TABLE registries ADD COLUMN auth_username TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE registries ADD COLUMN auth_token TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE registries ADD COLUMN auth_extra TEXT NOT NULL DEFAULT ''`)

	// v0.4: archive_manifests — store repo/tag/digest so archives survive image deletion
	_, _ = db.Exec(`ALTER TABLE archive_manifests ADD COLUMN repo TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE archive_manifests ADD COLUMN tag TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE archive_manifests ADD COLUMN digest TEXT NOT NULL DEFAULT ''`)

	// v0.4: backfill repo/tag from image_refs for existing archives
	_, _ = db.Exec(`UPDATE archive_manifests SET
		repo = COALESCE((SELECT repo FROM image_refs WHERE image_refs.id = archive_manifests.image_ref_id), repo),
		tag = COALESCE((SELECT tag FROM image_refs WHERE image_refs.id = archive_manifests.image_ref_id), tag),
		digest = COALESCE((SELECT digest FROM image_refs WHERE image_refs.id = archive_manifests.image_ref_id), digest)
		WHERE repo = ''`)

	return nil
}
