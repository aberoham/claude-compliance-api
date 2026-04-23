package store

import (
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/okta"
)

// OktaSSOSummary holds aggregated SSO data for one user.
type OktaSSOSummary struct {
	Email      string
	EventCount int
	FirstSSO   string
	LastSSO    string
}

// InsertOktaSSOEvents upserts Okta SSO events into the cache. Each
// event is identified by its Okta UUID, so repeated inserts are
// idempotent. Returns the number of rows written.
func (s *Store) InsertOktaSSOEvents(
	events []okta.LogEvent, fetchedAt time.Time,
) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO okta_sso_events
			(event_id, actor_email, published, app_instance_id, app_name, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close() //nolint:errcheck

	fetchedStr := fetchedAt.Format(time.RFC3339)
	n := 0
	for _, e := range events {
		email := strings.ToLower(e.Actor.AlternateID)
		_, err := stmt.Exec(
			e.UUID,
			email,
			e.Published.Format(time.RFC3339),
			e.MatchedAppID,
			e.MatchedAppName,
			fetchedStr,
		)
		if err != nil {
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// OktaSSOSummaries returns per-user aggregated SSO stats for events
// whose published timestamp falls within [since, until).
func (s *Store) OktaSSOSummaries(
	since, until time.Time,
) (map[string]OktaSSOSummary, error) {
	rows, err := s.db.Query(`
		SELECT actor_email, COUNT(*), MIN(published), MAX(published)
		FROM okta_sso_events
		WHERE published >= ? AND published < ?
		GROUP BY actor_email
	`, since.Format(time.RFC3339), until.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	result := make(map[string]OktaSSOSummary)
	for rows.Next() {
		var su OktaSSOSummary
		if err := rows.Scan(
			&su.Email, &su.EventCount, &su.FirstSSO, &su.LastSSO,
		); err != nil {
			return nil, err
		}
		result[su.Email] = su
	}
	return result, rows.Err()
}

