package store

import (
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// InsertChats upserts chat metadata records, updating the fetched_at timestamp.
func (s *Store) InsertChats(chats []compliance.Chat, fetchedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO chats
		(id, name, user_id, user_email, project_id, org_id, created_at, updated_at, deleted_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	for _, c := range chats {
		email := strings.ToLower(c.User.EmailAddress)
		if _, err := stmt.Exec(
			c.ID, c.Name, c.User.ID, email, c.ProjectID, c.OrganizationID,
			c.CreatedAt, c.UpdatedAt, c.DeletedAt, ts,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CachedChat is a chat record from the local cache.
type CachedChat struct {
	ID        string
	Name      string
	UserID    string
	UserEmail string
	ProjectID *string
	OrgID     string
	CreatedAt string
	UpdatedAt string
	DeletedAt *string
	FetchedAt string
}

// ChatQueryOpts specifies filters for querying cached chats.
type ChatQueryOpts struct {
	UserEmail string
	ProjectID string
	Since     *time.Time
	Until     *time.Time
	Limit     int
}

// Chats returns cached chat records matching the given filters.
func (s *Store) Chats(opts ChatQueryOpts) ([]CachedChat, error) {
	query := `SELECT id, name, user_id, user_email, project_id, org_id,
	          created_at, updated_at, deleted_at, fetched_at
	          FROM chats WHERE 1=1`

	var args []interface{}
	if opts.UserEmail != "" {
		query += " AND user_email = ?"
		args = append(args, strings.ToLower(opts.UserEmail))
	}
	if opts.ProjectID != "" {
		query += " AND project_id = ?"
		args = append(args, opts.ProjectID)
	}
	if opts.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, opts.Since.Format(time.RFC3339))
	}
	if opts.Until != nil {
		query += " AND created_at < ?"
		args = append(args, opts.Until.Format(time.RFC3339))
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []CachedChat
	for rows.Next() {
		var c CachedChat
		if err := rows.Scan(
			&c.ID, &c.Name, &c.UserID, &c.UserEmail, &c.ProjectID, &c.OrgID,
			&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt, &c.FetchedAt,
		); err != nil {
			return chats, err
		}
		chats = append(chats, c)
	}
	return chats, rows.Err()
}

// GetChat returns a single cached chat by ID.
func (s *Store) GetChat(id string) (*CachedChat, error) {
	var c CachedChat
	err := s.db.QueryRow(`
		SELECT id, name, user_id, user_email, project_id, org_id,
		       created_at, updated_at, deleted_at, fetched_at
		FROM chats WHERE id = ?
	`, id).Scan(
		&c.ID, &c.Name, &c.UserID, &c.UserEmail, &c.ProjectID, &c.OrgID,
		&c.CreatedAt, &c.UpdatedAt, &c.DeletedAt, &c.FetchedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ChatsFetchedAt returns when chats were last refreshed, or the zero time
// if no chats are cached.
func (s *Store) ChatsFetchedAt() (time.Time, error) {
	var ts *string
	err := s.db.QueryRow("SELECT MAX(fetched_at) FROM chats").Scan(&ts)
	if err != nil || ts == nil || *ts == "" {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, *ts)
}

// ChatCount returns the total number of cached chats.
func (s *Store) ChatCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chats").Scan(&count)
	return count, err
}
