package store

import (
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

func TestInsertAndRetrieveUsers(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	now := time.Now().UTC()
	users := []compliance.User{
		{ID: "user_01", FullName: "Alice", EmailAddress: "Alice@Example.com", CreatedAt: "2025-06-01T00:00:00Z"},
		{ID: "user_02", FullName: "Bob", Email: "bob@example.com", CreatedAt: "2025-07-01T00:00:00Z"},
	}

	if err := s.InsertUsers(users, now); err != nil {
		t.Fatal(err)
	}

	cached, err := s.Users()
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cached))
	}

	// Emails should be lowercased.
	if cached[0].Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %q", cached[0].Email)
	}
}

func TestInsertUsersUpsert(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	users := []compliance.User{
		{ID: "user_01", FullName: "Alice Old", EmailAddress: "alice@example.com"},
	}
	if err := s.InsertUsers(users, t1); err != nil {
		t.Fatal(err)
	}

	// Update with new name.
	t2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	users[0].FullName = "Alice New"
	if err := s.InsertUsers(users, t2); err != nil {
		t.Fatal(err)
	}

	cached, _ := s.Users()
	if len(cached) != 1 {
		t.Fatalf("expected 1 user after upsert, got %d", len(cached))
	}
	if cached[0].FullName != "Alice New" {
		t.Errorf("expected updated name, got %q", cached[0].FullName)
	}
}

func TestUsersFetchedAt(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	// No users yet: zero time.
	ts, err := s.UsersFetchedAt()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	users := []compliance.User{
		{ID: "user_01", EmailAddress: "alice@example.com"},
	}
	if err := s.InsertUsers(users, now); err != nil {
		t.Fatal(err)
	}

	ts, err = s.UsersFetchedAt()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.Equal(now) {
		t.Errorf("expected %v, got %v", now, ts)
	}
}
