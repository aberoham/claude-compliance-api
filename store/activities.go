package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// InsertActivities upserts activities into the database. Returns the number of
// newly inserted rows (existing rows with the same ID are silently skipped).
func (s *Store) InsertActivities(activities []compliance.Activity) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO activities (id, created_at, type, actor_email, actor_id, actor_type, org_id, raw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, a := range activities {
		raw, err := json.Marshal(a)
		if err != nil {
			return inserted, fmt.Errorf("marshaling activity %s: %w", a.ID, err)
		}

		var email *string
		if a.Actor.EmailAddress != nil {
			lower := strings.ToLower(*a.Actor.EmailAddress)
			email = &lower
		} else if a.Actor.UnauthenticatedEmailAddress != nil {
			lower := strings.ToLower(*a.Actor.UnauthenticatedEmailAddress)
			email = &lower
		}

		var actorID *string
		if a.Actor.UserID != nil {
			actorID = a.Actor.UserID
		}

		var orgID *string
		if a.OrganizationID != nil {
			orgID = a.OrganizationID
		}

		res, err := stmt.Exec(a.ID, a.CreatedAt, a.Type, email, actorID, a.Actor.Type, orgID, string(raw))
		if err != nil {
			return inserted, fmt.Errorf("inserting activity %s: %w", a.ID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return inserted, fmt.Errorf("checking rows affected for %s: %w", a.ID, err)
		}
		inserted += int(n)
	}

	return inserted, tx.Commit()
}

// HighWaterMark returns the ID of the newest activity we have stored, or ""
// if the database is empty. This is persisted in the sync_state table.
func (s *Store) HighWaterMark() (string, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM sync_state WHERE key = 'high_water_mark'").Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetHighWaterMark records the newest activity ID we have seen.
func (s *Store) SetHighWaterMark(id string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO sync_state (key, value) VALUES ('high_water_mark', ?)", id)
	return err
}

// OldestActivity returns the ID and created_at of the oldest stored activity.
// Both return values are empty if the database has no activities. This is
// derived directly from the data rather than a separately-maintained cursor,
// so it's always consistent.
func (s *Store) OldestActivity() (id string, createdAt string, err error) {
	err = s.db.QueryRow(
		"SELECT COALESCE(id, ''), COALESCE(created_at, '') FROM activities ORDER BY created_at ASC LIMIT 1",
	).Scan(&id, &createdAt)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return
}

// LastFetchedAt returns when we last completed a fetch, or the zero time.
func (s *Store) LastFetchedAt() (time.Time, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM sync_state WHERE key = 'last_fetched_at'").Scan(&val)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, val)
}

// SetLastFetchedAt records when the most recent fetch completed.
func (s *Store) SetLastFetchedAt(t time.Time) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO sync_state (key, value) VALUES ('last_fetched_at', ?)",
		t.Format(time.RFC3339))
	return err
}

// QueryOpts specifies filters for querying stored activities.
type QueryOpts struct {
	Email string     // filter by actor_email (case-insensitive)
	Type  string     // filter by activity type
	Since *time.Time // created_at >= since
	Until *time.Time // created_at < until
	Limit int        // max results (0 = no limit)
}

