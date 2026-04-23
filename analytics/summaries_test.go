package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSummaries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("starting_date") != "2026-02-01" {
			t.Errorf("expected starting_date=2026-02-01, got %q", q.Get("starting_date"))
		}
		if q.Get("ending_date") != "2026-02-03" {
			t.Errorf("expected ending_date=2026-02-03, got %q", q.Get("ending_date"))
		}

		json.NewEncoder(w).Encode(SummariesResponse{
			Summaries: []DailySummary{
				{
					StartingAt:             "2026-02-01T00:00:00Z",
					EndingAt:               "2026-02-02T00:00:00Z",
					DailyActiveUserCount:   23,
					WeeklyActiveUserCount:  34,
					MonthlyActiveUserCount: 42,
					AssignedSeatCount:      85,
					PendingInviteCount:     3,
				},
				{
					StartingAt:             "2026-02-02T00:00:00Z",
					EndingAt:               "2026-02-03T00:00:00Z",
					DailyActiveUserCount:   21,
					WeeklyActiveUserCount:  33,
					MonthlyActiveUserCount: 41,
					AssignedSeatCount:      85,
					PendingInviteCount:     3,
				},
				{
					StartingAt:             "2026-02-03T00:00:00Z",
					EndingAt:               "2026-02-04T00:00:00Z",
					DailyActiveUserCount:   25,
					WeeklyActiveUserCount:  35,
					MonthlyActiveUserCount: 43,
					AssignedSeatCount:      86,
					PendingInviteCount:     2,
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient("key")
	c.baseURL = srv.URL

	summaries, err := c.FetchSummaries(context.Background(), "2026-02-01", "2026-02-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}
	if summaries[0].DailyActiveUserCount != 23 {
		t.Errorf("day 1: expected 23 DAU, got %d", summaries[0].DailyActiveUserCount)
	}
	if summaries[0].Date() != "2026-02-01" {
		t.Errorf("day 1: expected date 2026-02-01, got %q", summaries[0].Date())
	}
	if summaries[2].AssignedSeatCount != 86 {
		t.Errorf("day 3: expected 86 seats, got %d", summaries[2].AssignedSeatCount)
	}
}

func TestFetchSummariesEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(SummariesResponse{Summaries: []DailySummary{}})
	}))
	defer srv.Close()

	c := NewClient("key")
	c.baseURL = srv.URL

	summaries, err := c.FetchSummaries(context.Background(), "2026-02-01", "2026-02-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
	}
}
