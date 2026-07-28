// Package cachesync mirrors the current Compliance API resource graph into
// SQLite and optionally downloads immutable content into a blob store.
package cachesync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aberoham/claude-compliance-api/compliance"
	"github.com/aberoham/claude-compliance-api/contentstore"
	"github.com/aberoham/claude-compliance-api/store"
)

type ContentMode string

const (
	ContentNone ContentMode = "none"
	ContentText ContentMode = "text"
	ContentAll  ContentMode = "all"
)

type Options struct {
	Content         ContentMode
	RefreshContent  bool
	MaxContentBytes int64
	SkipChats       bool
	Report          func(string, ...any)
}

type Syncer struct {
	Client  *compliance.Client
	DB      *store.Store
	Objects contentstore.Store
	Options Options

	seenFiles          map[string]bool
	seenGeneratedFiles map[string]bool
	seenArtifacts      map[string]bool
}

func (s *Syncer) Run(ctx context.Context) error {
	if s.Client == nil || s.DB == nil {
		return fmt.Errorf("Compliance client and SQLite store are required")
	}
	if s.Options.Content == "" {
		s.Options.Content = ContentNone
	}
	if s.Options.Content != ContentNone && s.Options.Content != ContentText && s.Options.Content != ContentAll {
		return fmt.Errorf("invalid content mode %q", s.Options.Content)
	}
	if s.Options.Content == ContentAll && s.Objects == nil {
		return fmt.Errorf("content mode all requires an object store")
	}
	s.seenFiles = make(map[string]bool)
	s.seenGeneratedFiles = make(map[string]bool)
	s.seenArtifacts = make(map[string]bool)

	organizations, err := s.syncOrganizations(ctx)
	if err != nil {
		return err
	}
	for _, organization := range organizations {
		if err := s.syncOrganization(ctx, organization.UUID); err != nil {
			return err
		}
	}
	if err := s.syncGroups(ctx); err != nil {
		return err
	}
	if err := s.syncProjects(ctx); err != nil {
		return err
	}
	if !s.Options.SkipChats {
		if err := s.syncChats(ctx); err != nil {
			return err
		}
	}
	if err := s.syncCodeArtifacts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Syncer) report(format string, args ...any) {
	if s.Options.Report != nil {
		s.Options.Report(format, args...)
	}
}

func (s *Syncer) syncOrganizations(ctx context.Context) ([]compliance.Organization, error) {
	run, err := s.DB.BeginResourceSync("organizations", "", "snapshot")
	if err != nil {
		return nil, err
	}
	err = s.Client.ForEachOrganizationPage(ctx, compliance.ListOptions{Page: run.Cursor}, func(page []compliance.Organization, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertOrganizations(page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return nil, fmt.Errorf("syncing organizations: %w", err)
	}
	if err := s.DB.CompleteResourceSync(run, true); err != nil {
		return nil, err
	}
	s.report("organizations: %d", run.Stored)
	return s.DB.CachedOrganizations()
}

func (s *Syncer) syncOrganization(ctx context.Context, orgID string) error {
	usersRun, err := s.DB.BeginResourceSync("organization_memberships", orgID, "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachOrganizationUserPage(ctx, orgID, compliance.ListOptions{Page: usersRun.Cursor}, func(page []compliance.User, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertOrganizationMemberships(orgID, page, usersRun.Generation, time.Now().UTC())
		if err == nil {
			err = s.DB.InsertUsers(page, time.Now().UTC())
		}
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(usersRun, cursor.NextCursor, len(page), stored)
	})
	if err != nil {
		_ = s.DB.FailResourceSync(usersRun, err)
		return fmt.Errorf("syncing users for %s: %w", orgID, err)
	}
	if err := s.DB.CompleteResourceSync(usersRun, true); err != nil {
		return err
	}

	rolesRun, err := s.DB.BeginResourceSync("roles", orgID, "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachRolePage(ctx, orgID, compliance.ListOptions{Page: rolesRun.Cursor}, func(page []compliance.Role, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertRoles(orgID, page, rolesRun.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(rolesRun, cursor.NextCursor, len(page), stored)
	})
	if err != nil {
		_ = s.DB.FailResourceSync(rolesRun, err)
		return fmt.Errorf("syncing roles for %s: %w", orgID, err)
	}
	if err := s.DB.CompleteResourceSync(rolesRun, true); err != nil {
		return err
	}
	roles, err := s.DB.CachedRoles(orgID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if err := s.syncRolePermissions(ctx, orgID, role.ID); err != nil {
			return err
		}
	}

	settingsRun, err := s.DB.BeginResourceSync("organization_settings", orgID, "snapshot")
	if err != nil {
		return err
	}
	settings, err := s.Client.GetOrganizationSettings(ctx, orgID)
	if err != nil {
		_ = s.DB.FailResourceSync(settingsRun, err)
		return fmt.Errorf("syncing settings for %s: %w", orgID, err)
	}
	stored, err := s.DB.UpsertOrganizationSettings(settings, settingsRun.Generation, time.Now().UTC())
	if err == nil {
		err = s.DB.CheckpointResourceSync(settingsRun, "", 1, stored)
	}
	if err != nil {
		_ = s.DB.FailResourceSync(settingsRun, err)
		return err
	}
	return s.DB.CompleteResourceSync(settingsRun, false)
}

