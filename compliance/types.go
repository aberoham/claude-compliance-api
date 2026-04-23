package compliance

import (
	"encoding/json"
	"time"
)

// Activity represents a single event from the Compliance API Activity Feed.
// Known fields are mapped to struct fields; activity-specific fields (e.g.,
// claude_chat_id, filename) land in Extra as raw JSON for lossless storage.
type Activity struct {
	ID               string          `json:"id"`
	CreatedAt        string          `json:"created_at"`
	OrganizationID   *string         `json:"organization_id"`
	OrganizationUUID *string         `json:"organization_uuid"`
	Type             string          `json:"type"`
	Actor            Actor           `json:"actor"`
	Extra            json.RawMessage `json:"-"` // overflow fields preserved as raw JSON
}

// knownActivityFields is the set of top-level JSON keys that are mapped to
// struct fields and should not appear in Extra.
var knownActivityFields = map[string]bool{
	"id":                true,
	"created_at":        true,
	"organization_id":   true,
	"organization_uuid": true,
	"type":              true,
	"actor":             true,
}

func (a *Activity) UnmarshalJSON(data []byte) error {
	// Unmarshal the known fields via an alias to avoid infinite recursion.
	type Alias Activity
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*a = Activity(alias)

	// Collect overflow fields into Extra.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	extra := make(map[string]json.RawMessage)
	for k, v := range raw {
		if !knownActivityFields[k] {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		b, err := json.Marshal(extra)
		if err != nil {
			return err
		}
		a.Extra = b
	}
	return nil
}

// MarshalJSON produces the canonical JSON by merging struct fields with Extra.
func (a Activity) MarshalJSON() ([]byte, error) {
	type Alias Activity
	b, err := json.Marshal(Alias(a))
	if err != nil {
		return nil, err
	}
	if len(a.Extra) == 0 {
		return b, nil
	}
	// Merge the two JSON objects.
	var base map[string]json.RawMessage
	if err := json.Unmarshal(b, &base); err != nil {
		return nil, err
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(a.Extra, &extra); err != nil {
		return nil, err
	}
	for k, v := range extra {
		base[k] = v
	}
	return json.Marshal(base)
}

// CreatedAtTime parses the CreatedAt string into a time.Time.
func (a *Activity) CreatedAtTime() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, a.CreatedAt)
}

// Actor identifies who performed an activity. The Type field determines which
// optional fields are populated (user_actor, api_actor, etc.).
type Actor struct {
	Type                       string  `json:"type"`
	EmailAddress               *string `json:"email_address,omitempty"`
	UserID                     *string `json:"user_id,omitempty"`
	IPAddress                  *string `json:"ip_address,omitempty"`
	UserAgent                  *string `json:"user_agent,omitempty"`
	APIKeyID                   *string `json:"api_key_id,omitempty"`
	UnauthenticatedEmailAddress *string `json:"unauthenticated_email_address,omitempty"`
	// ScimDirectorySyncActor fields (Rev G)
	WorkOSEventID     *string `json:"workos_event_id,omitempty"`
	DirectoryID       *string `json:"directory_id,omitempty"`
	IDPConnectionType *string `json:"idp_connection_type,omitempty"`
	// AdminApiKeyActor fields (Rev I)
	AdminAPIKeyID *string `json:"admin_api_key_id,omitempty"`
}

// User represents a licensed user from the organization users endpoint.
type User struct {
	ID           string `json:"id"`
	FullName     string `json:"full_name"`
	EmailAddress string `json:"email_address"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
}

// EffectiveEmail returns the best available email, preferring email_address
// over email (the API response format has varied).
func (u *User) EffectiveEmail() string {
	if u.EmailAddress != "" {
		return u.EmailAddress
	}
	return u.Email
}

// ActivitiesResponse is the paginated response from the activities endpoint.
type ActivitiesResponse struct {
	Data    []Activity `json:"data"`
	HasMore bool       `json:"has_more"`
	FirstID string     `json:"first_id"`
	LastID  string     `json:"last_id"`
}

// UsersResponse is the paginated response from the organization users endpoint.
// This endpoint uses cursor-based pagination via next_page/page, not after_id/last_id.
type UsersResponse struct {
	Data     []User  `json:"data"`
	HasMore  bool    `json:"has_more"`
	NextPage *string `json:"next_page"`
}

// Project represents project metadata from the Compliance API.
// The list endpoint returns User (id + email) while the detail endpoint
// returns the full Creator object plus Description/Instructions.
type Project struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description,omitempty"`
	Instructions     string       `json:"instructions,omitempty"`
	CreatedAt        string       `json:"created_at"`
	UpdatedAt        string       `json:"updated_at"`
	OrganizationID   string       `json:"organization_id"`
	CreatorID        string       `json:"creator_id,omitempty"`
	Creator          *User        `json:"creator,omitempty"`
	ArchivedAt       *string      `json:"archived_at,omitempty"`
	IsPrivate        *bool        `json:"is_private,omitempty"`
	User             *ProjectUser `json:"user,omitempty"`
	ChatsCount       *int         `json:"chats_count,omitempty"`
	AttachmentsCount *int         `json:"attachments_count,omitempty"`
}

// ProjectUser identifies a project's creator in list responses.
type ProjectUser struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

// ProjectsResponse is the paginated response from the projects endpoint.
// This endpoint uses cursor-based pagination via next_page/page (Rev H).
type ProjectsResponse struct {
	Data     []Project `json:"data"`
	HasMore  bool      `json:"has_more"`
	NextPage *string   `json:"next_page"`
}

// Chat represents metadata for a conversation.
type Chat struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	DeletedAt        *string `json:"deleted_at,omitempty"`
	OrganizationID   string  `json:"organization_id"`
	OrganizationUUID *string `json:"organization_uuid,omitempty"`
	ProjectID        *string `json:"project_id,omitempty"`
	User             struct {
		ID           string `json:"id"`
		EmailAddress string `json:"email_address"`
	} `json:"user"`
}

// ChatsResponse is the paginated response from the chats endpoint.
type ChatsResponse struct {
	Data    []Chat `json:"data"`
	HasMore bool   `json:"has_more"`
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
}

// ChatMessage represents a single turn in a conversation.
type ChatMessage struct {
	ID        string              `json:"id,omitempty"`
	UUID      string              `json:"uuid,omitempty"`
	Role      string              `json:"role"` // "user" or "assistant"
	CreatedAt string              `json:"created_at"`
	Content   []Content           `json:"content"`
	Files     []File              `json:"files,omitempty"`
	Artifacts []ArtifactReference `json:"artifacts,omitempty"`
}

// ArtifactReference identifies an artifact version generated by the assistant.
type ArtifactReference struct {
	ID           string  `json:"id"`
	VersionID    string  `json:"version_id"`
	Title        *string `json:"title"`
	ArtifactType *string `json:"artifact_type"`
}

// Content represents a content block in a message.
type Content struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// File represents an attached file.
type File struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
}

// ChatDetail includes full message history.
type ChatDetail struct {
	Chat
	ChatMessages []ChatMessage `json:"chat_messages"`
}

// UserActivitySummary aggregates activity metrics for a single user.
type UserActivitySummary struct {
	Email      string
	UserID     string
	EventCount int
	EventTypes map[string]int
	FirstSeen  time.Time
	LastSeen   time.Time
	ActiveDays map[string]bool // set of "YYYY-MM-DD" strings
}
