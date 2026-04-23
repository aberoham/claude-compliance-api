package compliance

// Activity type constants for the Anthropic Compliance API Rev I (2026-03-29).
// Organized by category to serve as both compile-time references and living
// documentation of supported types.

// Admin API key activity types.
const (
	ActivityAdminAPIKeyCreated = "admin_api_key_created"
	ActivityAdminAPIKeyDeleted = "admin_api_key_deleted"
	ActivityAdminAPIKeyUpdated = "admin_api_key_updated"
)

// API activity types.
const (
	ActivityAPIKeyCreated         = "api_key_created"
	ActivityComplianceAPIAccessed = "compliance_api_accessed"
)

// Artifact activity types.
const (
	ActivityArtifactAccessFailed   = "claude_artifact_access_failed"
	ActivityArtifactSharingUpdated = "claude_artifact_sharing_updated"
	ActivityArtifactViewed         = "claude_artifact_viewed"
)

// Authentication activity types.
const (
	ActivityAgeVerified              = "age_verified"
	ActivityAnonMobileLoginAttempted = "anonymous_mobile_login_attempted"
	ActivityMagicLinkLoginFailed     = "magic_link_login_failed"
	ActivityMagicLinkLoginInitiated  = "magic_link_login_initiated"
	ActivityMagicLinkLoginSucceeded  = "magic_link_login_succeeded"
	ActivityPhoneCodeSent            = "phone_code_sent"
	ActivityPhoneCodeVerified        = "phone_code_verified"
	ActivitySessionRevoked           = "session_revoked"
	ActivitySocialLoginSucceeded     = "social_login_succeeded"
	ActivitySSOLoginFailed           = "sso_login_failed"
	ActivitySSOLoginInitiated        = "sso_login_initiated"
	ActivitySSOLoginSucceeded        = "sso_login_succeeded"
	ActivitySSOSecondFactorMagicLink = "sso_second_factor_magic_link"
	ActivityUserLoggedOut            = "user_logged_out"
)

// Billing activity types.
const (
	ActivityExtraUsageBillingEnabled                   = "extra_usage_billing_enabled"
	ActivityExtraUsageCreditGranted                    = "extra_usage_credit_granted"
	ActivityExtraUsageSpendLimitCreated                = "extra_usage_spend_limit_created"
	ActivityExtraUsageSpendLimitDeleted                = "extra_usage_spend_limit_deleted"
	ActivityExtraUsageSpendLimitUpdated                = "extra_usage_spend_limit_updated"
	ActivityPrepaidExtraUsageAutoReloadDisabled        = "prepaid_extra_usage_auto_reload_disabled"
	ActivityPrepaidExtraUsageAutoReloadEnabled         = "prepaid_extra_usage_auto_reload_enabled"
	ActivityPrepaidExtraUsageAutoReloadSettingsUpdated = "prepaid_extra_usage_auto_reload_settings_updated"
)

// Chat activity types.
const (
	ActivityChatAccessFailed    = "claude_chat_access_failed"
	ActivityChatCreated         = "claude_chat_created"
	ActivityChatDeleted         = "claude_chat_deleted"
	ActivityChatDeletionFailed  = "claude_chat_deletion_failed"
	ActivityChatSettingsUpdated = "claude_chat_settings_updated"
	ActivityChatUpdated         = "claude_chat_updated"
	ActivityChatViewed          = "claude_chat_viewed"
)

// Chat snapshot activity types.
const (
	ActivityChatSnapshotCreated = "claude_chat_snapshot_created"
	ActivityChatSnapshotViewed  = "claude_chat_snapshot_viewed"
)

// Customization activity types (commands, plugins, skills).
const (
	ActivityCommandCreated  = "claude_command_created"
	ActivityCommandDeleted  = "claude_command_deleted"
	ActivityCommandReplaced = "claude_command_replaced"
	ActivityPluginCreated   = "claude_plugin_created"
	ActivityPluginDeleted   = "claude_plugin_deleted"
	ActivityPluginReplaced  = "claude_plugin_replaced"
	ActivityPluginUpdated   = "claude_plugin_updated"
	ActivitySkillCreated    = "claude_skill_created"
	ActivitySkillDeleted    = "claude_skill_deleted"
	ActivitySkillReplaced   = "claude_skill_replaced"
)