func (s *Syncer) syncRolePermissions(ctx context.Context, orgID, roleID string) error {
	scope := orgID + "/" + roleID
	run, err := s.DB.BeginResourceSync("role_permissions", scope, "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachRolePermissionPage(ctx, orgID, roleID, compliance.ListOptions{Page: run.Cursor}, func(page []compliance.RolePermission, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertRolePermissions(orgID, roleID, page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return fmt.Errorf("syncing permissions for role %s: %w", roleID, err)
	}
	return s.DB.CompleteResourceSync(run, true)
}

func (s *Syncer) syncGroups(ctx context.Context) error {
	run, err := s.DB.BeginResourceSync("groups", "", "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachGroupPage(ctx, compliance.GroupQuery{}, run.Cursor, func(page []compliance.Group, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertGroups(page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return fmt.Errorf("syncing groups: %w", err)
	}
	if err := s.DB.CompleteResourceSync(run, true); err != nil {
		return err
	}
	groups, err := s.DB.CachedGroups()
	if err != nil {
		return err
	}
	for _, group := range groups {
		membersRun, err := s.DB.BeginResourceSync("group_members", group.ID, "snapshot")
		if err != nil {
			return err
		}
		err = s.Client.ForEachGroupMemberPage(ctx, group.ID, compliance.ListOptions{Page: membersRun.Cursor}, func(page []compliance.GroupMember, cursor compliance.CursorPage) error {
			stored, err := s.DB.UpsertGroupMembers(group.ID, page, membersRun.Generation, time.Now().UTC())
			if err != nil {
				return err
			}
			return s.DB.CheckpointResourceSync(membersRun, cursor.NextCursor, len(page), stored)
		})
		if err != nil {
			_ = s.DB.FailResourceSync(membersRun, err)
			return fmt.Errorf("syncing members for group %s: %w", group.ID, err)
		}
		if err := s.DB.CompleteResourceSync(membersRun, true); err != nil {
			return err
		}
	}
	s.report("groups: %d", run.Stored)
	return nil
}

