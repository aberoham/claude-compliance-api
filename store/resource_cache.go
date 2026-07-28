package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
)

type SyncRun struct {
	ID         int64
	Resource   string
	Scope      string
	Generation string
	Mode       string
	Cursor     string
	Seen       int
	Stored     int
}

type ResourceCoverage struct {
	Resource string `json:"resource"`
	Count    int    `json:"count"`
}

func (s *Store) ResourceCoverage() ([]ResourceCoverage, error) {
	tables := []struct{ resource, table string }{
		{"organizations", "organizations"}, {"organization memberships", "organization_memberships"},
		{"roles", "compliance_roles"}, {"role permissions", "compliance_role_permissions"},
		{"groups", "compliance_groups"}, {"group members", "compliance_group_members"},
		{"organization settings", "organization_settings"}, {"projects", "projects"},
		{"project attachments", "project_attachments"}, {"project collaborators", "project_collaborators"},
		{"project documents", "project_documents"}, {"chats", "chats"}, {"chat transcripts", "chat_transcripts"},
		{"files", "compliance_files"}, {"generated files", "generated_files"},
		{"chat artifact versions", "chat_artifact_versions"}, {"code artifacts", "code_artifacts"},
		{"code artifact versions", "code_artifact_versions"}, {"content objects", "content_objects"},
		{"resource content links", "resource_content"},
	}
	values := make([]ResourceCoverage, 0, len(tables))
	for _, item := range tables {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + item.table).Scan(&count); err != nil {
			return values, err
		}
		values = append(values, ResourceCoverage{Resource: item.resource, Count: count})
	}
	return values, nil
}

