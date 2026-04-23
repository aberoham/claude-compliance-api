package store

import (
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

// InsertProjects upserts project records, updating the fetched_at timestamp.
func (s *Store) InsertProjects(projects []compliance.Project, fetchedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO projects
		(id, name, description, instructions, creator_id, creator_email, org_id, created_at, updated_at, archived_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	ts := fetchedAt.Format(time.RFC3339)
	for _, p := range projects {
		var creatorEmail *string
		creatorID := p.CreatorID
		if p.Creator != nil {
			email := strings.ToLower(p.Creator.EffectiveEmail())
			creatorEmail = &email
		} else if p.User != nil && p.User.EmailAddress != "" {
			email := strings.ToLower(p.User.EmailAddress)
			creatorEmail = &email
			if creatorID == "" {
				creatorID = p.User.ID
			}
		}

		if _, err := stmt.Exec(
			p.ID, p.Name, p.Description, p.Instructions,
			creatorID, creatorEmail, p.OrganizationID,
			p.CreatedAt, p.UpdatedAt, p.ArchivedAt, ts,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CachedProject is a project record from the local cache.
type CachedProject struct {
	ID           string
	Name         string
	Description  string
	Instructions string
	CreatorID    string
	CreatorEmail *string
	OrgID        string
	CreatedAt    string
	UpdatedAt    string
	ArchivedAt   *string
	FetchedAt    string
}

// ProjectQueryOpts specifies filters for querying cached projects.
type ProjectQueryOpts struct {
	CreatorEmail string
	Limit        int
}

// Projects returns cached project records matching the given filters.
func (s *Store) Projects(opts ProjectQueryOpts) ([]CachedProject, error) {
	query := `SELECT id, name, description, instructions, creator_id, creator_email,
	          org_id, created_at, updated_at, archived_at, fetched_at
	          FROM projects`

	var args []interface{}
	if opts.CreatorEmail != "" {
		query += " WHERE creator_email = ?"
		args = append(args, strings.ToLower(opts.CreatorEmail))
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

	var projects []CachedProject
	for rows.Next() {
		var p CachedProject
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Instructions,
			&p.CreatorID, &p.CreatorEmail, &p.OrgID,
			&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt, &p.FetchedAt,
		); err != nil {
			return projects, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProject returns a single cached project by ID.
func (s *Store) GetProject(id string) (*CachedProject, error) {
	var p CachedProject
	err := s.db.QueryRow(`
		SELECT id, name, description, instructions, creator_id, creator_email,
		       org_id, created_at, updated_at, archived_at, fetched_at
		FROM projects WHERE id = ?
	`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.Instructions,
		&p.CreatorID, &p.CreatorEmail, &p.OrgID,
		&p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt, &p.FetchedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ProjectsFetchedAt returns when projects were last refreshed, or the zero time
// if no projects are cached.
func (s *Store) ProjectsFetchedAt() (time.Time, error) {
	var ts *string
	err := s.db.QueryRow("SELECT MAX(fetched_at) FROM projects").Scan(&ts)
	if err != nil || ts == nil || *ts == "" {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, *ts)
}

// ProjectCount returns the total number of cached projects.
func (s *Store) ProjectCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	return count, err
}
