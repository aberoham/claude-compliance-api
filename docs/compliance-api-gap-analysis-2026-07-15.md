# Compliance API gap analysis — 2026-07-15

Contract source: the 60-page snapshot under
`2026-07-15-compliance-api-docs-scrape/`, pinned by the SHA-256 hashes in its
`extracted/manifest.json`. The snapshot contains 31 operations. The site's
generic "Include beta APIs" setting was enabled during discovery, but no
separate Compliance beta paths were identified.

## Executive summary

Before this work, the Go client had call paths for 7 of the 31 operations and
none of those paths represented the entire current contract. The largest gaps
were destructive operations, metadata/content separation, generated files and
artifacts, project attachments and collaborators, Code Artifacts, organization
discovery, RBAC groups/roles, and effective organization settings.

After this work, the `compliance` Go package has methods for all 31 operations.
The existing cached commands remain intact, while `audit compliance` exposes
the newly-supported resources as live JSON or streaming downloads. Permanent
deletes require an interactive resource-ID confirmation unless `--yes` is
explicitly supplied. `audit sync-resources` now mirrors the read-side resource
graph into normalized SQLite tables and can place immutable downloaded content
in a local SHA-256 object store.

## Operation matrix

| # | Method and path | Before | Current Go/CLI support |
|---:|---|---|---|
| 1 | `GET /v1/compliance/activities` | Partial | `FetchActivities`; all documented filters, ordering, cursors, optional inclusion of the client's own Compliance-access events, and unknown fields preserved; `audit fetch` |
| 2 | `GET /v1/compliance/apps/chats` | Partial | `FetchChats`; org-wide/per-user queries, current filters and both cursor directions; `audit chats` |
| 3 | `DELETE /v1/compliance/apps/chats/{id}` | Missing | `DeleteChat`; `audit compliance delete chat` |
| 4 | `GET /v1/compliance/apps/chats/{id}/messages` | Partial | `GetChat`, `GetChatRaw`, `FetchChatMessages`; tool blocks, generated files, filters, truncation controls, cursors; `audit chat` |
| 5 | `GET /v1/compliance/apps/chats/files/{id}` | Missing | `GetFileMetadata`; `audit compliance get file` |
| 6 | `DELETE /v1/compliance/apps/chats/files/{id}` | Missing | `DeleteFile`; `audit compliance delete file` |
| 7 | `GET /v1/compliance/apps/chats/files/{id}/content` | Partial | Streaming `DownloadFileContent` with filename, content type, and `Content-MD5`; existing `audit file` plus `audit compliance download file` |
| 8 | `GET /v1/compliance/apps/chats/generated-files/{id}` | Missing | `GetGeneratedFileMetadata`; `audit compliance get generated-file` |
| 9 | `GET /v1/compliance/apps/chats/generated-files/{id}/content` | Missing | `DownloadGeneratedFile`; `audit compliance download generated-file` |
| 10 | `GET /v1/compliance/apps/artifacts/{version_id}` | Missing | `GetChatArtifactMetadata`; `audit compliance get artifact` |
| 11 | `GET /v1/compliance/apps/artifacts/{version_id}/content` | Missing | `DownloadChatArtifact`; `audit compliance download artifact` |
| 12 | `GET /v1/compliance/apps/projects` | Partial | `FetchProjects`; all documented time/org/user filters; `audit projects` |
| 13 | `GET /v1/compliance/apps/projects/{id}` | Partial | `GetProject` with current organization/deletion fields; `audit project` |
| 14 | `DELETE /v1/compliance/apps/projects/{id}` | Missing | `DeleteProject`; `audit compliance delete project` |
| 15 | `GET /v1/compliance/apps/projects/{id}/attachments` | Missing | Discriminated file/document references via `FetchProjectAttachments`; `audit compliance list project-attachments` |
| 16 | `GET /v1/compliance/apps/projects/{id}/collaborators` | Missing | User/group/org/org-role grants via `FetchProjectCollaborators`; `audit compliance list project-collaborators` |
| 17 | `GET /v1/compliance/apps/projects/documents/{id}` | Missing | `GetProjectDocument`; `audit compliance get project-document` |
| 18 | `GET /v1/compliance/apps/projects/documents/{id}/metadata` | Missing | `GetProjectDocumentMetadata`; `audit compliance get project-document-metadata` |
| 19 | `DELETE /v1/compliance/apps/projects/documents/{id}` | Missing | `DeleteProjectDocument`; `audit compliance delete project-document` |
| 20 | `GET /v1/compliance/apps/code/artifacts` | Missing | `FetchCodeArtifacts`, including empty intermediate pages and current filters; `audit compliance list code-artifacts` |
| 21 | `GET /v1/compliance/apps/code/artifacts/{id}/versions/{version}` | Missing | Streaming `DownloadCodeArtifactVersion`; `audit compliance download code-artifact` |
| 22 | `DELETE /v1/compliance/apps/code/artifacts/{id}` | Missing | `DeleteCodeArtifact`; `audit compliance delete code-artifact` |
| 23 | `GET /v1/compliance/organizations` | Missing | `FetchOrganizations`; `audit compliance list organizations` |
| 24 | `GET /v1/compliance/organizations/{uuid}/users` | Partial | `FetchOrganizationUsers`, including `organization_role`; existing `FetchUsers`/`audit users` remain |
| 25 | `GET /v1/compliance/organizations/{uuid}/roles` | Missing | `FetchRoles`; `audit compliance list roles` |
| 26 | `GET /v1/compliance/organizations/{uuid}/roles/{id}` | Missing | `GetRole`; `audit compliance get role` |
| 27 | `GET /v1/compliance/organizations/{uuid}/roles/{id}/permissions` | Missing | `FetchRolePermissions`; `audit compliance list role-permissions` |
| 28 | `GET /v1/compliance/groups` | Missing | `FetchGroups` with name-prefix filter; `audit compliance list groups` |
| 29 | `GET /v1/compliance/groups/{id}` | Missing | `GetGroup`; `audit compliance get group` |
| 30 | `GET /v1/compliance/groups/{id}/members` | Missing | `FetchGroupMembers`; `audit compliance list group-members` |
| 31 | `GET /v1/compliance/organizations/{uuid}/settings` | Missing | `GetOrganizationSettings`; `audit compliance get settings` |

