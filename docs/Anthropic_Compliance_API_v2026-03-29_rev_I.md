


Here is the complete and well-formed Markdown extraction of the Anthropic Compliance API document, with all watermarks, headers, and footers removed for clean processing by an LLM.

***

# Compliance API: Activity Feed, Chats, Files, Organizations, Users, and Projects
**API Version:** 2026-03-29 (Rev I)

## Table of Contents
* Overview
* Key capabilities
* Availability
* Use Cases
* Data Retention
* Authentication and Authorization
* API Specification
  * Notices
  * Endpoints
  * Activity Feed
  * Objects APIs
  * Types
* Getting Started
* Error Handling
* Appendix A: Changelog
* Appendix B: Comparison with Audit Log Events

---

## Overview
The Compliance API helps Enterprise customers meet regulatory compliance requirements by providing API access to organizational data and activity logs across Claude.ai, Claude Console, and Claude API.

### Key capabilities
* **Activity Feed:** Track user activities including authentication, chat interactions, file uploads, and administrative actions
* **Organizations and Users:** List all organizations under your parent organization and retrieve member users for compliance reporting
* **Chats and Files:** Retrieve chat messages and file contents to assist with data loss prevention, e-discovery, and other efforts
* **Projects:** Access and manage project data including names, descriptions, instructions, and attachments
* **Data Management:** Delete specific chats, files, projects, and project documents when required for compliance

## Availability
The Compliance API is available to Enterprise customers across Claude.ai, Claude Console, and Claude API. To enable or disable Compliance API access and/or activity logging for the Activity Feed:
* **Claude.ai:** Have the **Primary Owner** of your organization enable it in the **Data and Privacy** section of your organization settings.
* **Console / API:** Have the **admin** of your organization send the request to your Anthropic representative.

## Use Cases

### eDiscovery (Claude.ai)
Legal teams can export organizational communications for document preservation and analysis:
1. Use the Organizations and Users API to identify all users across your organizations
2. Use the Activity Feed API to identify relevant time periods and users
3. List chats and projects for specific employees during date ranges
4. Export chat contents, project data, and file attachments for legal review

### Data Loss Prevention (Claude.ai)
Security teams can monitor and remediate potential policy violations:
1. Monitor file uploads and chat activities via Activity Feed
2. Review chat contents for sensitive information
3. Delete non-compliant chats and files

### Audit and Compliance (Claude.ai / Console / API)
Compliance officers can maintain comprehensive audit trails:
1. Track all user activities with IP addresses and timestamps
2. Monitor administrative actions and API access
3. Generate compliance reports for regulatory audits

### API Key and Workspace Governance (Console / API)
Security teams can audit administrative actions taken in Claude Console and via the Admin API:
1. Track creation, update, and rotation of API keys and Admin keys
2. Monitor workspace lifecycle events and membership changes
3. Review rate-limit changes and usage-report access

### Developer Resource Auditing (Console / API)
Platform teams can track developer resources managed through Claude Console and Claude API:
1. Monitor file uploads, downloads, and deletions via the Files API
2. Track skill and skill-version lifecycle events via the Skills API
3. Correlate resource changes with the API key that performed them

## Data Retention
* Activity Feed data is retained for 6 years
* Activity Feed activities are queryable after a short delay of up to a minute of the actual event
* Chat, File, and Project content follows organization retention policies. The default retention is indefinite and adjustable in **Settings → Data management**
* Content deleted through the Compliance API is immediately and permanently deleted
* Content deleted by users is not accessible via the Compliance API

## Authentication and Authorization
All requests to the Compliance API must include an `x-api-key` header with either a **Compliance Access Key** (created in Claude.ai) or an **Admin key** (created in Claude Console). Which key type you use depends on which product your organization uses.

Admin keys created through Claude Console are limited to the Activity Feed and cannot access chat, file, or project retrieval endpoints (because those features are only applicable to Claude.ai).

### Claude.ai
**Compliance Access Keys**
Compliance Access Keys are created by the Primary Owner at the parent organization level and provide scoped access to data across all linked organizations. These keys are separate from other Anthropic API keys and are bound to specific authorized scopes for compliance operations.

**Creating a Key**
Keys are created in the **Compliance access keys** section of Data Management Settings. Press **Create key** to name your key, choose its scopes, and receive a secret access key.

*If you do not see the Compliance access keys section, it means that either you are not a Primary Owner of the organization, or that the Compliance API is not enabled for your organization and the Primary Owner needs to enable it in the Data and Privacy section of your organization settings.*

**Choosing a Scope**
All Compliance Access Keys must be granted one or more specific scopes at creation time. Scopes are immutable once assigned.

| Scope | Description |
|---|---|
| `read:compliance_activities` | Grants ability to read organizations and user activity data |
| `read:compliance_user_data` | Grants ability to read user chats and files |
| `delete:compliance_user_data` | Grants ability to delete user chats and files |
| `read:compliance_org_data` | Grants ability to read organization metadata including organization names and types |

**Managing Keys**
Created keys appear in the list. Keys can be disabled or deleted. When a key is disabled it cannot be used to make requests to the API and can be later re-enabled. When a key is deleted it is permanently deleted and can no longer be used.

### Console / API
**Admin Keys**
Admin keys are created by an organization admin in Claude Console and serve double duty: they authenticate Admin API requests for managing your organization, and — when the Compliance API is enabled — they authenticate Activity Feed requests.

**Creating a Key**
Keys are created in the **Admin keys** section of Console Settings. Press **Create key** to name your key and receive a secret access key.
*If the Compliance API is enabled for your organization, Admin keys created here are automatically granted the `read:compliance_activities` scope. If the Compliance API is not yet enabled, contact your Anthropic representative to request access.*

**Scope**
Admin keys carry a fixed `read:compliance_activities` scope, granting access to the Activity Feed only. All other endpoints return a 401 authentication error when called with an Admin key (because those features are only applicable to Claude.ai).

**Managing Keys**
Keys are listed, disabled, and deleted from the same **Admin keys** section in Console Settings.

---

## API Specification

### Notices
* **August 19, 2025:** The API specification is in development and subject to change. In case of breaking changes, Anthropic will reach out to you ahead of time with migration steps and a timeline to ensure a smooth transition.

### Endpoints

| Method | Endpoint | Scope | Description |
|---|---|---|---|
| GET | `/v1/compliance/activities` | `read:compliance_activities` | Query compliance activities |
| GET | `/v1/compliance/apps/chats` | `read:compliance_user_data` | List chats |
| GET | `/v1/compliance/apps/chats/{claude_chat_id}/messages` | `read:compliance_user_data` | Get chat messages |
| DELETE | `/v1/compliance/apps/chats/{claude_chat_id}` | `delete:compliance_user_data` | Delete chat |
| GET | `/v1/compliance/apps/chats/files/{claude_file_id}/content` | `read:compliance_user_data` | Download file content |
| GET | `/v1/compliance/apps/artifacts/{artifact_version_id}/content` | `read:compliance_user_data` | Download artifact content |
| DELETE | `/v1/compliance/apps/chats/files/{claude_file_id}` | `delete:compliance_user_data` | Delete file |
| GET | `/v1/compliance/organizations` | `read:compliance_org_data` | List organizations |
| GET | `/v1/compliance/organizations/{org_uuid}/users` | `read:compliance_user_data` | List organization users |
| GET | `/v1/compliance/apps/projects` | `read:compliance_user_data` | List projects |
| GET | `/v1/compliance/apps/projects/{project_id}` | `read:compliance_user_data` | Get project details |
| DELETE | `/v1/compliance/apps/projects/{project_id}` | `delete:compliance_user_data` | Delete project |
| GET | `/v1/compliance/apps/projects/{project_id}/attachments` | `read:compliance_user_data` | List project attachments |
| GET | `/v1/compliance/apps/projects/documents/{document_id}` | `read:compliance_user_data` | Get project document content |
| DELETE | `/v1/compliance/apps/projects/documents/{document_id}` | `delete:compliance_user_data` | Delete project document |
| GET | `/v1/compliance/organizations/{org_uuid}/roles` | `read:compliance_org_data` | List Compliance Roles |
| GET | `/v1/compliance/organizations/{org_uuid}/roles/{role_id}` | `read:compliance_org_data` | Get Compliance Role |
| GET | `/v1/compliance/groups` | `read:compliance_org_data` | List Compliance Groups |
| GET | `/v1/compliance/groups/{group_id}` | `read:compliance_org_data` | Get Compliance Group |
| GET | `/v1/compliance/groups/{group_id}/members` | `read:compliance_user_data` | List Compliance Group Members |

**Notes:**
* All endpoints use host: `https://api.anthropic.com`
* Throughout this reference, a **chat** refers to a conversation of **chat messages** between a **user** and an **assistant** (Claude).

---

## Activity Feed
The Activity Feed provides comprehensive logging of activities within your organization.

