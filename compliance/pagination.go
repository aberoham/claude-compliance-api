package compliance

import (
	"context"
	"fmt"
	"net/url"
)

// ListOptions controls page size for cursor-paginated Compliance endpoints.
// Fetch methods always continue until next_page is absent.
type ListOptions struct {
	Limit int
	// Page resumes an interrupted next_page/page traversal. Callers should
	// persist the NextCursor supplied to the page callback after committing
	// that page.
	Page string
}

// CursorPage describes one committed unit of a cursor-paginated response.
// NextCursor is empty after the final page.
type CursorPage struct {
	NextCursor string
	HasMore    bool
}

type pageCallback[T any] func([]T, CursorPage) error

type pageResponse[T any] struct {
	Data     []T     `json:"data"`
	HasMore  bool    `json:"has_more"`
	NextPage *string `json:"next_page"`
}

func fetchAllPages[T any](ctx context.Context, c *Client, endpoint string, params url.Values, opts ListOptions, defaultLimit, maxLimit int, onPage pageCallback[T]) ([]T, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	var all []T
	page := opts.Page
	for {
		if page == "" {
			params.Del("page")
		} else {
			params.Set("page", page)
		}
		var response pageResponse[T]
		if err := c.get(ctx, endpoint, params, &response); err != nil {
			return all, err
		}
		// Some endpoints document short or empty intermediate pages. The
		// presence of next_page, rather than data length, is authoritative.
		next := ""
		if response.NextPage != nil {
			next = *response.NextPage
		}
		if onPage != nil {
			if err := onPage(response.Data, CursorPage{NextCursor: next, HasMore: response.HasMore}); err != nil {
				return all, err
			}
		} else {
			all = append(all, response.Data...)
		}
		if next == "" {
			break
		}
		page = next
	}
	return all, nil
}
