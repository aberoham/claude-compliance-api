package compliance

import (
	"encoding/json"
	"testing"
)

func TestActivityUnmarshalExtra(t *testing.T) {
	raw := `{
		"id": "activity_01abc",
		"created_at": "2026-01-10T00:00:00Z",
		"organization_id": "org_01",
		"organization_uuid": "uuid-1",
		"type": "claude_chat_created",
		"actor": {"type": "user_actor", "email_address": "alice@example.com"},
		"claude_chat_id": "chat_123",
		"claude_project_id": "proj_456"
	}`

	var a Activity
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}

	if a.ID != "activity_01abc" {
		t.Errorf("ID = %q", a.ID)
	}
	if a.Type != "claude_chat_created" {
		t.Errorf("Type = %q", a.Type)
	}
	if a.Actor.Type != "user_actor" {
		t.Errorf("Actor.Type = %q", a.Actor.Type)
	}

	// Verify extra fields were captured.
	if len(a.Extra) == 0 {
		t.Fatal("expected Extra to contain overflow fields")
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(a.Extra, &extra); err != nil {
		t.Fatal(err)
	}
	if _, ok := extra["claude_chat_id"]; !ok {
		t.Error("missing claude_chat_id in Extra")
	}
	if _, ok := extra["claude_project_id"]; !ok {
		t.Error("missing claude_project_id in Extra")
	}
	// Known fields should not be in Extra.
	if _, ok := extra["id"]; ok {
		t.Error("id should not be in Extra")
	}
	if _, ok := extra["actor"]; ok {
		t.Error("actor should not be in Extra")
	}
}

func TestActivityMarshalRoundTrip(t *testing.T) {
	raw := `{"id":"a1","created_at":"2026-01-10T00:00:00Z","organization_id":null,"organization_uuid":null,"type":"claude_file_uploaded","actor":{"type":"user_actor","email_address":"bob@example.com"},"filename":"report.pdf","claude_file_id":"file_01"}`

	var a Activity
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}

	b, err := a.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Verify the re-marshaled JSON contains the extra fields.
	var roundTrip map[string]json.RawMessage
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if _, ok := roundTrip["filename"]; !ok {
		t.Error("filename lost in round-trip")
	}
	if _, ok := roundTrip["claude_file_id"]; !ok {
		t.Error("claude_file_id lost in round-trip")
	}
}

func TestAdminAPIKeyActorUnmarshal(t *testing.T) {
	raw := `{
		"id": "activity_admin01",
		"created_at": "2026-04-01T12:00:00Z",
		"organization_id": "org_01",
		"organization_uuid": "uuid-1",
		"type": "admin_api_key_created",
		"actor": {
			"type": "admin_api_key_actor",
			"admin_api_key_id": "admin_key_abc123"
		},
		"admin_api_key_id": "admin_key_abc123",
		"scopes": ["read:compliance_activities"]
	}`

	var a Activity
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatal(err)
	}

	if a.Actor.Type != "admin_api_key_actor" {
		t.Errorf("Actor.Type = %q, want admin_api_key_actor", a.Actor.Type)
	}
	if a.Actor.AdminAPIKeyID == nil || *a.Actor.AdminAPIKeyID != "admin_key_abc123" {
		t.Errorf("Actor.AdminAPIKeyID = %v, want admin_key_abc123", a.Actor.AdminAPIKeyID)
	}
	if a.Actor.EmailAddress != nil {
		t.Errorf("Actor.EmailAddress should be nil for admin_api_key_actor, got %q", *a.Actor.EmailAddress)
	}

	// Verify round-trip preserves extra fields.
	b, err := a.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]json.RawMessage
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if _, ok := roundTrip["scopes"]; !ok {
		t.Error("scopes lost in round-trip")
	}
}

func TestActivityCreatedAtTime(t *testing.T) {
	a := Activity{CreatedAt: "2026-01-10T15:30:00.123456Z"}
	ts, err := a.CreatedAtTime()
	if err != nil {
		t.Fatal(err)
	}
	if ts.Year() != 2026 || ts.Month() != 1 || ts.Day() != 10 {
		t.Errorf("unexpected date: %v", ts)
	}
}