### List Activities
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/activities`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key) or `x-api-key: sk-ant-admin01-...` (Admin key)
**Required Scope:** `read:compliance_activities`

Retrieves a paginated list of compliance activities with optional filtering. Activities are returned in reverse chronological order (newest first), with ties broken by activity ID.

#### Query Parameters
| Parameter | Type | Description | Example |
|---|---|---|---|
| `organization_ids[]` *(Repeatable)* | array | Filter by organization IDs (accepts `org_...` or organization UUID) | `organization_ids[]=org_123abc&organization_ids[]=org_234bcd` |
| `actor_ids[]` *(Repeatable)* | array | Filter by user | `actor_ids[]=user_abc123&actor_ids[]=user_bcd234` |
| `activity_types[]` *(Repeatable)* | array | Filter by activity types | `activity_types[]=claude_file_viewed&activity_types[]=api_key_created` |
| `created_at.gte` | string | Activities created at or after (RFC 3339) | `created_at.gte=2024-01-01T00:00:00Z` |
| `created_at.gt` | string | Activities created after (RFC 3339) | `created_at.gt=2024-01-01T00:00:00Z` |
| `created_at.lte` | string | Activities created at or before (RFC 3339) | `created_at.lte=2024-12-31T23:59:59Z` |
| `created_at.lt` | string | Activities created before (RFC 3339) | `created_at.lt=2024-12-31T23:59:59Z` |
| `after_id` | string | Activity ID for forward pagination | `after_id=activity_abc123` |
| `before_id` | string | Activity ID for backward pagination | `before_id=activity_xyz789` |
| `limit` | integer | Maximum results (default: 100, max: 5000) | `limit=500` |

**Notes:**
* Since activities are returned in newest-to-oldest order, `before_id` returns activities that occurred after the specified ID in time (but before it in the API's sort order). Only one of `before_id` or `after_id` can be specified per request.
* To retrieve the next page of results (heading backwards in time) set `after_id` to the value of `last_id`.
* To retrieve the previous page of results (heading forwards in time) set `before_id` to the value of `first_id`.
* Clients should treat pagination values as opaque strings and not attempt to parse or interpret contents, as formats may change without notice.

#### Response
A JSON object containing:
| Parameter | Type | Description |
|---|---|---|
| `data` | array of `Activity` | Array of `Activity` objects |
| `has_more` | boolean | Whether more results are available |
| `first_id` | string | Cursor for pagination. Set this value as the `before_id` query parameter to retrieve the previous page of results (heading forwards in time). |
| `last_id` | string | Cursor for pagination. Set this value as the `after_id` query parameter to retrieve the next page of results (heading backwards in time). |

#### Activity
| Parameter | Type | Description |
|---|---|---|
| `id` | string | Unique identifier for the activity |
| `created_at` | string | When the activity occurred (RFC 3339) |
| `organization_id` | string or null | Organization ID where the activity occurred (null when not tied to an organization, e.g. during login/log out and activities representing calls to the Compliance API) |
| `organization_uuid` | string or null | Organization UUID where the activity occurred (null when not tied to an organization, e.g. during login/log out and activities representing calls to the Compliance API) |
| `actor` | Actor | `UserActor`, `ApiActor`, `AdminApiKeyActor`, `UnauthenticatedUserActor`, `AnthropicActor`, `ScimDirectorySyncActor` |
| `type` | string | Type of activity that occurred (see Activity Types and Fields) |
| *Additional fields* | various | Activity-specific fields based on the type (see Activity Types and Fields) |

#### Actor Types
**UserActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `user_actor` |
| `email_address` | string | Email address of actor |
| `user_id` | string | User ID |
| `ip_address` | string | Originating IP address of the activity |
| `user_agent` | string | Originating user agent of the activity |

**ApiActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `api_actor` |
| `api_key_id` | string | ID of the API key used to do the activity |
| `ip_address` | string | Originating IP address of the activity |
| `user_agent` | string | Originating user agent of the activity |

**UnauthenticatedUserActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `unauthenticated_user_actor` |
| `unauthenticated_email_address` | string | Email address provided by unauthenticated user |
| `ip_address` | string | Originating IP address of the activity |
| `user_agent` | string | Originating user agent of the activity |

**AnthropicActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `anthropic_actor` |
| `email_address` | null | Always `null` |

**ScimDirectorySyncActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `scim_directory_sync_actor` |
| `workos_event_id` | string | Unique identifier of the directory sync event |
| `directory_id` | string | Identifier of the directory sync connection |
| `idp_connection_type` | string or null | Identity provider type (e.g., `OktaSCIMV2`, `AzureSCIMV2`) |

**AdminApiKeyActor**
| Parameter | Type | Description |
|---|---|---|
| `type` | string | `admin_api_key_actor` |
| `admin_api_key_id` | string | Tagged identifier of the Admin key that performed the action |

#### Example Response
```json
{
  "data":[
    {
      "id": "activity_abc123",
      "created_at": "2025-06-07T08:09:10Z",
      "organization_id": "org_abc123",
      "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
      "actor": {
        "type": "user_actor",
        "email_address": "user@example.com",
        "user_id": "user_xyz456",
        "ip_address": "192.0.2.34",
        "user_agent": "Mozilla/5.0..."
      },
      "type": "claude_chat_created",
      "claude_chat_id": "claude_chat_xyz789",
      "claude_project_id": null
    }
  ],
  "has_more": true,
  "first_id": "activity_abc123",
  "last_id": "activity_xyz789"
}
```

---

### Activity Types and Fields

#### API
| Activity Type | Additional Fields | Description |
|---|---|---|
| `api_key_created` | `api_key_id`, `scopes` | Activity logged when a new API key is created. |
| `compliance_api_accessed` | `request_id`, `url`, `request_method`, `request_body`, `status_code` | Logging event auto-generated for each compliance API request. |

#### Admin Api Keys
| Activity Type | Additional Fields | Description |
|---|---|---|
| `admin_api_key_created` | `admin_api_key_id`, `scopes` | An admin API key was created. |
| `admin_api_key_deleted` | `admin_api_key_id` | An admin API key was deleted. |
| `admin_api_key_updated` | `admin_api_key_id`, `updates` | An admin API key was updated (renamed or activated/deactivated). |

#### Artifacts
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_artifact_access_failed` | `claude_artifact_id`, `claude_artifact_version_id` | |
| `claude_artifact_sharing_updated` | `claude_artifact_id`, `audience`, `claude_artifact_version_id` | |
| `claude_artifact_viewed` | `claude_artifact_id` | |

#### Authentication
| Activity Type | Additional Fields | Description |
|---|---|---|
| `age_verified` | *(none)* | User age was verified. |
| `anonymous_mobile_login_attempted` | *(none)* | Anonymous mobile login was attempted. |
| `magic_link_login_failed` | *(none)* | |
| `magic_link_login_initiated` | *(none)* | |
| `magic_link_login_succeeded` | *(none)* | |
| `phone_code_sent` | *(none)* | User requested a phone verification code. |
| `phone_code_verified` | *(none)* | User successfully verified their phone code. |
| `session_revoked` | *(none)* | User revoked a specific session. |
| `social_login_succeeded` | `provider` | |
| `sso_login_failed` | *(none)* | |
| `sso_login_initiated` | *(none)* | |
| `sso_login_succeeded` | *(none)* | |
| `sso_second_factor_magic_link` | *(none)* | SSO second factor magic link was used. |
| `user_logged_out` | *(none)* | |

#### Billing
| Activity Type | Additional Fields | Description |
|---|---|---|
| `extra_usage_billing_enabled` | *(none)* | Overage billing was enabled for an organization. |
| `extra_usage_credit_granted` | *(none)* | A promotional credit grant was claimed for overage usage. |
| `extra_usage_spend_limit_created` | `limit_type`, `is_enabled`, `amount`, `user_id` | Extra usage spend limit was created. |
| `extra_usage_spend_limit_deleted` | `user_id` | Extra usage spend limit was deleted. |
| `extra_usage_spend_limit_updated` | `limit_type`, `is_enabled`, `amount`, `user_id` | Extra usage spend limit was updated. |
| `prepaid_extra_usage_auto_reload_disabled` | *(none)* | Prepaid extra usage auto-reload was disabled. |
| `prepaid_extra_usage_auto_reload_enabled` | *(none)* | Prepaid extra usage auto-reload was enabled. |
| `prepaid_extra_usage_auto_reload_settings_updated` | *(none)* | Prepaid extra usage auto-reload settings were updated. |

#### Chat Snapshots
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_chat_snapshot_created` | `claude_chat_snapshot_id`, `claude_chat_id` | User created/shared a chat snapshot. |
| `claude_chat_snapshot_viewed` | `claude_chat_snapshot_id`, `claude_chat_id` | User viewed a chat snapshot (authenticated or public/unauthenticated). |

#### Chats
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_chat_access_failed` | `claude_chat_id` | User tried to load a chat but did not have permissions. |
| `claude_chat_created` | `claude_chat_id`, `claude_project_id` | User created a chat. |
| `claude_chat_deleted` | `claude_chat_id`, `claude_project_id` | User deleted a chat. |
| `claude_chat_deletion_failed` | `claude_chat_id` | User tried to delete a chat unseccesfully. |
| `claude_chat_settings_updated` | `claude_chat_id`, `claude_project_id` | User updated the settings for a conversation. |
| `claude_chat_updated` | `claude_chat_id`, `claude_project_id` | User updated the chat metadata (e.g name, model). |
| `claude_chat_viewed` | `claude_chat_id`, `claude_project_id` | User viewed a chat. |

#### Customizations
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_command_created` | `command_id`, `command_name` | Command was created. |
| `claude_command_deleted` | `command_id`, `command_name` | Command was deleted. |
| `claude_command_replaced` | `command_id`, `command_name` | Command was replaced. |
| `claude_plugin_created` | `plugin_id`, `plugin_name` | Plugin was created. |
| `claude_plugin_deleted` | `plugin_id`, `plugin_name` | Plugin was deleted. |
| `claude_plugin_replaced` | `plugin_id`, `plugin_name` | Plugin was replaced. |
| `claude_plugin_updated` | `plugin_id`, `plugin_name` | Plugin was updated. |
| `claude_skill_created` | `skill_id`, `skill_name` | Skill was created. |
| `claude_skill_deleted` | `skill_id`, `skill_name` | Skill was deleted. |
| `claude_skill_replaced` | `skill_id`, `skill_name` | Skill was replaced. |

#### Files
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_file_access_failed` | `claude_file_id`, `filename`, `claude_project_id`, `claude_artifact_id` | |
| `claude_file_deleted` | `claude_file_id`, `filename` | |
| `claude_file_uploaded` | `claude_file_id`, `filename`, `claude_chat_id`, `claude_project_id` | |
| `claude_file_viewed` | `claude_file_id`, `filename`, `claude_project_id`, `claude_artifact_id` | |

#### GitHub Enterprise
| Activity Type | Additional Fields | Description |
|---|---|---|
| `ghe_configuration_created` | `ghe_configuration_id` | Admin created a GHE configuration. |
| `ghe_configuration_deleted` | `ghe_configuration_id` | Admin deleted a GHE configuration. |
| `ghe_configuration_updated` | `ghe_configuration_id` | Admin updated a GHE configuration. |
| `ghe_user_connected` | `ghe_configuration_id` | User connected to a GHE instance. |
| `ghe_user_disconnected` | `ghe_configuration_id` | User disconnected from a GHE instance. |
| `ghe_webhook_signature_invalid` | `ghe_configuration_id` | Webhook signature validation failed. |

#### Groups
| Activity Type | Additional Fields | Description |
|---|---|---|
| `group_created` | `group_id`, `group_name` | A group was created (RBAC admin or SCIM provisioning). |
| `group_deleted` | `group_id` | A group was deleted (RBAC admin or SCIM provisioning). |
| `group_list_viewed` | *(none)* | Admin viewed the list of RBAC groups. |
| `group_member_added` | `group_id`, `member_ids` | One or more members were added to a group. |
| `group_member_list_viewed` | `group_id` | Admin viewed the members of an RBAC group. |
| `group_member_removed` | `group_id`, `member_ids` | One or more members were removed from a group. |
| `group_updated` | `group_id` | A group was updated (RBAC admin or SCIM provisioning). |
| `group_viewed` | `group_id` | A group was viewed. |

