package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/analytics"
)

// InsertUserDailyMetrics upserts per-user analytics for a single date.
// Returns the number of rows written. Re-inserting the same user+date
// replaces the previous row (INSERT OR REPLACE on the composite PK).
func (s *Store) InsertUserDailyMetrics(
	metrics []analytics.UserMetrics,
	date string,
	fetchedAt time.Time,
) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO analytics_user_daily (
			user_email, date, user_id,
			conversations, messages,
			projects_created, projects_used,
			files_uploaded, artifacts_created,
			thinking_messages, skills_used, connectors_used,
			cc_commits, cc_pull_requests,
			cc_lines_added, cc_lines_removed, cc_sessions,
			web_searches, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	inserted := 0
	for _, m := range metrics {
		email := strings.ToLower(m.User.EmailAddress)
		if email == "" {
			continue
		}
		cc := m.ClaudeCodeMetrics.CoreMetrics
		ch := m.ChatMetrics

		_, err := stmt.Exec(
			email, date, m.User.ID,
			ch.ConversationCount, ch.MessageCount,
			ch.ProjectsCreatedCount, ch.ProjectsUsedCount,
			ch.FilesUploadedCount, ch.ArtifactsCreatedCount,
			ch.ThinkingMessageCount, ch.SkillsUsedCount,
			ch.ConnectorsUsedCount,
			cc.CommitCount, cc.PullRequestCount,
			cc.LinesOfCode.AddedCount, cc.LinesOfCode.RemovedCount,
			cc.DistinctSessionCount,
			m.WebSearchCount, ts,
		)
		if err != nil {
			return inserted, err
		}
		inserted++
	}

	return inserted, tx.Commit()
}

// InsertOrgDailySummaries upserts org-level daily summary rows.
func (s *Store) InsertOrgDailySummaries(
	summaries []analytics.DailySummary,
	fetchedAt time.Time,
) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO analytics_org_daily (
			date, daily_active_users, weekly_active_users,
			monthly_active_users, assigned_seats, pending_invites,
			fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	for _, d := range summaries {
		_, err := stmt.Exec(
			d.Date(),
			d.DailyActiveUserCount, d.WeeklyActiveUserCount,
			d.MonthlyActiveUserCount, d.AssignedSeatCount,
			d.PendingInviteCount, ts,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AnalyticsUserSummary holds aggregated analytics across a date range.
type AnalyticsUserSummary struct {
	Email            string
	Conversations    int
	Messages         int
	ProjectsCreated  int
	ProjectsUsed     int
	FilesUploaded    int
	ArtifactsCreated int
	ThinkingMessages int
	SkillsUsed       int
	ConnectorsUsed   int
	CCCommits        int
	CCPullRequests   int
	CCLinesAdded     int
	CCLinesRemoved   int
	CCSessions       int
	WebSearches      int
	ActiveDays       int
}

// AnalyticsUserSummaries returns per-user aggregated analytics for the
// given date range (inclusive on both ends). ActiveDays counts dates where
// the user had any non-zero metric.
func (s *Store) AnalyticsUserSummaries(
	since, until string,
) ([]AnalyticsUserSummary, error) {
	rows, err := s.db.Query(`
		SELECT
			user_email,
			SUM(conversations),
			SUM(messages),
			SUM(projects_created),
			SUM(projects_used),
			SUM(files_uploaded),
			SUM(artifacts_created),
			SUM(thinking_messages),
			SUM(skills_used),
			SUM(connectors_used),
			SUM(cc_commits),
			SUM(cc_pull_requests),
			SUM(cc_lines_added),
			SUM(cc_lines_removed),
			SUM(cc_sessions),
			SUM(web_searches),
			COUNT(DISTINCT CASE
				WHEN conversations > 0 OR messages > 0
				  OR cc_commits > 0 OR cc_sessions > 0
				  OR web_searches > 0 OR connectors_used > 0
				  OR skills_used > 0 OR artifacts_created > 0
				THEN date
			END)
		FROM analytics_user_daily
		WHERE date >= ? AND date <= ?
		GROUP BY user_email
		ORDER BY SUM(messages) DESC
	`, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AnalyticsUserSummary
	for rows.Next() {
		var s AnalyticsUserSummary
		if err := rows.Scan(
			&s.Email, &s.Conversations, &s.Messages,
			&s.ProjectsCreated, &s.ProjectsUsed,
			&s.FilesUploaded, &s.ArtifactsCreated,
			&s.ThinkingMessages, &s.SkillsUsed, &s.ConnectorsUsed,
			&s.CCCommits, &s.CCPullRequests,
			&s.CCLinesAdded, &s.CCLinesRemoved, &s.CCSessions,
			&s.WebSearches, &s.ActiveDays,
		); err != nil {
			return results, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// AnalyticsLastActiveDates returns the most recent date with any non-zero
// metric for each user in the given range. Users with no activity are omitted.
func (s *Store) AnalyticsLastActiveDates(
	since, until string,
) (map[string]string, error) {
	rows, err := s.db.Query(`
		SELECT user_email, MAX(date)
		FROM analytics_user_daily
		WHERE date >= ? AND date <= ?
		  AND (conversations > 0 OR messages > 0
		    OR cc_commits > 0 OR cc_sessions > 0
		    OR web_searches > 0 OR connectors_used > 0
		    OR skills_used > 0 OR artifacts_created > 0)
		GROUP BY user_email
	`, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var email, date string
		if err := rows.Scan(&email, &date); err != nil {
			return result, err
		}
		result[email] = date
	}
	return result, rows.Err()
}

// AnalyticsFetchedDates returns the distinct dates that have been fetched
// and stored in analytics_user_daily, sorted ascending.
func (s *Store) AnalyticsFetchedDates() ([]string, error) {
	rows, err := s.db.Query(
		"SELECT DISTINCT date FROM analytics_user_daily ORDER BY date",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return dates, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// StoredOrgDailySummary is a row from analytics_org_daily.
type StoredOrgDailySummary struct {
	Date               string
	DailyActiveUsers   int
	WeeklyActiveUsers  int
	MonthlyActiveUsers int
	AssignedSeats      int
	PendingInvites     int
	FetchedAt          string
}

// OrgDailySummaries returns org-level summaries for the given date range.
func (s *Store) OrgDailySummaries(
	since, until string,
) ([]StoredOrgDailySummary, error) {
	rows, err := s.db.Query(`
		SELECT date, daily_active_users, weekly_active_users,
		       monthly_active_users, assigned_seats, pending_invites,
		       fetched_at
		FROM analytics_org_daily
		WHERE date >= ? AND date <= ?
		ORDER BY date DESC
	`, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StoredOrgDailySummary
	for rows.Next() {
		var s StoredOrgDailySummary
		if err := rows.Scan(
			&s.Date, &s.DailyActiveUsers, &s.WeeklyActiveUsers,
			&s.MonthlyActiveUsers, &s.AssignedSeats,
			&s.PendingInvites, &s.FetchedAt,
		); err != nil {
			return results, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// AnalyticsLastFetchedAt returns when analytics data was last fetched,
// or the zero time if never fetched.
func (s *Store) AnalyticsLastFetchedAt() (time.Time, error) {
	var val string
	err := s.db.QueryRow(
		"SELECT value FROM sync_state WHERE key = 'analytics_last_fetched_at'",
	).Scan(&val)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, val)
}

// SetAnalyticsLastFetchedAt records when analytics data was last fetched.
func (s *Store) SetAnalyticsLastFetchedAt(t time.Time) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO sync_state (key, value) VALUES ('analytics_last_fetched_at', ?)",
		t.Format(time.RFC3339),
	)
	return err
}