// Activities queries stored activities with optional filters. Results are
// returned in reverse chronological order.
func (s *Store) Activities(opts QueryOpts) ([]compliance.Activity, error) {
	var clauses []string
	var args []interface{}

	if opts.Email != "" {
		clauses = append(clauses, "actor_email = ?")
		args = append(args, strings.ToLower(opts.Email))
	}
	if opts.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, opts.Type)
	}
	if opts.Since != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, opts.Since.Format(time.RFC3339Nano))
	}
	if opts.Until != nil {
		clauses = append(clauses, "created_at < ?")
		args = append(args, opts.Until.Format(time.RFC3339Nano))
	}

	q := "SELECT raw FROM activities"
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []compliance.Activity
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return results, err
		}
		var a compliance.Activity
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return results, fmt.Errorf("unmarshaling stored activity: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// StoredUserSummary holds per-user aggregated stats from the SQL query.
type StoredUserSummary struct {
	Email           string
	EventCount      int
	EventTypes      int
	ChatsCreated    int
	ProjectsCreated int
	Shares          int
	FilesUploaded   int
	Integrations    int
	FirstSeen       string
	LastSeen        string
	ActiveDays      int
}

// UserSummaries returns per-user aggregated stats computed entirely in SQL.
// Only activities with a non-null actor_email and created at or after the
// given time are included. Tracks Rev E activity types for projects, sharing,
// files, and integrations alongside the original chat creation count.
func (s *Store) UserSummaries(since time.Time) ([]StoredUserSummary, error) {
	rows, err := s.db.Query(`
		SELECT actor_email, COUNT(*) as event_count,
			   COUNT(DISTINCT type) as event_types,
			   SUM(CASE WHEN type = 'claude_chat_created' THEN 1 ELSE 0 END) as chats_created,
			   SUM(CASE WHEN type = 'claude_project_created' THEN 1 ELSE 0 END) as projects_created,
			   SUM(CASE WHEN type IN ('claude_chat_snapshot_created', 'session_share_created') THEN 1 ELSE 0 END) as shares,
			   SUM(CASE WHEN type = 'claude_file_uploaded' THEN 1 ELSE 0 END) as files_uploaded,
			   SUM(CASE WHEN type = 'integration_user_connected' THEN 1 ELSE 0 END) as integrations,
			   MIN(created_at) as first_seen, MAX(created_at) as last_seen,
			   COUNT(DISTINCT date(created_at)) as active_days
		FROM activities
		WHERE actor_email IS NOT NULL AND created_at >= ?
		GROUP BY actor_email
		ORDER BY event_count DESC
	`, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StoredUserSummary
	for rows.Next() {
		var su StoredUserSummary
		if err := rows.Scan(
			&su.Email, &su.EventCount, &su.EventTypes,
			&su.ChatsCreated, &su.ProjectsCreated, &su.Shares,
			&su.FilesUploaded, &su.Integrations,
			&su.FirstSeen, &su.LastSeen, &su.ActiveDays,
		); err != nil {
			return results, err
		}
		results = append(results, su)
	}
	return results, rows.Err()
}

// UsersWithActiveIntegrations returns emails that have an
// integration_user_connected event without a later
// integration_user_disconnected. The returned map goes from email to the
// list of integration types (extracted from the raw JSON) still active.
func (s *Store) UsersWithActiveIntegrations() (map[string]bool, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT a1.actor_email
		FROM activities a1
		WHERE a1.type = 'integration_user_connected'
		  AND a1.actor_email IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM activities a2
		    WHERE a2.type = 'integration_user_disconnected'
		      AND a2.actor_email = a1.actor_email
		      AND a2.created_at > a1.created_at
		  )
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return result, err
		}
		result[email] = true
	}
	return result, rows.Err()
}

// ActivityCount returns the total number of stored activities.
func (s *Store) ActivityCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM activities").Scan(&count)
	return count, err
}

// DateRange returns the earliest and latest created_at timestamps stored.
func (s *Store) DateRange() (earliest, latest string, err error) {
	err = s.db.QueryRow("SELECT COALESCE(MIN(created_at), ''), COALESCE(MAX(created_at), '') FROM activities").
		Scan(&earliest, &latest)
	return
}

// UniqueUserCount returns the number of distinct actor emails.
func (s *Store) UniqueUserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(DISTINCT actor_email) FROM activities WHERE actor_email IS NOT NULL").Scan(&count)
	return count, err
}

// EventTypeCounts returns a map of event type to count, ordered by count desc.
func (s *Store) EventTypeCounts() (map[string]int, error) {
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM activities GROUP BY type ORDER BY COUNT(*) DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return counts, err
		}
		counts[t] = c
	}
	return counts, rows.Err()
}

// UserAgentCount holds a unique user agent string and how many activities
// reference it.
type UserAgentCount struct {
	UserAgent string
	Count     int
}

// UserAgents extracts distinct user agent strings from stored activities using
// SQLite's JSON functions. If email is non-empty, results are filtered to that
// actor. Results are ordered by count descending.
func (s *Store) UserAgents(email string) ([]UserAgentCount, error) {
	q := `
		SELECT json_extract(raw, '$.actor.user_agent') as ua, COUNT(*) as cnt
		FROM activities
		WHERE json_extract(raw, '$.actor.user_agent') IS NOT NULL
	`
	var args []interface{}
	if email != "" {
		q += " AND actor_email = ?"
		args = append(args, strings.ToLower(email))
	}
	q += " GROUP BY ua ORDER BY cnt DESC"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UserAgentCount
	for rows.Next() {
		var uc UserAgentCount
		if err := rows.Scan(&uc.UserAgent, &uc.Count); err != nil {
			return results, err
		}
		results = append(results, uc)
	}
	return results, rows.Err()
}
