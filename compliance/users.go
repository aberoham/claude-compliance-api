package compliance

import (
	"context"
	"fmt"
)

// FetchUsers retrieves all licensed users for the configured organization,
// paginating through the full result set using cursor-based pagination.
func (c *Client) FetchUsers(ctx context.Context) ([]User, error) {
	if c.orgID == "" {
		return nil, fmt.Errorf("organization ID is required; configure the client with an organization UUID")
	}
	return c.FetchOrganizationUsers(ctx, c.orgID, ListOptions{Limit: 500})
}
