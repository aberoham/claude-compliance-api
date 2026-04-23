package okta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Client is an authenticated HTTP client for the Okta System Log API.
type Client struct {
	httpClient      *http.Client
	apiToken        string
	domain          string
	baseURLOverride string // for testing; bypasses https:// + domain
}

// NewClient creates a Client with an explicit API token and domain.
func NewClient(apiToken, domain string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		apiToken:   apiToken,
		domain:     domain,
	}
}

// DefaultDomain reads the Okta tenant domain from OKTA_DOMAIN.
func DefaultDomain() string {
	return os.Getenv("OKTA_DOMAIN")
}

// DefaultOPItem reads the 1Password item name from OKTA_OP_ITEM.
func DefaultOPItem() string {
	return os.Getenv("OKTA_OP_ITEM")
}

// DefaultOPField reads the 1Password field name from OKTA_OP_FIELD.
func DefaultOPField() string {
	return os.Getenv("OKTA_OP_FIELD")
}

// NewClientFrom1Password retrieves the API token from 1Password CLI.
// If opItem or opField are empty, the OKTA_OP_ITEM and OKTA_OP_FIELD
// environment variables are used. If domain is empty, OKTA_DOMAIN is
// used.
func NewClientFrom1Password(
	opItem, opField, domain string,
) (*Client, error) {
	if opItem == "" {
		opItem = DefaultOPItem()
	}
	if opItem == "" {
		return nil, fmt.Errorf(
			"OKTA_OP_ITEM not set (configure in .env or pass explicitly)",
		)
	}
	if opField == "" {
		opField = DefaultOPField()
	}
	if opField == "" {
		return nil, fmt.Errorf(
			"OKTA_OP_FIELD not set (configure in .env or pass --okta-api-key)",
		)
	}
	if domain == "" {
		domain = DefaultDomain()
	}
	if domain == "" {
		return nil, fmt.Errorf(
			"OKTA_DOMAIN not set (configure in .env)",
		)
	}

	out, err := exec.Command(
		"op", "item", "get", opItem, "--field", opField, "--reveal",
	).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf(
				"1Password CLI error: %s",
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return nil, fmt.Errorf(
			"failed to run op CLI (is it installed and signed in?): %w",
			err,
		)
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return nil, fmt.Errorf("empty API token returned from 1Password")
	}
	return NewClient(token, domain), nil
}

// doRequest executes an authenticated GET against the Okta API and
// returns the raw response body plus the next page URL (from the Link
// header). An empty nextURL signals the last page.
//
// Okta rate limits use X-Rate-Limit-Reset (Unix epoch) rather than
// Retry-After, so the wait calculation differs from the Anthropic
// clients.
func (c *Client) doRequest(
	ctx context.Context, fullURL string,
) (body []byte, nextURL string, err error) {
	var lastErr error
	for attempt := range 5 {
		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodGet, fullURL, nil,
		)
		if reqErr != nil {
			return nil, "", reqErr
		}
		req.Header.Set("Authorization", "SSWS "+c.apiToken)
		req.Header.Set("Accept", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			if isTimeout(doErr) {
				lastErr = fmt.Errorf(
					"request to %s failed: %w", fullURL, doErr,
				)
				wait := time.Duration(attempt+1) * 10 * time.Second
				fmt.Fprintf(os.Stderr,
					"  Timeout (attempt %d/5), retrying in %v...\n",
					attempt+1, wait)
				select {
				case <-ctx.Done():
					return nil, "", ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return nil, "", fmt.Errorf(
				"request to %s failed: %w", fullURL, doErr,
			)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("reading response body: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			wait := rateLimitWait(resp.Header)
			fmt.Fprintf(os.Stderr,
				"  Rate limited (attempt %d/5), waiting %v...\n",
				attempt+1, wait)
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(wait):
				continue
			}
		}

		if resp.StatusCode >= 400 {
			return nil, "", fmt.Errorf(
				"HTTP %d from %s: %s",
				resp.StatusCode, fullURL,
				truncate(string(respBody), 200),
			)
		}

		next := parseLinkNext(resp.Header.Get("Link"))
		return respBody, next, nil
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("exceeded retry limit for %s", fullURL)
}

// get is a convenience wrapper that unmarshals the JSON response into
// dest and returns the next page URL.
func (c *Client) get(
	ctx context.Context, fullURL string, dest interface{},
) (string, error) {
	body, next, err := c.doRequest(ctx, fullURL)
	if err != nil {
		return "", err
	}
	return next, json.Unmarshal(body, dest)
}

// baseURL returns the API base URL for this Okta tenant.
func (c *Client) baseURL() string {
	if c.baseURLOverride != "" {
		return c.baseURLOverride
	}
	return "https://" + c.domain
}

// rateLimitWait reads X-Rate-Limit-Reset (Unix epoch seconds) and
// returns the duration until that time. Falls back to 5 seconds if
// the header is missing or unparseable.
func rateLimitWait(h http.Header) time.Duration {
	resetStr := h.Get("X-Rate-Limit-Reset")
	if resetStr == "" {
		return 5 * time.Second
	}
	epoch, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return 5 * time.Second
	}
	wait := time.Until(time.Unix(epoch, 0))
	if wait < time.Second {
		wait = time.Second
	}
	return wait
}

// parseLinkNext extracts the URL with rel="next" from an RFC 5988
// Link header. Returns empty string if no next link is present.
func parseLinkNext(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		if strings.Contains(part, `rel="next"`) {
			url := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
			return strings.Trim(url, "<>")
		}
	}
	return ""
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
