package compliance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchActivitiesPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			json.NewEncoder(w).Encode(ActivitiesResponse{
				Data: []Activity{
					{ID: "a1", CreatedAt: "2026-01-10T00:00:00Z", Type: "claude_chat_created", Actor: Actor{Type: "user_actor"}},
					{ID: "a2", CreatedAt: "2026-01-09T00:00:00Z", Type: "claude_chat_viewed", Actor: Actor{Type: "user_actor"}},
				},
				HasMore: true,
				FirstID: "a1",
				LastID:  "a2",
			})
		case 2:
			// Verify after_id was sent.
			if r.URL.Query().Get("after_id") != "a2" {
				t.Errorf("expected after_id=a2, got %q", r.URL.Query().Get("after_id"))
			}
			json.NewEncoder(w).Encode(ActivitiesResponse{
				Data: []Activity{
					{ID: "a3", CreatedAt: "2026-01-08T00:00:00Z", Type: "claude_file_uploaded", Actor: Actor{Type: "user_actor"}},
				},
				HasMore: false,
				FirstID: "a3",
				LastID:  "a3",
			})
		default:
			t.Error("unexpected extra page request")
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c := NewClient("key", "org-1")
	c.baseURL = srv.URL

	activities, err := c.FetchActivities(context.Background(), ActivityQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(activities))
	}
	if activities[0].ID != "a1" || activities[2].ID != "a3" {
		t.Error("activities not in expected order")
	}
}

func TestFetchActivitiesFiltersComplianceEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ActivitiesResponse{
			Data: []Activity{
				{ID: "a1", CreatedAt: "2026-01-10T00:00:00Z", Type: "claude_chat_created", Actor: Actor{Type: "user_actor"}},
				{ID: "a2", CreatedAt: "2026-01-10T00:00:00Z", Type: "compliance_api_accessed", Actor: Actor{Type: "api_actor"}},
				{ID: "a3", CreatedAt: "2026-01-09T00:00:00Z", Type: "claude_file_viewed", Actor: Actor{Type: "user_actor"}},
			},
			HasMore: false,
		})
	}))
	defer srv.Close()

	c := NewClient("key", "org-1")
	c.baseURL = srv.URL

	activities, err := c.FetchActivities(context.Background(), ActivityQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 {
		t.Fatalf("expected 2 activities (compliance_api_accessed filtered), got %d", len(activities))
	}
	for _, a := range activities {
		if a.Type == "compliance_api_accessed" {
			t.Error("compliance_api_accessed should have been filtered out")
		}
	}
}

func TestFetchActivitiesQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("created_at.gte") == "" {
			t.Error("expected created_at.gte to be set")
		}
		if q.Get("organization_ids[]") != "org-test" {
			t.Errorf("expected organization_ids[]=org-test, got %q", q.Get("organization_ids[]"))
		}
		json.NewEncoder(w).Encode(ActivitiesResponse{HasMore: false})
	}))
	defer srv.Close()

	c := NewClient("key", "org-test")
	c.baseURL = srv.URL

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.FetchActivities(context.Background(), ActivityQuery{CreatedAtGte: &since})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFetchActivitiesForwardPagination(t *testing.T) {
	page := 0
	var sawAfterID bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("after_id") != "" {
			sawAfterID = true
		}

		page++
		switch page {
		case 1:
			if q.Get("before_id") != "hwm" {
				t.Errorf("page 1: expected before_id=hwm, got %q",
					q.Get("before_id"))
			}
			json.NewEncoder(w).Encode(ActivitiesResponse{
				Data: []Activity{
					{ID: "f1", CreatedAt: "2026-01-12T00:00:00Z",
						Type: "claude_chat_created", Actor: Actor{Type: "user_actor"}},
					{ID: "f2", CreatedAt: "2026-01-11T00:00:00Z",
						Type: "claude_chat_viewed", Actor: Actor{Type: "user_actor"}},
				},
				HasMore: true,
				FirstID: "cursor_f1",
				LastID:  "cursor_l1",
			})
		case 2:
			if q.Get("before_id") != "cursor_f1" {
				t.Errorf("page 2: expected before_id=cursor_f1, got %q",
					q.Get("before_id"))
			}
			json.NewEncoder(w).Encode(ActivitiesResponse{
				Data: []Activity{
					{ID: "f3", CreatedAt: "2026-01-13T00:00:00Z",
						Type: "claude_file_uploaded", Actor: Actor{Type: "user_actor"}},
				},
				HasMore: false,
				FirstID: "cursor_f2",
				LastID:  "cursor_l2",
			})
		default:
			t.Error("unexpected extra page request")
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c := NewClient("key", "org-1")
	c.baseURL = srv.URL

	activities, err := c.FetchActivities(context.Background(), ActivityQuery{
		BeforeID: "hwm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(activities))
	}
	if activities[0].ID != "f1" || activities[1].ID != "f2" || activities[2].ID != "f3" {
		t.Errorf("unexpected activity order: %s, %s, %s",
			activities[0].ID, activities[1].ID, activities[2].ID)
	}
	if sawAfterID {
		t.Error("forward pagination must never send after_id")
	}
}

func TestSummarizeByUser(t *testing.T) {
	email1 := "alice@example.com"
	email2 := "bob@example.com"
	uid1 := "user_01"

	activities := []Activity{
		{ID: "a1", CreatedAt: "2026-01-10T10:00:00Z", Type: "claude_chat_created",
			Actor: Actor{Type: "user_actor", EmailAddress: &email1, UserID: &uid1}},
		{ID: "a2", CreatedAt: "2026-01-10T14:00:00Z", Type: "claude_chat_viewed",
			Actor: Actor{Type: "user_actor", EmailAddress: &email1, UserID: &uid1}},
		{ID: "a3", CreatedAt: "2026-01-11T09:00:00Z", Type: "claude_chat_created",
			Actor: Actor{Type: "user_actor", EmailAddress: &email2}},
		{ID: "a4", CreatedAt: "2026-01-05T08:00:00Z", Type: "sso_login_succeeded",
			Actor: Actor{Type: "user_actor", EmailAddress: &email1, UserID: &uid1}},
	}

	summaries := SummarizeByUser(activities)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 users, got %d", len(summaries))
	}

	alice := summaries["alice@example.com"]
	if alice == nil {
		t.Fatal("missing alice")
	}
	if alice.EventCount != 3 {
		t.Errorf("alice: expected 3 events, got %d", alice.EventCount)
	}
	if alice.UserID != "user_01" {
		t.Errorf("alice: expected user_01, got %q", alice.UserID)
	}
	if len(alice.ActiveDays) != 2 {
		t.Errorf("alice: expected 2 active days, got %d", len(alice.ActiveDays))
	}
	if alice.EventTypes["claude_chat_created"] != 1 {
		t.Error("alice: expected 1 claude_chat_created")
	}

	expected := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	if !alice.FirstSeen.Equal(expected) {
		t.Errorf("alice: expected FirstSeen=%v, got %v", expected, alice.FirstSeen)
	}

	bob := summaries["bob@example.com"]
	if bob == nil {
		t.Fatal("missing bob")
	}
	if bob.EventCount != 1 {
		t.Errorf("bob: expected 1 event, got %d", bob.EventCount)
	}
}

func TestSummarizeByUserSkipsActorsWithoutEmail(t *testing.T) {
	activities := []Activity{
		{ID: "a1", CreatedAt: "2026-01-10T00:00:00Z", Type: "compliance_api_accessed",
			Actor: Actor{Type: "api_actor"}},
	}
	summaries := SummarizeByUser(activities)
	if len(summaries) != 0 {
		t.Logf("%v", summaries)
		t.Errorf("expected 0 summaries for actors without email, got %d", len(summaries))
	}
}