#### Integrations
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_gdrive_integration_created` | `integration_id`, `folder_id` | |
| `claude_gdrive_integration_deleted` | `integration_id`, `folder_id` | |
| `claude_gdrive_integration_updated` | `integration_id`, `folder_id` | |
| `claude_github_integration_created` | `integration_id`, `repository_name`, `organization_name` | |
| `claude_github_integration_deleted` | `integration_id`, `repository_name`, `organization_name` | |
| `claude_github_integration_updated` | `integration_id`, `repository_name`, `organization_name` | |
| `integration_user_connected` | `integration_type` | User connected to an integration. |
| `integration_user_disconnected` | `integration_type` | User disconnected from an integration. |

#### LTI (Learning Tools Interoperability)
| Activity Type | Additional Fields | Description |
|---|---|---|
| `lti_launch_initiated` | *(none)* | LTI launch was initiated. |
| `lti_launch_success` | *(none)* | LTI launch completed successfully. |

#### Marketplaces
| Activity Type | Additional Fields | Description |
|---|---|---|
| `marketplace_created` | `marketplace_id` | Admin created an organization marketplace. |
| `marketplace_deleted` | `marketplace_id` | Admin deleted an organization marketplace. |
| `marketplace_updated` | `marketplace_id` | Admin updated an organization marketplace. |

#### Mcp Servers
| Activity Type | Additional Fields | Description |
|---|---|---|
| `mcp_server_created` | `mcp_server_id`, `mcp_server_name` | Admin added an MCP server to the organization. |
| `mcp_server_deleted` | `mcp_server_id`, `mcp_server_name` | Admin removed an MCP server from the organization. |
| `mcp_server_updated` | `mcp_server_id`, `mcp_server_name` | Admin updated an MCP server configuration. |
| `mcp_tool_policy_updated` | `mcp_server_id`, `mcp_server_name`, `tool_name`, `max_permission` | Admin set or cleared the permission restriction for a single MCP tool. |

#### Org Management
| Activity Type | Additional Fields | Description |
|---|---|---|
| `org_invite_viewed` | `invite_id` | An organization invite was viewed. |
| `org_invites_listed` | *(none)* | Organization invites were listed. |
| `org_user_viewed` | `user_id` | An organization user was viewed. |
| `org_users_listed` | *(none)* | Organization users were listed. |

#### Organization Discoverability
| Activity Type | Additional Fields | Description |
|---|---|---|
| `org_discoverability_disabled` | *(none)* | Admin disabled organization discoverability. |
| `org_discoverability_enabled` | *(none)* | Admin enabled organization discoverability. |
| `org_discoverability_settings_updated` | *(none)* | Admin updated organization discoverability settings. |
| `org_join_request_approved` | *(none)* | Admin approved a join request. |
| `org_join_request_created` | *(none)* | User requested to join an organization. |
| `org_join_request_dismissed` | *(none)* | Admin dismissed a join request. |
| `org_join_request_instant_approved` | *(none)* | Join request was instantly approved. |
| `org_join_requests_bulk_dismissed` | *(none)* | Admin bulk-dismissed join requests. |
| `org_member_invites_disabled` | *(none)* | Admin disabled member invites for the organization. |
| `org_member_invites_enabled` | *(none)* | Admin enabled member invites for the organization. |

#### Organization Settings
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_organization_settings_updated` | `updates` | Organization settings were updated. |
| `org_analytics_api_capability_updated` | *(none)* | Organization analytics_api capability was enabled or disabled. |
| `org_bulk_delete_initiated` | *(none)* | Organization bulk deletion was initiated. |
| `org_claude_code_data_sharing_disabled` | *(none)* | Organization Claude Code data sharing was disabled. |
| `org_claude_code_data_sharing_enabled` | *(none)* | Organization Claude Code data sharing was enabled. |
| `org_claude_code_desktop_disabled` | *(none)* | Organization Claude Code Desktop was disabled. |
| `org_claude_code_desktop_enabled` | *(none)* | Organization Claude Code Desktop was enabled. |
| `org_compliance_api_settings_updated` | `compliance_api_enabled`, `compliance_api_logging_enabled` | Organization compliance API settings were updated. |
| `org_cowork_agent_disabled` | *(none)* | Organization Cowork Agent was disabled. |
| `org_cowork_agent_enabled` | *(none)* | Organization Cowork Agent was enabled. |
| `org_cowork_disabled` | *(none)* | Organization cowork was disabled. |
| `org_cowork_enabled` | *(none)* | Organization cowork was enabled. |
| `org_creation_blocked` | `reason` | Organization creation was blocked. |
| `org_data_export_completed` | *(none)* | Organization data export was completed. |
| `org_data_export_started` | *(none)* | Organization data export was started. |
| `org_deleted_via_bulk` | *(none)* | Organization was deleted via bulk operation. |
| `org_deletion_requested` | *(none)* | Organization deletion was requested. |
| `org_domain_add_initiated` | *(none)* | Organization domain verification was initiated. |
| `org_domain_removed` | `domain` | Organization domain was removed. |
| `org_domain_verified` | `domain` | Organization domain was verified. |
| `org_invite_link_disabled` | *(none)* | Organization invite link was disabled. |
| `org_invite_link_generated` | *(none)* | Organization invite link was generated. |
| `org_invite_link_regenerated` | *(none)* | Organization invite link was regenerated (previous link invalidated). |
| `org_ip_restriction_created` | *(none)* | Organization IP restriction was created. |
| `org_ip_restriction_deleted` | *(none)* | Organization IP restriction was deleted. |
| `org_ip_restriction_updated` | *(none)* | Organization IP restriction was updated. |
| `org_members_exported` | *(none)* | Organization members list was exported as CSV. |
| `org_parent_join_proposal_created` | *(none)* | Organization parent join proposal was created. |
| `org_parent_search_performed` | *(none)* | Organization parent search was performed. |
| `org_sync_deleting_synchronized_files_started` | *(none)* | Organization started deleting synchronized files. |
| `org_sync_synchronized_files_deleted` | *(none)* | Organization synchronized files were deleted. |
| `org_taint_added` | `taint` | A taint was added to an organization. |
| `org_taint_removed` | `taint` | A taint was removed from an organization. |
| `org_user_deleted` | `deleted_user_id`, `deleted_user_email` | User was removed from organization. |
| `org_user_invite_accepted` | `invite_id` | Organization user invite was accepted. |
| `org_user_invite_deleted` | `invite_id` | Organization user invite was deleted. |
| `org_user_invite_re_sent` | `invited_email` | Organization user invite was re-sent. |
| `org_user_invite_rejected` | `invite_id` | Organization user invite was rejected. |
| `org_user_invite_sent` | `invited_email`, `invited_role` | Organization user invite was sent. |
| `org_work_across_apps_disabled` | *(none)* | Organization Work Across Apps was disabled. |
| `org_work_across_apps_enabled` | *(none)* | Organization Work Across Apps was enabled. |
| `owned_projects_access_restored` | `user_id` | Access to owned projects was restored. |
| `role_assignment_granted` | `target_id`, `target_type`, `role`, `resource_type`, `resource_id` | Role assignment was granted. |
| `role_assignment_revoked` | `target_id`, `target_type`, `role`, `resource_type`, `resource_id` | Role assignment was revoked. |

#### Platform Files
| Activity Type | Additional Fields | Description |
|---|---|---|
| `platform_file_content_downloaded` | `file_id` | Activity logged when file content is downloaded via GET `/v1/files/{file_id}/content`. |
| `platform_file_deleted` | `file_id` | Activity logged when a file is deleted via DELETE `/v1/files/{file_id}`. |
| `platform_file_uploaded` | `file_id`, `session_id` | Activity logged when a file is uploaded via POST `/v1/files`. |

#### Platform Org Management
| Activity Type | Additional Fields | Description |
|---|---|---|
| `platform_api_key_created` | `api_key_id` | An API key was created. |
| `platform_api_key_updated` | `api_key_id`, `updates` | An API key was updated. |
| `platform_cost_report_viewed` | *(none)* | The cost report was viewed. |
| `platform_usage_report_claude_code_viewed` | *(none)* | The Claude Code usage report was viewed. |
| `platform_usage_report_messages_viewed` | *(none)* | The messages usage report was viewed. |
| `platform_workspace_archived` | `workspace_id` | A workspace was archived. |
| `platform_workspace_created` | `workspace_id` | A workspace was created. |
| `platform_workspace_member_added` | `workspace_id`, `user_id` | A member was added to a workspace. |
| `platform_workspace_member_removed` | `workspace_id`, `user_id` | A member was removed from a workspace. |
| `platform_workspace_member_updated` | `workspace_id`, `user_id`, `updates` | A workspace member was updated. |
| `platform_workspace_member_viewed` | `workspace_id`, `user_id` | A workspace member was viewed. |
| `platform_workspace_members_listed` | `workspace_id` | Workspace members were listed. |
| `platform_workspace_rate_limit_deleted` | `workspace_id`, `model_group`, `limiter_type` | A workspace rate limit was deleted. |
| `platform_workspace_rate_limit_updated` | `workspace_id`, `model_group`, `limiter_type`, `value` | A workspace rate limit was created or updated. |
| `platform_workspace_updated` | `workspace_id`, `updates` | A workspace was updated. |

#### Platform Skills
| Activity Type | Additional Fields | Description |
|---|---|---|
| `platform_skill_version_created` | `skill_id`, `version` | Activity logged when a skill version is created via POST `/v1/skills/{skill_id}/versions`. |
| `platform_skill_version_deleted` | `skill_id`, `version` | Activity logged when a skill version is deleted via DELETE `/v1/skills/{skill_id}/versions/{version}`. |