// File activity types.
const (
	ActivityFileAccessFailed = "claude_file_access_failed"
	ActivityFileDeleted      = "claude_file_deleted"
	ActivityFileUploaded     = "claude_file_uploaded"
	ActivityFileViewed       = "claude_file_viewed"
)

// GitHub Enterprise activity types.
const (
	ActivityGHEConfigCreated           = "ghe_configuration_created"
	ActivityGHEConfigDeleted           = "ghe_configuration_deleted"
	ActivityGHEConfigUpdated           = "ghe_configuration_updated"
	ActivityGHEUserConnected           = "ghe_user_connected"
	ActivityGHEUserDisconnected        = "ghe_user_disconnected"
	ActivityGHEWebhookSignatureInvalid = "ghe_webhook_signature_invalid"
)

// Group (RBAC) activity types.
const (
	ActivityGroupCreated          = "group_created"
	ActivityGroupDeleted          = "group_deleted"
	ActivityGroupListViewed       = "group_list_viewed"
	ActivityGroupMemberAdded      = "group_member_added"
	ActivityGroupMemberListViewed = "group_member_list_viewed"
	ActivityGroupMemberRemoved    = "group_member_removed"
	ActivityGroupUpdated          = "group_updated"
	ActivityGroupViewed           = "group_viewed"
)

// Integration activity types.
const (
	ActivityGDriveIntegrationCreated    = "claude_gdrive_integration_created"
	ActivityGDriveIntegrationDeleted    = "claude_gdrive_integration_deleted"
	ActivityGDriveIntegrationUpdated    = "claude_gdrive_integration_updated"
	ActivityGitHubIntegrationCreated    = "claude_github_integration_created"
	ActivityGitHubIntegrationDeleted    = "claude_github_integration_deleted"
	ActivityGitHubIntegrationUpdated    = "claude_github_integration_updated"
	ActivityIntegrationUserConnected    = "integration_user_connected"
	ActivityIntegrationUserDisconnected = "integration_user_disconnected"
)

// LTI (Learning Tools Interoperability) activity types.
const (
	ActivityLTILaunchInitiated = "lti_launch_initiated"
	ActivityLTILaunchSuccess   = "lti_launch_success"
)

// Marketplace activity types.
const (
	ActivityMarketplaceCreated = "marketplace_created"
	ActivityMarketplaceDeleted = "marketplace_deleted"
	ActivityMarketplaceUpdated = "marketplace_updated"
)

// MCP server activity types.
const (
	ActivityMCPServerCreated     = "mcp_server_created"
	ActivityMCPServerDeleted     = "mcp_server_deleted"
	ActivityMCPServerUpdated     = "mcp_server_updated"
	ActivityMCPToolPolicyUpdated = "mcp_tool_policy_updated"
)

// Org management activity types.
const (
	ActivityOrgInviteViewed = "org_invite_viewed"
	ActivityOrgInvitesListed = "org_invites_listed"
	ActivityOrgUserViewed   = "org_user_viewed"
	ActivityOrgUsersListed  = "org_users_listed"
)

// Organization discoverability activity types.
const (
	ActivityOrgDiscoverabilityDisabled        = "org_discoverability_disabled"
	ActivityOrgDiscoverabilityEnabled         = "org_discoverability_enabled"
	ActivityOrgDiscoverabilitySettingsUpdated = "org_discoverability_settings_updated"
	ActivityOrgJoinRequestApproved            = "org_join_request_approved"
	ActivityOrgJoinRequestCreated             = "org_join_request_created"
	ActivityOrgJoinRequestDismissed           = "org_join_request_dismissed"
	ActivityOrgJoinRequestInstantApproved     = "org_join_request_instant_approved"
	ActivityOrgJoinRequestsBulkDismissed      = "org_join_requests_bulk_dismissed"
	ActivityOrgMemberInvitesDisabled          = "org_member_invites_disabled"
	ActivityOrgMemberInvitesEnabled           = "org_member_invites_enabled"
)

