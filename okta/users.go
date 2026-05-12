package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// UserProfile is the subset of an Okta user record needed to translate
// between Anthropic-side identity (email, used as the Compliance API key)
// and Okta-side identity (profile.login, which appears as actor.alternateId
// in System Log SSO events).
//
// For AD-mastered tenants these two values frequently differ: the AD UPN
// becomes profile.login (e.g. "userlogin@corp.example.com") while the
// SCIM proxy address becomes profile.email (e.g. "first.last@example.com").
// Joining SSO events to Compliance users by email alone silently drops
// every such user from the reclaim cross-reference.
type UserProfile struct {
	ID    string // Okta user UUID, stable across login/email changes
	Login string // profile.login (lowercased)
	Email string // profile.email (lowercased)
}

type oktaUserResponse struct {
	ID      string `json:"id"`
	Profile struct {
		Login string `json:"login"`
		Email string `json:"email"`
	} `json:"profile"`
}

// LookupUserByEmail resolves an Anthropic-side email to its Okta user
// record. Returns nil, nil when no matching user exists (a soft miss is
// not an error). The search field is case-sensitive in Okta, so the email
// is passed through unchanged; callers should normalise upstream if needed.
func (c *Client) LookupUserByEmail(
	ctx context.Context, email string,
) (*UserProfile, error) {
	if email == "" {
		return nil, fmt.Errorf("LookupUserByEmail: empty email")
	}
	params := url.Values{
		"search": {fmt.Sprintf(`profile.email eq %q`, email)},
		"limit":  {"1"},
	}
	reqURL := c.baseURL() + "/api/v1/users?" + params.Encode()

	body, _, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("LookupUserByEmail(%s): %w", email, err)
	}
	var users []oktaUserResponse
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf(
			"LookupUserByEmail(%s): decoding response: %w", email, err,
		)
	}
	if len(users) == 0 {
		return nil, nil
	}
	u := users[0]
	return &UserProfile{
		ID:    u.ID,
		Login: strings.ToLower(u.Profile.Login),
		Email: strings.ToLower(u.Profile.Email),
	}, nil
}
