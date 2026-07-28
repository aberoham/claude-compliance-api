package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.anthropic.com"

// DefaultOrgID returns the org ID from ANTHROPIC_ORG_ID.
func DefaultOrgID() string { return os.Getenv("ANTHROPIC_ORG_ID") }

// DefaultOPItem returns the 1Password item name from ANTHROPIC_OP_ITEM.
func DefaultOPItem() string { return os.Getenv("ANTHROPIC_OP_ITEM") }

// DefaultOPField returns the 1Password field name from COMPLIANCE_OP_FIELD.
func DefaultOPField() string { return os.Getenv("COMPLIANCE_OP_FIELD") }

// Client is an authenticated HTTP client for the Anthropic Compliance API.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	orgID      string
}

// APIError is returned for non-successful Compliance API responses.
type APIError struct {
	StatusCode int
	Endpoint   string
	RequestID  string
	Body       string
}

func (e *APIError) Error() string {
	requestID := ""
	if e.RequestID != "" {
		requestID = fmt.Sprintf(" (request-id: %s)", e.RequestID)
	}
	return fmt.Sprintf("HTTP %d from %s%s: %s", e.StatusCode, e.Endpoint, requestID, truncate(e.Body, 500))
}

// NewClient creates a Client with an explicit API key.
func NewClient(apiKey, orgID string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		orgID:      orgID,
	}
}

// NewClientFrom1Password retrieves the API key from 1Password CLI and returns
// a ready-to-use Client. This mirrors the Python scripts' auth pattern.
func NewClientFrom1Password(opItem, opField, orgID string) (*Client, error) {
	if opItem == "" {
		opItem = DefaultOPItem()
	}
	if opItem == "" {
		return nil, fmt.Errorf("ANTHROPIC_OP_ITEM not set (configure in .env or pass explicitly)")
	}
	if opField == "" {
		opField = DefaultOPField()
	}
	if opField == "" {
		return nil, fmt.Errorf("COMPLIANCE_OP_FIELD not set (configure in .env or pass explicitly)")
	}
	if orgID == "" {
		orgID = DefaultOrgID()
	}

	out, err := exec.Command("op", "item", "get", opItem, "--field", opField, "--reveal").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("1Password CLI error: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("failed to run op CLI (is it installed and signed in?): %w", err)
	}

	apiKey := strings.TrimSpace(string(out))
	if apiKey == "" {
		return nil, fmt.Errorf("empty API key returned from 1Password")
	}
	return NewClient(apiKey, orgID), nil
}

// OrgID returns the organization ID this client is configured for.
func (c *Client) OrgID() string {
	return c.orgID
}

// doRequest executes an authenticated GET against the Compliance API.
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	body, _, err := c.doRequestMethod(ctx, http.MethodGet, endpoint, params)
	return body, err
}

// doRequestMethod executes an authenticated request and returns the response
// body and headers. It retries documented transient statuses and honors
// Retry-After. Callers should still make destructive operations explicit at
// their own user-interface boundary.
func (c *Client) doRequestMethod(ctx context.Context, method, endpoint string, params url.Values) ([]byte, http.Header, error) {
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isTimeout(err) {
				lastErr = fmt.Errorf("request to %s failed: %w", endpoint, err)
				wait := time.Duration(attempt+1) * 10 * time.Second
				fmt.Fprintf(os.Stderr, "  Timeout (attempt %d/5), retrying in %v...\n", attempt+1, wait)
				select {
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return nil, nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("reading response body: %w", err)
		}

		if shouldRetry(resp.StatusCode, resp.Header) && attempt < 4 {
			wait := retryDelay(resp.Header.Get("Retry-After"), attempt)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		if resp.StatusCode >= 400 {
			return nil, resp.Header.Clone(), &APIError{
				StatusCode: resp.StatusCode,
				Endpoint:   endpoint,
				RequestID:  resp.Header.Get("request-id"),
				Body:       string(body),
			}
		}
		return body, resp.Header.Clone(), nil
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}
	return nil, nil, fmt.Errorf("exceeded retry limit for %s", endpoint)
}

func shouldRetry(status int, header http.Header) bool {
	if status == http.StatusInternalServerError && strings.EqualFold(header.Get("x-should-retry"), "false") {
		return false
	}
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError ||
		status == http.StatusBadGateway || status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout || status == 529
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
		if when, err := http.ParseTime(retryAfter); err == nil {
			if wait := time.Until(when); wait > 0 {
				return wait
			}
			return 0
		}
	}
	return time.Duration(1<<attempt) * time.Second
}

// get is a convenience wrapper that unmarshals the JSON response into dest.
func (c *Client) get(ctx context.Context, endpoint string, params url.Values, dest interface{}) error {
	body, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func (c *Client) delete(ctx context.Context, endpoint string, dest interface{}) error {
	body, _, err := c.doRequestMethod(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