// Organization settings activity types.
const (
	ActivityComplianceAPISettingsUpdated      = "org_compliance_api_settings_updated"
	ActivityOrgAnalyticsAPICapUpdated         = "org_analytics_api_capability_updated"
	ActivityOrgBulkDeleteInitiated            = "org_bulk_delete_initiated"
	ActivityOrgClaudeCodeDataSharingDisabled  = "org_claude_code_data_sharing_disabled"
	ActivityOrgClaudeCodeDataSharingEnabled   = "org_claude_code_data_sharing_enabled"
	ActivityOrgClaudeCodeDesktopDisabled      = "org_claude_code_desktop_disabled"
	ActivityOrgClaudeCodeDesktopEnabled       = "org_claude_code_desktop_enabled"
	ActivityOrgCoworkAgentDisabled            = "org_cowork_agent_disabled"
	ActivityOrgCoworkAgentEnabled             = "org_cowork_agent_enabled"
	ActivityOrgCoworkDisabled                 = "org_cowork_disabled"
	ActivityOrgCoworkEnabled                  = "org_cowork_enabled"
	ActivityOrgCreationBlocked               = "org_creation_blocked"
	ActivityOrgDataExportCompleted            = "org_data_export_completed"
	ActivityOrgDataExportStarted             = "org_data_export_started"
	ActivityOrgDeletedViaBulk                = "org_deleted_via_bulk"
	ActivityOrgDeletionRequested             = "org_deletion_requested"
	ActivityOrgDomainAddInitiated            = "org_domain_add_initiated"
	ActivityOrgDomainRemoved                 = "org_domain_removed"
	ActivityOrgDomainVerified                = "org_domain_verified"
	ActivityOrgIPRestrictionCreated           = "org_ip_restriction_created"
	ActivityOrgIPRestrictionDeleted           = "org_ip_restriction_deleted"
	ActivityOrgIPRestrictionUpdated           = "org_ip_restriction_updated"
	ActivityOrgInviteLinkDisabled            = "org_invite_link_disabled"
	ActivityOrgInviteLinkGenerated           = "org_invite_link_generated"
	ActivityOrgInviteLinkRegenerated         = "org_invite_link_regenerated"
	ActivityOrgMembersExported               = "org_members_exported"
	ActivityOrgParentJoinProposalCreated     = "org_parent_join_proposal_created"
	ActivityOrgParentSearchPerformed         = "org_parent_search_performed"
	ActivityOrgSettingsUpdated               = "claude_organization_settings_updated"
	ActivityOrgSyncDeletingSyncFilesStarted  = "org_sync_deleting_synchronized_files_started"
	ActivityOrgSyncSynchronizedFilesDeleted  = "org_sync_synchronized_files_deleted"
	ActivityOrgTaintAdded                    = "org_taint_added"
	ActivityOrgTaintRemoved                  = "org_taint_removed"
	ActivityOrgUserDeleted                   = "org_user_deleted"
	ActivityOrgUserInviteAccepted            = "org_user_invite_accepted"
	ActivityOrgUserInviteDeleted             = "org_user_invite_deleted"
	ActivityOrgUserInviteReSent              = "org_user_invite_re_sent"
	ActivityOrgUserInviteRejected            = "org_user_invite_rejected"
	ActivityOrgUserInviteSent                = "org_user_invite_sent"
	ActivityOrgWorkAcrossAppsDisabled        = "org_work_across_apps_disabled"
	ActivityOrgWorkAcrossAppsEnabled         = "org_work_across_apps_enabled"
	ActivityOwnedProjectsAccessRestored      = "owned_projects_access_restored"
	ActivityRoleAssignmentGranted            = "role_assignment_granted"
	ActivityRoleAssignmentRevoked            = "role_assignment_revoked"
)

// Platform file activity types.
const (
	ActivityPlatformFileContentDownloaded = "platform_file_content_downloaded"
	ActivityPlatformFileDeleted           = "platform_file_deleted"
	ActivityPlatformFileUploaded          = "platform_file_uploaded"
)