#### Projects
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_project_archived` | `claude_project_id` | |
| `claude_project_created` | `claude_project_id` | |
| `claude_project_deleted` | `claude_project_id` | |
| `claude_project_document_access_failed` | `claude_project_id`, `claude_project_document_id`, `filename` | |
| `claude_project_document_deleted` | `claude_project_id`, `claude_project_document_id`, `filename` | |
| `claude_project_document_deletion_failed` | `claude_project_id`, `claude_project_document_id`, `filename` | |
| `claude_project_document_uploaded` | `claude_project_id`, `claude_project_document_id`, `filename` | |
| `claude_project_document_viewed` | `claude_project_id`, `claude_project_document_id`, `filename` | |
| `claude_project_file_access_failed` | `claude_project_id`, `claude_file_id` | |
| `claude_project_file_deleted` | `claude_project_id`, `claude_file_id` | |
| `claude_project_file_deletion_failed` | `claude_project_id`, `claude_file_id` | |
| `claude_project_file_uploaded` | `claude_project_id`, `claude_file_id`, `filename` | |
| `claude_project_reported` | `claude_project_id` | |
| `claude_project_sharing_updated` | `claude_project_id`, `audience` | |
| `claude_project_viewed` | `claude_project_id`, `preview_only` | |

#### Roles
| Activity Type | Additional Fields | Description |
|---|---|---|
| `rbac_role_assigned` | `role_id`, `principal_type`, `principal_id` | Admin assigned an RBAC custom role to a principal. |
| `rbac_role_created` | `role_id`, `role_name` | Admin created an RBAC custom role. |
| `rbac_role_deleted` | `role_id` | Admin deleted an RBAC custom role. |
| `rbac_role_permission_added` | `role_id`, `resource_type`, `resource_id`, `action` | Admin added a permission to an RBAC custom role. |
| `rbac_role_permission_removed` | `role_id`, `resource_type`, `resource_id`, `action` | Admin removed a permission from an RBAC custom role. |
| `rbac_role_unassigned` | `role_id`, `principal_type`, `principal_id` | Admin unassigned an RBAC custom role from a principal. |
| `rbac_role_updated` | `role_id` | Admin updated an RBAC custom role. |

#### SCIM Provisioning
| Activity Type | Additional Fields | Description |
|---|---|---|
| `scim_user_created` | `user_id` | A SCIM user was provisioned. |
| `scim_user_deleted` | `user_id` | A SCIM user was deleted. |
| `scim_user_updated` | `user_id` | A SCIM user was updated. |

#### SSO & Directory Sync
| Activity Type | Additional Fields | Description |
|---|---|---|
| `org_directory_resync_completed` | `resync_uuid` | Organization directory resync completed successfully. |
| `org_directory_resync_failed` | `resync_uuid` | Organization directory resync failed. |
| `org_directory_resync_started` | `resync_uuid`, `sync_destinations` | Organization directory resync was started asynchronously. |
| `org_directory_sync_activated` | *(none)* | Organization directory sync was activated. |
| `org_directory_sync_add_initiated` | *(none)* | Organization directory sync setup was initiated. |
| `org_directory_sync_deleted` | *(none)* | Organization directory sync was deleted. |
| `org_magic_link_second_factor_toggled` | `enabled` | Organization magic link second factor was toggled. |
| `org_sso_add_initiated` | *(none)* | Organization SSO setup was initiated. |
| `org_sso_connection_activated` | `connection_id`, `connection_type` | Organization SSO connection was activated. |
| `org_sso_connection_deactivated` | `connection_id` | Organization SSO connection was deactivated. |
| `org_sso_connection_deleted` | `connection_id` | Organization SSO connection was deleted. |
| `org_sso_group_role_mappings_updated` | *(none)* | Organization SSO group role mappings were updated. |
| `org_sso_provisioning_mode_changed` | `previous_mode`, `new_mode` | Organization SSO provisioning mode was changed. |
| `org_sso_seat_tier_assignment_toggled` | `enabled` | Organization SSO seat tier assignment was toggled. |
| `org_sso_seat_tier_mappings_updated` | *(none)* | Organization SSO seat tier mappings were updated. |
| `org_sso_toggled` | `enabled` | Organization SSO was toggled on or off. |

#### Service Keys
| Activity Type | Additional Fields | Description |
|---|---|---|
| `service_created` | `service_name` | Activity logged when an org service is explicitly created. |
| `service_deleted` | `service_name` | Activity logged when an org service is deleted. |
| `service_key_created` | `service_key_id`, `service_name`, `key_name`, `scopes`, `is_service_created` | Activity logged when a new org service key is created. |
| `service_key_revoked` | `service_key_id`, `service_name` | Activity logged when an org service key is revoked. |

#### Session Shares
| Activity Type | Additional Fields | Description |
|---|---|---|
| `session_share_accessed` | `share_id` | Session share was accessed. |
| `session_share_created` | `share_id` | Session share was created. |
| `session_share_revoked` | `share_id` | Session share was revoked. |

#### User Settings
| Activity Type | Additional Fields | Description |
|---|---|---|
| `claude_user_role_updated` | `user_id`, `user_email`, `previous_role`, `current_role` | |
| `claude_user_settings_updated` | `updates` | User updated their personal settings. |

### Activity Examples

**Chat Creation**
```json
{
  "id": "activity_1a2b3c4d5e",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "actor": {
    "type": "user_actor",
    "email_address": "user@example.com",
    "user_id": "user_xyz456",
    "ip_address": "192.0.2.34",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "claude_chat_created",
  "claude_chat_id": "claude_chat_ijk012",
  "claude_project_id": "claude_proj_art274"
}
```

**File Upload**
```json
{
  "id": "activity_2b3c4d5e6f",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "actor": {
    "type": "user_actor",
    "email_address": "user@example.com",
    "user_id": "user_xyz456",
    "ip_address": "192.0.2.34",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "claude_file_uploaded",
  "filename": "quarterly_report.pdf",
  "claude_file_id": "claude_file_fgh789",
  "claude_chat_id": "claude_chat_cde345"
}
```

**Project Sharing Update**
```json
{
  "id": "activity_5e6f7g8h9i",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "actor": {
    "type": "user_actor",
    "email_address": "user@example.com",
    "user_id": "user_xyz456",
    "ip_address": "192.0.2.34",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "claude_project_sharing_updated",
  "claude_project_id": "claude_proj_def234",
  "audience":[
    {
      "type": "user",
      "user_id": "user_xyz456",
      "role": "admin"
    },
    {
      "type": "organization"
    }
  ]
}
```

**Compliance API Access**
```json
{
  "id": "activity_3c4d5e6f7g",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "actor": {
    "type": "api_actor",
    "api_key_id": "apikey_fghij567890",
    "ip_address": "10.0.0.1",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "compliance_api_accessed",
  "request_id": "req_123456",
  "api_key_id": "apikey_fghij567890",
  "url": "https://api.anthropic.com/v1/compliance/activities?organization_ids[]=org_123abc",
  "request_method": "GET",
  "request_body": null,
  "status_code": 200
}
```

**SSO Login**
```json
{
  "id": "activity_4d5e6f7g8h",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": null,
  "organization_uuid": null,
  "actor": {
    "type": "unauthenticated_user_actor",
    "unauthenticated_email_address": "user@example.com",
    "ip_address": "192.0.2.34",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "sso_login_initiated"
}
```

**Social Login**
```json
{
  "id": "activity_6f7g8h9i0j",
  "created_at": "2025-06-07T08:09:10Z",
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "actor": {
    "type": "user_actor",
    "email_address": "user@example.com",
    "user_id": "user_xyz456",
    "ip_address": "192.0.2.34",
    "user_agent": "Mozilla/5.0..."
  },
  "type": "social_login_succeeded",
  "provider": "google"
}
```

---

## Objects APIs
These endpoints allow the retrieval of different resources (chats, organization metadata, messages, files) and their mutation.

### List chats
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/chats`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Lists chat metadata with filtering capabilities for targeted compliance review. Results are sorted chronologically (time ascending) by `created_at`, with ties broken by `id`.

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `organization_ids[]` *(Repeatable)* | array | Filter by organization IDs (accepts `org_...` or organization UUID) |
| `project_ids[]` *(Repeatable)* | array | Filter by project IDs (accepts `claude_proj_...`) |
| `user_ids[]` *(Repeatable)* | array | Filter by user IDs |
| `created_at.gte/gt/lte/lt` | string | Filter by creation time (RFC 3339). The meaning of the bracketed inequality is the same as defined in the Activity Feed. |
| `updated_at.gte/gt/lte/lt` | string | Filter by last update time (RFC 3339). The meaning of the bracketed inequality is the same as defined in the Activity Feed. |
| `after_id` | string | Pagination cursor for retrieving the next page of results (heading backwards in time). To paginate, pass the `last_id` value from the most recent response. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |
| `before_id` | string | Pagination cursor for retrieving the previous page of results (heading forwards in time). To paginate, pass the `first_id` value from the most recent response. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |
| `limit` | integer | Maximum results (default: 100, max: 1000) |

**Response**
```json
{
  "data":[
    {
      "id": "claude_chat_abc123",
      "name": "Product Requirements Discussion",
      "created_at": "2025-06-07T08:09:10Z",
      "updated_at": "2025-06-07T09:10:11Z",
      "deleted_at": null,
      "organization_id": "org_abc123",
      "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
      "project_id": "claude_proj_xyz789",
      "user": {
        "id": "user_xyz456",
        "email_address": "user@example.com"
      }
    }
  ],
  "has_more": false,
  "first_id": "claude_chat_abc123",
  "last_id": "claude_chat_abc123"
}
```
**Returns:** `ComplianceChatList`

### Get chat messages
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/chats/{claude_chat_id}/messages`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Retrieves the complete message history and file metadata for a specific chat.

**Response**
```json
{
  "id": "claude_chat_abc123",
  "name": "Product Requirements Discussion",
  "created_at": "2025-06-07T08:09:10Z",
  "updated_at": "2025-06-07T08:09:11Z",
  "deleted_at": null,
  "organization_id": "org_abc123",
  "organization_uuid": "abcdef0123-4567-89ab-cdef-0123456789ab",
  "project_id": "claude_proj_xyz789",
  "user": {
    "id": "user_xyz456",
    "email_address": "user@example.com"
  },
  "chat_messages":[
    {
      "id": "claude_chat_msg_abc123",
      "role": "user",
      "created_at": "2025-06-07T08:09:10Z",
      "content":[
        {
          "type": "text",
          "text": "Can you help me draft requirements for our new dashboard feature?"
        }
      ],
      "files":[
        {
          "id": "claude_file_xyz789",
          "filename": "dashboard_mockup_v1.pdf",
          "mime_type": "application/pdf"
        }
      ]
    },
    {
      "id": "claude_chat_msg_def456",
      "role": "assistant",
      "created_at": "2025-06-07T08:09:11Z",
      "content":[
        {
          "type": "text",
          "text": "I'd be happy to help you draft requirements for your dashboard feature..."
        }
      ],
      "artifacts":[
        {
          "id": "claude_artifact_abc123",
          "version_id": "claude_artifact_version_xyz789",
          "title": "Dashboard Requirements Draft",
          "artifact_type": "text/markdown"
        }
      ]
    }
  ]
}
```
**Returns:** `ComplianceChat`

### Delete chat
**Endpoint:** `DELETE https://api.anthropic.com/v1/compliance/apps/chats/{claude_chat_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `delete:compliance_user_data`

Permanently deletes a chat and all associated messages and files. This is a destructive operation that cannot be undone.

**Response**
```json
{
  "id": "claude_chat_abc123",
  "type": "claude_chat_deleted"
}
```
**Returns:** `ClaudeChatDeleteResponse`

### Download file content
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/chats/files/{claude_file_id}/content`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Downloads the binary content of a file referenced in chat messages.

**Response**
Standard file download with headers:
* `Content-Disposition: attachment; filename="<filename>"`
* `Content-Type: <mime-type>`
* `Transfer-Encoding: chunked`

### Download artifact content
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/artifacts/{artifact_version_id}/content`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Download the content of an artifact version for compliance purposes. Returns the full text content of the artifact version.

### Delete file
**Endpoint:** `DELETE https://api.anthropic.com/v1/compliance/apps/chats/files/{claude_file_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `delete:compliance_user_data`

Permanently deletes a specific file. This is a destructive operation that cannot be undone.

**Response**
```json
{
  "id": "claude_file_xyz789",
  "type": "claude_file_deleted"
}
```
**Returns:** `ComplianceFileDeleteResponse`

### List organizations
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/organizations`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_org_data`

List organizations under the parent organization. Returns a list of organizations sorted by creation date in ascending order. This endpoint does not support pagination and will return an error if the response would exceed 1,000 organizations.

**Returns:** `OrganizationList`

### List organization users
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/organizations/{org_uuid}/users`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

List current user members of an organization.

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `limit` | integer | Maximum results (default: 500, max: 1000) |
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |

**Returns:** `ComplianceUserMemberList`

### List projects
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/projects`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Lists project metadata with filtering capabilities. Results are sorted chronologically (time ascending) by `created_at`.

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `organization_ids[]` *(Repeatable)* | array | Filter by organization IDs (accepts `org_...` or organization UUID) |
| `user_ids[]` *(Repeatable)* | array | Filter by user IDs |
| `created_at.gte/gt/lte/lt` | string | Filter by creation time (RFC 3339). The meaning of the bracketed inequality is the same as defined in the Activity Feed. |
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |
| `limit` | integer | Maximum results (default: 20, max: 100) |

**Response**
```json
{
  "data":[
    {
      "id": "claude_proj_abc123",
      "name": "Q4 Product Planning",
      "created_at": "2025-06-01T10:00:00Z",
      "updated_at": "2025-06-15T14:30:00Z",
      "is_private": true,
      "organization_id": "org_abc123",
      "user": {
        "id": "user_xyz456",
        "email_address": "user@example.com"
      }
    }
  ],
  "has_more": true,
  "next_page": "page_eyJjcmVhdGVkX2F0IjoiMjAyNS0wNi0wMVQxMDowMDowMFoiLCJ1dWlkIjoiYWJjMTIzIn0="
}
```
**Returns:** `ComplianceProjectList`

### Get project details
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/projects/{project_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Get detailed information for a specific project.

**Returns:** `ComplianceProjectDetail`

### Delete project
**Endpoint:** `DELETE https://api.anthropic.com/v1/compliance/apps/projects/{project_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `delete:compliance_user_data`

Delete a project for compliance purposes. Hard-deletes the project and all its associated data including:
- All project documents and files
- All role assignments
- Knowledge base (if RAG is enabled)
- Sync sources
Project must have no attached chats - returns 409 if chats exist.

**Returns:** `ClaudeProjectDeleteResponse`

### List project attachments
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/projects/{project_id}/attachments`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

List files and documents attached to a project. List files and project documents attached to the project referenced by `project_id`. This includes the IDs of attached files, and attached project documents. The raw binary content of attached files can be downloaded using the `GET /v1/compliance/apps/chats/files/{claude_file_id}/content` endpoint. The text content of attached project documents can be fetched using the `GET /v1/compliance/apps/projects/documents/{claude_proj_doc_id}` endpoint.

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |
| `limit` | integer | Maximum results (default: 20, max: 100) |

**Returns:** `ComplianceProjectAttachmentList`

### Get project document content
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/apps/projects/documents/{document_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

Get detailed information for a specific project document.

**Returns:** `ComplianceProjectDocument`

### Delete project document
**Endpoint:** `DELETE https://api.anthropic.com/v1/compliance/apps/projects/documents/{document_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `delete:compliance_user_data`

Delete a project document for compliance purposes. Hard-deletes the project document permanently.

**Returns:** `ComplianceProjectDocumentDeleteResponse`

### List Compliance Roles
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/organizations/{org_uuid}/roles`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_org_data`

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `limit` | integer | Maximum results (default: 500, max: 1000) |
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |

**Returns:** `ComplianceRoleList`

### Get Compliance Role
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/organizations/{org_uuid}/roles/{role_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_org_data`

**Returns:** `ComplianceRole`

### List Compliance Groups
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/groups`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_org_data`

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `limit` | integer | Maximum results (default: 500, max: 1000) |
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |

**Returns:** `ComplianceGroupList`

### Get Compliance Group
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/groups/{group_id}`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_org_data`

**Returns:** `ComplianceGroup`

### List Compliance Group Members
**Endpoint:** `GET https://api.anthropic.com/v1/compliance/groups/{group_id}/members`
**Required Header:** `x-api-key: sk-ant-api01-...` (Compliance Access Key)
**Required Scope:** `read:compliance_user_data`

**Query Parameters**
| Parameter | Type | Description |
|---|---|---|
| `limit` | integer | Maximum results (default: 500, max: 1000) |
| `page` | string | Opaque pagination token from a previous response's `next_page` field. Pass this to retrieve the next page of results. Clients should treat this value as an opaque string and not attempt to parse or interpret its contents, as the format may change without notice. |

**Returns:** `ComplianceGroupMemberList`

---

## Project Attachments: Files vs Documents
Projects can contain two types of attachments, distinguished by the `type` field in the response:

| Type | ID Prefix | Description | Content Storage | MIME Type |
|---|---|---|---|---|
| `project_file` | `claude_file_...` | Binary file uploads (PDFs, images, spreadsheets, etc.) | Cloud blob storage | Variable (e.g. `application/pdf`, `image/png`) |
| `project_doc` | `claude_proj_doc_...` | Plain text documents (custom instructions, reference material) | Database | Always `text/plain` |

**Retrieving content:**
- **Project files** (`project_file`): Use `GET /v1/compliance/apps/chats/files/{claude_file_id}/content` to download binary content
- **Project documents** (`project_doc`): Use `GET /v1/compliance/apps/projects/documents/{document_id}` to retrieve text content

**Deleting:**
- **Project files:** Use `DELETE /v1/compliance/apps/chats/files/{claude_file_id}`
- **Project documents:** Use `DELETE /v1/compliance/apps/projects/documents/{document_id}`

---

## Types

#### ActivityList
List of compliance activities with pagination info.
| Field | Type | Description |
|---|---|---|
| `data` | array of object | |
| `has_more` | boolean | |
| `first_id` | string or null | |
| `last_id` | string or null | |

#### ClaudeChatDeleteResponse
Response for deleting a Claude chat.
| Field | Type | Description |
|---|---|---|
| `id` | string | The ID of the Claude chat that was deleted |
| `type` | `"claude_chat_deleted"` | Constant string confirming deletion |

#### ClaudeProjectDeleteResponse
Response for deleting a Claude project.
| Field | Type | Description |
|---|---|---|
| `id` | string | The ID of the Claude project that was deleted |
| `type` | `"claude_project_deleted"` | Constant string confirming deletion. |

#### ComplianceArtifactReference
Reference to an artifact version generated by the assistant. Use `GET /v1/compliance/apps/artifacts/{version_id}/content` to download the full artifact content.
| Field | Type | Description |
|---|---|---|
| `id` | string | Artifact ID e.g. 'claude_artifact_abc123' |
| `version_id` | string | Artifact version ID e.g. 'claude_artifact_version_abc123' |
| `title` | string or null | Artifact title |
| `artifact_type` | string or null | MIME-like artifact type e.g. 'application/vnd.ant.code' |

#### ComplianceChat
Complete chat conversation data for compliance purposes.
| Field | Type | Description |
|---|---|---|
| `id` | string | Chat ID |
| `name` | string | Chat name |
| `created_at` | string (RFC 3339) | Creation timestamp |
| `updated_at` | string (RFC 3339) | Last update timestamp |
| `deleted_at` | string (RFC 3339) or null | Deletion timestamp if deleted |
| `organization_id` | string | Organization ID this chat belongs to |
| `organization_uuid` | string | Organization UUID this chat belongs to |
| `project_id` | string or null | Project ID this chat belongs to |
| `user` | `ComplianceUser` | User information |
| `chat_messages` | array of `ComplianceChatMessage` | Array of chat messages in order of created_at |

#### ComplianceChatList
List of chat metadata with pagination info.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceChatMetadata` | List of chat metadata sorted chronologically by created_at, tie break by id |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `first_id` | string or null | First chat ID in the current result set. To get the previous page, use this as before_id in your next request |
| `last_id` | string or null | Last chat ID in the current result set. To get the next page, use this as after_id in your next request |

#### ComplianceChatMessage
A single message in a chat conversation.
| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier for the message e.g. 'claude_chat_msg_abcd1234' |
| `role` | "user" or "assistant" | Message sender (user or assistant) |
| `created_at` | string (RFC 3339) | Message creation timestamp - For human: when they sent the message, For assistant: when it completed the last content block |
| `content` | array of `ComplianceTextBlock` | Content blocks within the message |
| `files` | array of `ComplianceFileReference` or null | File attachments |
| `artifacts` | array of `ComplianceArtifactReference` or null | Artifacts generated or updated by this message |

#### ComplianceChatMetadata
Chat metadata for listing chats (without messages).
| Field | Type | Description |
|---|---|---|
| `id` | string | Chat ID |
| `name` | string | Chat name/title |
| `created_at` | string (RFC 3339) | Creation timestamp |
| `updated_at` | string (RFC 3339) | Last update timestamp |
| `deleted_at` | string (RFC 3339) or null | Deletion timestamp if deleted |
| `organization_id` | string | Organization ID this chat belongs to |
| `organization_uuid` | string | Organization UUID this chat belongs to |
| `project_id` | string or null | Project ID this chat belongs to |
| `user` | `ComplianceUser` | User information for the chat creator |

#### ComplianceFileDeleteResponse
Response for deleting a compliance file.
| Field | Type | Description |
|---|---|---|
| `type` | string | Constant string confirming deletion |
| `id` | string | The ID of the file that was deleted |

#### ComplianceFileReference
File attachment reference in a message.
| Field | Type | Description |
|---|---|---|
| `id` | string | File ID |
| `filename` | string | Display name of the file |
| `mime_type` | string | MIME type of the file when it was uploaded (e.g. 'application/pdf') |

#### ComplianceGroup
Group information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | Group identifier (tagged ID) |
| `name` | string | Group name |
| `description` | string | Group description |
| `source_type` | string | How the group was created ('direct' or 'scim') |
| `created_at` | string or null | Group creation timestamp (ISO 8601) |
| `updated_at` | string or null | Group last-updated timestamp (ISO 8601) |

#### ComplianceGroupList
Paginated list of groups.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceGroup` | List of groups |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | Token to retrieve the next page. Use this as the 'page' parameter in your next request |

#### ComplianceGroupMember
Group member for compliance responses.
| Field | Type | Description |
|---|---|---|
| `user_id` | string | Member user identifier (tagged ID) |
| `email` | string | Member email address |
| `created_at` | string or null | Membership creation timestamp (ISO 8601) |
| `updated_at` | string or null | Membership last-updated timestamp (ISO 8601) |

#### ComplianceGroupMemberList
Paginated list of group members.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceGroupMember` | List of group members |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | Token to retrieve the next page. Use this as the 'page' parameter in your next request |

#### ComplianceOrganizationInfo
Information about an organization.
| Field | Type | Description |
|---|---|---|
| `uuid` | string | Unique identifier for the organization (UUID format) |
| `name` | string | Organization name |
| `created_at` | string | Organization creation time (RFC 3339 format) |

#### ComplianceProject
Project information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | Project identifier (tagged ID) |
| `name` | string | Project name |
| `created_at` | string (RFC 3339) | Project creation timestamp |
| `updated_at` | string (RFC 3339) | Project last update timestamp |
| `organization_id` | string | Organization identifier (tagged ID) |
| `user` | `ComplianceProjectUser` | Project creator information |
| `is_private` | boolean | If false, the project is visible to all organization members; if true the project is accessible only to the creator and specified collaborators |

#### ComplianceProjectAttachmentList
List of project attachments with pagination info.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceProjectFileReference` or `ComplianceProjectDocReference` | List of attachments sorted chronologically by created_at, tie break by id |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | To get the next page, use the 'next_page' from the current response as the 'page' in your next request |

#### ComplianceProjectDetail
Detailed project information for compliance responses. Extends `ComplianceProject`, inheriting all its fields.
| Field | Type | Description |
|---|---|---|
| `description` | string | Project description |
| `instructions` | string | Project's custom instructions / prompt |
| `chats_count` | integer | Number of chats contained within this project |
| `attachments_count` | integer | Number of attachments contained within this project |

#### ComplianceProjectDocReference
Project document attachment reference for compliance responses.
| Field | Type | Description |
|---|---|---|
| `type` | `"project_doc"` | Discriminator marking this as a plain text document |
| `id` | string | Project document identifier (e.g., 'claude_proj_doc_abcd') |
| `created_at` | string (RFC 3339) | Creation timestamp (RFC 3339 format) |
| `filename` | string | Display name of the document (e.g., 'document.txt') |
| `mime_type` | `"text/plain"` | MIME type of the project document, always set to plain text |

#### ComplianceProjectDocument
Project document information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | Project document identifier (tagged ID) |
| `filename` | string | Document filename |
| `content` | string | Document text content |
| `created_at` | string (RFC 3339) | Document creation timestamp |
| `user` | `ComplianceProjectUser` | User information |

#### ComplianceProjectDocumentDeleteResponse
Response for deleting a project document.
| Field | Type | Description |
|---|---|---|
| `id` | string | The ID of the project document that was deleted |
| `type` | `"claude_project_document_deleted"` | Constant string confirming deletion. |

#### ComplianceProjectFileReference
File attachment reference for compliance responses.
| Field | Type | Description |
|---|---|---|
| `type` | `"project_file"` | Discriminator marking this as a binary file |
| `id` | string | File identifier (e.g., 'claude_file_abcd') |
| `created_at` | string (RFC 3339) | Creation timestamp (RFC 3339 format) |
| `filename` | string | Display name of the file (e.g., 'document.pdf') |
| `mime_type` | string | MIME type of the file when it was uploaded (e.g., 'application/pdf') |

#### ComplianceProjectList
List of projects with pagination info.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceProject` | List of projects sorted by creation date ascending |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | Token to retrieve the next page. Use this as the 'page' parameter in your next request |

#### ComplianceProjectUser
User information for project creator.
| Field | Type | Description |
|---|---|---|
| `id` | string | User identifier (tagged ID) |
| `email_address` | string | User's email address |

#### ComplianceRole
Role information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | Role identifier (tagged ID) |
| `name` | string | Role name |
| `description` | string | Role description |
| `created_at` | string or null | Role creation timestamp (ISO 8601) |
| `updated_at` | string or null | Role last-updated timestamp (ISO 8601) |

#### ComplianceRoleList
Paginated list of roles.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceRole` | List of roles |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | Token to retrieve the next page. Use this as the 'page' parameter in your next request |

#### ComplianceTextBlock
Text content block.
| Field | Type | Description |
|---|---|---|
| `type` | `"text"` | |
| `text` | string | Text content from human or assistant |

#### ComplianceUser
User information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | User identifier |
| `email_address` | string | User's email address |

#### ComplianceUserMember
User member information for compliance responses.
| Field | Type | Description |
|---|---|---|
| `id` | string | User identifier (tagged ID) |
| `full_name` | string | User's current full name |
| `email` | string | User's current email address |
| `created_at` | string (RFC 3339) | User account creation timestamp |

#### ComplianceUserMemberList
List of user members with pagination info.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceUserMember` | List of current organization members sorted by account creation date ascending |
| `has_more` | boolean | Whether more records exist beyond the current result set |
| `next_page` | string or null | Token to retrieve the next page. Use this as the 'page' parameter in your next request |

#### OrganizationList
List of organizations under a parent organization.
| Field | Type | Description |
|---|---|---|
| `data` | array of `ComplianceOrganizationInfo` | List of organizations sorted by creation date, ascending |

---

## Getting Started
The following examples use `curl` to demonstrate how to form the request and assume that you have created an access key with all available scopes. The below examples assume you have stored your access key in the `ANTHROPIC_COMPLIANCE_ACCESS_KEY` environment variable.

```bash
# Store your Compliance Access Key in an environment variable
export ANTHROPIC_COMPLIANCE_ACCESS_KEY=sk-ant-api01-...
```

### Activity Feed
```bash
# Get recent activities
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/activities?limit=20"

# Get activities for a specific user
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/activities?actor_ids[]=user_abc123"

# Get activities for specific child organization after a given point in time
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/activities?organization_ids[]=org_123abc&created_at.gte=2025-06-07T08:09:10Z"
```

### Organizations and Users
```bash
# List all organizations under parent organization
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/organizations"

# List users in a specific organization
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/organizations/91012d09-e48b-438e-a489-1bebfd8fa6f9/users?limit=100"

# Paginate through users
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/organizations/91012d09-e48b-438e-a489-1bebfd8fa6f9/users?page=page_01Dd11ZX"
```

### Chats and Files Retrieval
```bash
# Find chats for a specific user after a given timestamp
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/chats?user_ids[]=user_abc123&created_at.gte=2025-06-07T08:09:10Z"

# Get full chat content
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/chats/claude_chat_abc123/messages"

# Download a file from the chat
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/chats/files/claude_file_xyz789/content" --output document.pdf
```

### Chats and Files Deletion
```bash
# Permanently delete a chat
curl -X DELETE -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/chats/claude_chat_abc123"

# Permanently delete a specific file
curl -X DELETE -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/chats/files/claude_file_xyz789"
```

### Project Retrieval
```bash
# List all projects with filtering
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
  "https://api.anthropic.com/v1/compliance/apps/projects?limit=20"





```markdown
```bash
"https://api.anthropic.com/v1/compliance/apps/projects/claude_proj_01KGp4eZNug9ri4kE35RSppq/attachments"

# Get project document content
curl -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
"https://api.anthropic.com/v1/compliance/apps/projects/documents/claude_proj_doc_01xyz4eZNug9ri4kE35RSppq"
```

## Project Deletion

```bash
# Delete a project document
curl -X DELETE -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
"https://api.anthropic.com/v1/compliance/apps/projects/documents/claude_proj_doc_01xyz4eZNug9ri4kE35RSppq"

# Delete a project (requires all chats to be deleted/detached first)
curl -X DELETE -H "x-api-key: $ANTHROPIC_COMPLIANCE_ACCESS_KEY" \
"https://api.anthropic.com/v1/compliance/apps/projects/claude_proj_01KGp4eZNug9ri4kE35RSppq"
```

## Error Handling

### Invalid Request (400)

#### Invalid Timestamp Format
```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "The `created_at.gte` parameter contains an invalid timestamp format. Timestamps must be provided in RF"
  }
}
```

#### Invalid Limit
```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "The limit parameter must be between 1 and 1000, inclusive. Got 1500."
  }
}
```

#### Invalid Pagination ID
```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid `after_id`. No activity found for `after_id` \"activity_invalid123\""
  }
}
```
Current messaging may reference internal implementation details of the pagination ID, but clients should treat pagination values as opaque strings as they may change without notice.

### Authentication Errors (401)

#### Invalid API Key
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided is invalid or has been revoked."
  }
}
```

