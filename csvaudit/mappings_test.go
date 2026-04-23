package csvaudit

import "testing"

func TestCSVToAPIMappingsNoDuplicateCSVKeys(t *testing.T) {
	csvSeen := make(map[string]bool)
	for _, m := range CSVToAPIMappings {
		if csvSeen[m.CSV] && !knownDuplicateCSVKeys[m.CSV] {
			t.Errorf("unexpected duplicate CSV event type: %s", m.CSV)
		}
		csvSeen[m.CSV] = true
	}

	// Verify that every entry in knownDuplicateCSVKeys actually appears
	// more than once, so the allowlist stays honest.
	counts := make(map[string]int)
	for _, m := range CSVToAPIMappings {
		counts[m.CSV]++
	}
	for key := range knownDuplicateCSVKeys {
		if counts[key] < 2 {
			t.Errorf("knownDuplicateCSVKeys contains %q but it only appears %d time(s)", key, counts[key])
		}
	}
}

func TestMappedCSVTypes(t *testing.T) {
	m := MappedCSVTypes()
	if len(m) == 0 {
		t.Fatal("expected non-empty MappedCSVTypes")
	}
	// Unique CSV types will be fewer than total entries due to duplicates.
	uniqueCSV := make(map[string]bool)
	for _, em := range CSVToAPIMappings {
		uniqueCSV[em.CSV] = true
	}
	if len(m) != len(uniqueCSV) {
		t.Errorf("expected %d unique CSV types, got %d", len(uniqueCSV), len(m))
	}
	for _, want := range []string{
		"conversation_created",
		"file_uploaded",
		"session_share_created",
		"user_signed_in_google",
		"user_requested_magic_link",
		"org_cowork_agent_enabled",
		"integration_org_enabled",
	} {
		if !m[want] {
			t.Errorf("expected %q in MappedCSVTypes", want)
		}
	}
}

func TestMappedAPITypes(t *testing.T) {
	m := MappedAPITypes()
	if len(m) == 0 {
		t.Error("expected non-empty MappedAPITypes")
	}
	for _, want := range []string{
		"claude_chat_created",
		"claude_file_uploaded",
		"session_share_created",
		"social_login_succeeded",
		"magic_link_login_initiated",
		"org_cowork_agent_enabled",
	} {
		if !m[want] {
			t.Errorf("expected %q in MappedAPITypes", want)
		}
	}
}

func TestCSVToAPIMap(t *testing.T) {
	m := CSVToAPIMap()
	cases := []struct {
		csv, api string
	}{
		{"conversation_created", "claude_chat_created"},
		{"role_assignment_granted", "role_assignment_granted"},
		{"user_signed_in_google", "social_login_succeeded"},
		{"user_requested_magic_link", "magic_link_login_initiated"},
		{"user_name_changed", "claude_user_settings_updated"},
		{"org_user_updated", "claude_user_role_updated"},
		{"org_data_retention_policy_changed", "claude_organization_settings_updated"},
		{"org_cowork_agent_enabled", "org_cowork_agent_enabled"},
		{"org_claude_code_desktop_enabled", "org_claude_code_desktop_enabled"},
	}
	for _, tc := range cases {
		if got := m[tc.csv]; got != tc.api {
			t.Errorf("%s mapped to %q, want %q", tc.csv, got, tc.api)
		}
	}
}

func TestCSVToAPIMapLossyForDuplicates(t *testing.T) {
	m := CSVToAPIMap()
	// For one-to-many CSV keys, CSVToAPIMap returns the first mapping.
	// Verify it returns something (not empty), but don't assert which one
	// since the spec allows either GDrive or GitHub.
	for key := range knownDuplicateCSVKeys {
		if m[key] == "" {
			t.Errorf("CSVToAPIMap missing entry for duplicate key %q", key)
		}
	}
}