// Platform org management activity types.
const (
	ActivityPlatformAPIKeyCreated               = "platform_api_key_created"
	ActivityPlatformAPIKeyUpdated               = "platform_api_key_updated"
	ActivityPlatformCostReportViewed            = "platform_cost_report_viewed"
	ActivityPlatformUsageReportCCViewed         = "platform_usage_report_claude_code_viewed"
	ActivityPlatformUsageReportMessagesViewed   = "platform_usage_report_messages_viewed"
	ActivityPlatformWorkspaceArchived           = "platform_workspace_archived"
	ActivityPlatformWorkspaceCreated            = "platform_workspace_created"
	ActivityPlatformWorkspaceMemberAdded        = "platform_workspace_member_added"
	ActivityPlatformWorkspaceMemberRemoved      = "platform_workspace_member_removed"
	ActivityPlatformWorkspaceMemberUpdated      = "platform_workspace_member_updated"
	ActivityPlatformWorkspaceMemberViewed       = "platform_workspace_member_viewed"
	ActivityPlatformWorkspaceMembersListed      = "platform_workspace_members_listed"
	ActivityPlatformWorkspaceRateLimitDeleted   = "platform_workspace_rate_limit_deleted"
	ActivityPlatformWorkspaceRateLimitUpdated   = "platform_workspace_rate_limit_updated"
	ActivityPlatformWorkspaceUpdated            = "platform_workspace_updated"
)

// Platform skill activity types.
const (
	ActivityPlatformSkillVersionCreated = "platform_skill_version_created"
	ActivityPlatformSkillVersionDeleted = "platform_skill_version_deleted"
)

// Project activity types.
const (
	ActivityProjectArchived           = "claude_project_archived"
	ActivityProjectCreated            = "claude_project_created"
	ActivityProjectDeleted            = "claude_project_deleted"
	ActivityProjectDocAccessFailed    = "claude_project_document_access_failed"
	ActivityProjectDocDeleted         = "claude_project_document_deleted"
	ActivityProjectDocDeletionFailed  = "claude_project_document_deletion_failed"
	ActivityProjectDocUploaded        = "claude_project_document_uploaded"
	ActivityProjectDocViewed          = "claude_project_document_viewed"
	ActivityProjectFileAccessFailed   = "claude_project_file_access_failed"
	ActivityProjectFileDeleted        = "claude_project_file_deleted"
	ActivityProjectFileDeletionFailed = "claude_project_file_deletion_failed"
	ActivityProjectFileUploaded       = "claude_project_file_uploaded"
	ActivityProjectReported           = "claude_project_reported"
	ActivityProjectSharingUpdated     = "claude_project_sharing_updated"
	ActivityProjectViewed             = "claude_project_viewed"
)

// RBAC role activity types.
const (
	ActivityRBACRoleAssigned          = "rbac_role_assigned"
	ActivityRBACRoleCreated           = "rbac_role_created"
	ActivityRBACRoleDeleted           = "rbac_role_deleted"
	ActivityRBACRolePermissionAdded   = "rbac_role_permission_added"
	ActivityRBACRolePermissionRemoved = "rbac_role_permission_removed"
	ActivityRBACRoleUnassigned        = "rbac_role_unassigned"
	ActivityRBACRoleUpdated           = "rbac_role_updated"
)

// SCIM provisioning activity types.
const (
	ActivitySCIMUserCreated = "scim_user_created"
	ActivitySCIMUserDeleted = "scim_user_deleted"
	ActivitySCIMUserUpdated = "scim_user_updated"
)

// Service key activity types.
const (
	ActivityServiceCreated    = "service_created"
	ActivityServiceDeleted    = "service_deleted"
	ActivityServiceKeyCreated = "service_key_created"
	ActivityServiceKeyRevoked = "service_key_revoked"
)

// Session share activity types.
const (
	ActivitySessionShareAccessed = "session_share_accessed"
	ActivitySessionShareCreated  = "session_share_created"
	ActivitySessionShareRevoked  = "session_share_revoked"
)

