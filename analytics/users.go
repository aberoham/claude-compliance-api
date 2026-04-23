package analytics

import (
	"context"
	"fmt"
	"net/url"
)

// FetchUserMetrics retrieves per-user daily analytics for the given date
// (YYYY-MM-DD format). It paginates through the full result set using the
// cursor from the next_page field.
func (c *Client) FetchUserMetrics(
	ctx context.Context, date string,
) ([]UserMetrics, error) {
	var all []UserMetrics
	cursor := ""

	for {
		params := url.Values{}
		params.Set("date", date)
		params.Set("limit", "1000")
		if cursor != "" {
			params.Set("page", cursor)
		}

		var resp UsersResponse
		err := c.get(
			ctx,
			"/v1/organizations/analytics/users",
			params,
			&resp,
		)
		if err != nil {
			return all, fmt.Errorf("fetching analytics users for %s: %w", date, err)
		}

		all = append(all, resp.Data...)

		if resp.NextPage == nil || *resp.NextPage == "" {
			break
		}
		cursor = *resp.NextPage
	}

	return all, nil
}
