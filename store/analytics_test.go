package store

import (
	"testing"
	"time"

	"github.com/aberoham/claude-compliance-api/analytics"
)

func TestInsertAndQueryUserDailyMetrics(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	metrics := []analytics.UserMetrics{
		{
			User: analytics.UserRef{ID: "u1", EmailAddress: "Alice@Example.com"},
			ChatMetrics: analytics.ChatMetrics{
				ConversationCount: 5,
				MessageCount:      20,
			},
			ClaudeCodeMetrics: analytics.ClaudeCodeMetrics{
				CoreMetrics: analytics.CoreMetrics{
					CommitCount:          3,
					DistinctSessionCount: 2,
					LinesOfCode:          analytics.LinesOfCode{AddedCount: 100},
				},
			},
			WebSearchCount: 4,
		},
		{
			User: analytics.UserRef{ID: "u2", EmailAddress: "bob@example.com"},
			ChatMetrics: analytics.ChatMetrics{
				ConversationCount: 2,
				MessageCount:      8,
			},
		},
	}

	now := time.Now().UTC()
	n, err := s.InsertUserDailyMetrics(metrics, "2026-02-14", now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 inserted, got %d", n)
	}

	summaries, err := s.AnalyticsUserSummaries("2026-02-14", "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// Summaries ordered by messages DESC, so alice (20) comes first.
	alice := summaries[0]
	if alice.Email != "alice@example.com" {
		t.Errorf("expected alice, got %q", alice.Email)
	}
	if alice.Conversations != 5 {
		t.Errorf("expected 5 conversations, got %d", alice.Conversations)
	}
	if alice.Messages != 20 {
		t.Errorf("expected 20 messages, got %d", alice.Messages)
	}
	if alice.CCCommits != 3 {
		t.Errorf("expected 3 commits, got %d", alice.CCCommits)
	}
	if alice.CCSessions != 2 {
		t.Errorf("expected 2 sessions, got %d", alice.CCSessions)
	}
	if alice.WebSearches != 4 {
		t.Errorf("expected 4 web searches, got %d", alice.WebSearches)
	}
	if alice.ActiveDays != 1 {
		t.Errorf("expected 1 active day, got %d", alice.ActiveDays)
	}
}

func TestUpsertIdempotency(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	metrics := []analytics.UserMetrics{
		{
			User: analytics.UserRef{ID: "u1", EmailAddress: "alice@example.com"},
			ChatMetrics: analytics.ChatMetrics{
				ConversationCount: 5,
				MessageCount:      20,
			},
		},
	}

	now := time.Now().UTC()
	if _, err := s.InsertUserDailyMetrics(metrics, "2026-02-14", now); err != nil {
		t.Fatal(err)
	}

	// Update the value and re-insert — should replace.
	metrics[0].ChatMetrics.MessageCount = 30
	if _, err := s.InsertUserDailyMetrics(metrics, "2026-02-14", now); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.AnalyticsUserSummaries("2026-02-14", "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary after upsert, got %d", len(summaries))
	}
	if summaries[0].Messages != 30 {
		t.Errorf("expected 30 messages after upsert, got %d", summaries[0].Messages)
	}
}

func TestAggregationAcrossMultipleDates(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	now := time.Now().UTC()

	day1 := []analytics.UserMetrics{
		{
			User: analytics.UserRef{ID: "u1", EmailAddress: "alice@example.com"},
			ChatMetrics: analytics.ChatMetrics{
				ConversationCount: 3,
				MessageCount:      10,
			},
			ClaudeCodeMetrics: analytics.ClaudeCodeMetrics{
				CoreMetrics: analytics.CoreMetrics{
					CommitCount:          2,
					DistinctSessionCount: 1,
				},
			},
		},
	}
	day2 := []analytics.UserMetrics{
		{
			User: analytics.UserRef{ID: "u1", EmailAddress: "alice@example.com"},
			ChatMetrics: analytics.ChatMetrics{
				ConversationCount: 4,
				MessageCount:      15,
			},
			ClaudeCodeMetrics: analytics.ClaudeCodeMetrics{
				CoreMetrics: analytics.CoreMetrics{
					CommitCount:          1,
					DistinctSessionCount: 3,
				},
			},
		},
	}

	if _, err := s.InsertUserDailyMetrics(day1, "2026-02-13", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertUserDailyMetrics(day2, "2026-02-14", now); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.AnalyticsUserSummaries("2026-02-13", "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 aggregated summary, got %d", len(summaries))
	}

	alice := summaries[0]
	if alice.Conversations != 7 {
		t.Errorf("expected 7 conversations, got %d", alice.Conversations)
	}
	if alice.Messages != 25 {
		t.Errorf("expected 25 messages, got %d", alice.Messages)
	}
	if alice.CCCommits != 3 {
		t.Errorf("expected 3 commits, got %d", alice.CCCommits)
	}
	if alice.CCSessions != 4 {
		t.Errorf("expected 4 sessions, got %d", alice.CCSessions)
	}
	if alice.ActiveDays != 2 {
		t.Errorf("expected 2 active days, got %d", alice.ActiveDays)
	}
}

