# Anthropic Compliance API public documentation snapshot

Observed 2026-07-15 from Anthropic's public Claude Platform documentation.
This is a source-linked research snapshot, not an Anthropic-issued API version.

## Versioning and raw-spec findings

| Question | Finding |
|---|---|
| Is the old PDF revision scheme still visible? | No. The current landing page and API reference have no `API Version`, revision letter, release tag, or downloadable Compliance-specific PDF. |
| Is a public Compliance OpenAPI/Swagger file linked? | No. Neither the landing page nor the API-reference landing page links one. Obvious `/openapi.json` probes did not expose one. |
| Is there a raw documentation feed? | Yes. Anthropic serves a 204,482-byte `llms.txt` index and a 90,697,636-byte `llms-full.txt` corpus with full rendered content for 1,893 English pages. They are site-wide and unversioned, not Compliance-specific API contracts. |
| Is the API itself versioned? | Only by the stable-looking path prefix `/v1/compliance/*`. The documentation explicitly requires forward-compatible handling of unknown activity and actor types, indicating additive evolution within `v1`. |
| Can a snapshot still be pinned? | Yes: retain raw page bytes with fetch time, final URL, response headers, and SHA-256 hashes. The downloader in this directory does that. |

Evidence that the pages mutate in place:

- The current organization-data and error pages contain a dated migration
  note: `read:compliance_org_settings` was retired on 2026-06-30 and the
  settings endpoint now requires `read:compliance_org_data`.
- The captured Activity Feed reference describes its type filter as three
  named values "or 399 more" (402 advertised values total), far beyond the
  last local PDF revision.
- The Activity Feed guide tells consumers to pass through unknown `type` and
  `actor.type` values and ignore fields they do not recognize.

## Narrative documentation set

The landing page identifies these eight pages as the Compliance API section.

| Page | URL | What it covers |
|---|---|---|
| Compliance API | <https://platform.claude.com/docs/en/manage-claude/compliance-api> | Scope, tenant model, key types, base path, shared rate limit, and links to the rest of the section. |
| Set up the Compliance API | <https://platform.claude.com/docs/en/manage-claude/compliance-api-access> | Enablement, Compliance Access Keys versus Admin API keys, scopes, key rotation, and compromise response. |
| Query the Activity Feed | <https://platform.claude.com/docs/en/manage-claude/compliance-activity-feed> | Filtering, array query syntax, cursor behavior, Activity envelope, and actor union. |
| Retrieve and delete chats, files, and projects | <https://platform.claude.com/docs/en/manage-claude/compliance-content-data> | Chat/message retrieval, uploaded/generated files, artifacts, projects, documents, collaborators, and permanent deletion. |
| List organizations, users, roles, groups, and settings | <https://platform.claude.com/docs/en/manage-claude/compliance-org-data> | Directory traversal, RBAC/SCIM groups, stable identifiers, and effective organization settings. |
| Design your compliance integration | <https://platform.claude.com/docs/en/manage-claude/compliance-integration-patterns> | Window polling versus cursor catch-up, SIEM joins, retention, delivery guarantees, and chain of custody. |
| Handle Compliance API errors | <https://platform.claude.com/docs/en/manage-claude/compliance-errors> | 400/401/403/404/409/429/5xx catalog, retry rules, headers, and scope-specific failures. |
| Compliance API FAQ | <https://platform.claude.com/docs/en/manage-claude/compliance-faq> | Key/access, retention and blind spots, organization hierarchy, ordering, and sandbox questions. |

API-reference landing page:
<https://platform.claude.com/docs/en/api/compliance>

The raw corpus yielded 60 Compliance pages: eight narrative pages and 52 API
reference pages. They are available individually under `extracted/pages/` and
as the combined `extracted/anthropic-compliance-api-all.md`. All internal links
to other Compliance pages resolved within the extracted set.

## Current contract summary

### Authentication and scope model

All calls use `https://api.anthropic.com/v1/compliance/*` and the `x-api-key`
header.

| Key/scope | Current reach |
|---|---|
| Compliance Access Key (`sk-ant-api01-...`) | Can reach the full Compliance API, limited to scopes selected at creation. |
| Admin API key (`sk-ant-admin01-...`) | Activity Feed only, and only when the Compliance API was enabled before the key was created. |
| `read:compliance_activities` | Read the Activity Feed. |
| `read:compliance_user_data` | Read chats, messages, files, projects, organization users, and group members. |
| `delete:compliance_user_data` | Permanently delete supported chats, files, project documents, projects, and Code Artifacts. |
| `read:compliance_org_data` | Read organizations, roles, groups, and effective settings. This replaced the retired settings-only scope. |

Compliance Access Key scopes are immutable. Rotation is create-new,
cut-over, verify, delete-old. Stored pagination cursors survive key rotation
because they are scoped to the organization rather than the key.

### Tenant and data coverage

- A tenant has one parent organization with linked claude.ai and Claude Console
  organizations.
- Directory endpoints span linked organizations of both kinds.
- Content endpoints serve claude.ai chats/files/projects only.
- The Activity Feed spans authentication, content, administrative, platform,
  and API activity.
