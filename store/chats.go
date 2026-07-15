package store

import (
	"database/sql"
	"encoding/json"
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
		(id, name, user_id, user_email, project_id, org_id, created_at, updated_at, deleted_at, fetched_at,
		 org_uuid, model, href, raw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	for _, c := range chats {
		raw, err := json.Marshal(c)
		if err != nil {
			return err
		}
		email := strings.ToLower(c.User.EmailAddress)
		if _, err := stmt.Exec(
			c.ID, c.Name, c.User.ID, email, c.ProjectID, c.OrganizationID,
			c.CreatedAt, c.UpdatedAt, c.DeletedAt, ts,
			c.OrganizationUUID, c.Model, c.Href, string(raw),
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

// ChatsMissingTranscripts returns cached chat metadata rows in the requested
// window that do not yet have a full transcript row.
func (s *Store) ChatsMissingTranscripts(since time.Time) ([]compliance.Chat, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.user_id, c.user_email, c.project_id, c.org_id,
		       c.created_at, c.updated_at, c.deleted_at
		FROM chats c
		LEFT JOIN chat_transcripts t ON t.chat_id = c.id
		WHERE c.created_at >= ? AND t.chat_id IS NULL
		ORDER BY c.created_at
	`, since.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []compliance.Chat
	for rows.Next() {
		var chat compliance.Chat
		var projectID *string
		var deletedAt *string
		var userID, userEmail string
		if err := rows.Scan(
			&chat.ID, &chat.Name, &userID, &userEmail, &projectID, &chat.OrganizationID,
			&chat.CreatedAt, &chat.UpdatedAt, &deletedAt,
		); err != nil {
			return chats, err
		}
		chat.User.ID = userID
		chat.User.EmailAddress = userEmail
		chat.ProjectID = projectID
		chat.DeletedAt = deletedAt
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

// InsertChatTranscript upserts a full chat transcript into the local cache.
// raw should be the exact JSON response body returned by the Compliance API.
func (s *Store) InsertChatTranscript(chat *compliance.ChatDetail, raw []byte, fetchedAt time.Time) error {
	if raw == nil {
		var err error
		raw, err = json.Marshal(chat)
		if err != nil {
			return err
		}
	}

	email := strings.ToLower(chat.User.EmailAddress)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO chat_transcripts
		(chat_id, user_id, user_email, created_at, updated_at, message_count, raw, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		chat.ID, chat.User.ID, email, chat.CreatedAt, chat.UpdatedAt,
		len(chat.ChatMessages), string(raw), fetchedAt.Format(time.RFC3339),
	)
	return err
}

// HasChatTranscript reports whether a transcript is already cached.
func (s *Store) HasChatTranscript(chatID string) (bool, error) {
	var one int
	err := s.db.QueryRow("SELECT 1 FROM chat_transcripts WHERE chat_id = ?", chatID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// ChatTranscriptCount returns the number of cached full chat transcripts.
func (s *Store) ChatTranscriptCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chat_transcripts").Scan(&count)
	return count, err
}