func (s *Syncer) syncProjects(ctx context.Context) error {
	run, err := s.DB.BeginResourceSync("projects", "", "snapshot")
	if err != nil {
		return err
	}
	_, err = s.Client.FetchProjects(ctx, compliance.ProjectQuery{Page: run.Cursor, OnPage: func(page []compliance.Project, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertProjectsSnapshot(page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		for _, project := range page {
			if err := s.syncProjectChildren(ctx, project.ID); err != nil {
				return err
			}
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	}})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return fmt.Errorf("syncing projects: %w", err)
	}
	if err := s.DB.CompleteResourceSync(run, true); err != nil {
		return err
	}
	s.report("projects: %d", run.Stored)
	return nil
}

func (s *Syncer) syncProjectChildren(ctx context.Context, projectID string) error {
	attachmentsRun, err := s.DB.BeginResourceSync("project_attachments", projectID, "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachProjectAttachmentPage(ctx, projectID, compliance.ListOptions{Page: attachmentsRun.Cursor}, func(page []compliance.ProjectAttachment, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertProjectAttachments(projectID, page, attachmentsRun.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		for _, attachment := range page {
			if attachment.Type == "project_doc" {
				metadata, err := s.Client.GetProjectDocumentMetadata(ctx, attachment.ID)
				if isUnavailable(err) {
					s.report("project document %s is no longer available", attachment.ID)
					continue
				}
				if err != nil {
					return err
				}
				if _, err := s.DB.UpsertProjectDocumentMetadata(metadata, attachmentsRun.Generation, time.Now().UTC()); err != nil {
					return err
				}
				if s.Options.Content == ContentText || s.Options.Content == ContentAll {
					doc, err := s.Client.GetProjectDocument(ctx, attachment.ID)
					if isUnavailable(err) {
						s.report("project document content %s is no longer available", attachment.ID)
						continue
					}
					if err != nil {
						return err
					}
					if err := s.DB.UpsertProjectDocumentContent(doc, time.Now().UTC()); err != nil {
						return err
					}
				}
			} else if attachment.Type == "project_file" {
				if err := s.syncFile(ctx, attachment.ID, attachmentsRun.Generation); err != nil {
					return err
				}
			}
		}
		return s.DB.CheckpointResourceSync(attachmentsRun, cursor.NextCursor, len(page), stored)
	})
	if isUnavailable(err) {
		s.report("attachments for project %s are no longer available", projectID)
		err = nil
	}
	if err != nil {
		_ = s.DB.FailResourceSync(attachmentsRun, err)
		return fmt.Errorf("syncing attachments for project %s: %w", projectID, err)
	}
	if err := s.DB.CompleteResourceSync(attachmentsRun, true); err != nil {
		return err
	}

	collaboratorsRun, err := s.DB.BeginResourceSync("project_collaborators", projectID, "snapshot")
	if err != nil {
		return err
	}
	err = s.Client.ForEachProjectCollaboratorPage(ctx, projectID, compliance.ListOptions{Page: collaboratorsRun.Cursor}, func(page []compliance.ProjectCollaborator, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertProjectCollaborators(projectID, page, collaboratorsRun.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.DB.CheckpointResourceSync(collaboratorsRun, cursor.NextCursor, len(page), stored)
	})
	if isUnavailable(err) {
		s.report("collaborators for project %s are no longer available", projectID)
		err = nil
	}
	if err != nil {
		_ = s.DB.FailResourceSync(collaboratorsRun, err)
		return fmt.Errorf("syncing collaborators for project %s: %w", projectID, err)
	}
	return s.DB.CompleteResourceSync(collaboratorsRun, true)
}