- The Compliance API does not return Claude Console or Claude API prompt/model
  response content.
- Content already removed by retention or hard-deleted through the Compliance
  API is unavailable.

### Limits, ordering, and delivery

- All `/v1/compliance/*` calls share **600 requests/minute per parent
  organization** across keys and linked organizations.
- Activities are documented as queryable within roughly one minute and
  retained for six years.
- Activity lists default to newest first; chats/messages and several directory
  resources use their own documented ordering.
- Cursor/page values are opaque. Do not parse them.
- Treat delivery as at-least-once and deduplicate on activity `id`.
- A failed request does not advance a cursor. Persist a cursor only after the
  covered page(s) have been stored.
- For completeness evidence, retain start/terminal cursor, record count, run
  timestamp, final `request-id`, and content hashes.

### Actor union currently documented

| `actor.type` | Principal | Important fields |
|---|---|---|
| `user_actor` | Signed-in user | `email_address`, `user_id`, `ip_address`, `user_agent` |
| `api_actor` | Customer API or Compliance API key | `api_key_id`, `ip_address`, `user_agent` |
| `admin_api_key_actor` | Admin API key used for organization administration | `admin_api_key_id` |
| `unauthenticated_user_actor` | Pre-authentication action | `unauthenticated_email_address`, `ip_address`, `user_agent` |
| `anthropic_actor` | Anthropic internal action | nullable `email_address` for shape consistency |
| `scim_directory_sync_actor` | Identity-provider SCIM action | `workos_event_id`, `directory_id`, nullable `idp_connection_type` |
| `service_account_actor` | Service account | `service_account_id`, `ip_address`, `user_agent` |
| `federated_identity_actor` | Verified OIDC workload | `issuer`, `subject`, optional `audience`, `ip_address`, `user_agent` |

The API reference currently includes actor variants beyond the six listed in
the narrative Activity Feed table, another reason to preserve the rendered
reference and implement unknown-variant handling.

## Endpoint inventory

This table is the current non-beta Compliance API surface visible in the
public group pages. Links point to the stable group/reference pages because
individual operation URLs are generated beneath them.

