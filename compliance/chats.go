package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"
)

// ChatQuery specifies filters for fetching chats.
type ChatQuery struct {
	UserIDs         []string
	ProjectIDs      []string
	OrganizationIDs []string
	CreatedAtGT     *time.Time
	CreatedAtGte    *time.Time
	CreatedAtLt     *time.Time
	CreatedAtLTE    *time.Time
	UpdatedAtGT     *time.Time
	UpdatedAtGTE    *time.Time
	UpdatedAtLT     *time.Time
	UpdatedAtLTE    *time.Time
	OrderBy         string
	AfterID         string
	BeforeID        string
	Limit           int
	OnPage          func([]Chat, CursorPage) error
}

// FetchChats retrieves all chats matching the query, paginating automatically.
// Results are returned chronologically by created_at (oldest first).
func (c *Client) FetchChats(ctx context.Context, opts ChatQuery) ([]Chat, error) {
	if len(opts.UserIDs) > 10 {
		return nil, fmt.Errorf("chat queries accept at most 10 user IDs")
	}
	if len(opts.ProjectIDs) > 0 && len(opts.UserIDs) == 0 {
		return nil, fmt.Errorf("project chat filters require at least one user ID")
	}
	if opts.OrderBy != "" && opts.OrderBy != "created_at" && opts.OrderBy != "updated_at" {
		return nil, fmt.Errorf("chat order_by must be created_at or updated_at")
	}
	if opts.AfterID != "" && opts.BeforeID != "" {
		return nil, fmt.Errorf("chat after_id and before_id cannot be combined")
	}
	if opts.BeforeID != "" && len(opts.UserIDs) == 0 {
		return nil, fmt.Errorf("chat before_id pagination requires at least one user ID")
	}
	limit := opts.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	var all []Chat
	afterID := opts.AfterID
	beforeID := opts.BeforeID
	backward := beforeID != ""
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
		for _, id := range opts.ProjectIDs {
			params.Add("project_ids[]", id)
		}
		if opts.OrderBy != "" {
			params.Set("order_by", opts.OrderBy)
		}
		if afterID != "" {
			params.Set("after_id", afterID)
		}
		if beforeID != "" {
			params.Set("before_id", beforeID)
		}

		var resp ChatsResponse
		if err := c.get(ctx, "/v1/compliance/apps/chats", params, &resp); err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}

		cursor := resp.LastID
		if backward {
			cursor = resp.FirstID
		}
		if !resp.HasMore {
			cursor = ""
		}
		if opts.OnPage != nil {
			if err := opts.OnPage(resp.Data, CursorPage{NextCursor: cursor, HasMore: resp.HasMore}); err != nil {
				return all, err
			}
		} else {
			all = append(all, resp.Data...)
		}
		total += len(resp.Data)
		page++

		if page == 1 || page%5 == 0 {
			fmt.Fprintf(os.Stderr, "  Page %d: %d chats\n", page, total)
		}

		if cursor == "" {
			break
		}
		if backward {
			beforeID = cursor
		} else {
			afterID = cursor
		}
	}

	return all, nil
}

// ChatMessageQuery exposes the message endpoint's optional filtering,
// truncation, ordering, and cursor controls.
type ChatMessageQuery struct {
	CreatedAtGT          *time.Time
	CreatedAtGTE         *time.Time
	CreatedAtLT          *time.Time
	CreatedAtLTE         *time.Time
	UpdatedAtGT          *time.Time
	UpdatedAtGTE         *time.Time
	UpdatedAtLT          *time.Time
	UpdatedAtLTE         *time.Time
	Order                string
	AfterID              string
	BeforeID             string
	Limit                int
	ToolResultMaxChars   *int
	ToolUseInputMaxChars *int
}

