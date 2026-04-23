package store

import (
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// InsertUsers replaces the cached user table with the provided list. The full
// table is cleared first so that deprovisioned users (reclaimed seats) are
// removed. This is safe because callers always pass the complete licensed user
// list from the API.
func (s *Store) InsertUsers(users []compliance.User, fetchedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM users"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO users (id, email, full_name, created_at, fetched_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	for _, u := range users {
		email := strings.ToLower(u.EffectiveEmail())
		if _, err := stmt.Exec(u.ID, email, u.FullName, u.CreatedAt, ts); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CachedUser is a user record from the local cache.
type CachedUser struct {
	ID        string
	Email     string
	FullName  string
	CreatedAt string
	FetchedAt string
}

// Users returns all cached user records.
func (s *Store) Users() ([]CachedUser, error) {
	rows, err := s.db.Query("SELECT id, email, full_name, created_at, fetched_at FROM users ORDER BY email")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []CachedUser
	for rows.Next() {
		var u CachedUser
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.CreatedAt, &u.FetchedAt); err != nil {
			return users, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UsersFetchedAt returns when users were last refreshed, or the zero time
// if no users are cached.
func (s *Store) UsersFetchedAt() (time.Time, error) {
	var ts *string
	err := s.db.QueryRow("SELECT MAX(fetched_at) FROM users").Scan(&ts)
	if err != nil || ts == nil || *ts == "" {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, *ts)
}

// UserByEmail returns the user with the given email address, or nil if not found.
func (s *Store) UserByEmail(email string) (*CachedUser, error) {
	var u CachedUser
	err := s.db.QueryRow(
		"SELECT id, email, full_name, created_at, fetched_at FROM users WHERE email = ?",
		strings.ToLower(email),
	).Scan(&u.ID, &u.Email, &u.FullName, &u.CreatedAt, &u.FetchedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