| Method | Path | Reference group |
|---|---|---|
| `GET` | `/v1/compliance/activities` | [Activities](https://platform.claude.com/docs/en/api/compliance/activities/list) |
| `GET` | `/v1/compliance/apps/chats` | [Chats](https://platform.claude.com/docs/en/api/compliance/apps/chats) |
| `DELETE` | `/v1/compliance/apps/chats/{claude_chat_id}` | [Chats](https://platform.claude.com/docs/en/api/compliance/apps/chats) |
| `GET` | `/v1/compliance/apps/chats/{claude_chat_id}/messages` | [Messages](https://platform.claude.com/docs/en/api/compliance/apps/chats/messages) |
| `GET` | `/v1/compliance/apps/chats/files/{claude_file_id}` | [Files](https://platform.claude.com/docs/en/api/compliance/apps/chats/files) |
| `DELETE` | `/v1/compliance/apps/chats/files/{claude_file_id}` | [Files](https://platform.claude.com/docs/en/api/compliance/apps/chats/files) |
| `GET` | `/v1/compliance/apps/chats/files/{claude_file_id}/content` | [Files](https://platform.claude.com/docs/en/api/compliance/apps/chats/files) |
| `GET` | `/v1/compliance/apps/chats/generated-files/{claude_gen_file_id}` | [Generated files](https://platform.claude.com/docs/en/api/compliance/apps/chats/generated_files) |
| `GET` | `/v1/compliance/apps/chats/generated-files/{claude_gen_file_id}/content` | [Generated files](https://platform.claude.com/docs/en/api/compliance/apps/chats/generated_files) |
| `GET` | `/v1/compliance/apps/artifacts/{artifact_version_id}` | [Chat artifacts](https://platform.claude.com/docs/en/api/compliance/apps/artifacts) |
| `GET` | `/v1/compliance/apps/artifacts/{artifact_version_id}/content` | [Chat artifacts](https://platform.claude.com/docs/en/api/compliance/apps/artifacts) |
| `GET` | `/v1/compliance/apps/projects` | [Projects](https://platform.claude.com/docs/en/api/compliance/apps/projects) |
| `GET` | `/v1/compliance/apps/projects/{project_id}` | [Projects](https://platform.claude.com/docs/en/api/compliance/apps/projects) |
| `DELETE` | `/v1/compliance/apps/projects/{project_id}` | [Projects](https://platform.claude.com/docs/en/api/compliance/apps/projects) |
| `GET` | `/v1/compliance/apps/projects/{project_id}/attachments` | [Project attachments](https://platform.claude.com/docs/en/api/compliance/apps/projects/attachments) |
| `GET` | `/v1/compliance/apps/projects/{project_id}/collaborators` | [Project collaborators](https://platform.claude.com/docs/en/api/compliance/apps/projects/collaborators) |
| `GET` | `/v1/compliance/apps/projects/documents/{document_id}` | [Project documents](https://platform.claude.com/docs/en/api/compliance/apps/projects/documents) |
| `GET` | `/v1/compliance/apps/projects/documents/{document_id}/metadata` | [Project documents](https://platform.claude.com/docs/en/api/compliance/apps/projects/documents) |
| `DELETE` | `/v1/compliance/apps/projects/documents/{document_id}` | [Project documents](https://platform.claude.com/docs/en/api/compliance/apps/projects/documents) |
| `GET` | `/v1/compliance/apps/code/artifacts` | [Code Artifacts](https://platform.claude.com/docs/en/api/compliance/code/artifacts) |
| `GET` | `/v1/compliance/apps/code/artifacts/{artifact_id}/versions/{version_id}` | [Code Artifacts](https://platform.claude.com/docs/en/api/compliance/code/artifacts) |
| `DELETE` | `/v1/compliance/apps/code/artifacts/{artifact_id}` | [Code Artifacts](https://platform.claude.com/docs/en/api/compliance/code/artifacts) |
| `GET` | `/v1/compliance/organizations` | [Organizations](https://platform.claude.com/docs/en/api/compliance/organizations/list) |
| `GET` | `/v1/compliance/organizations/{org_uuid}/users` | [Organization users](https://platform.claude.com/docs/en/api/compliance/organizations/users) |
| `GET` | `/v1/compliance/organizations/{org_uuid}/roles` | [Roles](https://platform.claude.com/docs/en/api/compliance/organizations/roles) |
| `GET` | `/v1/compliance/organizations/{org_uuid}/roles/{role_id}` | [Roles](https://platform.claude.com/docs/en/api/compliance/organizations/roles) |
| `GET` | `/v1/compliance/organizations/{org_uuid}/roles/{role_id}/permissions` | [Role permissions](https://platform.claude.com/docs/en/api/compliance/organizations/roles/permissions) |
| `GET` | `/v1/compliance/groups` | [Groups](https://platform.claude.com/docs/en/api/compliance/groups) |
| `GET` | `/v1/compliance/groups/{group_id}` | [Groups](https://platform.claude.com/docs/en/api/compliance/groups) |
| `GET` | `/v1/compliance/groups/{group_id}/members` | [Group members](https://platform.claude.com/docs/en/api/compliance/groups/members) |
| `GET` | `/v1/compliance/organizations/{org_uuid}/settings` | [Effective settings](https://platform.claude.com/docs/en/api/compliance/organizations/settings) |

## Resource-specific details worth preserving

- `GET /activities` accepts activity filters, actor/user filters,
  organization filters, timestamp bounds, ordering, and opaque cursors. Array
  filters use repeated `name[]=value` syntax.
- A chat list may be queried organization-wide; `user_ids[]` is only needed
  when filtering by user. Project filters on chats require user IDs too.
- Uploaded files, generated files, chat artifacts, and project documents now
  have explicit metadata/content splits. Metadata includes hashes and sizes
  where available.
- `Content-MD5` on the served file response is authoritative if it differs from
  the stored metadata hash.
- Project attachments are a discriminated union: binary `project_file` versus
  text `project_doc`. Fetch their contents through different endpoints.
- Project deletion fails with 409 while chats remain attached.
- Every documented delete is immediate and permanent.
- Code Artifacts are a separate current resource: list hosted sites, download a
  specific version, or delete the artifact.
- Organization UUID is the canonical cross-endpoint join key. Some older
  content objects still include a deprecated `org_`-prefixed
  `organization_id`.
- Users disappear from organization membership lists when removed, but their
  historical activities remain addressable by stable `user_...` ID.
- Effective settings are resolved state, not necessarily the administrator's
  raw configured value, and access can be enabled separately per parent
  organization.

## Error and retry snapshot

| Status | Current handling |
|---|---|
| 400 | Fix the request; includes invalid timestamps, limits, and cursors. |
| 401 | Invalid/missing/revoked key; do not retry until fixed. |
| 403 | Valid key with wrong scope or key type; scope-specific messages list `Got` and `Needed`. |
| 404 | Missing/deleted resource, inaccessible org/role/group, or unavailable settings endpoint. |
| 409 | Resolve resource state, notably attached chats blocking project deletion. |
| 429 | Honor `retry-after`; do not advance the cursor. |
| 500 | Do not retry when `x-should-retry: false`; otherwise back off. |
| 502/503/504/529 | Retry with exponential backoff. |

Authenticated responses expose request-budget headers. Authentication failures
do not consume quota; a valid but under-scoped request does.

## Gaps that remain without a raw schema export

- An OpenAPI/JSON-Schema representation of the 402 advertised Activity
  variants. Their rendered Markdown definitions are present in the extraction.
- A stable schema identifier or changelog corresponding to the current live
  reference.
- Confirmation that beta-only operations are absent/present when the site's
  **Include beta APIs** toggle changes.
- Proof of the internal source format used to render the API reference (it
  resembles a generated schema, but no public OpenAPI file was exposed).

For code generation or strict compatibility work, the rendered pages are not a
substitute for an Anthropic-issued OpenAPI artifact. Until one is published,
the safest local contract is a dated raw capture plus forward-compatible
decoding and regression fixtures from observed API responses.
