package okta

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// FetchClaudeSSOEvents fetches all successful SSO events for the org
// within the given time window, filters for the Claude app, and
// returns only matching events.
//
// The query is bounded by both since and until to avoid unbounded
// polling. The Okta System Log API supports at most 90 days of
// history.
//
// App matching prefers appID (stable) over appName (display name).
// If appID is empty, appName is used. The caller should pass
// DefaultClaudeAppID() and DefaultClaudeAppName() for the defaults.
func (c *Client) FetchClaudeSSOEvents(
	ctx context.Context,
	since, until time.Time,
	appID, appName string,
) ([]LogEvent, error) {
	filter := `eventType eq "user.authentication.sso"`
	if appID != "" {
		filter += ` AND target.id eq "` + appID + `"`
	}
	params := url.Values{
		"filter":    {filter},
		"since":     {since.UTC().Format(time.RFC3339)},
		"until":     {until.UTC().Format(time.RFC3339)},
		"limit":     {"1000"},
		"sortOrder": {"ASCENDING"},
	}

	reqURL := c.baseURL() + "/api/v1/logs?" + params.Encode()

	var matched []LogEvent
	totalFetched := 0

	for reqURL != "" {
		var page []LogEvent
		nextURL, err := c.get(ctx, reqURL, &page)
		if err != nil {
			return nil, fmt.Errorf("fetching SSO events: %w", err)
		}

		for i := range page {
			e := &page[i]
			if !isSuccessfulClaudeSSO(e, appID, appName) {
				continue
			}
			if e.Actor.AlternateID == "" {
				continue
			}
			e.Actor.AlternateID = strings.ToLower(
				e.Actor.AlternateID,
			)
			matched = append(matched, *e)
		}

		totalFetched += len(page)
		fmt.Fprintf(os.Stderr,
			"  ...fetched %d SSO events, %d matched Claude so far\n",
			totalFetched, len(matched))

		if len(page) == 0 {
			break
		}
		reqURL = nextURL
	}

	return matched, nil
}

// isSuccessfulClaudeSSO checks that an event is a successful SSO
// authentication targeting the Claude app. On match it sets
// e.MatchedAppID and e.MatchedAppName so the store layer can
// persist the correct target without re-deriving the match.
func isSuccessfulClaudeSSO(
	e *LogEvent, appID, appName string,
) bool {
	if e.Outcome.Result != "SUCCESS" {
		return false
	}
	for _, t := range e.Target {
		if t.Type != "AppInstance" {
			continue
		}
		if appID != "" && t.ID == appID {
			e.MatchedAppID = t.ID
			e.MatchedAppName = t.DisplayName
			return true
		}
		if appID == "" && t.DisplayName == appName {
			e.MatchedAppID = t.ID
			e.MatchedAppName = t.DisplayName
			return true
		}
	}
	return false
}