// SSO and directory sync activity types.
const (
	ActivityOrgDirectoryResyncCompleted     = "org_directory_resync_completed"
	ActivityOrgDirectoryResyncFailed        = "org_directory_resync_failed"
	ActivityOrgDirectoryResyncStarted       = "org_directory_resync_started"
	ActivityOrgDirectorySyncActivated       = "org_directory_sync_activated"
	ActivityOrgDirectorySyncAddInitiated    = "org_directory_sync_add_initiated"
	ActivityOrgDirectorySyncDeleted         = "org_directory_sync_deleted"
	ActivityOrgMagicLinkSecondFactorToggled = "org_magic_link_second_factor_toggled"
	ActivityOrgSSOAddInitiated              = "org_sso_add_initiated"
	ActivityOrgSSOConnectionActivated       = "org_sso_connection_activated"
	ActivityOrgSSOConnectionDeactivated     = "org_sso_connection_deactivated"
	ActivityOrgSSOConnectionDeleted         = "org_sso_connection_deleted"
	ActivityOrgSSOGroupRoleMappingsUpdated  = "org_sso_group_role_mappings_updated"
	ActivityOrgSSOProvisioningModeChanged   = "org_sso_provisioning_mode_changed"
	ActivityOrgSSOSeatTierAssignmentToggled = "org_sso_seat_tier_assignment_toggled"
	ActivityOrgSSOSeatTierMappingsUpdated   = "org_sso_seat_tier_mappings_updated"
	ActivityOrgSSOToggled                   = "org_sso_toggled"
)

// User settings activity types.
const (
	ActivityUserRoleUpdated     = "claude_user_role_updated"
	ActivityUserSettingsUpdated = "claude_user_settings_updated"
)

// Category groupings for filtering and reporting.

var AdminAPIKeyActivities = []string{
	ActivityAdminAPIKeyCreated,
	ActivityAdminAPIKeyDeleted,
	ActivityAdminAPIKeyUpdated,
}

var APIActivities = []string{
	ActivityAPIKeyCreated,
	ActivityComplianceAPIAccessed,
}

var ArtifactActivities = []string{
	ActivityArtifactAccessFailed,
	ActivityArtifactSharingUpdated,
	ActivityArtifactViewed,
}

var AuthenticationActivities = []string{
	ActivityAgeVerified,
	ActivityAnonMobileLoginAttempted,
	ActivityMagicLinkLoginFailed,
	ActivityMagicLinkLoginInitiated,
	ActivityMagicLinkLoginSucceeded,
	ActivityPhoneCodeSent,
	ActivityPhoneCodeVerified,
	ActivitySessionRevoked,
	ActivitySocialLoginSucceeded,
	ActivitySSOLoginFailed,
	ActivitySSOLoginInitiated,
	ActivitySSOLoginSucceeded,
	ActivitySSOSecondFactorMagicLink,
	ActivityUserLoggedOut,
}

var BillingActivities = []string{
	ActivityExtraUsageBillingEnabled,
	ActivityExtraUsageCreditGranted,
	ActivityExtraUsageSpendLimitCreated,
	ActivityExtraUsageSpendLimitDeleted,
	ActivityExtraUsageSpendLimitUpdated,
	ActivityPrepaidExtraUsageAutoReloadDisabled,
	ActivityPrepaidExtraUsageAutoReloadEnabled,
	ActivityPrepaidExtraUsageAutoReloadSettingsUpdated,
}

var ChatActivities = []string{
	ActivityChatAccessFailed,
	ActivityChatCreated,
	ActivityChatDeleted,
	ActivityChatDeletionFailed,
	ActivityChatSettingsUpdated,
	ActivityChatUpdated,
	ActivityChatViewed,
}

var ChatSnapshotActivities = []string{
	ActivityChatSnapshotCreated,
	ActivityChatSnapshotViewed,
}

var CustomizationActivities = []string{
	ActivityCommandCreated,
	ActivityCommandDeleted,
	ActivityCommandReplaced,
	ActivityPluginCreated,
	ActivityPluginDeleted,
	ActivityPluginReplaced,
	ActivityPluginUpdated,
	ActivitySkillCreated,
	ActivitySkillDeleted,
	ActivitySkillReplaced,
}

var FileActivities = []string{
	ActivityFileAccessFailed,
	ActivityFileDeleted,
	ActivityFileUploaded,
	ActivityFileViewed,
}

