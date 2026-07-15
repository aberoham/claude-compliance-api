package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

func TestResourceCacheMigration(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != latestSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, latestSchemaVersion)
	}
	for _, table := range []string{
		"sync_runs", "resource_sync_state", "organizations", "organization_memberships",
		"compliance_roles", "compliance_role_permissions", "compliance_groups",
		"compliance_group_members", "organization_settings", "compliance_files",
		"generated_files", "chat_artifact_versions", "project_attachments",
		"project_collaborators", "project_documents", "code_artifacts",
		"code_artifact_versions", "content_objects", "resource_content",
	} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}

func TestOpenUpgradesUnversionedExistingCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, email, fetched_at) VALUES ('user-1', 'user@example.com', '2026-07-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version, users int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id='user-1'").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if version != latestSchemaVersion || users != 1 {
		t.Fatalf("version=%d users=%d", version, users)
	}
	if _, err := s.db.Exec(`UPDATE users SET raw='{}' WHERE id='user-1'`); err != nil {
		t.Fatalf("new raw column unavailable: %v", err)
	}
}

func TestResourceSyncResumesGenerationThenPrunesFreshSnapshot(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()
	now := time.Now().UTC()

	first, err := s.BeginResourceSync("organizations", "", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertOrganizations([]compliance.Organization{{UUID: "org-a", Name: "A", CreatedAt: now.Format(time.RFC3339)}}, first.Generation, now); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckpointResourceSync(first, "next-page", 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.FailResourceSync(first, errors.New("interrupted")); err != nil {
		t.Fatal(err)
	}

	resumed, err := s.BeginResourceSync("organizations", "", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Cursor != "next-page" || resumed.Generation != first.Generation || resumed.Mode != "resume" {
		t.Fatalf("resume = %#v, first generation %q", resumed, first.Generation)
	}
	if _, err := s.UpsertOrganizations([]compliance.Organization{{UUID: "org-b", Name: "B", CreatedAt: now.Format(time.RFC3339)}}, resumed.Generation, now); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckpointResourceSync(resumed, "", 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteResourceSync(resumed, true); err != nil {
		t.Fatal(err)
	}
	values, err := s.CachedOrganizations()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("resumed snapshot has %d organizations, want 2", len(values))
	}

	fresh, err := s.BeginResourceSync("organizations", "", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Generation == resumed.Generation || fresh.Cursor != "" {
		t.Fatalf("fresh run = %#v", fresh)
	}
	if _, err := s.UpsertOrganizations([]compliance.Organization{{UUID: "org-b", Name: "B2", CreatedAt: now.Format(time.RFC3339)}}, fresh.Generation, now); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckpointResourceSync(fresh, "", 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteResourceSync(fresh, true); err != nil {
		t.Fatal(err)
	}
	values, err = s.CachedOrganizations()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].UUID != "org-b" || values[0].Name != "B2" {
		t.Fatalf("pruned snapshot = %#v", values)
	}
}

func TestLinkContentObject(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()
	object := ContentObjectRecord{
		SHA256: "abc", MD5Hex: "def", SizeBytes: 12, MimeType: "application/octet-stream",
		Backend: "local", ObjectKey: "ab/cd/abc", LocalPath: "/tmp/object", VerifiedMD5: true,
	}
	if err := s.LinkContentObject(object, "file", "file-1", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	exists, err := s.HasResourceContent("file", "file-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("content link not found")
	}
	var backend, key string
	if err := s.db.QueryRow(`SELECT storage_backend, object_key FROM content_objects WHERE sha256='abc'`).Scan(&backend, &key); err != nil {
		t.Fatal(err)
	}
	if backend != "local" || key != "ab/cd/abc" {
		t.Fatalf("backend/key = %q/%q", backend, key)
	}
}