## Cross-cutting contract changes addressed

- Added `service_account_actor` and `federated_identity_actor` fields while
  retaining forward-compatible activity overflow JSON.
- Added `tool_use` and `tool_result` transcript blocks, truncation indicators,
  uploaded-file hashes/sizes, and generated-file references.
- Added organization-wide chat enumeration; `--user` is no longer mandatory
  when refreshing `audit chats`.
- Added `gt`, `gte`, `lt`, and `lte` time bounds where documented, multi-org
  filtering, current ordering controls, and opaque cursor support.
- Cursor-paginated resources continue until `next_page` is absent, including
  Code Artifact pages that are short or empty.
- Cursor pages can be committed through callbacks. SQLite records the next
  cursor only after its page and dependent resource work succeed, and a resumed
  snapshot reuses the same generation before stale rows are pruned.
- Added structured `APIError` values with HTTP status and `request-id`.
- Added retries for 429, 500, 502, 503, 504, and 529, honoring `Retry-After`
  and `x-should-retry: false`.
- Streaming downloads expose `Content-MD5`; callers can validate the exact
  served bytes rather than relying only on stored metadata hashes.
- Organizations and memberships, roles and permissions, settings, groups and
  members, project attachments/collaborators/documents, chat-discovered files,
  generated files and artifacts, and Code Artifacts/versions have normalized
  cache tables. Raw JSON is retained for additive API fields.
- Binary content is streamed outside SQLite through a backend-neutral object
  store interface. The current local backend verifies optional MD5 values,
  hashes with SHA-256, atomically renames completed objects, and deduplicates
  identical bytes. SQLite stores the backend, object key, and resource link.
- Effective settings retain polymorphic and future setting values as
  `json.RawMessage`, preventing a newly-added setting shape from breaking
  decoding.

## Deliberate remaining limitations

- The current content backend is local filesystem only. Its interface and
  relational records are ready for a later Google Cloud Storage backend, but
  no remote upload or lifecycle policy is implemented yet.
- File, generated-file, and chat-artifact endpoints do not provide collection
  listings. Their metadata is discovered from project attachments and full chat
  transcripts, so `--skip-chats` necessarily omits chat-only content resources.
- The 402 advertised activity variants are losslessly retained but are not
  represented as 402 separate Go structs. Stable envelope fields are typed and
  event-specific fields remain in `Activity.Extra`.
- Effective setting values are intentionally raw JSON discriminated by `type`;
  this is safer than a closed union for an unversioned, additive API.
- No Compliance-specific beta-only endpoints were found. If Anthropic adds one
  outside the current navigation/LLM corpus, it will require another discovery
  pass rather than being guessed from the site-wide beta toggle.
- The client reacts to the documented shared 600 requests/minute limit but
  does not proactively coordinate that budget across multiple processes or
  API keys.

## Validation

The new transport tests use an in-memory `http.RoundTripper`, covering opaque
pagination through an empty page, DELETE semantics, streaming headers,
forward-compatible settings, transient retry behavior, and non-retryable 500
responses. Store tests cover schema migration, interrupted-generation resume,
snapshot pruning, object links, local SHA-256 deduplication, size limits, and
MD5 verification. Existing `httptest`-server tests require local TCP listeners;
in the Codex sandbox those fail at listener creation with `operation not
permitted`, which is an environment limitation rather than a product failure.
