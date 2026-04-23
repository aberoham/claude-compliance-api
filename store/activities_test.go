package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

func makeActivity(id, createdAt, typ, email string) compliance.Activity {
	a := compliance.Activity{
		ID:        id,
		CreatedAt: createdAt,
		Type:      typ,
		Actor:     compliance.Actor{Type: "user_actor"},
	}
	if email != "" {
		a.Actor.EmailAddress = &email
	}
	return a
}

func TestInsertAndQueryActivities(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	activities := []compliance.Activity{
		makeActivity("a1", "2026-01-10T10:00:00Z", "claude_chat_created", "alice@example.com"),
		makeActivity("a2", "2026-01-10T14:00:00Z", "claude_chat_viewed", "alice@example.com"),
		makeActivity("a3", "2026-01-11T09:00:00Z", "claude_file_uploaded", "bob@example.com"),
	}

	inserted, err := s.InsertActivities(activities)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 3 {
		t.Errorf("expected 3 inserted, got %d", inserted)
	}

	// Query all.
	all, err := s.Activities(QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(all))
	}

	// Query by email.
	aliceOnly, err := s.Activities(QueryOpts{Email: "alice@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceOnly) != 2 {
		t.Errorf("expected 2 alice activities, got %d", len(aliceOnly))
	}

	// Query by type.
	viewsOnly, err := s.Activities(QueryOpts{Type: "claude_chat_viewed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(viewsOnly) != 1 {
		t.Errorf("expected 1 view activity, got %d", len(viewsOnly))
	}
}

func TestInsertIdempotency(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	a := makeActivity("dup-1", "2026-01-10T00:00:00Z", "claude_chat_created", "alice@example.com")

	n1, err := s.InsertActivities([]compliance.Activity{a})
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 {
		t.Errorf("first insert: expected 1, got %d", n1)
	}

	n2, err := s.InsertActivities([]compliance.Activity{a})
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("duplicate insert: expected 0, got %d", n2)
	}

	count, _ := s.ActivityCount()
	if count != 1 {
		t.Errorf("expected 1 total activity, got %d", count)
	}
}

func TestHighWaterMark(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	hwm, err := s.HighWaterMark()
	if err != nil {
		t.Fatal(err)
	}
	if hwm != "" {
		t.Errorf("expected empty initial HWM, got %q", hwm)
	}

	if err := s.SetHighWaterMark("activity_999"); err != nil {
		t.Fatal(err)
	}

	hwm, err = s.HighWaterMark()
	if err != nil {
		t.Fatal(err)
	}
	if hwm != "activity_999" {
		t.Errorf("expected activity_999, got %q", hwm)
	}

	// Overwrite.
	if err := s.SetHighWaterMark("activity_1000"); err != nil {
		t.Fatal(err)
	}
	hwm, _ = s.HighWaterMark()
	if hwm != "activity_1000" {
		t.Errorf("expected activity_1000 after overwrite, got %q", hwm)
	}
}

func TestQueryDateRange(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	activities := []compliance.Activity{
		makeActivity("a1", "2026-01-05T10:00:00Z", "claude_chat_created", "alice@example.com"),
		makeActivity("a2", "2026-01-10T14:00:00Z", "claude_chat_viewed", "alice@example.com"),
		makeActivity("a3", "2026-01-15T09:00:00Z", "claude_file_uploaded", "bob@example.com"),
	}
	if _, err := s.InsertActivities(activities); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)

	results, err := s.Activities(QueryOpts{Since: &since, Until: &until})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result in date range, got %d", len(results))
	}
	if results[0].ID != "a2" {
		t.Errorf("expected a2, got %s", results[0].ID)
	}
}

func TestUserSummaries(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	activities := []compliance.Activity{
		makeActivity("a1", "2026-01-10T10:00:00Z", "claude_chat_created", "alice@example.com"),
		makeActivity("a2", "2026-01-10T14:00:00Z", "claude_chat_viewed", "alice@example.com"),
		makeActivity("a3", "2026-01-11T09:00:00Z", "claude_chat_created", "alice@example.com"),
		makeActivity("a4", "2026-01-10T12:00:00Z", "claude_file_uploaded", "bob@example.com"),
	}
	if _, err := s.InsertActivities(activities); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	summaries, err := s.UserSummaries(since)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 users, got %d", len(summaries))
	}

	// Summaries are ordered by event_count DESC.
	if summaries[0].Email != "alice@example.com" {
		t.Errorf("expected alice first, got %s", summaries[0].Email)
	}
	if summaries[0].EventCount != 3 {
		t.Errorf("alice: expected 3 events, got %d", summaries[0].EventCount)
	}
	if summaries[0].ActiveDays != 2 {
		t.Errorf("alice: expected 2 active days, got %d", summaries[0].ActiveDays)
	}
	if summaries[0].EventTypes != 2 {
		t.Errorf("alice: expected 2 event types, got %d", summaries[0].EventTypes)
	}
	if summaries[0].ChatsCreated != 2 {
		t.Errorf("alice: expected 2 chats created, got %d", summaries[0].ChatsCreated)
	}
	if summaries[1].ChatsCreated != 0 {
		t.Errorf("bob: expected 0 chats created, got %d", summaries[1].ChatsCreated)
	}
}

func TestEventTypeCounts(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	activities := []compliance.Activity{
		makeActivity("a1", "2026-01-10T10:00:00Z", "claude_chat_created", "alice@example.com"),
		makeActivity("a2", "2026-01-10T14:00:00Z", "claude_chat_created", "bob@example.com"),
		makeActivity("a3", "2026-01-10T15:00:00Z", "claude_file_viewed", "alice@example.com"),
	}
	if _, err := s.InsertActivities(activities); err != nil {
		t.Fatal(err)
	}

	counts, err := s.EventTypeCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["claude_chat_created"] != 2 {
		t.Errorf("expected 2 chat_created, got %d", counts["claude_chat_created"])
	}
	if counts["claude_file_viewed"] != 1 {
		t.Errorf("expected 1 file_viewed, got %d", counts["claude_file_viewed"])
	}
}

func TestStoredActivityPreservesRawJSON(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	// Create an activity with extra fields.
	raw := `{"id":"a1","created_at":"2026-01-10T00:00:00Z","type":"claude_file_uploaded","actor":{"type":"user_actor","email_address":"alice@example.com"},"filename":"report.pdf","claude_file_id":"file_01"}`
	var a compliance.Activity
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}

	if _, err := s.InsertActivities([]compliance.Activity{a}); err != nil {
		t.Fatal(err)
	}

	results, err := s.Activities(QueryOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}

	// The stored activity should preserve the extra fields via raw JSON.
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(results[0].Extra, &extra); err != nil {
		t.Fatal(err)
	}
	if _, ok := extra["filename"]; !ok {
		t.Error("filename lost in storage round-trip")
	}
}