func (s *Store) BeginResourceSync(resource, scope, mode string) (*SyncRun, error) {
	var generation, cursor string
	var savedCursor, savedGeneration *string
	err := s.db.QueryRow(`SELECT cursor, generation FROM resource_sync_state WHERE resource=? AND scope=?`, resource, scope).Scan(&savedCursor, &savedGeneration)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if savedCursor != nil && *savedCursor != "" && savedGeneration != nil && *savedGeneration != "" {
		cursor = *savedCursor
		generation = *savedGeneration
		mode = "resume"
	} else {
		generation, err = newGeneration()
		if err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.Exec(`
		INSERT INTO sync_runs (resource, scope, generation, mode, status, cursor, started_at)
		VALUES (?, ?, ?, ?, 'running', ?, ?)
	`, resource, scope, generation, mode, nullableString(cursor), now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`
		INSERT INTO resource_sync_state (resource, scope, generation, last_attempted_at, last_error)
		VALUES (?, ?, ?, ?, NULL)
		ON CONFLICT(resource, scope) DO UPDATE SET
			generation=excluded.generation,
			last_attempted_at=excluded.last_attempted_at,
			last_error=NULL
	`, resource, scope, generation, now)
	if err != nil {
		return nil, err
	}
	return &SyncRun{ID: id, Resource: resource, Scope: scope, Generation: generation, Mode: mode, Cursor: cursor}, nil
}

func (s *Store) CheckpointResourceSync(run *SyncRun, nextCursor string, seen, stored int) error {
	run.Cursor = nextCursor
	run.Seen += seen
	run.Stored += stored
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		UPDATE sync_runs SET cursor=?, records_seen=?, records_stored=? WHERE id=?
	`, nullableString(nextCursor), run.Seen, run.Stored, run.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE resource_sync_state SET cursor=?, generation=?, last_attempted_at=?
		WHERE resource=? AND scope=?
	`, nullableString(nextCursor), run.Generation, now, run.Resource, run.Scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CompleteResourceSync(run *SyncRun, pruneSnapshot bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if pruneSnapshot {
		if err := pruneResourceSnapshot(tx, run.Resource, run.Scope, run.Generation); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`
		UPDATE sync_runs SET status='complete', cursor=NULL, completed_at=?, records_seen=?, records_stored=?
		WHERE id=?
	`, now, run.Seen, run.Stored, run.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE resource_sync_state SET cursor=NULL, generation=?, last_completed_at=?, last_error=NULL
		WHERE resource=? AND scope=?
	`, run.Generation, now, run.Resource, run.Scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FailResourceSync(run *SyncRun, cause error) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	message := cause.Error()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		UPDATE sync_runs SET status='failed', completed_at=?, last_error=?, cursor=?, records_seen=?, records_stored=?
		WHERE id=?
	`, now, message, nullableString(run.Cursor), run.Seen, run.Stored, run.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE resource_sync_state SET cursor=?, last_error=?, last_attempted_at=?
		WHERE resource=? AND scope=?
	`, nullableString(run.Cursor), message, now, run.Resource, run.Scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ResourceCursor(resource, scope string) (string, error) {
	var cursor *string
	err := s.db.QueryRow(`SELECT cursor FROM resource_sync_state WHERE resource=? AND scope=?`, resource, scope).Scan(&cursor)
	if err == sql.ErrNoRows || cursor == nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return *cursor, nil
}

func (s *Store) CachedOrganizations() ([]compliance.Organization, error) {
	rows, err := s.db.Query(`SELECT raw FROM organizations ORDER BY uuid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []compliance.Organization
	for rows.Next() {
		var raw string
		var value compliance.Organization
		if err := rows.Scan(&raw); err != nil {
			return values, err
		}
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return values, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) CachedRoles(orgUUID string) ([]compliance.Role, error) {
	rows, err := s.db.Query(`SELECT raw FROM compliance_roles WHERE organization_uuid=? ORDER BY role_id`, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []compliance.Role
	for rows.Next() {
		var raw string
		var value compliance.Role
		if err := rows.Scan(&raw); err != nil {
			return values, err
		}
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return values, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) CachedGroups() ([]compliance.Group, error) {
	rows, err := s.db.Query(`SELECT raw FROM compliance_groups ORDER BY group_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []compliance.Group
	for rows.Next() {
		var raw string
		var value compliance.Group
		if err := rows.Scan(&raw); err != nil {
			return values, err
		}
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return values, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func pruneResourceSnapshot(tx *sql.Tx, resource, scope, generation string) error {
	if resource == "organizations" {
		for _, query := range []string{
			`DELETE FROM organization_memberships WHERE organization_uuid NOT IN (SELECT uuid FROM organizations WHERE seen_generation=?)`,
			`DELETE FROM compliance_role_permissions WHERE organization_uuid NOT IN (SELECT uuid FROM organizations WHERE seen_generation=?)`,
			`DELETE FROM compliance_roles WHERE organization_uuid NOT IN (SELECT uuid FROM organizations WHERE seen_generation=?)`,
			`DELETE FROM organization_settings WHERE organization_id NOT IN (SELECT uuid FROM organizations WHERE seen_generation=?)`,
			`DELETE FROM organizations WHERE seen_generation <> ?`,
		} {
			if _, err := tx.Exec(query, generation); err != nil {
				return err
			}
		}
		return nil
	}
	if resource == "roles" {
		if _, err := tx.Exec(`DELETE FROM compliance_roles WHERE organization_uuid=? AND seen_generation <> ?`, scope, generation); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM compliance_role_permissions WHERE organization_uuid=? AND role_id NOT IN (SELECT role_id FROM compliance_roles WHERE organization_uuid=?)`, scope, scope)
		return err
	}
	if resource == "groups" {
		if _, err := tx.Exec(`DELETE FROM compliance_groups WHERE seen_generation <> ?`, generation); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM compliance_group_members WHERE group_id NOT IN (SELECT group_id FROM compliance_groups)`)
		return err
	}
	if resource == "project_attachments" {
		if _, err := tx.Exec(`DELETE FROM project_attachments WHERE project_id=? AND seen_generation <> ?`, scope, generation); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM project_documents WHERE project_id=? AND document_id NOT IN (SELECT attachment_id FROM project_attachments WHERE project_id=? AND attachment_type='project_doc')`, scope, scope)
		return err
	}
	if resource == "projects" {
		if _, err := tx.Exec(`DELETE FROM projects WHERE seen_generation <> ?`, generation); err != nil {
			return err
		}
		for _, query := range []string{
			`DELETE FROM project_attachments WHERE project_id NOT IN (SELECT id FROM projects)`,
			`DELETE FROM project_collaborators WHERE project_id NOT IN (SELECT id FROM projects)`,
			`DELETE FROM project_documents WHERE project_id IS NOT NULL AND project_id NOT IN (SELECT id FROM projects)`,
		} {
			if _, err := tx.Exec(query); err != nil {
				return err
			}
		}
		return nil
	}
	if resource == "code_artifacts" {
		if _, err := tx.Exec(`DELETE FROM code_artifacts WHERE seen_generation <> ?`, generation); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM code_artifact_versions WHERE seen_generation <> ? OR artifact_id NOT IN (SELECT artifact_id FROM code_artifacts)`, generation)
		return err
	}
	statements := map[string]struct {
		query string
		args  []any
	}{
		"organization_memberships": {`DELETE FROM organization_memberships WHERE organization_uuid=? AND seen_generation <> ?`, []any{scope, generation}},
		"role_permissions":         {`DELETE FROM compliance_role_permissions WHERE organization_uuid || '/' || role_id=? AND seen_generation <> ?`, []any{scope, generation}},
		"group_members":            {`DELETE FROM compliance_group_members WHERE group_id=? AND seen_generation <> ?`, []any{scope, generation}},
		"project_collaborators":    {`DELETE FROM project_collaborators WHERE project_id=? AND seen_generation <> ?`, []any{scope, generation}},
		"chats":                    {`DELETE FROM chats WHERE seen_generation <> ?`, []any{generation}},
	}
	statement, ok := statements[resource]
	if !ok {
		return nil
	}
	_, err := tx.Exec(statement.query, statement.args...)
	return err
}

func (s *Store) UpsertProjectsSnapshot(values []compliance.Project, generation string, fetchedAt time.Time) (int, error) {
	if err := s.InsertProjects(values, fetchedAt); err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE projects SET seen_generation=? WHERE id=?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		if _, err := stmt.Exec(generation, value.ID); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertChatsSnapshot(values []compliance.Chat, generation string, fetchedAt time.Time) (int, error) {
	if err := s.InsertChats(values, fetchedAt); err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE chats SET seen_generation=? WHERE id=?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		if _, err := stmt.Exec(generation, value.ID); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertOrganizations(values []compliance.Organization, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO organizations (uuid, name, created_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(uuid) DO UPDATE SET name=excluded.name, created_at=excluded.created_at,
			raw=excluded.raw, fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			return 0, err
		}
		if _, err := stmt.Exec(value.UUID, value.Name, value.CreatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertOrganizationMemberships(orgUUID string, values []compliance.User, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO organization_memberships
		(organization_uuid, user_id, email, full_name, organization_role, created_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(organization_uuid, user_id) DO UPDATE SET email=excluded.email, full_name=excluded.full_name,
			organization_role=excluded.organization_role, created_at=excluded.created_at, raw=excluded.raw,
			fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(orgUUID, value.ID, strings.ToLower(value.EffectiveEmail()), value.FullName,
			value.OrganizationRole, value.CreatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertRoles(orgUUID string, values []compliance.Role, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO compliance_roles
		(organization_uuid, role_id, name, description, created_at, updated_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(organization_uuid, role_id) DO UPDATE SET name=excluded.name, description=excluded.description,
			created_at=excluded.created_at, updated_at=excluded.updated_at, raw=excluded.raw,
			fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(orgUUID, value.ID, value.Name, value.Description, value.CreatedAt,
			value.UpdatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertRolePermissions(orgUUID, roleID string, values []compliance.RolePermission, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO compliance_role_permissions
		(organization_uuid, role_id, action, resource_type, resource_id, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(organization_uuid, role_id, action, resource_type, resource_id) DO UPDATE SET
			raw=excluded.raw, fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(orgUUID, roleID, value.Action, value.ResourceType, value.ResourceID,
			string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertGroups(values []compliance.Group, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO compliance_groups
		(group_id, name, description, source_type, roles_json, created_at, updated_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET name=excluded.name, description=excluded.description,
			source_type=excluded.source_type, roles_json=excluded.roles_json, created_at=excluded.created_at,
			updated_at=excluded.updated_at, raw=excluded.raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		roles, _ := json.Marshal(value.Roles)
		if _, err := stmt.Exec(value.ID, value.Name, value.Description, value.SourceType, string(roles),
			value.CreatedAt, value.UpdatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertGroupMembers(groupID string, values []compliance.GroupMember, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO compliance_group_members
		(group_id, user_id, email, created_at, updated_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id, user_id) DO UPDATE SET email=excluded.email, created_at=excluded.created_at,
			updated_at=excluded.updated_at, raw=excluded.raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(groupID, value.UserID, strings.ToLower(value.Email), value.CreatedAt,
			value.UpdatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertOrganizationSettings(value *compliance.EffectiveOrganizationSettings, generation string, fetchedAt time.Time) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`
		INSERT INTO organization_settings (organization_id, settings_type, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(organization_id) DO UPDATE SET settings_type=excluded.settings_type, raw=excluded.raw,
			fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`, value.OrganizationID, value.Type, string(raw), fetchedAt.Format(time.RFC3339Nano), generation)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) UpsertFileMetadata(value *compliance.FileMetadata, generation string, fetchedAt time.Time) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	chatIDs, _ := json.Marshal(value.ClaudeChatIDs)
	messageIDs, _ := json.Marshal(value.MessageIDs)
	_, err = s.db.Exec(`
		INSERT INTO compliance_files
		(file_id, filename, mime_type, size_bytes, md5, created_at, chat_ids_json, message_ids_json, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET filename=excluded.filename, mime_type=excluded.mime_type,
			size_bytes=excluded.size_bytes, md5=excluded.md5, created_at=excluded.created_at,
			chat_ids_json=excluded.chat_ids_json, message_ids_json=excluded.message_ids_json,
			raw=excluded.raw, fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`, value.ID, value.Filename, value.MimeType, value.SizeBytes, value.MD5, value.CreatedAt,
		string(chatIDs), string(messageIDs), string(raw), fetchedAt.Format(time.RFC3339Nano), generation)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) UpsertGeneratedFileMetadata(value *compliance.GeneratedFileMetadata, generation string, fetchedAt time.Time) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`
		INSERT INTO generated_files
		(generated_file_id, chat_id, filename, mime_type, size_bytes, md5, created_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(generated_file_id) DO UPDATE SET chat_id=excluded.chat_id, filename=excluded.filename,
			mime_type=excluded.mime_type, size_bytes=excluded.size_bytes, md5=excluded.md5,
			created_at=excluded.created_at, raw=excluded.raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`, value.ID, value.ClaudeChatID, value.Filename, value.MimeType, value.SizeBytes, value.MD5,
		value.CreatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) UpsertChatArtifactMetadata(value *compliance.ChatArtifactMetadata, generation string, fetchedAt time.Time) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`
		INSERT INTO chat_artifact_versions
		(version_id, artifact_id, chat_id, title, artifact_type, size_bytes, md5, created_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(version_id) DO UPDATE SET artifact_id=excluded.artifact_id, chat_id=excluded.chat_id,
			title=excluded.title, artifact_type=excluded.artifact_type, size_bytes=excluded.size_bytes,
			md5=excluded.md5, created_at=excluded.created_at, raw=excluded.raw,
			fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`, value.VersionID, value.ID, value.ClaudeChatID, value.Title, value.ArtifactType, value.SizeBytes,
		value.MD5, value.CreatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) UpsertProjectAttachments(projectID string, values []compliance.ProjectAttachment, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO project_attachments
		(project_id, attachment_id, attachment_type, filename, mime_type, size_bytes, md5, created_at, updated_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, attachment_id) DO UPDATE SET attachment_type=excluded.attachment_type,
			filename=excluded.filename, mime_type=excluded.mime_type, size_bytes=excluded.size_bytes,
			md5=excluded.md5, created_at=excluded.created_at, updated_at=excluded.updated_at,
			raw=excluded.raw, fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(projectID, value.ID, value.Type, value.Filename, value.MimeType, value.SizeBytes,
			value.MD5, value.CreatedAt, value.UpdatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertProjectCollaborators(projectID string, values []compliance.ProjectCollaborator, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO project_collaborators
		(project_id, collaborator_type, principal_key, role, granted_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, collaborator_type, principal_key, role) DO UPDATE SET
			granted_at=excluded.granted_at, raw=excluded.raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, value := range values {
		principal := collaboratorPrincipal(value)
		raw, _ := json.Marshal(value)
		if _, err := stmt.Exec(projectID, value.Type, principal, value.Role, value.GrantedAt,
			string(raw), fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
	}
	return len(values), tx.Commit()
}

func (s *Store) UpsertProjectDocumentMetadata(value *compliance.ProjectDocumentMetadata, generation string, fetchedAt time.Time) (int, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	var userID, email *string
	if value.User != nil {
		userID = &value.User.ID
		email = &value.User.EmailAddress
	}
	_, err = s.db.Exec(`
		INSERT INTO project_documents
		(document_id, project_id, filename, mime_type, size_bytes, md5, creator_user_id, creator_email,
		 created_at, metadata_raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(document_id) DO UPDATE SET project_id=excluded.project_id, filename=excluded.filename,
			mime_type=excluded.mime_type, size_bytes=excluded.size_bytes, md5=excluded.md5,
			creator_user_id=excluded.creator_user_id, creator_email=excluded.creator_email,
			created_at=excluded.created_at, metadata_raw=excluded.metadata_raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`, value.ID, value.ClaudeProjectID, value.Filename, value.MimeType, value.SizeBytes, value.MD5,
		userID, email, value.CreatedAt, string(raw), fetchedAt.Format(time.RFC3339Nano), generation)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *Store) UpsertProjectDocumentContent(value *compliance.ProjectDocument, fetchedAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO project_documents
		(document_id, filename, creator_user_id, creator_email, created_at, content_text, content_fetched_at, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')
		ON CONFLICT(document_id) DO UPDATE SET content_text=excluded.content_text,
			content_fetched_at=excluded.content_fetched_at
	`, value.ID, value.Filename, projectUserID(value.User), projectUserEmail(value.User), value.CreatedAt,
		value.Content, fetchedAt.Format(time.RFC3339Nano), fetchedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) UpsertCodeArtifacts(values []compliance.CodeArtifact, generation string, fetchedAt time.Time) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	artifactStmt, err := tx.Prepare(`
		INSERT INTO code_artifacts
		(artifact_id, organization_uuid, owner_user_id, owner_email, published_version_id, read_mode,
		 updated_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artifact_id) DO UPDATE SET organization_uuid=excluded.organization_uuid,
			owner_user_id=excluded.owner_user_id, owner_email=excluded.owner_email,
			published_version_id=excluded.published_version_id, read_mode=excluded.read_mode,
			updated_at=excluded.updated_at, raw=excluded.raw, fetched_at=excluded.fetched_at,
			seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer artifactStmt.Close()
	versionStmt, err := tx.Prepare(`
		INSERT INTO code_artifact_versions
		(artifact_id, version_id, name, created_at, raw, fetched_at, seen_generation)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artifact_id, version_id) DO UPDATE SET name=excluded.name, created_at=excluded.created_at,
			raw=excluded.raw, fetched_at=excluded.fetched_at, seen_generation=excluded.seen_generation
	`)
	if err != nil {
		return 0, err
	}
	defer versionStmt.Close()
	for _, value := range values {
		raw, _ := json.Marshal(value)
		if _, err := artifactStmt.Exec(value.ID, value.OrganizationUUID, value.OwnerUserID, projectUserEmail(value.User),
			value.PublishedVersionID, value.ReadMode, value.UpdatedAt, string(raw),
			fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
			return 0, err
		}
		for _, version := range value.Versions {
			versionRaw, _ := json.Marshal(version)
			if _, err := versionStmt.Exec(value.ID, version.ID, version.Name, version.CreatedAt, string(versionRaw),
				fetchedAt.Format(time.RFC3339Nano), generation); err != nil {
				return 0, err
			}
		}
	}
	return len(values), tx.Commit()
}

type ContentObjectRecord struct {
	SHA256      string
	MD5Hex      string
	SizeBytes   int64
	MimeType    string
	Backend     string
	ObjectKey   string
	LocalPath   string
	VerifiedMD5 bool
}

func (s *Store) LinkContentObject(object ContentObjectRecord, resourceType, resourceID, versionID string, fetchedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		INSERT INTO content_objects
		(sha256, md5_hex, size_bytes, mime_type, storage_backend, object_key, local_path, created_at, verified_md5)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO UPDATE SET md5_hex=excluded.md5_hex, size_bytes=excluded.size_bytes,
			mime_type=excluded.mime_type, storage_backend=excluded.storage_backend,
			object_key=excluded.object_key, local_path=excluded.local_path,
			verified_md5=MAX(content_objects.verified_md5, excluded.verified_md5)
	`, object.SHA256, object.MD5Hex, object.SizeBytes, object.MimeType, object.Backend,
		object.ObjectKey, nullableString(object.LocalPath),
		fetchedAt.Format(time.RFC3339Nano), object.VerifiedMD5); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO resource_content (resource_type, resource_id, version_id, sha256, fetched_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(resource_type, resource_id, version_id) DO UPDATE SET sha256=excluded.sha256,
			fetched_at=excluded.fetched_at
	`, resourceType, resourceID, versionID, object.SHA256, fetchedAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) HasResourceContent(resourceType, resourceID, versionID string) (bool, error) {
	var one int
	err := s.db.QueryRow(`
		SELECT 1 FROM resource_content WHERE resource_type=? AND resource_id=? AND version_id=?
	`, resourceType, resourceID, versionID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func newGeneration() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("creating sync generation: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func collaboratorPrincipal(value compliance.ProjectCollaborator) string {
	for _, candidate := range []*string{value.UserID, value.GroupID, value.OrganizationUUID, value.OrganizationRole} {
		if candidate != nil {
			return *candidate
		}
	}
	return ""
}

func projectUserID(value *compliance.ProjectUser) any {
	if value == nil {
		return nil
	}
	return value.ID
}

func projectUserEmail(value *compliance.ProjectUser) any {
	if value == nil {
		return nil
	}
	return strings.ToLower(value.EmailAddress)
}
