package csvaudit

// EventMapping holds one row of the Rev I Appendix B mapping table,
// pairing a CSV audit log event type with its Compliance API equivalent.
type EventMapping struct {
	CSV string
	API string
}

// CSVToAPIMappings is the complete set of event type equivalences from
// Compliance API Rev I (2026-03-29) Appendix B. Entries where the CSV
// audit log event has no API equivalent (marked "-" in the spec) are
// omitted; the compare command detects those as unmapped CSV types.
//
// Three CSV events map to multiple API types (integration_org_enabled,
// integration_org_disabled, integration_org_config_updated) because the
// API distinguishes Google Drive vs GitHub integrations. These appear as
// separate entries here, making CSVToAPIMap() lossy for those keys.
var CSVToAPIMappings = []EventMapping{
	// Authentication
	{"user_sent_phone_code", "phone_code_sent"},
	{"user_verified_phone_code", "phone_code_verified"},
	{"user_signed_in_google", "social_login_succeeded"},
	{"user_signed_in_apple", "social_login_succeeded"},
	{"user_signed_in_microsoft", "social_login_succeeded"},
	{"user_signed_in_sso", "sso_login_succeeded"},
	{"user_requested_magic_link", "magic_link_login_initiated"},
	{"user_age_verified", "age_verified"},
	{"anonymous_mobile_login_attempted", "anonymous_mobile_login_attempted"},
	{"sso_second_factor_magic_link", "sso_second_factor_magic_link"},
	{"user_attempted_magic_link_verification", "magic_link_login_succeeded"},
	{"user_signed_out", "user_logged_out"},
	{"user_signed_out_all_sessions", "user_logged_out"},
	{"user_revoked_session", "session_revoked"},

	// User settings
	{"user_name_changed", "claude_user_settings_updated"},

	// Org user management
	{"org_user_invite_sent", "org_user_invite_sent"},
	{"org_user_invite_re_sent", "org_user_invite_re_sent"},
	{"org_user_invite_deleted", "org_user_invite_deleted"},
	{"org_user_invite_accepted", "org_user_invite_accepted"},
	{"org_user_invite_rejected", "org_user_invite_rejected"},
	{"org_user_deleted", "org_user_deleted"},
	{"org_user_updated", "claude_user_role_updated"},

	// Domain and SSO
	{"org_domain_add_initiated", "org_domain_add_initiated"},
	{"org_domain_verified", "org_domain_verified"},
	{"org_domain_removed", "org_domain_removed"},
	{"org_sso_add_initiated", "org_sso_add_initiated"},
	{"org_sso_connection_activated", "org_sso_connection_activated"},
	{"org_sso_connection_deactivated", "org_sso_connection_deactivated"},
	{"org_sso_connection_deleted", "org_sso_connection_deleted"},
	{"org_sso_toggled", "org_sso_toggled"},
	{"org_magic_link_second_factor_toggled", "org_magic_link_second_factor_toggled"},

	// Directory sync
	{"org_directory_sync_add_initiated", "org_directory_sync_add_initiated"},
	{"org_directory_sync_activated", "org_directory_sync_activated"},
	{"org_directory_sync_deleted", "org_directory_sync_deleted"},
	{"org_directory_resync_started", "org_directory_resync_started"},
	{"org_directory_resync_completed", "org_directory_resync_completed"},
	{"org_directory_resync_failed", "org_directory_resync_failed"},
	{"org_sso_provisioning_mode_changed", "org_sso_provisioning_mode_changed"},
	{"org_sso_seat_tier_assignment_toggled", "org_sso_seat_tier_assignment_toggled"},
	{"org_sso_seat_tier_mappings_updated", "org_sso_seat_tier_mappings_updated"},
	{"org_sso_group_role_mappings_updated", "org_sso_group_role_mappings_updated"},

	// Organization settings
	{"org_data_retention_policy_changed", "claude_organization_settings_updated"},
	{"org_sync_deleting_synchronized_files_started", "org_sync_deleting_synchronized_files_started"},
	{"org_sync_synchronized_files_deleted", "org_sync_synchronized_files_deleted"},
	{"org_creation_blocked", "org_creation_blocked"},
	{"org_claude_code_data_sharing_enabled", "org_claude_code_data_sharing_enabled"},
	{"org_claude_code_data_sharing_disabled", "org_claude_code_data_sharing_disabled"},
	{"org_cowork_enabled", "org_cowork_enabled"},
	{"org_cowork_disabled", "org_cowork_disabled"},
	{"org_work_across_apps_enabled", "org_work_across_apps_enabled"},
	{"org_work_across_apps_disabled", "org_work_across_apps_disabled"},
	{"org_cowork_agent_enabled", "org_cowork_agent_enabled"},
	{"org_cowork_agent_disabled", "org_cowork_agent_disabled"},
	{"org_claude_code_desktop_enabled", "org_claude_code_desktop_enabled"},
	{"org_claude_code_desktop_disabled", "org_claude_code_desktop_disabled"},
	{"org_claude_code_managed_settings_created", "claude_organization_settings_updated"},
	{"org_claude_code_managed_settings_updated", "claude_organization_settings_updated"},
	{"org_claude_code_managed_settings_deleted", "claude_organization_settings_updated"},
	{"org_parent_join_proposal_created", "org_parent_join_proposal_created"},
	{"org_parent_search_performed", "org_parent_search_performed"},
	{"org_compliance_api_settings_updated", "org_compliance_api_settings_updated"},
	{"org_analytics_api_capability_updated", "org_analytics_api_capability_updated"},
	{"org_deletion_requested", "org_deletion_requested"},
	{"org_taint_added", "org_taint_added"},
	{"org_taint_removed", "org_taint_removed"},

	// Organization discoverability
	{"org_discoverability_enabled", "org_discoverability_enabled"},
	{"org_discoverability_disabled", "org_discoverability_disabled"},
	{"org_discoverability_settings_updated", "org_discoverability_settings_updated"},
	{"org_join_request_created", "org_join_request_created"},
	{"org_join_request_approved", "org_join_request_approved"},
	{"org_join_request_instant_approved", "org_join_request_instant_approved"},
	{"org_join_request_dismissed", "org_join_request_dismissed"},
	{"org_join_requests_bulk_dismissed", "org_join_requests_bulk_dismissed"},
	{"org_member_invites_enabled", "org_member_invites_enabled"},
	{"org_member_invites_disabled", "org_member_invites_disabled"},
	{"org_invite_link_generated", "org_invite_link_generated"},
	{"org_invite_link_disabled", "org_invite_link_disabled"},
	{"org_invite_link_regenerated", "org_invite_link_regenerated"},

	// Projects
	{"project_created", "claude_project_created"},
	{"project_deleted", "claude_project_deleted"},
	{"project_visibility_changed", "claude_project_sharing_updated"},
	{"project_document_created", "claude_project_document_uploaded"},
	{"project_document_deleted", "claude_project_document_deleted"},

	// Files
	{"file_uploaded", "claude_file_uploaded"},
	{"file_deleted", "claude_file_deleted"},

	// Customizations (skills, commands, plugins)
	{"skill_created", "claude_skill_created"},
	{"skill_replaced", "claude_skill_replaced"},
	{"skill_deleted", "claude_skill_deleted"},
	{"command_created", "claude_command_created"},
	{"command_replaced", "claude_command_replaced"},
	{"command_deleted", "claude_command_deleted"},
	{"plugin_created", "claude_plugin_created"},
	{"plugin_replaced", "claude_plugin_replaced"},
	{"plugin_updated", "claude_plugin_updated"},
	{"plugin_deleted", "claude_plugin_deleted"},

	// Marketplaces
	{"marketplace_created", "marketplace_created"},
	{"marketplace_updated", "marketplace_updated"},
	{"marketplace_deleted", "marketplace_deleted"},

	// Chats
	{"conversation_created", "claude_chat_created"},
	{"conversation_deleted", "claude_chat_deleted"},
	{"conversation_renamed", "claude_chat_settings_updated"},

	// Session shares
	{"session_share_created", "session_share_created"},
	{"session_share_revoked", "session_share_revoked"},
	{"session_share_accessed", "session_share_accessed"},

	// Integrations (one-to-many: CSV doesn't distinguish GDrive vs GitHub)
	{"integration_org_enabled", "claude_gdrive_integration_created"},
	{"integration_org_enabled", "claude_github_integration_created"},
	{"integration_org_disabled", "claude_gdrive_integration_deleted"},
	{"integration_org_disabled", "claude_github_integration_deleted"},
	{"integration_org_config_updated", "claude_gdrive_integration_updated"},
	{"integration_org_config_updated", "claude_github_integration_updated"},
	{"integration_user_connected", "integration_user_connected"},
	{"integration_user_disconnected", "integration_user_disconnected"},

	// LTI
	{"lti_launch_initiated", "lti_launch_initiated"},
	{"lti_launch_success", "lti_launch_success"},

	// Data export
	{"org_data_export_started", "org_data_export_started"},
	{"org_data_export_completed", "org_data_export_completed"},
	{"org_members_exported", "org_members_exported"},

	// Roles
	{"role_assignment_granted", "role_assignment_granted"},
	{"role_assignment_revoked", "role_assignment_revoked"},
	{"owned_projects_access_restored", "owned_projects_access_restored"},

	// Billing
	{"prepaid_extra_usage_auto_reload_enabled", "prepaid_extra_usage_auto_reload_enabled"},
	{"prepaid_extra_usage_auto_reload_disabled", "prepaid_extra_usage_auto_reload_disabled"},
	{"prepaid_extra_usage_auto_reload_settings_updated", "prepaid_extra_usage_auto_reload_settings_updated"},
	{"extra_usage_spend_limit_created", "extra_usage_spend_limit_created"},
	{"extra_usage_spend_limit_updated", "extra_usage_spend_limit_updated"},
	{"extra_usage_spend_limit_deleted", "extra_usage_spend_limit_deleted"},

	// IP restrictions
	{"org_ip_restriction_created", "org_ip_restriction_created"},
	{"org_ip_restriction_updated", "org_ip_restriction_updated"},
	{"org_ip_restriction_deleted", "org_ip_restriction_deleted"},

	// Bulk operations
	{"org_bulk_delete_initiated", "org_bulk_delete_initiated"},
	{"org_deleted_via_bulk", "org_deleted_via_bulk"},

	// GitHub Enterprise
	{"ghe_configuration_created", "ghe_configuration_created"},
	{"ghe_configuration_updated", "ghe_configuration_updated"},
	{"ghe_configuration_deleted", "ghe_configuration_deleted"},
	{"ghe_user_connected", "ghe_user_connected"},
	{"ghe_user_disconnected", "ghe_user_disconnected"},
	{"ghe_webhook_signature_invalid", "ghe_webhook_signature_invalid"},
}