#### Insufficient Scope: Activity Feed
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided does not have the `read:compliance_activities` scope required for this endpoint."
  }
}
```

#### Insufficient Scope: Organization Data
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided does not have the `read:compliance_org_data` scope required for this endpoint."
  }
}
```

#### Insufficient Scope: User Data
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided does not have the `read:compliance_user_data` scope required for this endpoint."
  }
}
```

#### Insufficient Scope: Delete
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided does not have the `delete:compliance_user_data` scope required for this endpoint."
  }
}
```

#### Insufficient Scope: Chat Access
```json
{
  "error": {
    "type": "authentication_error",
    "message": "The API key provided does not have the `read:compliance_user_data` scope required for this endpoint."
  }
}
```

### Not Found (404)

#### Chat Not Found
```json
{
  "error": {
    "type": "not_found_error",
    "message": "Chat {id} not found."
  }
}
```

#### File Not Found
```json
{
  "error": {
    "type": "not_found_error",
    "message": "No file found with provided id, or it has already been deleted."
  }
}
```

#### Projects Not Found
```json
{
  "error": {
    "type": "not_found_error",
    "message": "No project is found with the provided id."
  }
}
```

#### Project Document Not Found
```json
{
  "error": {
    "type": "not_found_error",
    "message": "No project document found with provided id, or it has already been deleted."
  }
}
```

