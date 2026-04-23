package analytics

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

const defaultBaseURL = "https://api.anthropic.com"

// Client is an authenticated HTTP client for the Anthropic Analytics API.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// NewClient creates a Client with an explicit API key.
func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
	}
}

// DefaultOPField reads the 1Password field name from the ANALYTICS_OP_FIELD
// environment variable. There is no hard-coded fallback — the env var must
// be set when using 1Password auth without an explicit field name.
func DefaultOPField() string {
	return os.Getenv("ANALYTICS_OP_FIELD")
}

// NewClientFrom1Password retrieves the API key from 1Password CLI.
// If opItem is empty, the default item name is used. If opField is empty,
// the ANALYTICS_OP_FIELD environment variable is read; an error is returned
// if that is also unset.
func NewClientFrom1Password(opItem, opField string) (*Client, error) {
	if opItem == "" {
		opItem = os.Getenv("ANTHROPIC_OP_ITEM")
	}
	if opItem == "" {
		return nil, fmt.Errorf(
			"ANTHROPIC_OP_ITEM not set (configure in .env or pass explicitly)",
		)
	}
	if opField == "" {
		opField = DefaultOPField()
	}
	if opField == "" {
		return nil, fmt.Errorf(
			"ANALYTICS_OP_FIELD not set (configure in .env or pass --analytics-api-key)",
		)
	}

	out, err := exec.Command(
		"op", "item", "get", opItem, "--field", opField, "--reveal",
	).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf(
				"1Password CLI error: %s",
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return nil, fmt.Errorf(
			"failed to run op CLI (is it installed and signed in?): %w", err,
		)
	}

	apiKey := strings.TrimSpace(string(out))
	if apiKey == "" {
		return nil, fmt.Errorf("empty API key returned from 1Password")
	}
	return NewClient(apiKey), nil
}

// doRequest executes an authenticated GET against the Analytics API and
// returns the raw response body. It retries on HTTP 429 (rate limit) and
// HTTP 503 (transient unavailability, documented by the Analytics API).
func (c *Client) doRequest(
	ctx context.Context, endpoint string, params url.Values,
) ([]byte, error) {
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(
			ctx, http.MethodGet, u.String(), nil,
		)
		if err != nil {
			return nil, err
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
					return nil, ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusServiceUnavailable {
			wait := 5 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf(
				"HTTP %d from %s: %s",
				resp.StatusCode, endpoint, truncate(string(body), 200),
			)
		}
		return body, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("exceeded retry limit for %s", endpoint)
}

// get is a convenience wrapper that unmarshals the JSON response into dest.
func (c *Client) get(
	ctx context.Context, endpoint string,
	params url.Values, dest interface{},
) error {
	body, err := c.doRequest(ctx, endpoint, params)
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
