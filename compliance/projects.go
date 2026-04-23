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
	UserIDs      []string   // Filter by creator user IDs
	CreatedAtGte *time.Time // Filter by created_at >= this time
	CreatedAtLt  *time.Time // Filter by created_at < this time
	Limit        int        // Per-page limit (default 100)
}

// FetchProjects retrieves all projects matching the query, paginating automatically.
// Results are returned in reverse chronological order (newest first).
func (c *Client) FetchProjects(ctx context.Context, opts ProjectQuery) ([]Project, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	var all []Project
	cursor := ""
	page := 0

	for {
		params := url.Values{}
		params.Set("limit", fmt.Sprintf("%d", limit))

		if c.orgID != "" {
			params.Add("organization_ids[]", c.orgID)
		}
		if opts.CreatedAtGte != nil {
			params.Set("created_at.gte", opts.CreatedAtGte.Format(time.RFC3339))
		}
		if opts.CreatedAtLt != nil {
			params.Set("created_at.lt", opts.CreatedAtLt.Format(time.RFC3339))
		}
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

		all = append(all, resp.Data...)
		page++

		if page == 1 || page%5 == 0 {
			fmt.Fprintf(os.Stderr, "  Page %d: %d projects\n", page, len(all))
		}

		if !resp.HasMore || resp.NextPage == nil || *resp.NextPage == "" {
			break
		}
		cursor = *resp.NextPage
	}

	return all, nil
}

// GetProject retrieves a single project by ID, including its full details.
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	endpoint := fmt.Sprintf("/v1/compliance/apps/projects/%s", projectID)
	var project Project
	if err := c.get(ctx, endpoint, nil, &project); err != nil {
		return nil, err
	}
	return &project, nil
}