### Conflict (409)

#### Project Has Attached Chats
```json
{
  "error": {
    "type": "conflict_error",
    "message": "The \"\\claude_proj_01KGp4eZNug9ri4kE35RSppq\\\" project cannot be deleted as it has chats attached to it."
  }
}
```

### Internal Server Error (500)

#### Maximum Response Size Exceeded
```json
{
  "error": {
    "type": "internal_error",
    "message": "This response would have exceeded the maximum of 1,000 organizations returned in one request."
  }
}
```

---

## Appendix A: Changelog

This section lists the changes introduced with each version of the API.

### Revision I
**FEATURE**: The Compliance API now covers **Claude Console** and **Claude API** in addition to Claude.ai. Activity logging is available across all these surfaces.

**FEATURE**: New authentication method: **Admin keys** created in Claude Console can now authenticate Activity Feed requests. Admin keys carry a fixed `read:compliance_activities` scope. All other endpoints return 401 when called with an Admin key (those features are only applicable to Claude.ai).

**FEATURE**: New actor type `AdminApiKeyActor` added for activities performed via the Admin API using an Admin key. Includes `admin_api_key_id` field.

**FEATURE**: The following activity types have been added to the activity feed:
*   **Admin API Keys**: `admin_api_key_created`, `admin_api_key_deleted`, `admin_api_key_updated`
*   **MCP Servers**: `mcp_server_created`, `mcp_server_deleted`, `mcp_server_updated`, `mcp_tool_policy_updated`
*   **Org Management**: `org_invite_viewed`, `org_invites_listed`, `org_user_viewed`, `org_users_listed`
*   **Organization Settings**: `org_claude_code_desktop_enabled`, `org_claude_code_desktop_disabled`, `org_cowork_agent_enabled`, `org_cowork_agent_disabled`
*   **Platform Files**: `platform_file_content_downloaded`, `platform_file_deleted`, `platform_file_uploaded`
*   **Platform Org Management**: `platform_api_key_created`, `platform_api_key_updated`, `platform_cost_report_viewed`, `platform_usage_report_claude_code_viewed`, `platform_usage_report_messages_viewed`, `platform_workspace_archived`, `platform_workspace_created`, `platform_workspace_member_added`, `platform_workspace_member_removed`, `platform_workspace_member_updated`, `platform_workspace_member_viewed`, `platform_workspace_members_listed`, `platform_workspace_rate_limit_deleted`, `platform_workspace_rate_limit_updated`, `platform_workspace_updated`
*   **Platform Skills**: `platform_skill_version_created`, `platform_skill_version_deleted`

