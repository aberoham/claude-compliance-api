package compliance

import (
	"context"
	"fmt"
	"net/url"
)

// FetchUsers retrieves all licensed users for the configured organization,
// paginating through the full result set using cursor-based pagination.
func (c *Client) FetchUsers(ctx context.Context) ([]User, error) {
	var all []User
	cursor := ""

	for {
		params := url.Values{}
		params.Set("limit", "500")
		if cursor != "" {
			params.Set("page", cursor)
		}

		var resp UsersResponse
		endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/users", c.orgID)
		if err := c.get(ctx, endpoint, params, &resp); err != nil {
			return all, err
		}

		all = append(all, resp.Data...)

		if !resp.HasMore || resp.NextPage == nil || *resp.NextPage == "" {
			break
		}
		cursor = *resp.NextPage
	}

	return all, nil
}
