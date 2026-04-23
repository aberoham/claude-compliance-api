package csvaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseActorInfo(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantName, wantEmail, wantUUID string
	}{
		{
			name:      "standard format",
			input:     `{'name': 'Jane Doe', 'uuid': 'abc-123', 'metadata': {'email_address': 'jane@example.com'}}`,
			wantName:  "Jane Doe",
			wantEmail: "jane@example.com",
			wantUUID:  "abc-123",
		},
		{
			name:      "name with apostrophe",
			input:     `{'name': 'Reece O'Sullivan', 'uuid': 'def-456', 'metadata': {'email_address': 'reece@example.com'}}`,
			wantName:  "Reece O'Sullivan",
			wantEmail: "reece@example.com",
			wantUUID:  "def-456",
		},
		{
			name:      "empty string",
			input:     "",
			wantName:  "",
			wantEmail: "",
			wantUUID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, email, uuid := parseActorInfo(tt.input)
			if name != tt.wantName {
				t.Errorf("name: got %q, want %q", name, tt.wantName)
			}
			if email != tt.wantEmail {
				t.Errorf("email: got %q, want %q", email, tt.wantEmail)
			}
			if uuid != tt.wantUUID {
				t.Errorf("uuid: got %q, want %q", uuid, tt.wantUUID)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	tests := []struct {
		s, key, want string
	}{
		{
			s:    `{'name': 'Alice', 'uuid': '123'}`,
			key:  "'name': '",
			want: "Alice",
		},
		{
			s:    `{'name': 'Bob O'Brien', 'uuid': '456'}`,
			key:  "'name': '",
			want: "Bob O'Brien",
		},
		{
			s:    `{'name': 'End'}`,
			key:  "'name': '",
			want: "End",
		},
		{
			s:    `{'other': 'val'}`,
			key:  "'missing': '",
			want: "",
		},
	}

	for _, tt := range tests {
		got := extractValue(tt.s, tt.key)
		if got != tt.want {
			t.Errorf("extractValue(%q, %q) = %q, want %q", tt.s, tt.key, got, tt.want)
		}
	}
}

func TestParseCSV(t *testing.T) {
	csv := `created_at,event,client_platform,actor_info
2026-01-10T10:00:00Z,conversation_created,web,"{'name': 'Alice', 'uuid': 'u1', 'metadata': {'email_address': 'alice@example.com'}}"
2026-01-10T14:00:00Z,conversation_renamed,web,"{'name': 'Bob O'Brien', 'uuid': 'u2', 'metadata': {'email_address': 'bob@example.com'}}"
2026-01-11T09:00:00Z,file_uploaded,desktop,"{'name': 'Alice', 'uuid': 'u1', 'metadata': {'email_address': 'alice@example.com'}}"
`
	path := filepath.Join(t.TempDir(), "test.csv")
	if err := os.WriteFile(path, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := ParseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].ActorName != "Alice" {
		t.Errorf("event 0 name: %q", events[0].ActorName)
	}
	if events[1].ActorName != "Bob O'Brien" {
		t.Errorf("event 1 name: got %q, want %q", events[1].ActorName, "Bob O'Brien")
	}
	if events[0].ActorEmail != "alice@example.com" {
		t.Errorf("event 0 email: %q", events[0].ActorEmail)
	}
	if events[2].ClientPlatform != "desktop" {
		t.Errorf("event 2 platform: %q", events[2].ClientPlatform)
	}
}

func TestSummarizeByUser(t *testing.T) {
	csv := `created_at,event,client_platform,actor_info
2026-01-10T10:00:00Z,conversation_created,web,"{'name': 'Alice', 'uuid': 'u1', 'metadata': {'email_address': 'alice@example.com'}}"
2026-01-10T14:00:00Z,conversation_renamed,web,"{'name': 'Alice', 'uuid': 'u1', 'metadata': {'email_address': 'alice@example.com'}}"
2026-01-11T09:00:00Z,file_uploaded,web,"{'name': 'Bob', 'uuid': 'u2', 'metadata': {'email_address': 'bob@example.com'}}"
`
	path := filepath.Join(t.TempDir(), "test.csv")
	os.WriteFile(path, []byte(csv), 0o644)

	events, _ := ParseCSV(path)
	summaries := SummarizeByUser(events)

	if len(summaries) != 2 {
		t.Fatalf("expected 2 users, got %d", len(summaries))
	}

	alice := summaries["alice@example.com"]
	if alice == nil {
		t.Fatal("missing alice")
	}
	if alice.EventCount != 2 {
		t.Errorf("alice events: %d", alice.EventCount)
	}
	if alice.EventTypes["conversation_created"] != 1 {
		t.Error("alice missing conversation_created")
	}
	if len(alice.ActiveDays) != 1 {
		t.Errorf("alice active days: expected 1, got %d", len(alice.ActiveDays))
	}
}
