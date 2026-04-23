package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDBAndSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Verify tables exist by running a query against each.
	for _, table := range []string{"activities", "sync_state", "users"} {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}

func TestOpenCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

func TestReset(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	if err := s.SetHighWaterMark("hwm-1"); err != nil {
		t.Fatal(err)
	}

	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}

	hwm, err := s.HighWaterMark()
	if err != nil {
		t.Fatal(err)
	}
	if hwm != "" {
		t.Errorf("expected empty high water mark after reset, got %q", hwm)
	}
}

func openTestDB(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
