package analytics

import (
	"context"
	"fmt"
	"net/url"
)

// FetchSummaries retrieves org-level daily summaries (DAU, WAU, MAU, seats)
// for the given date range. The API enforces a maximum 31-day span per
// request, so callers should chunk larger ranges.
func (c *Client) FetchSummaries(
	ctx context.Context, startDate, endDate string,
) ([]DailySummary, error) {
	params := url.Values{}
	params.Set("starting_date", startDate)
	params.Set("ending_date", endDate)

	var resp SummariesResponse
	err := c.get(
		ctx,
		"/v1/organizations/analytics/summaries",
		params,
		&resp,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"fetching analytics summaries (%s to %s): %w",
			startDate, endDate, err,
		)
	}

	return resp.Summaries, nil
}