// knownDuplicateCSVKeys lists CSV event types that legitimately map to
// multiple API activity types. The CSV audit log doesn't distinguish
// between integration providers, while the API has separate event types
// for Google Drive and GitHub.
var knownDuplicateCSVKeys = map[string]bool{
	"integration_org_enabled":        true,
	"integration_org_disabled":       true,
	"integration_org_config_updated": true,
}

// MappedCSVTypes returns the set of CSV event types that have a known
// API equivalent.
func MappedCSVTypes() map[string]bool {
	m := make(map[string]bool, len(CSVToAPIMappings))
	for _, em := range CSVToAPIMappings {
		m[em.CSV] = true
	}
	return m
}

// MappedAPITypes returns the set of API activity types that have a known
// CSV equivalent.
func MappedAPITypes() map[string]bool {
	m := make(map[string]bool, len(CSVToAPIMappings))
	for _, em := range CSVToAPIMappings {
		m[em.API] = true
	}
	return m
}

// CSVToAPIMap returns a map from CSV event type to API activity type.
// For CSV events that map to multiple API types (integration_org_*),
// only the first encountered mapping is returned.
func CSVToAPIMap() map[string]string {
	m := make(map[string]string, len(CSVToAPIMappings))
	for _, em := range CSVToAPIMappings {
		if _, exists := m[em.CSV]; !exists {
			m[em.CSV] = em.API
		}
	}
	return m
}