var GHEActivities = []string{
	ActivityGHEConfigCreated,
	ActivityGHEConfigDeleted,
	ActivityGHEConfigUpdated,
	ActivityGHEUserConnected,
	ActivityGHEUserDisconnected,
	ActivityGHEWebhookSignatureInvalid,
}

var GroupActivities = []string{
	ActivityGroupCreated,
	ActivityGroupDeleted,
	ActivityGroupListViewed,
	ActivityGroupMemberAdded,
	ActivityGroupMemberListViewed,
	ActivityGroupMemberRemoved,
	ActivityGroupUpdated,
	ActivityGroupViewed,
}

var IntegrationActivities = []string{
	ActivityGDriveIntegrationCreated,
	ActivityGDriveIntegrationDeleted,
	ActivityGDriveIntegrationUpdated,
	ActivityGitHubIntegrationCreated,
	ActivityGitHubIntegrationDeleted,
	ActivityGitHubIntegrationUpdated,
	ActivityIntegrationUserConnected,
	ActivityIntegrationUserDisconnected,
}

var LTIActivities = []string{
	ActivityLTILaunchInitiated,
	ActivityLTILaunchSuccess,
}

var MarketplaceActivities = []string{
	ActivityMarketplaceCreated,
	ActivityMarketplaceDeleted,
	ActivityMarketplaceUpdated,
}

var MCPServerActivities = []string{
	ActivityMCPServerCreated,
	ActivityMCPServerDeleted,
	ActivityMCPServerUpdated,
	ActivityMCPToolPolicyUpdated,
}

var OrgDiscoverabilityActivities = []string{
	ActivityOrgDiscoverabilityDisabled,
	ActivityOrgDiscoverabilityEnabled,
	ActivityOrgDiscoverabilitySettingsUpdated,
	ActivityOrgJoinRequestApproved,
	ActivityOrgJoinRequestCreated,
	ActivityOrgJoinRequestDismissed,
	ActivityOrgJoinRequestInstantApproved,
	ActivityOrgJoinRequestsBulkDismissed,
	ActivityOrgMemberInvitesDisabled,
	ActivityOrgMemberInvitesEnabled,
}

var OrgManagementActivities = []string{
	ActivityOrgInviteViewed,
	ActivityOrgInvitesListed,
	ActivityOrgUserViewed,
	ActivityOrgUsersListed,
}

var PlatformFileActivities = []string{
	ActivityPlatformFileContentDownloaded,
	ActivityPlatformFileDeleted,
	ActivityPlatformFileUploaded,
}

var PlatformOrgManagementActivities = []string{
	ActivityPlatformAPIKeyCreated,
	ActivityPlatformAPIKeyUpdated,
	ActivityPlatformCostReportViewed,
	ActivityPlatformUsageReportCCViewed,
	ActivityPlatformUsageReportMessagesViewed,
	ActivityPlatformWorkspaceArchived,
	ActivityPlatformWorkspaceCreated,
	ActivityPlatformWorkspaceMemberAdded,
	ActivityPlatformWorkspaceMemberRemoved,
	ActivityPlatformWorkspaceMemberUpdated,
	ActivityPlatformWorkspaceMemberViewed,
	ActivityPlatformWorkspaceMembersListed,
	ActivityPlatformWorkspaceRateLimitDeleted,
	ActivityPlatformWorkspaceRateLimitUpdated,
	ActivityPlatformWorkspaceUpdated,
}

var PlatformSkillActivities = []string{
	ActivityPlatformSkillVersionCreated,
	ActivityPlatformSkillVersionDeleted,
}

var ProjectActivities = []string{
	ActivityProjectArchived,
	ActivityProjectCreated,
	ActivityProjectDeleted,
	ActivityProjectDocAccessFailed,
	ActivityProjectDocDeleted,
	ActivityProjectDocDeletionFailed,
	ActivityProjectDocUploaded,
	ActivityProjectDocViewed,
	ActivityProjectFileAccessFailed,
	ActivityProjectFileDeleted,
	ActivityProjectFileDeletionFailed,
	ActivityProjectFileUploaded,
	ActivityProjectReported,
	ActivityProjectSharingUpdated,
	ActivityProjectViewed,
}

