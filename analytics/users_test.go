package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUserMetricsPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("date") != "2026-02-14" {
			t.Errorf("expected date=2026-02-14, got %q", q.Get("date"))
		}

		page++
		switch page {
		case 1:
			if q.Get("page") != "" {
				t.Errorf("page 1: expected no page param, got %q", q.Get("page"))
			}
			cursor := "cursor_page2"
			json.NewEncoder(w).Encode(UsersResponse{
				Data: []UserMetrics{
					{
						User: UserRef{ID: "u1", EmailAddress: "alice@example.com"},
						ChatMetrics: ChatMetrics{
							ConversationCount: 5,
							MessageCount:      20,
						},
					},
					{
						User: UserRef{ID: "u2", EmailAddress: "bob@example.com"},
						ClaudeCodeMetrics: ClaudeCodeMetrics{
							CoreMetrics: CoreMetrics{
								CommitCount:          3,
								DistinctSessionCount: 2,
							},
						},
					},
				},
				NextPage: &cursor,
			})
		case 2:
			if q.Get("page") != "cursor_page2" {
				t.Errorf("page 2: expected page=cursor_page2, got %q", q.Get("page"))
			}
			json.NewEncoder(w).Encode(UsersResponse{
				Data: []UserMetrics{
					{
						User:           UserRef{ID: "u3", EmailAddress: "carol@example.com"},
						WebSearchCount: 7,
					},
				},
				NextPage: nil,
			})
		default:
			t.Error("unexpected extra page request")
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	c := NewClient("key")
	c.baseURL = srv.URL

	metrics, err := c.FetchUserMetrics(context.Background(), "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 users, got %d", len(metrics))
	}
	if metrics[0].User.EmailAddress != "alice@example.com" {
		t.Errorf("expected alice, got %q", metrics[0].User.EmailAddress)
	}
	if metrics[1].ClaudeCodeMetrics.CoreMetrics.CommitCount != 3 {
		t.Error("bob should have 3 commits")
	}
	if metrics[2].WebSearchCount != 7 {
		t.Errorf("carol should have 7 web searches, got %d", metrics[2].WebSearchCount)
	}
}

func TestFetchUserMetricsSinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(UsersResponse{
			Data: []UserMetrics{
				{User: UserRef{ID: "u1", EmailAddress: "alice@example.com"}},
			},
			NextPage: nil,
		})
	}))
	defer srv.Close()

	c := NewClient("key")
	c.baseURL = srv.URL

	metrics, err := c.FetchUserMetrics(context.Background(), "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 user, got %d", len(metrics))
	}
}

func TestFetchUserMetricsEmptyPage(t *testing.T) {
	empty := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(UsersResponse{
			Data:     []UserMetrics{},
			NextPage: &empty,
		})
	}))
	defer srv.Close()

	c := NewClient("key")
	c.baseURL = srv.URL

	metrics, err := c.FetchUserMetrics(context.Background(), "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 0 {
		t.Fatalf("expected 0 users, got %d", len(metrics))
	}
}