func TestDateRangeFiltering(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	now := time.Now().UTC()

	for _, date := range []string{"2026-02-12", "2026-02-13", "2026-02-14"} {
		metrics := []analytics.UserMetrics{
			{
				User:        analytics.UserRef{ID: "u1", EmailAddress: "alice@example.com"},
				ChatMetrics: analytics.ChatMetrics{MessageCount: 5},
			},
		}
		if _, err := s.InsertUserDailyMetrics(metrics, date, now); err != nil {
			t.Fatal(err)
		}
	}

	// Query only the last two days.
	summaries, err := s.AnalyticsUserSummaries("2026-02-13", "2026-02-14")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Messages != 10 {
		t.Errorf("expected 10 messages (2 days), got %d", summaries[0].Messages)
	}
	if summaries[0].ActiveDays != 2 {
		t.Errorf("expected 2 active days, got %d", summaries[0].ActiveDays)
	}
}

func TestAnalyticsFetchedDates(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	now := time.Now().UTC()
	metrics := []analytics.UserMetrics{
		{
			User:        analytics.UserRef{ID: "u1", EmailAddress: "alice@example.com"},
			ChatMetrics: analytics.ChatMetrics{MessageCount: 1},
		},
	}

	for _, date := range []string{"2026-02-14", "2026-02-12", "2026-02-13"} {
		if _, err := s.InsertUserDailyMetrics(metrics, date, now); err != nil {
			t.Fatal(err)
		}
	}

	dates, err := s.AnalyticsFetchedDates()
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	// Should be sorted ascending.
	if dates[0] != "2026-02-12" || dates[1] != "2026-02-13" || dates[2] != "2026-02-14" {
		t.Errorf("dates not sorted: %v", dates)
	}
}

func TestInsertAndQueryOrgDailySummaries(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	summaries := []analytics.DailySummary{
		{
			StartingAt:             "2026-02-14T00:00:00Z",
			EndingAt:               "2026-02-15T00:00:00Z",
			DailyActiveUserCount:   23,
			WeeklyActiveUserCount:  34,
			MonthlyActiveUserCount: 42,
			AssignedSeatCount:      85,
			PendingInviteCount:     3,
		},
		{
			StartingAt:             "2026-02-15T00:00:00Z",
			EndingAt:               "2026-02-16T00:00:00Z",
			DailyActiveUserCount:   25,
			WeeklyActiveUserCount:  35,
			MonthlyActiveUserCount: 43,
			AssignedSeatCount:      86,
			PendingInviteCount:     2,
		},
	}

	now := time.Now().UTC()
	if err := s.InsertOrgDailySummaries(summaries, now); err != nil {
		t.Fatal(err)
	}

	stored, err := s.OrgDailySummaries("2026-02-14", "2026-02-15")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(stored))
	}
	// Ordered by date DESC.
	if stored[0].Date != "2026-02-15" {
		t.Errorf("expected first row 2026-02-15, got %q", stored[0].Date)
	}
	if stored[0].DailyActiveUsers != 25 {
		t.Errorf("expected 25 DAU, got %d", stored[0].DailyActiveUsers)
	}
	if stored[1].AssignedSeats != 85 {
		t.Errorf("expected 85 seats, got %d", stored[1].AssignedSeats)
	}
}

func TestAnalyticsLastFetchedAt(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	// Initially zero.
	ts, err := s.AnalyticsLastFetchedAt()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}

	// Set and read back.
	now := time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC)
	if err := s.SetAnalyticsLastFetchedAt(now); err != nil {
		t.Fatal(err)
	}

	ts, err = s.AnalyticsLastFetchedAt()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.Equal(now) {
		t.Errorf("expected %v, got %v", now, ts)
	}
}

func TestSkipsEmptyEmail(t *testing.T) {
	s := openTestDB(t)
	defer s.Close()

	metrics := []analytics.UserMetrics{
		{
			User:        analytics.UserRef{ID: "u1", EmailAddress: ""},
			ChatMetrics: analytics.ChatMetrics{MessageCount: 10},
		},
		{
			User:        analytics.UserRef{ID: "u2", EmailAddress: "bob@example.com"},
			ChatMetrics: analytics.ChatMetrics{MessageCount: 5},
		},
	}

	now := time.Now().UTC()
	n, err := s.InsertUserDailyMetrics(metrics, "2026-02-14", now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 inserted (skipping empty email), got %d", n)
	}
}