**FEATURE**: Additional fields added to existing activity types:
*   `org_compliance_api_settings_updated`: `compliance_api_enabled`, `compliance_api_logging_enabled`

**FEATURE**: `ComplianceFileDeleteResponse` description and field descriptions corrected.

**FEATURE**: List Compliance Group Members endpoint scope corrected from `read:compliance_org_data` to `read:compliance_user_data`.

### Revision H
**FEATURE**: New RBAC endpoints added:
*   `GET /v1/compliance/organizations/{org_uuid}/roles` — List roles
*   `GET /v1/compliance/organizations/{org_uuid}/roles/{role_id}` — Get role details
*   `GET /v1/compliance/groups` — List groups
*   `GET /v1/compliance/groups/{group_id}` — Get group details
*   `GET /v1/compliance/groups/{group_id}/members` — List group members

**FEATURE**: New project management endpoints added:
*   `GET /v1/compliance/apps/projects/{project_id}` — Get project details
*   `DELETE /v1/compliance/apps/projects/{project_id}` — Delete project
*   `GET /v1/compliance/apps/projects/{project_id}/attachments` — List project attachments
*   `GET /v1/compliance/apps/projects/documents/{document_id}` — Get project document content
*   `DELETE /v1/compliance/apps/projects/documents/{document_id}` — Delete project document

**FEATURE**: The following activity types have been added to the activity feed:
*   **Organization Settings**: `org_work_across_apps_enabled`, `org_work_across_apps_disabled`

### Revision G
**FEATURE**: New actor type `ScimDirectorySyncActor` added for activities triggered by Identity Provider (IdP) directory sync (e.g., Okta, Azure AD, JumpCloud).

**FEATURE**: The following activity types have been added to the activity feed:
*   **Billing**: `extra_usage_billing_enabled`, `extra_usage_credit_granted`
*   **Organization Discoverability**: `org_member_invites_disabled`, `org_member_invites_enabled`
*   **SCIM Provisioning**: `scim_user_created`, `scim_user_deleted`, `scim_user_updated`

**FEATURE**: Additional fields added to existing activity types:
*   `extra_usage_spend_limit_created`: `limit_type`, `is_enabled`, `amount`, `user_id`
*   `extra_usage_spend_limit_deleted`: `user_id`
*   `extra_usage_spend_limit_updated`: `limit_type`, `is_enabled`, `amount`, `user_id`
*   `role_assignment_granted`, `role_assignment_revoked`: `resource_type`, `resource_id`
*   `group_member_added`, `group_member_removed`: `member_id` replaced by `member_ids` to support bulk operations

**FEATURE**: Chat message responses now include an `id` field per message.

**FEATURE**: List Projects endpoint now includes full query parameter documentation and response format.

**FEATURE**: Pagination cursor descriptions updated across all endpoints to clarify their role and emphasize opaque handling.

**FEATURE**: Message list now includes references to artifacts

**FEATURE**: added `GET /v1/compliance/apps/artifacts/{artifact_version_id}/content` to download artifact content

### Revision F
**FEATURE**: The maximum `limit` for the Activity Feed endpoint has been increased from 1,000 to 5,000.

**FEATURE**: The following activity types have been added to the activity feed:
*   **GitHub Enterprise**: `ghe_configuration_created`, `ghe_configuration_deleted`, `ghe_configuration_updated`, `ghe_user_connected`, `ghe_user_disconnected`, `ghe_webhook_signature_invalid`
*   **LTI**: `lti_launch_initiated`, `lti_launch_success`
*   **Marketplaces**: `marketplace_created`, `marketplace_deleted`, `marketplace_updated`
*   **Organization Discoverability**: `org_discoverability_disabled`, `org_discoverability_enabled`, `org_discoverability_settings_updated`, `org_join_request_approved`, `org_join_request_created`, `org_join_request_dismissed`, `org_join_request_instant_approved`, `org_join_requests_bulk_dismissed`
*   **Organization Settings**: `org_invite_link_disabled`, `org_invite_link_generated`, `org_invite_link_regenerated`, `org_members_exported`
*   **Roles**: `rbac_role_assigned`, `rbac_role_created`, `rbac_role_deleted`, `rbac_role_permission_added`, `rbac_role_permission_removed`, `rbac_role_unassigned`, `rbac_role_updated`
*   **SSO & Directory Sync**: `org_directory_resync_completed`, `org_directory_resync_failed`, `org_directory_resync_started`

### Revision E
**DEPRECATION**: The pagination cursors must be treated as opaque (previously, they represented the resource ID). This should not impact your workflow if you are simply taking the returned cursor and using it for the next pagination request. However, if you are explicitly treating the cursor as an ID, this might break your workflow.

**FEATURE**: The following activity types have been added to the activity feed: `phone_code_sent`, `phone_code_verified`, `social_login_succeeded`, `magic_link_login_initiated`, `age_verified`, `anonymous_mobile_login_attempted`, `sso_second_factor_magic_link`, `session_revoked`, `org_user_invite_sent`, `org_user_invite_re_sent`, `org_user_invite_deleted`, `org_user_invite_accepted`, `org_user_invite_rejected`, `org_user_deleted`, `org_domain_add_initiated`, `org_domain_verified`, `org_domain_removed`, `org_sso_add_initiated`, `org_sso_connection_activated`, `org_sso_connection_deactivated`, `org_sso_connection_deleted`, `org_sso_toggled`, `org_magic_link_second_factor_toggled`, `org_sso_provisioning_mode_changed`, `org_sso_seat_tier_assignment_toggled`, `org_sso_seat_tier_mappings_updated`, `org_sso_group_role_mappings_updated`, `org_directory_sync_add_initiated`, `org_directory_sync_activated`, `org_directory_sync_deleted`, `org_sync_deleting_synchronized_files_started`, `org_sync_synchronized_files_deleted`, `org_creation_blocked`, `org_claude_code_data_sharing_enabled`, `org_claude_code_data_sharing_disabled`, `org_parent_join_proposal_created`, `org_parent_search_performed`, `org_compliance_api_settings_updated`, `claude_file_uploaded`, `claude_file_deleted`, `integration_user_connected`, `integration_user_disconnected`, `org_data_export_started`, `org_data_export_completed`, `role_assignment_granted`, `role_assignment_revoked`, `owned_projects_access_restored`, `prepaid_extra_usage_auto_reload_enabled`, `prepaid_extra_usage_auto_reload_disabled`, `prepaid_extra_usage_auto_reload_settings_updated`, `extra_usage_spend_limit_created`, `extra_usage_spend_limit_updated`, `extra_usage_spend_limit_deleted`, `org_ip_restriction_created`, `org_ip_restriction_updated`, `org_ip_restriction_deleted`, `org_bulk_delete_initiated`, `org_deleted_via_bulk`, `claude_skill_created`, `claude_skill_replaced`, `claude_skill_deleted`, `claude_command_created`, `claude_command_replaced`, `claude_command_deleted`, `claude_plugin_created`, `claude_plugin_updated`, `claude_plugin_replaced`, `claude_plugin_deleted`, `session_share_created`, `session_share_revoked`, `session_share_accessed`, `org_deletion_requested`, `org_cowork_enabled`, `org_cowork_disabled`, `org_taint_added`, `org_taint_removed`

---

## Appendix B: Comparison with Audit Log Events

Audit Log Export is a separate Data Management feature allowing organization owners to export their organization's audit log history. Audit logs record notable events that occur while processing user requests to the service.

The Compliance API Activity Feed records the initiation and/or completion of select user requests. Activities sometimes correspond to an audit log event and vice-versa. The following table shows the mappings of activities and audit log events, when they exist.

