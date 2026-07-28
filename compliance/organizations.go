package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Organization is an organization beneath the API key's parent organization.
type Organization struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Role is a custom Compliance RBAC role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// RolePermission is an action granted by a custom role.
type RolePermission struct {
	Action       string `json:"action"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

// Group is a direct or SCIM-synchronized RBAC group.
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Roles       []string `json:"roles"`
	SourceType  string   `json:"source_type"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// GroupMember is a current user membership in a Compliance group.
type GroupMember struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// GroupQuery controls Compliance group enumeration.
type GroupQuery struct {
	Limit      int
	NamePrefix string
}

// ComplianceAPIKey is key metadata returned with effective settings. Secret
// values are never returned by the API.
type ComplianceAPIKey struct {
	ID          string   `json:"id"`
	CreatedAt   string   `json:"created_at"`
	CreatedByID *string  `json:"created_by_id"`
	IsActive    bool     `json:"is_active"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   *string  `json:"expires_at"`
	Type        string   `json:"type"`
}

// EffectiveSetting preserves the polymorphic setting value as raw JSON so
// newly-added setting types remain forward compatible.
type EffectiveSetting struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// EffectiveOrganizationSettings is the enforced state for an organization.
type EffectiveOrganizationSettings struct {
	APIKeys        []ComplianceAPIKey `json:"api_keys"`
	OrganizationID string             `json:"organization_id"`
	Settings       []EffectiveSetting `json:"settings"`
	Type           string             `json:"type"`
}

func (c *Client) FetchOrganizations(ctx context.Context, opts ListOptions) ([]Organization, error) {
	return fetchAllPages[Organization](ctx, c, "/v1/compliance/organizations", nil, opts, 1000, 1000, nil)
}

func (c *Client) ForEachOrganizationPage(ctx context.Context, opts ListOptions, fn func([]Organization, CursorPage) error) error {
	_, err := fetchAllPages[Organization](ctx, c, "/v1/compliance/organizations", nil, opts, 1000, 1000, fn)
	return err
}

func (c *Client) FetchOrganizationUsers(ctx context.Context, orgUUID string, opts ListOptions) ([]User, error) {
	if orgUUID == "" {
		return nil, fmt.Errorf("organization UUID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/users", url.PathEscape(orgUUID))
	return fetchAllPages[User](ctx, c, endpoint, nil, opts, 500, 1000, nil)
}

func (c *Client) ForEachOrganizationUserPage(ctx context.Context, orgUUID string, opts ListOptions, fn func([]User, CursorPage) error) error {
	if orgUUID == "" {
		return fmt.Errorf("organization UUID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/users", url.PathEscape(orgUUID))
	_, err := fetchAllPages[User](ctx, c, endpoint, nil, opts, 500, 1000, fn)
	return err
}

func (c *Client) FetchRoles(ctx context.Context, orgUUID string, opts ListOptions) ([]Role, error) {
	if orgUUID == "" {
		return nil, fmt.Errorf("organization UUID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/roles", url.PathEscape(orgUUID))
	return fetchAllPages[Role](ctx, c, endpoint, nil, opts, 500, 1000, nil)
}

func (c *Client) ForEachRolePage(ctx context.Context, orgUUID string, opts ListOptions, fn func([]Role, CursorPage) error) error {
	if orgUUID == "" {
		return fmt.Errorf("organization UUID is required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/roles", url.PathEscape(orgUUID))
	_, err := fetchAllPages[Role](ctx, c, endpoint, nil, opts, 500, 1000, fn)
	return err
}

func (c *Client) GetRole(ctx context.Context, orgUUID, roleID string) (*Role, error) {
	if orgUUID == "" || roleID == "" {
		return nil, fmt.Errorf("organization UUID and role ID are required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/roles/%s", url.PathEscape(orgUUID), url.PathEscape(roleID))
	var role Role
	if err := c.get(ctx, endpoint, nil, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (c *Client) FetchRolePermissions(ctx context.Context, orgUUID, roleID string, opts ListOptions) ([]RolePermission, error) {
	if orgUUID == "" || roleID == "" {
		return nil, fmt.Errorf("organization UUID and role ID are required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/roles/%s/permissions", url.PathEscape(orgUUID), url.PathEscape(roleID))
	return fetchAllPages[RolePermission](ctx, c, endpoint, nil, opts, 500, 1000, nil)
}

func (c *Client) ForEachRolePermissionPage(ctx context.Context, orgUUID, roleID string, opts ListOptions, fn func([]RolePermission, CursorPage) error) error {
	if orgUUID == "" || roleID == "" {
		return fmt.Errorf("organization UUID and role ID are required")
	}
	endpoint := fmt.Sprintf("/v1/compliance/organizations/%s/roles/%s/permissions", url.PathEscape(orgUUID), url.PathEscape(roleID))
	_, err := fetchAllPages[RolePermission](ctx, c, endpoint, nil, opts, 500, 1000, fn)
	return err
}

func (c *Client) FetchGroups(ctx context.Context, opts GroupQuery) ([]Group, error) {
	params := url.Values{}
	if opts.NamePrefix != "" {
		params.Set("name_prefix", opts.NamePrefix)
	}
	return fetchAllPages[Group](ctx, c, "/v1/compliance/groups", params, ListOptions{Limit: opts.Limit}, 500, 1000, nil)
}

func (c *Client) ForEachGroupPage(ctx context.Context, opts GroupQuery, page string, fn func([]Group, CursorPage) error) error {
	params := url.Values{}
	if opts.NamePrefix != "" {
		params.Set("name_prefix", opts.NamePrefix)
	}
	_, err := fetchAllPages[Group](ctx, c, "/v1/compliance/groups", params, ListOptions{Limit: opts.Limit, Page: page}, 500, 1000, fn)
	return err
}

func (c *Client) GetGroup(ctx context.Context, groupID string) (*Group, error) {
	if groupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}
	var group Group
	if err := c.get(ctx, "/v1/compliance/groups/"+url.PathEscape(groupID), nil, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

func (c *Client) FetchGroupMembers(ctx context.Context, groupID string, opts ListOptions) ([]GroupMember, error) {
	if groupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}
	endpoint := "/v1/compliance/groups/" + url.PathEscape(groupID) + "/members"
	return fetchAllPages[GroupMember](ctx, c, endpoint, nil, opts, 500, 1000, nil)
}

func (c *Client) ForEachGroupMemberPage(ctx context.Context, groupID string, opts ListOptions, fn func([]GroupMember, CursorPage) error) error {
	if groupID == "" {
		return fmt.Errorf("group ID is required")
	}
	endpoint := "/v1/compliance/groups/" + url.PathEscape(groupID) + "/members"
	_, err := fetchAllPages[GroupMember](ctx, c, endpoint, nil, opts, 500, 1000, fn)
	return err
}

func (c *Client) GetOrganizationSettings(ctx context.Context, organizationID string) (*EffectiveOrganizationSettings, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("organization ID is required")
	}
	endpoint := "/v1/compliance/organizations/" + url.PathEscape(organizationID) + "/settings"
	var settings EffectiveOrganizationSettings
	if err := c.get(ctx, endpoint, nil, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}
