package store

import (
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/okta"
)

func TestInsertAndQueryOktaSSO(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	events := []okta.LogEvent{
		{
			UUID:           "evt-1",
			Published:      time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "alice@example.com"},
			MatchedAppID:   "app-123",
			MatchedAppName: "Anthropic Claude",
		},
		{
			UUID:           "evt-2",
			Published:      time.Date(2026, 3, 15, 14, 0, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "alice@example.com"},
			MatchedAppID:   "app-123",
			MatchedAppName: "Anthropic Claude",
		},
		{
			UUID:           "evt-3",
			Published:      time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "bob@example.com"},
			MatchedAppID:   "app-123",
			MatchedAppName: "Anthropic Claude",
		},
	}

	now := time.Now().UTC()
	n, err := s.InsertOktaSSOEvents(events, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 inserted, got %d", n)
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	summaries, err := s.OktaSSOSummaries(since, until)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 user summaries, got %d", len(summaries))
	}

	alice := summaries["alice@example.com"]
	if alice.EventCount != 2 {
		t.Errorf("expected 2 events for alice, got %d", alice.EventCount)
	}
	if alice.FirstSSO[:10] != "2026-03-10" {
		t.Errorf("expected first SSO 2026-03-10, got %q", alice.FirstSSO)
	}
	if alice.LastSSO[:10] != "2026-03-15" {
		t.Errorf("expected last SSO 2026-03-15, got %q", alice.LastSSO)
	}

	bob := summaries["bob@example.com"]
	if bob.EventCount != 1 {
		t.Errorf("expected 1 event for bob, got %d", bob.EventCount)
	}
}

func TestOktaUpsertIdempotency(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	event := okta.LogEvent{
		UUID:           "evt-1",
		Published:      time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Actor:          okta.Actor{AlternateID: "alice@example.com"},
		MatchedAppID:   "app-123",
		MatchedAppName: "Anthropic Claude",
	}

	now := time.Now().UTC()
	if _, err := s.InsertOktaSSOEvents([]okta.LogEvent{event}, now); err != nil {
		t.Fatal(err)
	}
	// Insert same event again — should not duplicate.
	if _, err := s.InsertOktaSSOEvents([]okta.LogEvent{event}, now); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	summaries, err := s.OktaSSOSummaries(since, until)
	if err != nil {
		t.Fatal(err)
	}
	if summaries["alice@example.com"].EventCount != 1 {
		t.Errorf("expected 1 after upsert, got %d",
			summaries["alice@example.com"].EventCount)
	}
}

func TestOktaEmailNormalizedAtStorageBoundary(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	event := okta.LogEvent{
		UUID:           "evt-1",
		Published:      time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Actor:          okta.Actor{AlternateID: "Alice@Example.COM"},
		MatchedAppName: "Anthropic Claude",
	}

	now := time.Now().UTC()
	if _, err := s.InsertOktaSSOEvents([]okta.LogEvent{event}, now); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	summaries, err := s.OktaSSOSummaries(since, until)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := summaries["alice@example.com"]; !ok {
		t.Fatal("expected summary keyed by lowercase email")
	}
	if _, ok := summaries["Alice@Example.COM"]; ok {
		t.Error("should not have mixed-case key in summaries")
	}
}

func TestOktaMatchedTargetPersisted(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	// Event has two AppInstance targets. The filter matched the second
	// one (app-claude), so MatchedAppID/MatchedAppName reflect that.
	event := okta.LogEvent{
		UUID:      "evt-1",
		Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		Actor:     okta.Actor{AlternateID: "alice@example.com"},
		Target: []okta.Target{
			{Type: "AppInstance", ID: "app-other", DisplayName: "Other App"},
			{Type: "AppInstance", ID: "app-claude", DisplayName: "Anthropic Claude"},
		},
		MatchedAppID:   "app-claude",
		MatchedAppName: "Anthropic Claude",
	}

	now := time.Now().UTC()
	if _, err := s.InsertOktaSSOEvents([]okta.LogEvent{event}, now); err != nil {
		t.Fatal(err)
	}

	// Verify the stored app_instance_id is the matched one, not
	// the first AppInstance target.
	var storedAppID, storedAppName string
	err := s.db.QueryRow(
		"SELECT app_instance_id, app_name FROM okta_sso_events WHERE event_id = ?",
		"evt-1",
	).Scan(&storedAppID, &storedAppName)
	if err != nil {
		t.Fatal(err)
	}
	if storedAppID != "app-claude" {
		t.Errorf("expected app-claude, got %q", storedAppID)
	}
	if storedAppName != "Anthropic Claude" {
		t.Errorf("expected Anthropic Claude, got %q", storedAppName)
	}
}

func TestOktaTimestampBoundaryFiltering(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	events := []okta.LogEvent{
		{
			UUID:           "evt-before",
			Published:      time.Date(2026, 2, 28, 23, 59, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "alice@example.com"},
			MatchedAppName: "Anthropic Claude",
		},
		{
			UUID:           "evt-inside",
			Published:      time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "alice@example.com"},
			MatchedAppName: "Anthropic Claude",
		},
		{
			UUID:           "evt-after",
			Published:      time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			Actor:          okta.Actor{AlternateID: "alice@example.com"},
			MatchedAppName: "Anthropic Claude",
		},
	}

	now := time.Now().UTC()
	if _, err := s.InsertOktaSSOEvents(events, now); err != nil {
		t.Fatal(err)
	}

	// Query [March 1, April 1) — should only include evt-inside.
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	summaries, err := s.OktaSSOSummaries(since, until)
	if err != nil {
		t.Fatal(err)
	}
	if summaries["alice@example.com"].EventCount != 1 {
		t.Errorf("expected 1 event in range, got %d",
			summaries["alice@example.com"].EventCount)
	}
}