var RBACRoleActivities = []string{
	ActivityRBACRoleAssigned,
	ActivityRBACRoleCreated,
	ActivityRBACRoleDeleted,
	ActivityRBACRolePermissionAdded,
	ActivityRBACRolePermissionRemoved,
	ActivityRBACRoleUnassigned,
	ActivityRBACRoleUpdated,
}

var SCIMActivities = []string{
	ActivitySCIMUserCreated,
	ActivitySCIMUserDeleted,
	ActivitySCIMUserUpdated,
}

var ServiceKeyActivities = []string{
	ActivityServiceCreated,
	ActivityServiceDeleted,
	ActivityServiceKeyCreated,
	ActivityServiceKeyRevoked,
}

var SessionShareActivities = []string{
	ActivitySessionShareAccessed,
	ActivitySessionShareCreated,
	ActivitySessionShareRevoked,
}

var SSODirectorySyncActivities = []string{
	ActivityOrgDirectoryResyncCompleted,
	ActivityOrgDirectoryResyncFailed,
	ActivityOrgDirectoryResyncStarted,
	ActivityOrgDirectorySyncActivated,
	ActivityOrgDirectorySyncAddInitiated,
	ActivityOrgDirectorySyncDeleted,
	ActivityOrgMagicLinkSecondFactorToggled,
	ActivityOrgSSOAddInitiated,
	ActivityOrgSSOConnectionActivated,
	ActivityOrgSSOConnectionDeactivated,
	ActivityOrgSSOConnectionDeleted,
	ActivityOrgSSOGroupRoleMappingsUpdated,
	ActivityOrgSSOProvisioningModeChanged,
	ActivityOrgSSOSeatTierAssignmentToggled,
	ActivityOrgSSOSeatTierMappingsUpdated,
	ActivityOrgSSOToggled,
}

var UserSettingsActivities = []string{
	ActivityUserRoleUpdated,
	ActivityUserSettingsUpdated,
}

// AdminActivities is the broad set of organization administration events.
var AdminActivities = []string{
	ActivityComplianceAPISettingsUpdated,
	ActivityOrgAnalyticsAPICapUpdated,
	ActivityOrgBulkDeleteInitiated,
	ActivityOrgClaudeCodeDataSharingDisabled,
	ActivityOrgClaudeCodeDataSharingEnabled,
	ActivityOrgClaudeCodeDesktopDisabled,
	ActivityOrgClaudeCodeDesktopEnabled,
	ActivityOrgCoworkAgentDisabled,
	ActivityOrgCoworkAgentEnabled,
	ActivityOrgCoworkDisabled,
	ActivityOrgCoworkEnabled,
	ActivityOrgCreationBlocked,
	ActivityOrgDataExportCompleted,
	ActivityOrgDataExportStarted,
	ActivityOrgDeletedViaBulk,
	ActivityOrgDeletionRequested,
	ActivityOrgDomainAddInitiated,
	ActivityOrgDomainRemoved,
	ActivityOrgDomainVerified,
	ActivityOrgIPRestrictionCreated,
	ActivityOrgIPRestrictionDeleted,
	ActivityOrgIPRestrictionUpdated,
	ActivityOrgInviteLinkDisabled,
	ActivityOrgInviteLinkGenerated,
	ActivityOrgInviteLinkRegenerated,
	ActivityOrgMembersExported,
	ActivityOrgParentJoinProposalCreated,
	ActivityOrgParentSearchPerformed,
	ActivityOrgSettingsUpdated,
	ActivityOrgSyncDeletingSyncFilesStarted,
	ActivityOrgSyncSynchronizedFilesDeleted,
	ActivityOrgTaintAdded,
	ActivityOrgTaintRemoved,
	ActivityOrgUserDeleted,
	ActivityOrgUserInviteAccepted,
	ActivityOrgUserInviteDeleted,
	ActivityOrgUserInviteReSent,
	ActivityOrgUserInviteRejected,
	ActivityOrgUserInviteSent,
	ActivityOrgWorkAcrossAppsDisabled,
	ActivityOrgWorkAcrossAppsEnabled,
	ActivityOwnedProjectsAccessRestored,
	ActivityRoleAssignmentGranted,
	ActivityRoleAssignmentRevoked,
}