// FetchChatMessages retrieves and, when a page limit is supplied, paginates
// chat messages while preserving the chat metadata returned by the endpoint.
func (c *Client) FetchChatMessages(ctx context.Context, chatID string, opts ChatMessageQuery) (*ChatDetail, error) {
	if chatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}
	if opts.Order != "" && opts.Order != "asc" && opts.Order != "desc" {
		return nil, fmt.Errorf("message order must be asc or desc")
	}
	if opts.AfterID != "" && opts.BeforeID != "" {
		return nil, fmt.Errorf("message after_id and before_id cannot be combined")
	}
	if opts.Limit < 0 || opts.Limit > 1000 {
		return nil, fmt.Errorf("message limit must be between 0 and 1000")
	}

	endpoint := fmt.Sprintf("/v1/compliance/apps/chats/%s/messages", url.PathEscape(chatID))
	afterID := opts.AfterID
	beforeID := opts.BeforeID
	backward := beforeID != ""
	var combined *ChatDetail
	for {
		params := url.Values{}
		addTimeParam(params, "created_at.gt", opts.CreatedAtGT)
		addTimeParam(params, "created_at.gte", opts.CreatedAtGTE)
		addTimeParam(params, "created_at.lt", opts.CreatedAtLT)
		addTimeParam(params, "created_at.lte", opts.CreatedAtLTE)
		addTimeParam(params, "updated_at.gt", opts.UpdatedAtGT)
		addTimeParam(params, "updated_at.gte", opts.UpdatedAtGTE)
		addTimeParam(params, "updated_at.lt", opts.UpdatedAtLT)
		addTimeParam(params, "updated_at.lte", opts.UpdatedAtLTE)
		if opts.Order != "" {
			params.Set("order", opts.Order)
		}
		if opts.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.ToolResultMaxChars != nil {
			params.Set("tool_result_max_chars", fmt.Sprintf("%d", *opts.ToolResultMaxChars))
		}
		if opts.ToolUseInputMaxChars != nil {
			params.Set("tool_use_input_max_chars", fmt.Sprintf("%d", *opts.ToolUseInputMaxChars))
		}
		if afterID != "" {
			params.Set("after_id", afterID)
		}
		if beforeID != "" {
			params.Set("before_id", beforeID)
		}

		var page ChatDetail
		if err := c.get(ctx, endpoint, params, &page); err != nil {
			return nil, err
		}
		if combined == nil {
			combined = &page
		} else {
			combined.ChatMessages = append(combined.ChatMessages, page.ChatMessages...)
			combined.HasMore = page.HasMore
			combined.LastID = page.LastID
		}

		// Omitting limit asks the API for the full result in one response.
		if opts.Limit == 0 || !page.HasMore {
			break
		}
		cursor := page.LastID
		if backward {
			cursor = page.FirstID
		}
		if cursor == "" {
			break
		}
		if backward {
			beforeID = cursor
		} else {
			afterID = cursor
		}
	}
	return combined, nil
}

// GetChat retrieves a single chat by ID, including its full message history.
func (c *Client) GetChat(ctx context.Context, chatID string) (*ChatDetail, error) {
	chat, _, err := c.GetChatRaw(ctx, chatID)
	return chat, err
}

// GetChatRaw retrieves a chat and also returns the exact JSON response body.
func (c *Client) GetChatRaw(ctx context.Context, chatID string) (*ChatDetail, []byte, error) {
	if chatID == "" {
		return nil, nil, fmt.Errorf("chat ID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/apps/chats/%s/messages", url.PathEscape(chatID))
	params := url.Values{}
	params.Set("tool_result_max_chars", "-1")
	params.Set("tool_use_input_max_chars", "-1")
	var chat ChatDetail
	body, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return nil, nil, err
	}
	return &chat, body, nil
}

// DownloadFile retrieves the content of a file attachment. Returns the body
// (which the caller must close), the filename, and any error.
func (c *Client) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, string, error) {
	download, err := c.DownloadFileContent(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	return download.Body, download.Filename, nil
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
