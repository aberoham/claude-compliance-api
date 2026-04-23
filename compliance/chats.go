package compliance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// ChatQuery specifies filters for fetching chats.
type ChatQuery struct {
	UserIDs      []string   // Filter by user IDs
	ProjectIDs   []string   // Filter by project IDs
	CreatedAtGte *time.Time // Filter by created_at >= this time
	CreatedAtLt  *time.Time // Filter by created_at < this time
	Limit        int        // Per-page limit (default 100)
}

// FetchChats retrieves all chats matching the query, paginating automatically.
// Results are returned in reverse chronological order (newest first).
func (c *Client) FetchChats(ctx context.Context, opts ChatQuery) ([]Chat, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	var all []Chat
	afterID := ""
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
		for _, id := range opts.ProjectIDs {
			params.Add("project_ids[]", id)
		}
		if afterID != "" {
			params.Set("after_id", afterID)
		}

		var resp ChatsResponse
		if err := c.get(ctx, "/v1/compliance/apps/chats", params, &resp); err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}

		all = append(all, resp.Data...)
		page++

		if page == 1 || page%5 == 0 {
			fmt.Fprintf(os.Stderr, "  Page %d: %d chats\n", page, len(all))
		}

		if !resp.HasMore || resp.LastID == "" {
			break
		}
		afterID = resp.LastID
	}

	return all, nil
}

// GetChat retrieves a single chat by ID, including its full message history.
func (c *Client) GetChat(ctx context.Context, chatID string) (*ChatDetail, error) {
	endpoint := fmt.Sprintf("/v1/compliance/apps/chats/%s/messages", chatID)
	var chat ChatDetail
	if err := c.get(ctx, endpoint, nil, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// DownloadFile retrieves the content of a file attachment. Returns the body
// (which the caller must close), the filename, and any error.
func (c *Client) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, string, error) {
	endpoint := fmt.Sprintf("/v1/compliance/apps/chats/files/%s/content", fileID)
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, "", fmt.Errorf("invalid endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Extract filename from Content-Disposition header if present.
	filename := fileID
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := parseContentDisposition(cd); err == nil {
			if fn, ok := params["filename"]; ok {
				filename = fn
			}
		}
	}

	return resp.Body, filename, nil
}

// parseContentDisposition parses a Content-Disposition header value.
func parseContentDisposition(s string) (string, map[string]string, error) {
	params := make(map[string]string)
	disposition := ""

	// Simple parsing: split by semicolon, first part is disposition type.
	parts := splitParams(s)
	if len(parts) > 0 {
		disposition = parts[0]
	}

	for _, part := range parts[1:] {
		if idx := indexOf(part, '='); idx >= 0 {
			key := trimSpace(part[:idx])
			value := trimSpace(part[idx+1:])
			// Remove surrounding quotes if present.
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			params[key] = value
		}
	}

	return disposition, params, nil
}

func splitParams(s string) []string {
	var parts []string
	var current []byte
	inQuote := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
		}
		if c == ';' && !inQuote {
			parts = append(parts, string(current))
			current = nil
		} else {
			current = append(current, c)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
