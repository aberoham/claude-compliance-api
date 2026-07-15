package compliance

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"
)

// ProjectQuery specifies filters for fetching projects.
type ProjectQuery struct {
	UserIDs         []string
	OrganizationIDs []string
	CreatedAtGT     *time.Time
	CreatedAtGte    *time.Time
	CreatedAtLt     *time.Time
	CreatedAtLTE    *time.Time
	UpdatedAtGT     *time.Time
	UpdatedAtGTE    *time.Time
	UpdatedAtLT     *time.Time
	UpdatedAtLTE    *time.Time
	Limit           int
	Page            string
	OnPage          func([]Project, CursorPage) error
}

// FetchProjects retrieves all projects matching the query, paginating automatically.
// Results are returned in reverse chronological order (newest first).
func (c *Client) FetchProjects(ctx context.Context, opts ProjectQuery) ([]Project, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	var all []Project
	cursor := opts.Page
	page := 0
	total := 0

	for {
		params := url.Values{}
		params.Set("limit", fmt.Sprintf("%d", limit))

		organizationIDs := opts.OrganizationIDs
		if len(organizationIDs) == 0 && c.orgID != "" {
			organizationIDs = []string{c.orgID}
		}
		for _, id := range organizationIDs {
			params.Add("organization_ids[]", id)
		}
		addTimeParam(params, "created_at.gt", opts.CreatedAtGT)
		if opts.CreatedAtGte != nil {
			params.Set("created_at.gte", opts.CreatedAtGte.Format(time.RFC3339))
		}
		if opts.CreatedAtLt != nil {
			params.Set("created_at.lt", opts.CreatedAtLt.Format(time.RFC3339))
		}
		addTimeParam(params, "created_at.lte", opts.CreatedAtLTE)
		addTimeParam(params, "updated_at.gt", opts.UpdatedAtGT)
		addTimeParam(params, "updated_at.gte", opts.UpdatedAtGTE)
		addTimeParam(params, "updated_at.lt", opts.UpdatedAtLT)
		addTimeParam(params, "updated_at.lte", opts.UpdatedAtLTE)
		for _, id := range opts.UserIDs {
			params.Add("user_ids[]", id)
		}
		if cursor != "" {
			params.Set("page", cursor)
		}

		var resp ProjectsResponse
		if err := c.get(ctx, "/v1/compliance/apps/projects", params, &resp); err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}

		next := ""
		if resp.NextPage != nil {
			next = *resp.NextPage
		}
		if opts.OnPage != nil {
			if err := opts.OnPage(resp.Data, CursorPage{NextCursor: next, HasMore: resp.HasMore}); err != nil {
				return all, err
			}
		} else {
			all = append(all, resp.Data...)
		}
		total += len(resp.Data)
		page++

		if page == 1 || page%5 == 0 {
			fmt.Fprintf(os.Stderr, "  Page %d: %d projects\n", page, total)
		}

		if next == "" {
			break
		}
		cursor = next
	}

	return all, nil
}

// GetProject retrieves a single project by ID, including its full details.
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/apps/projects/%s", url.PathEscape(projectID))
	var project Project
	if err := c.get(ctx, endpoint, nil, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ProjectAttachment is a discriminated union. Type is project_file or
// project_doc; fields that do not apply to that variant remain nil.
type ProjectAttachment struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	Filename  string  `json:"filename"`
	MD5       *string `json:"md5,omitempty"`
	MimeType  string  `json:"mime_type"`
	SizeBytes *int64  `json:"size_bytes,omitempty"`
	Type      string  `json:"type"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

// ProjectCollaborator is a user, group, organization, or organization-role
// grant. Type determines which principal identifier is populated.
type ProjectCollaborator struct {
	GrantedAt        string  `json:"granted_at"`
	Role             string  `json:"role"`
	Type             string  `json:"type"`
	UserID           *string `json:"user_id,omitempty"`
	GroupID          *string `json:"group_id,omitempty"`
	OrganizationUUID *string `json:"organization_uuid,omitempty"`
	OrganizationRole *string `json:"organization_role,omitempty"`
}

type ProjectDocument struct {
	ID        string       `json:"id"`
	Content   string       `json:"content"`
	CreatedAt string       `json:"created_at"`
	Filename  string       `json:"filename"`
	User      *ProjectUser `json:"user"`
}

type ProjectDocumentMetadata struct {
	ID              string       `json:"id"`
	ClaudeProjectID string       `json:"claude_project_id"`
	CreatedAt       string       `json:"created_at"`
	Filename        string       `json:"filename"`
	MD5             string       `json:"md5"`
	MimeType        string       `json:"mime_type"`
	SizeBytes       int64        `json:"size_bytes"`
	User            *ProjectUser `json:"user"`
}

func (c *Client) DeleteProject(ctx context.Context, projectID string) (*DeletionReceipt, error) {
	return c.deleteResource(ctx, "/v1/compliance/apps/projects/", projectID)
}

func (c *Client) FetchProjectAttachments(ctx context.Context, projectID string, opts ListOptions) ([]ProjectAttachment, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	endpoint := "/v1/compliance/apps/projects/" + url.PathEscape(projectID) + "/attachments"
	return fetchAllPages[ProjectAttachment](ctx, c, endpoint, nil, opts, 20, 100, nil)
}

func (c *Client) ForEachProjectAttachmentPage(ctx context.Context, projectID string, opts ListOptions, fn func([]ProjectAttachment, CursorPage) error) error {
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}
	endpoint := "/v1/compliance/apps/projects/" + url.PathEscape(projectID) + "/attachments"
	_, err := fetchAllPages[ProjectAttachment](ctx, c, endpoint, nil, opts, 20, 100, fn)
	return err
}

func (c *Client) FetchProjectCollaborators(ctx context.Context, projectID string, opts ListOptions) ([]ProjectCollaborator, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	endpoint := "/v1/compliance/apps/projects/" + url.PathEscape(projectID) + "/collaborators"
	return fetchAllPages[ProjectCollaborator](ctx, c, endpoint, nil, opts, 20, 100, nil)
}

func (c *Client) ForEachProjectCollaboratorPage(ctx context.Context, projectID string, opts ListOptions, fn func([]ProjectCollaborator, CursorPage) error) error {
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}
	endpoint := "/v1/compliance/apps/projects/" + url.PathEscape(projectID) + "/collaborators"
	_, err := fetchAllPages[ProjectCollaborator](ctx, c, endpoint, nil, opts, 20, 100, fn)
	return err
}

func (c *Client) GetProjectDocument(ctx context.Context, documentID string) (*ProjectDocument, error) {
	if documentID == "" {
		return nil, fmt.Errorf("document ID is required")
	}
	var document ProjectDocument
	endpoint := "/v1/compliance/apps/projects/documents/" + url.PathEscape(documentID)
	if err := c.get(ctx, endpoint, nil, &document); err != nil {
		return nil, err
	}
	return &document, nil
}

func (c *Client) GetProjectDocumentMetadata(ctx context.Context, documentID string) (*ProjectDocumentMetadata, error) {
	if documentID == "" {
		return nil, fmt.Errorf("document ID is required")
	}
	var metadata ProjectDocumentMetadata
	endpoint := "/v1/compliance/apps/projects/documents/" + url.PathEscape(documentID) + "/metadata"
	if err := c.get(ctx, endpoint, nil, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) DeleteProjectDocument(ctx context.Context, documentID string) (*DeletionReceipt, error) {
	return c.deleteResource(ctx, "/v1/compliance/apps/projects/documents/", documentID)
}
