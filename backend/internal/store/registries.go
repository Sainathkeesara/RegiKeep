package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const registryCols = `id, name, registry_type, endpoint, region, tenancy, credential_source, auth_username, auth_token, auth_extra, created_at`

// CreateRegistry inserts a new registry configuration.
func (d *DB) CreateRegistry(r RegistryConfig) (*RegistryConfig, error) {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	r.CreatedAt = time.Now().UTC()
	_, err := d.sql.Exec(
		`INSERT INTO registries (id, name, registry_type, endpoint, region, tenancy, credential_source, auth_username, auth_token, auth_extra, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.RegistryType, r.Endpoint, r.Region, r.Tenancy, r.CredentialSource,
		r.AuthUsername, r.AuthToken, r.AuthExtra, r.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}
	return &r, nil
}

// GetRegistry returns a registry by ID.
func (d *DB) GetRegistry(id string) (*RegistryConfig, error) {
	row := d.sql.QueryRow(
		`SELECT `+registryCols+` FROM registries WHERE id = ?`, id,
	)
	return scanRegistry(row)
}

// ListRegistries returns all registries.
func (d *DB) ListRegistries() ([]RegistryConfig, error) {
	rows, err := d.sql.Query(
		`SELECT ` + registryCols + ` FROM registries ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list registries: %w", err)
	}
	defer rows.Close()

	var out []RegistryConfig
	for rows.Next() {
		r, err := scanRegistry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// DeleteRegistry removes a registry by ID.
func (d *DB) DeleteRegistry(id string) error {
	_, err := d.sql.Exec(`DELETE FROM registries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete registry: %w", err)
	}
	return nil
}

// UpdateRegistryCredential updates the credential source for a registry.
func (d *DB) UpdateRegistryCredential(id, source string) error {
	_, err := d.sql.Exec(`UPDATE registries SET credential_source = ? WHERE id = ?`, source, id)
	if err != nil {
		return fmt.Errorf("update registry credential: %w", err)
	}
	return nil
}

// GetRegistryByEndpoint returns a registry matching the given endpoint.
func (d *DB) GetRegistryByEndpoint(endpoint string) (*RegistryConfig, error) {
	row := d.sql.QueryRow(
		`SELECT `+registryCols+` FROM registries WHERE endpoint = ?`, endpoint,
	)
	return scanRegistry(row)
}

type registryScanner interface {
	Scan(dest ...any) error
}

func scanRegistry(s registryScanner) (*RegistryConfig, error) {
	var r RegistryConfig
	err := s.Scan(&r.ID, &r.Name, &r.RegistryType, &r.Endpoint, &r.Region, &r.Tenancy,
		&r.CredentialSource, &r.AuthUsername, &r.AuthToken, &r.AuthExtra, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan registry: %w", err)
	}
	return &r, nil
}
