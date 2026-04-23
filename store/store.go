package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database that persists activities, sync state, and
// cached user data. All writes use upsert semantics so repeated inserts of
// the same activity are idempotent.
type Store struct {
	db   *sql.DB
	path string
}

const schema = `
CREATE TABLE IF NOT EXISTS activities (
    id          TEXT PRIMARY KEY,
    created_at  TEXT NOT NULL,
    type        TEXT NOT NULL,
    actor_email TEXT,
    actor_id    TEXT,
    actor_type  TEXT NOT NULL,
    org_id      TEXT,
    raw         TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities(created_at);
CREATE INDEX IF NOT EXISTS idx_activities_actor_email ON activities(actor_email);
CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(type);

CREATE TABLE IF NOT EXISTS sync_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    full_name  TEXT,
    created_at TEXT,
    fetched_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS projects (
    id             TEXT PRIMARY KEY,
    name           TEXT,
    description    TEXT,
    instructions   TEXT,
    creator_id     TEXT,
    creator_email  TEXT,
    org_id         TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    archived_at    TEXT,
    fetched_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_projects_creator_id ON projects(creator_id);
CREATE INDEX IF NOT EXISTS idx_projects_creator_email ON projects(creator_email);

CREATE TABLE IF NOT EXISTS chats (
    id             TEXT PRIMARY KEY,
    name           TEXT,
    user_id        TEXT,
    user_email     TEXT,
    project_id     TEXT,
    org_id         TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    deleted_at     TEXT,
    fetched_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chats_user_email ON chats(user_email);
CREATE INDEX IF NOT EXISTS idx_chats_created_at ON chats(created_at);
CREATE INDEX IF NOT EXISTS idx_chats_project_id ON chats(project_id);

CREATE TABLE IF NOT EXISTS classifications (
    message_id       TEXT PRIMARY KEY,
    chat_id          TEXT NOT NULL,
    user_email       TEXT NOT NULL,
    message_created  TEXT NOT NULL,
    work_related     INTEGER,
    intent           TEXT,
    topic_fine       TEXT,
    topic_coarse     TEXT,
    classified_at    TEXT NOT NULL,
    classifier_model TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_classifications_chat_id ON classifications(chat_id);
CREATE INDEX IF NOT EXISTS idx_classifications_user_email ON classifications(user_email);
CREATE INDEX IF NOT EXISTS idx_classifications_message_created ON classifications(message_created);

CREATE TABLE IF NOT EXISTS analytics_user_daily (
    user_email        TEXT NOT NULL,
    date              TEXT NOT NULL,
    user_id           TEXT NOT NULL,
    conversations     INTEGER NOT NULL DEFAULT 0,
    messages          INTEGER NOT NULL DEFAULT 0,
    projects_created  INTEGER NOT NULL DEFAULT 0,
    projects_used     INTEGER NOT NULL DEFAULT 0,
    files_uploaded    INTEGER NOT NULL DEFAULT 0,
    artifacts_created INTEGER NOT NULL DEFAULT 0,
    thinking_messages INTEGER NOT NULL DEFAULT 0,
    skills_used       INTEGER NOT NULL DEFAULT 0,
    connectors_used   INTEGER NOT NULL DEFAULT 0,
    cc_commits        INTEGER NOT NULL DEFAULT 0,
    cc_pull_requests  INTEGER NOT NULL DEFAULT 0,
    cc_lines_added    INTEGER NOT NULL DEFAULT 0,
    cc_lines_removed  INTEGER NOT NULL DEFAULT 0,
    cc_sessions       INTEGER NOT NULL DEFAULT 0,
    web_searches      INTEGER NOT NULL DEFAULT 0,
    fetched_at        TEXT NOT NULL,
    PRIMARY KEY (user_email, date)
);

CREATE INDEX IF NOT EXISTS idx_aud_date ON analytics_user_daily(date);

CREATE TABLE IF NOT EXISTS analytics_org_daily (
    date                 TEXT PRIMARY KEY,
    daily_active_users   INTEGER NOT NULL,
    weekly_active_users  INTEGER NOT NULL,
    monthly_active_users INTEGER NOT NULL,
    assigned_seats       INTEGER NOT NULL,
    pending_invites      INTEGER NOT NULL,
    fetched_at           TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS okta_sso_events (
    event_id        TEXT PRIMARY KEY,
    actor_email     TEXT NOT NULL,
    published       TEXT NOT NULL,
    app_instance_id TEXT,
    app_name        TEXT NOT NULL,
    fetched_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okta_sso_actor ON okta_sso_events(actor_email);
CREATE INDEX IF NOT EXISTS idx_okta_sso_published ON okta_sso_events(published);
`

// DefaultPath returns the conventional database path under the user's local
// data directory (~/.local/share/claude-audit/audit.db).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share", "claude-audit", "audit.db")
}

// Open creates or opens the SQLite database at path, applying the schema if
// needed. Parent directories are created automatically.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Wait up to 10s for a lock before returning SQLITE_BUSY. WAL serializes
	// writers, so two concurrent invocations (e.g. `fetch` and `analytics-users`
	// run side by side) would otherwise fail immediately on contended commits.
	if _, err := db.Exec("PRAGMA busy_timeout=10000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}

	return &Store{db: db, path: path}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the filesystem path of the database.
func (s *Store) Path() string {
	return s.path
}

// Reset drops and recreates the activities table, clearing all cached activity
// data. Sync state is also reset. User cache is left intact.
func (s *Store) Reset() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DROP TABLE IF EXISTS activities"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sync_state"); err != nil {
		return err
	}
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	return tx.Commit()
}
