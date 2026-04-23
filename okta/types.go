package okta

import (
	"os"
	"strconv"
	"time"
)

const defaultClaudeAppName = "Anthropic Claude"
const defaultSessionDurationDays = 0

// DefaultClaudeAppID reads the preferred app instance ID from the
// OKTA_CLAUDE_APP_ID environment variable. When set, app matching
// uses ID comparison instead of display name.
func DefaultClaudeAppID() string {
	return os.Getenv("OKTA_CLAUDE_APP_ID")
}

// DefaultClaudeAppName reads the app display name from the
// OKTA_CLAUDE_APP_NAME environment variable, falling back to
// "Anthropic Claude" if unset.
func DefaultClaudeAppName() string {
	if v := os.Getenv("OKTA_CLAUDE_APP_NAME"); v != "" {
		return v
	}
	return defaultClaudeAppName
}

// DefaultSessionDurationDays reads the Claude session duration from
// the CLAUDE_SESSION_DURATION_DAYS environment variable, falling back
// to 0 (unlimited/disabled) if unset or unparseable. A value of 0
// disables the session-based Okta override entirely.
func DefaultSessionDurationDays() int {
	if v := os.Getenv("CLAUDE_SESSION_DURATION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultSessionDurationDays
}

// LogEvent represents a single Okta system log event. MatchedAppID
// and MatchedAppName are set by FetchClaudeSSOEvents to record which
// AppInstance target satisfied the filter, so downstream consumers
// (e.g., the store layer) do not need to re-derive the match.
type LogEvent struct {
	UUID      string    `json:"uuid"`
	Published time.Time `json:"published"`
	EventType string    `json:"eventType"`
	Actor     Actor     `json:"actor"`
	Target    []Target  `json:"target"`
	Outcome   Outcome   `json:"outcome"`

	MatchedAppID   string `json:"-"`
	MatchedAppName string `json:"-"`
}

// Actor identifies who performed the action.
type Actor struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	AlternateID string `json:"alternateId"`
	DisplayName string `json:"displayName"`
}

// Target identifies the resource acted upon.
type Target struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	AlternateID string `json:"alternateId"`
	DisplayName string `json:"displayName"`
}

// Outcome holds the result of the event.
type Outcome struct {
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
}
