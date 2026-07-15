package store

import (
	"database/sql"
	"fmt"
)

const latestSchemaVersion = 2

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{version: 1, sql: schema},
	{version: 2, sql: resourceCacheSchema},
}

func applyMigrations(db *sql.DB) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if current > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, latestSchemaVersion)
	}

	for _, item := range migrations {
		if item.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("starting schema migration %d: %w", item.version, err)
		}
		if _, err := tx.Exec(item.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("applying schema migration %d: %w", item.version, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", item.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("recording schema migration %d: %w", item.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing schema migration %d: %w", item.version, err)
		}
		current = item.version
	}
	return nil
}

const resourceCacheSchema = `
ALTER TABLE users ADD COLUMN raw TEXT;

ALTER TABLE projects ADD COLUMN org_uuid TEXT;
ALTER TABLE projects ADD COLUMN deleted_at TEXT;
ALTER TABLE projects ADD COLUMN is_private INTEGER;
ALTER TABLE projects ADD COLUMN raw TEXT;
ALTER TABLE projects ADD COLUMN seen_generation TEXT NOT NULL DEFAULT '';

ALTER TABLE chats ADD COLUMN org_uuid TEXT;
ALTER TABLE chats ADD COLUMN model TEXT;
ALTER TABLE chats ADD COLUMN href TEXT;
ALTER TABLE chats ADD COLUMN raw TEXT;
ALTER TABLE chats ADD COLUMN seen_generation TEXT NOT NULL DEFAULT '';

CREATE TABLE sync_runs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    resource       TEXT NOT NULL,
    scope          TEXT NOT NULL DEFAULT '',
    generation     TEXT NOT NULL,
    mode           TEXT NOT NULL,
    status         TEXT NOT NULL,
    cursor         TEXT,
    records_seen   INTEGER NOT NULL DEFAULT 0,
    records_stored INTEGER NOT NULL DEFAULT 0,
    started_at     TEXT NOT NULL,
    completed_at   TEXT,
    last_error     TEXT
);

CREATE INDEX idx_sync_runs_resource_scope ON sync_runs(resource, scope, started_at);

CREATE TABLE resource_sync_state (
    resource          TEXT NOT NULL,
    scope             TEXT NOT NULL DEFAULT '',
    cursor            TEXT,
    generation        TEXT,
    last_attempted_at TEXT,
    last_completed_at TEXT,
    last_error        TEXT,
    PRIMARY KEY (resource, scope)
);

CREATE TABLE organizations (
    uuid            TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL
);

CREATE TABLE organization_memberships (
    organization_uuid TEXT NOT NULL,
    user_id           TEXT NOT NULL,
    email             TEXT NOT NULL,
    full_name         TEXT,
    organization_role TEXT,
    created_at        TEXT,
    raw               TEXT NOT NULL,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL,
    PRIMARY KEY (organization_uuid, user_id)
);

CREATE INDEX idx_org_memberships_email ON organization_memberships(email);

CREATE TABLE compliance_roles (
    organization_uuid TEXT NOT NULL,
    role_id           TEXT NOT NULL,
    name              TEXT NOT NULL,
    description       TEXT,
    created_at        TEXT,
    updated_at        TEXT,
    raw               TEXT NOT NULL,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL,
    PRIMARY KEY (organization_uuid, role_id)
);

CREATE TABLE compliance_role_permissions (
    organization_uuid TEXT NOT NULL,
    role_id           TEXT NOT NULL,
    action            TEXT NOT NULL,
    resource_type     TEXT NOT NULL,
    resource_id       TEXT NOT NULL,
    raw               TEXT NOT NULL,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL,
    PRIMARY KEY (organization_uuid, role_id, action, resource_type, resource_id)
);

CREATE TABLE compliance_groups (
    group_id        TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    source_type     TEXT,
    roles_json      TEXT NOT NULL,
    created_at      TEXT,
    updated_at      TEXT,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL
);

CREATE TABLE compliance_group_members (
    group_id        TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    email           TEXT NOT NULL,
    created_at      TEXT,
    updated_at      TEXT,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_compliance_group_members_email ON compliance_group_members(email);

CREATE TABLE organization_settings (
    organization_id TEXT PRIMARY KEY,
    settings_type   TEXT,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL
);

CREATE TABLE compliance_files (
    file_id         TEXT PRIMARY KEY,
    filename        TEXT,
    mime_type       TEXT,
    size_bytes      INTEGER,
    md5             TEXT,
    created_at      TEXT,
    chat_ids_json   TEXT NOT NULL,
    message_ids_json TEXT NOT NULL,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL
);

CREATE TABLE generated_files (
    generated_file_id TEXT PRIMARY KEY,
    chat_id           TEXT,
    filename          TEXT,
    mime_type         TEXT,
    size_bytes        INTEGER,
    md5               TEXT,
    created_at        TEXT,
    raw               TEXT NOT NULL,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL
);

CREATE TABLE chat_artifact_versions (
    version_id      TEXT PRIMARY KEY,
    artifact_id     TEXT NOT NULL,
    chat_id         TEXT,
    title           TEXT,
    artifact_type   TEXT,
    size_bytes      INTEGER,
    md5             TEXT,
    created_at      TEXT,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL
);

CREATE TABLE project_attachments (
    project_id      TEXT NOT NULL,
    attachment_id   TEXT NOT NULL,
    attachment_type TEXT NOT NULL,
    filename        TEXT,
    mime_type       TEXT,
    size_bytes      INTEGER,
    md5             TEXT,
    created_at      TEXT,
    updated_at      TEXT,
    raw             TEXT NOT NULL,
    fetched_at      TEXT NOT NULL,
    seen_generation TEXT NOT NULL,
    PRIMARY KEY (project_id, attachment_id)
);

CREATE TABLE project_collaborators (
    project_id        TEXT NOT NULL,
    collaborator_type TEXT NOT NULL,
    principal_key     TEXT NOT NULL,
    role              TEXT NOT NULL,
    granted_at        TEXT,
    raw               TEXT NOT NULL,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL,
    PRIMARY KEY (project_id, collaborator_type, principal_key, role)
);

CREATE TABLE project_documents (
    document_id       TEXT PRIMARY KEY,
    project_id        TEXT,
    filename          TEXT,
    mime_type         TEXT,
    size_bytes        INTEGER,
    md5               TEXT,
    creator_user_id   TEXT,
    creator_email     TEXT,
    created_at        TEXT,
    metadata_raw      TEXT,
    content_text      TEXT,
    content_fetched_at TEXT,
    fetched_at        TEXT NOT NULL,
    seen_generation   TEXT NOT NULL
);

CREATE TABLE code_artifacts (
    artifact_id         TEXT PRIMARY KEY,
    organization_uuid   TEXT NOT NULL,
    owner_user_id        TEXT NOT NULL,
    owner_email          TEXT,
    published_version_id TEXT,
    read_mode             TEXT,
    updated_at            TEXT,
    raw                   TEXT NOT NULL,
    fetched_at            TEXT NOT NULL,
    seen_generation       TEXT NOT NULL
);

CREATE TABLE code_artifact_versions (
    artifact_id      TEXT NOT NULL,
    version_id       TEXT NOT NULL,
    name             TEXT,
    created_at       TEXT,
    raw              TEXT NOT NULL,
    fetched_at       TEXT NOT NULL,
    seen_generation TEXT NOT NULL,
    PRIMARY KEY (artifact_id, version_id)
);

CREATE TABLE content_objects (
    sha256         TEXT PRIMARY KEY,
    md5_hex        TEXT,
    size_bytes     INTEGER NOT NULL,
    mime_type      TEXT,
    storage_backend TEXT NOT NULL,
    object_key     TEXT NOT NULL,
    local_path     TEXT,
    created_at     TEXT NOT NULL,
    verified_md5   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE resource_content (
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    version_id    TEXT NOT NULL DEFAULT '',
    sha256        TEXT NOT NULL,
    fetched_at    TEXT NOT NULL,
    PRIMARY KEY (resource_type, resource_id, version_id),
    FOREIGN KEY (sha256) REFERENCES content_objects(sha256)
);

CREATE INDEX idx_resource_content_sha256 ON resource_content(sha256);
`
