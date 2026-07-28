package compliance

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

type CodeArtifactVersion struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
}

type CodeArtifact struct {
	ID                 string                `json:"id"`
	OrganizationUUID   string                `json:"organization_uuid"`
	OwnerUserID        string                `json:"owner_user_id"`
	PublishedVersionID string                `json:"published_version_id"`
	ReadMode           string                `json:"read_mode"`
	UpdatedAt          *string               `json:"updated_at"`
	User               *ProjectUser          `json:"user"`
	Versions           []CodeArtifactVersion `json:"versions"`
}

type CodeArtifactQuery struct {
	Limit           int
	OrganizationIDs []string
	UserIDs         []string
	UpdatedAtGT     *time.Time
	UpdatedAtGTE    *time.Time
	UpdatedAtLT     *time.Time
	UpdatedAtLTE    *time.Time
	Page            string
	OnPage          func([]CodeArtifact, CursorPage) error
}

func (c *Client) FetchCodeArtifacts(ctx context.Context, opts CodeArtifactQuery) ([]CodeArtifact, error) {
	params := url.Values{}
	for _, id := range opts.OrganizationIDs {
		params.Add("organization_ids[]", id)
	}
	for _, id := range opts.UserIDs {
		params.Add("user_ids[]", id)
	}
	addTimeParam(params, "updated_at.gt", opts.UpdatedAtGT)
	addTimeParam(params, "updated_at.gte", opts.UpdatedAtGTE)
	addTimeParam(params, "updated_at.lt", opts.UpdatedAtLT)
	addTimeParam(params, "updated_at.lte", opts.UpdatedAtLTE)
	return fetchAllPages[CodeArtifact](ctx, c, "/v1/compliance/apps/code/artifacts", params, ListOptions{Limit: opts.Limit, Page: opts.Page}, 20, 100, opts.OnPage)
}

func (c *Client) DownloadCodeArtifactVersion(ctx context.Context, artifactID, versionID string) (*Download, error) {
	if artifactID == "" || versionID == "" {
		return nil, fmt.Errorf("artifact ID and version ID are required")
	}
	prefix := "/v1/compliance/apps/code/artifacts/" + url.PathEscape(artifactID) + "/versions/"
	return c.download(ctx, prefix, versionID, "")
}

func (c *Client) DeleteCodeArtifact(ctx context.Context, artifactID string) (*DeletionReceipt, error) {
	return c.deleteResource(ctx, "/v1/compliance/apps/code/artifacts/", artifactID)
}

func addTimeParam(params url.Values, name string, value *time.Time) {
	if value != nil {
		params.Set(name, value.Format(time.RFC3339))
	}
}