*134 of 157 audit log events have a corresponding compliance activity.*

| Compliance Activity | Audit Log Event |
| :--- | :--- |
| `phone_code_sent` | `user_sent_phone_code` |
| `phone_code_verified` | `user_verified_phone_code` |
| `social_login_succeeded` | `user_signed_in_google` |
| `social_login_succeeded` | `user_signed_in_apple` |
| `social_login_succeeded` | `user_signed_in_microsoft` |
| `sso_login_succeeded` | `user_signed_in_sso` |
| `magic_link_login_initiated` | `user_requested_magic_link` |
| `age_verified` | `user_age_verified` |
| `anonymous_mobile_login_attempted` | `anonymous_mobile_login_attempted` |
| - | `user_aws_console_jwt_exchanged` |
| `sso_second_factor_magic_link` | `sso_second_factor_magic_link` |
| `magic_link_login_succeeded` | `user_attempted_magic_link_verification` |
| `user_logged_out` | `user_signed_out` |
| `user_logged_out` | `user_signed_out_all_sessions` |
| `session_revoked` | `user_revoked_session` |
| `claude_user_settings_updated` | `user_name_changed` |
| `org_user_invite_sent` | `org_user_invite_sent` |
| `org_user_invite_re_sent` | `org_user_invite_re_sent` |
| `org_user_invite_deleted` | `org_user_invite_deleted` |
| `org_user_invite_accepted` | `org_user_invite_accepted` |
| `org_user_invite_rejected` | `org_user_invite_rejected` |
| `org_user_deleted` | `org_user_deleted` |
| `claude_user_role_updated` | `org_user_updated` |
| `org_domain_add_initiated` | `org_domain_add_initiated` |
| `org_domain_verified` | `org_domain_verified` |
| `org_domain_removed` | `org_domain_removed` |
| `org_sso_add_initiated` | `org_sso_add_initiated` |
| `org_sso_connection_activated` | `org_sso_connection_activated` |
| `org_sso_connection_deactivated` | `org_sso_connection_deactivated` |
| `org_sso_connection_deleted` | `org_sso_connection_deleted` |
| `org_sso_toggled` | `org_sso_toggled` |
| - | `org_sso_auto_provisioned` |
| - | `org_jit_toggled` |
| `org_magic_link_second_factor_toggled` | `org_magic_link_second_factor_toggled` |
| `org_directory_sync_add_initiated` | `org_directory_sync_add_initiated` |
| `org_directory_sync_activated` | `org_directory_sync_activated` |
| `org_directory_sync_deleted` | `org_directory_sync_deleted` |
| `org_directory_resync_started` | `org_directory_resync_started` |
| `org_directory_resync_completed` | `org_directory_resync_completed` |
| `org_directory_resync_failed` | `org_directory_resync_failed` |
| `org_sso_provisioning_mode_changed` | `org_sso_provisioning_mode_changed` |
| `org_sso_seat_tier_assignment_toggled` | `org_sso_seat_tier_assignment_toggled` |
| `org_sso_seat_tier_mappings_updated` | `org_sso_seat_tier_mappings_updated` |
| `org_sso_group_role_mappings_updated` | `org_sso_group_role_mappings_updated` |
| `claude_organization_settings_updated` | `org_data_retention_policy_changed` |
| `org_sync_deleting_synchronized_files_started` | `org_sync_deleting_synchronized_files_started` |
| `org_sync_synchronized_files_deleted` | `org_sync_synchronized_files_deleted` |
| `org_creation_blocked` | `org_creation_blocked` |
| `org_claude_code_data_sharing_enabled` | `org_claude_code_data_sharing_enabled` |
| `org_claude_code_data_sharing_disabled` | `org_claude_code_data_sharing_disabled` |
| `org_cowork_enabled` | `org_cowork_enabled` |
| `org_cowork_disabled` | `org_cowork_disabled` |
| `org_work_across_apps_enabled` | `org_work_across_apps_enabled` |
| `org_work_across_apps_disabled` | `org_work_across_apps_disabled` |
| `org_cowork_agent_enabled` | `org_cowork_agent_enabled` |
| `org_cowork_agent_disabled` | `org_cowork_agent_disabled` |
| `org_claude_code_desktop_enabled` | `org_claude_code_desktop_enabled` |
| `org_claude_code_desktop_disabled` | `org_claude_code_desktop_disabled` |
| `claude_organization_settings_updated` | `org_claude_code_managed_settings_created` |
| `claude_organization_settings_updated` | `org_claude_code_managed_settings_updated` |
| `claude_organization_settings_updated` | `org_claude_code_managed_settings_deleted` |
| `org_parent_join_proposal_created` | `org_parent_join_proposal_created` |
| `org_parent_search_performed` | `org_parent_search_performed` |
| `org_compliance_api_settings_updated` | `org_compliance_api_settings_updated` |
| `org_analytics_api_capability_updated` | `org_analytics_api_capability_updated` |
| `org_deletion_requested` | `org_deletion_requested` |
| `org_taint_added` | `org_taint_added` |
| `org_taint_removed` | `org_taint_removed` |
| `org_discoverability_enabled` | `org_discoverability_enabled` |
| `org_discoverability_disabled` | `org_discoverability_disabled` |
| `org_discoverability_settings_updated` | `org_discoverability_settings_updated` |
| `org_join_request_created` | `org_join_request_created` |
| `org_join_request_approved` | `org_join_request_approved` |
| `org_join_request_instant_approved` | `org_join_request_instant_approved` |
| `org_join_request_dismissed` | `org_join_request_dismissed` |
| `org_join_requests_bulk_dismissed` | `org_join_requests_bulk_dismissed` |
| `org_member_invites_enabled` | `org_member_invites_enabled` |
| `org_member_invites_disabled` | `org_member_invites_disabled` |
| `org_invite_link_generated` | `org_invite_link_generated` |
| `org_invite_link_disabled` | `org_invite_link_disabled` |
| `org_invite_link_regenerated` | `org_invite_link_regenerated` |
| `claude_project_created` | `project_created` |
| `claude_project_deleted` | `project_deleted` |
| `claude_project_sharing_updated` | `project_visibility_changed` |
| - | `project_renamed` |
| `claude_project_document_uploaded` | `project_document_created` |
| `claude_project_document_deleted` | `project_document_deleted` |
| `claude_file_uploaded` | `file_uploaded` |
| `claude_file_deleted` | `file_deleted` |
| `claude_skill_created` | `skill_created` |
| `claude_skill_replaced` | `skill_replaced` |
| `claude_skill_deleted` | `skill_deleted` |
| `claude_command_created` | `command_created` |
| `claude_command_replaced` | `command_replaced` |
| `claude_command_deleted` | `command_deleted` |
| `claude_plugin_created` | `plugin_created` |
| `claude_plugin_replaced` | `plugin_replaced` |
| `claude_plugin_updated` | `plugin_updated` |
| `claude_plugin_deleted` | `plugin_deleted` |
| `marketplace_created` | `marketplace_created` |
| `marketplace_updated` | `marketplace_updated` |
| `marketplace_deleted` | `marketplace_deleted` |
| `claude_chat_created` | `conversation_created` |
| `claude_chat_deleted` | `conversation_deleted` |
| `claude_chat_settings_updated` | `conversation_renamed` |
| - | `conversation_deletion_requested` |
| - | `conversation_moved_to_project` |
| `session_share_created` | `session_share_created` |
| `session_share_revoked` | `session_share_revoked` |
| `session_share_accessed` | `session_share_accessed` |
| - | `webauthn_credential_created` |
| - | `webauthn_credential_deleted` |
| - | `webauthn_credential_updated` |
| - | `request_signing_key_created` |
| - | `request_signing_key_deleted` |
| - | `request_signing_key_updated` |
| `claude_gdrive_integration_created`, `claude_github_integration_created` | `integration_org_enabled` |
| `claude_gdrive_integration_deleted`, `claude_github_integration_deleted` | `integration_org_disabled` |
| `claude_gdrive_integration_updated`, `claude_github_integration_updated` | `integration_org_config_updated` |
| `integration_user_connected` | `integration_user_connected` |
| `integration_user_disconnected` | `integration_user_disconnected` |
| `lti_launch_initiated` | `lti_launch_initiated` |
| `lti_launch_success` | `lti_launch_success` |
| - | `lti_launch_failed` |
| - | `lti_account_created` |
| - | `lti_account_linked` |
| - | `lti_session_created` |
| - | `lti_session_expired` |
| - | `student_data_accessed` |
| - | `student_data_exported` |
| - | `educational_org_modified` |
| `org_data_export_started` | `org_data_export_started` |
| `org_data_export_completed` | `org_data_export_completed` |
| `org_members_exported` | `org_members_exported` |
| `role_assignment_granted` | `role_assignment_granted` |
| `role_assignment_revoked` | `role_assignment_revoked` |
| `owned_projects_access_restored` | `owned_projects_access_restored` |
| `prepaid_extra_usage_auto_reload_enabled` | `prepaid_extra_usage_auto_reload_enabled` |
| `prepaid_extra_usage_auto_reload_disabled` | `prepaid_extra_usage_auto_reload_disabled` |
| `prepaid_extra_usage_auto_reload_settings_updated` | `prepaid_extra_usage_auto_reload_settings_updated` |
| `extra_usage_spend_limit_created` | `extra_usage_spend_limit_created` |
| `extra_usage_spend_limit_updated` | `extra_usage_spend_limit_updated` |
| `extra_usage_spend_limit_deleted` | `extra_usage_spend_limit_deleted` |
| `org_ip_restriction_created` | `org_ip_restriction_created` |
| `org_ip_restriction_updated` | `org_ip_restriction_updated` |
| `org_ip_restriction_deleted` | `org_ip_restriction_deleted` |
| `org_bulk_delete_initiated` | `org_bulk_delete_initiated` |
| `org_deleted_via_bulk` | `org_deleted_via_bulk` |
| `ghe_configuration_created` | `ghe_configuration_created` |
| `ghe_configuration_updated` | `ghe_configuration_updated` |
| `ghe_configuration_deleted` | `ghe_configuration_deleted` |
| `ghe_user_connected` | `ghe_user_connected` |
| `ghe_user_disconnected` | `ghe_user_disconnected` |
| `ghe_webhook_signature_invalid` | `ghe_webhook_signature_invalid` |
| - | `prepaid_overages_auto_reload_enabled` |
| - | `prepaid_overages_auto_reload_disabled` |
| - | `prepaid_overages_auto_reload_settings_updated` |
```
