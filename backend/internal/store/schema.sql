PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS registries (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    registry_type     TEXT NOT NULL DEFAULT '',
    endpoint          TEXT NOT NULL,
    region            TEXT NOT NULL DEFAULT '',
    tenancy           TEXT NOT NULL DEFAULT '',
    credential_source TEXT NOT NULL DEFAULT 'env',
    auth_username     TEXT NOT NULL DEFAULT '',
    auth_token        TEXT NOT NULL DEFAULT '',
    auth_extra        TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS groups (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    interval   TEXT NOT NULL DEFAULT '7d',
    strategy   TEXT NOT NULL DEFAULT 'pull',
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS image_refs (
    id                 TEXT PRIMARY KEY,
    registry           TEXT NOT NULL DEFAULT '',
    repo               TEXT NOT NULL,
    tag                TEXT NOT NULL,
    digest             TEXT NOT NULL DEFAULT '',
    size               TEXT NOT NULL DEFAULT '',
    group_id           TEXT REFERENCES groups(id) ON DELETE SET NULL,
    added_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_keepalive_at  DATETIME,
    last_status        TEXT NOT NULL DEFAULT 'unpinned',
    last_error         TEXT,
    expires_in_days    INTEGER NOT NULL DEFAULT -1,
    pinned             INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_image_refs_registry ON image_refs(registry);
CREATE INDEX IF NOT EXISTS idx_image_refs_group_id ON image_refs(group_id);
CREATE INDEX IF NOT EXISTS idx_image_refs_status   ON image_refs(last_status);

CREATE TABLE IF NOT EXISTS keepalive_logs (
    id            TEXT PRIMARY KEY,
    image_ref_id  TEXT NOT NULL REFERENCES image_refs(id) ON DELETE CASCADE,
    ran_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status        TEXT NOT NULL,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    error         TEXT
);

CREATE INDEX IF NOT EXISTS idx_keepalive_logs_image ON keepalive_logs(image_ref_id, ran_at);

CREATE TABLE IF NOT EXISTS archive_manifests (
    id               TEXT PRIMARY KEY,
    image_ref_id     TEXT REFERENCES image_refs(id) ON DELETE SET NULL,
    repo             TEXT NOT NULL DEFAULT '',
    tag              TEXT NOT NULL DEFAULT '',
    digest           TEXT NOT NULL DEFAULT '',
    bucket           TEXT NOT NULL,
    key              TEXT NOT NULL,
    layers_count     INTEGER NOT NULL DEFAULT 0,
    original_bytes   INTEGER NOT NULL DEFAULT 0,
    compressed_bytes INTEGER NOT NULL DEFAULT 0,
    archived_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    restored_at      DATETIME,
    restore_status   TEXT NOT NULL DEFAULT 'idle',
    storage_backend  TEXT NOT NULL DEFAULT 's3'
);

CREATE INDEX IF NOT EXISTS idx_archive_image ON archive_manifests(image_ref_id);