func (s *Syncer) syncChats(ctx context.Context) error {
	run, err := s.DB.BeginResourceSync("chats", "", "snapshot")
	if err != nil {
		return err
	}
	_, err = s.Client.FetchChats(ctx, compliance.ChatQuery{AfterID: run.Cursor, OnPage: func(page []compliance.Chat, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertChatsSnapshot(page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		for _, chat := range page {
			detail, raw, err := s.Client.GetChatRaw(ctx, chat.ID)
			if isUnavailable(err) {
				s.report("chat %s is no longer available", chat.ID)
				continue
			}
			if err != nil {
				return err
			}
			if err := s.DB.InsertChatTranscript(detail, raw, time.Now().UTC()); err != nil {
				return err
			}
			if err := s.syncMessageResources(ctx, detail, run.Generation); err != nil {
				return err
			}
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	}})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return fmt.Errorf("syncing chats: %w", err)
	}
	if err := s.DB.CompleteResourceSync(run, true); err != nil {
		return err
	}
	s.report("chats: %d", run.Stored)
	return nil
}

func (s *Syncer) syncMessageResources(ctx context.Context, chat *compliance.ChatDetail, generation string) error {
	for _, message := range chat.ChatMessages {
		for _, file := range message.Files {
			if err := s.syncFile(ctx, file.ID, generation); err != nil {
				return err
			}
		}
		for _, file := range message.GeneratedFiles {
			if s.seenGeneratedFiles[file.ID] {
				continue
			}
			s.seenGeneratedFiles[file.ID] = true
			metadata, err := s.Client.GetGeneratedFileMetadata(ctx, file.ID)
			if isUnavailable(err) {
				s.report("generated file %s is no longer available", file.ID)
				continue
			}
			if err != nil {
				return err
			}
			if _, err := s.DB.UpsertGeneratedFileMetadata(metadata, generation, time.Now().UTC()); err != nil {
				return err
			}
			if s.Options.Content == ContentAll {
				if err := s.download(ctx, "generated_file", file.ID, "", metadata.MD5, func() (*compliance.Download, error) { return s.Client.DownloadGeneratedFile(ctx, file.ID) }); err != nil {
					return err
				}
			}
		}
		for _, artifact := range message.Artifacts {
			if s.seenArtifacts[artifact.VersionID] {
				continue
			}
			s.seenArtifacts[artifact.VersionID] = true
			metadata, err := s.Client.GetChatArtifactMetadata(ctx, artifact.VersionID)
			if isUnavailable(err) {
				s.report("chat artifact version %s is no longer available", artifact.VersionID)
				continue
			}
			if err != nil {
				return err
			}
			if _, err := s.DB.UpsertChatArtifactMetadata(metadata, generation, time.Now().UTC()); err != nil {
				return err
			}
			if s.Options.Content == ContentAll {
				md5 := metadata.MD5
				if err := s.download(ctx, "chat_artifact", metadata.ID, metadata.VersionID, &md5, func() (*compliance.Download, error) { return s.Client.DownloadChatArtifact(ctx, artifact.VersionID) }); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Syncer) syncFile(ctx context.Context, fileID, generation string) error {
	if s.seenFiles[fileID] {
		return nil
	}
	s.seenFiles[fileID] = true
	metadata, err := s.Client.GetFileMetadata(ctx, fileID)
	if isUnavailable(err) {
		s.report("file %s is no longer available", fileID)
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := s.DB.UpsertFileMetadata(metadata, generation, time.Now().UTC()); err != nil {
		return err
	}
	if s.Options.Content == ContentAll {
		return s.download(ctx, "file", fileID, "", metadata.MD5, func() (*compliance.Download, error) { return s.Client.DownloadFileContent(ctx, fileID) })
	}
	return nil
}

func (s *Syncer) syncCodeArtifacts(ctx context.Context) error {
	run, err := s.DB.BeginResourceSync("code_artifacts", "", "snapshot")
	if err != nil {
		return err
	}
	_, err = s.Client.FetchCodeArtifacts(ctx, compliance.CodeArtifactQuery{Page: run.Cursor, OnPage: func(page []compliance.CodeArtifact, cursor compliance.CursorPage) error {
		stored, err := s.DB.UpsertCodeArtifacts(page, run.Generation, time.Now().UTC())
		if err != nil {
			return err
		}
		if s.Options.Content == ContentAll {
			for _, artifact := range page {
				for _, version := range artifact.Versions {
					artifactID, versionID := artifact.ID, version.ID
					if err := s.download(ctx, "code_artifact", artifactID, versionID, nil, func() (*compliance.Download, error) {
						return s.Client.DownloadCodeArtifactVersion(ctx, artifactID, versionID)
					}); err != nil {
						return err
					}
				}
			}
		}
		return s.DB.CheckpointResourceSync(run, cursor.NextCursor, len(page), stored)
	}})
	if err != nil {
		_ = s.DB.FailResourceSync(run, err)
		return fmt.Errorf("syncing code artifacts: %w", err)
	}
	if err := s.DB.CompleteResourceSync(run, true); err != nil {
		return err
	}
	s.report("code artifacts: %d", run.Stored)
	return nil
}

func (s *Syncer) download(ctx context.Context, resourceType, resourceID, versionID string, expectedMD5 *string, fetch func() (*compliance.Download, error)) error {
	if !s.Options.RefreshContent {
		exists, err := s.DB.HasResourceContent(resourceType, resourceID, versionID)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
	download, err := fetch()
	if isUnavailable(err) {
		s.report("%s %s content is no longer available", resourceType, resourceID)
		return nil
	}
	if err != nil {
		return err
	}
	defer download.Body.Close()
	md5 := download.ContentMD5
	if md5 == "" && expectedMD5 != nil {
		md5 = strings.TrimSpace(*expectedMD5)
	}
	object, err := s.Objects.Put(ctx, download.Body, contentstore.PutOptions{ExpectedMD5: md5, MIMEType: download.ContentType, MaxBytes: s.Options.MaxContentBytes})
	if err != nil {
		return err
	}
	return s.DB.LinkContentObject(store.ContentObjectRecord{
		SHA256: object.SHA256, MD5Hex: object.MD5Hex, SizeBytes: object.SizeBytes,
		MimeType: object.MIMEType, Backend: object.Backend, ObjectKey: object.ObjectKey,
		LocalPath: object.LocalPath, VerifiedMD5: object.VerifiedMD5,
	}, resourceType, resourceID, versionID, time.Now().UTC())
}

func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var apiError *compliance.APIError
	return errors.As(err, &apiError) && (apiError.StatusCode == 404 || apiError.StatusCode == 410)
}
