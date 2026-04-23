package okta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testWindow covers all test event dates (March 2026).
var (
	testSince = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	testUntil = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
)

// testClient returns a Client whose baseURL points at the test server.
func testClient(srvURL string) *Client {
	c := NewClient("token", "test.okta.com")
	c.baseURLOverride = srvURL
	return c
}

func TestSuccessOnlyFiltering(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "alice@example.com"},
			Target: []Target{
				{Type: "AppInstance", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
		{
			UUID:      "evt-2",
			Published: time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "bob@example.com"},
			Target: []Target{
				{Type: "AppInstance", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "FAILURE"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"", defaultClaudeAppName)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 matched event, got %d", len(matched))
	}
	if matched[0].Actor.AlternateID != "alice@example.com" {
		t.Errorf("expected alice, got %q", matched[0].Actor.AlternateID)
	}
}

func TestAppIDPreferredOverName(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "alice@example.com"},
			Target: []Target{
				{Type: "AppInstance", ID: "app-claude-123", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
		{
			UUID:      "evt-2",
			Published: time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "bob@example.com"},
			Target: []Target{
				{Type: "AppInstance", ID: "app-other-456", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"app-claude-123", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 matched by ID, got %d", len(matched))
	}
	if matched[0].UUID != "evt-1" {
		t.Errorf("expected evt-1, got %q", matched[0].UUID)
	}
}

func TestFallbackToAppName(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "alice@example.com"},
			Target: []Target{
				{Type: "AppInstance", ID: "app-123", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
		{
			UUID:      "evt-2",
			Published: time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "bob@example.com"},
			Target: []Target{
				{Type: "AppInstance", ID: "app-456", DisplayName: "Other App"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"", "Anthropic Claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 matched by name, got %d", len(matched))
	}
	if matched[0].UUID != "evt-1" {
		t.Errorf("expected evt-1, got %q", matched[0].UUID)
	}
}

func TestEmailNormalization(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "Alice@Example.COM"},
			Target: []Target{
				{Type: "AppInstance", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"", defaultClaudeAppName)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatal("expected 1 match")
	}
	if matched[0].Actor.AlternateID != "alice@example.com" {
		t.Errorf("expected lowercased email, got %q",
			matched[0].Actor.AlternateID)
	}
}

func TestEmptyActorSkipped(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: ""},
			Target: []Target{
				{Type: "AppInstance", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"", defaultClaudeAppName)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 0 {
		t.Errorf("expected 0 matches (empty actor), got %d", len(matched))
	}
}

func TestFetchConstructsBoundedQuery(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			json.NewEncoder(w).Encode([]LogEvent{})
		}))
	defer srv.Close()

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	_, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), since, until,
		"", defaultClaudeAppName)
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/v1/logs" {
		t.Errorf("expected /api/v1/logs, got %q", gotPath)
	}
	checks := []string{
		"filter=eventType",
		"since=2026-01-01T00%3A00%3A00Z",
		"until=2026-04-01T00%3A00%3A00Z",
		"limit=1000",
		"sortOrder=ASCENDING",
	}
	for _, want := range checks {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query missing %q in %q", want, gotQuery)
		}
	}
}

func TestMatchedTargetPropagated(t *testing.T) {
	events := []LogEvent{
		{
			UUID:      "evt-1",
			Published: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
			EventType: "user.authentication.sso",
			Actor:     Actor{AlternateID: "alice@example.com"},
			Target: []Target{
				{Type: "AppUser", ID: "user-123", DisplayName: "Alice"},
				{Type: "AppInstance", ID: "app-other", DisplayName: "Other App"},
				{Type: "AppInstance", ID: "app-claude", DisplayName: "Anthropic Claude"},
			},
			Outcome: Outcome{Result: "SUCCESS"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(events)
		}))
	defer srv.Close()

	matched, err := testClient(srv.URL).FetchClaudeSSOEvents(
		context.Background(), testSince, testUntil,
		"app-claude", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].MatchedAppID != "app-claude" {
		t.Errorf("expected matched app ID app-claude, got %q",
			matched[0].MatchedAppID)
	}
	if matched[0].MatchedAppName != "Anthropic Claude" {
		t.Errorf("expected matched app name Anthropic Claude, got %q",
			matched[0].MatchedAppName)
	}
}
