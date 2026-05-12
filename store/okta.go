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
// whose published timestamp falls within [since, until). The map is
// keyed by actor_email — which Okta populates from profile.login. For
// AD-mastered tenants this is rarely the same string as the user's
// Anthropic-side email, so callers must translate before joining
// against Compliance API users (see SaveOktaUserLookup /
// LoadOktaUserLookups).
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

// OktaUserLookup pairs an Anthropic-side email with its corresponding
// Okta identity, so SSO events (keyed by profile.login) can be joined
// against Compliance users (keyed by email). NotFound is true when a
// prior lookup confirmed the email has no Okta record, so the cache can
// negative-cache misses and avoid re-querying Okta for known absences.
type OktaUserLookup struct {
	Email      string
	OktaUserID string
	OktaLogin  string
	FetchedAt  time.Time
	NotFound   bool
}

// LoadOktaUserLookups reads fresh lookup rows (younger than ttl) for the
// given emails. Returns the cached entries keyed by lowercased email and
// the subset of input emails still missing a fresh entry — those are the
// ones the caller needs to fetch from Okta.
func (s *Store) LoadOktaUserLookups(
	emails []string, ttl time.Duration,
) (map[string]OktaUserLookup, []string, error) {
	cached := make(map[string]OktaUserLookup, len(emails))
	if len(emails) == 0 {
		return cached, nil, nil
	}

	keys := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		keys[strings.ToLower(e)] = struct{}{}
	}

	cutoff := time.Now().UTC().Add(-ttl).Format(time.RFC3339)
	rows, err := s.db.Query(`
		SELECT email, okta_user_id, okta_login, fetched_at
		FROM okta_user_lookups
		WHERE fetched_at >= ?
	`, cutoff)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var (
			row OktaUserLookup
			ts  string
		)
		if err := rows.Scan(
			&row.Email, &row.OktaUserID, &row.OktaLogin, &ts,
		); err != nil {
			return nil, nil, err
		}
		if _, want := keys[row.Email]; !want {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			row.FetchedAt = t
		}
		row.NotFound = row.OktaUserID == "" && row.OktaLogin == ""
		cached[row.Email] = row
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	missing := make([]string, 0)
	for _, e := range emails {
		if _, ok := cached[strings.ToLower(e)]; !ok {
			missing = append(missing, e)
		}
	}
	return cached, missing, nil
}

// SaveOktaUserLookup upserts a single lookup row. A nil profile is stored
// as a negative cache entry (NotFound) so subsequent ranks do not re-query
// Okta for the same absent email.
func (s *Store) SaveOktaUserLookup(
	email string, profile *okta.UserProfile, fetchedAt time.Time,
) error {
	id, login := "", ""
	if profile != nil {
		id, login = profile.ID, strings.ToLower(profile.Login)
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO okta_user_lookups
			(email, okta_user_id, okta_login, fetched_at)
		VALUES (?, ?, ?, ?)
	`, strings.ToLower(email), id, login, fetchedAt.Format(time.RFC3339))
	return err
}

