# CLAUDE.md

## Overview

Go module (`github.com/aberoham/claude-compliance-api`) that fetches activity data from the Anthropic Compliance API and daily engagement metrics from the Analytics API, caches both in a local SQLite database, and provides tools for querying, ranking users by engagement, and comparing against manually-exported CSV audit logs. Built as a replacement and to augment prior focus on Python-based analysis scripts.

## Building and Running

```bash
go build -o audit ./cmd/audit/
```

Or run directly:

```bash
go run ./cmd/audit/ <command> [flags]
```

## Commands

### `audit fetch` — Fetch activities from the Compliance API

```bash
# Default: last 30 days, API key from 1Password
audit fetch

# Custom date range
audit fetch --days 45

# Drop local cache and re-fetch from scratch
audit fetch --refresh

# Explicit API key (bypasses 1Password)
audit fetch --api-key sk-ant-...

# Custom database path
audit fetch --db /path/to/audit.db
```

Flags:
- `--days N` — Number of days of history to fetch (default 30)
- `--refresh` — **DESTRUCTIVE.** Drops all cached activities and re-fetches from scratch. Requires interactive confirmation (`PROCEED`). Almost never needed — a plain `audit fetch` does incremental updates. Do not use this flag in scripts or automated workflows.
- `--db PATH` — SQLite database path (default `~/.local/share/claude-audit/audit.db`)
- `--org ORG_ID` — Organization ID (default from `ANTHROPIC_ORG_ID` env var)
- `--api-key KEY` — API key; if omitted, retrieved from 1Password CLI

Fetching uses a two-phase strategy that supports Ctrl+C and resumption:

1. **Phase 1 (forward/incremental):** If we have a high-water mark from a previous run, fetch activities newer than what we've seen using `before_id`. The HWM updates on every page during the forward fetch, so progress survives interruptions.
2. **Phase 2 (backfill):** If the oldest stored activity hasn't reached the target date, resume paginating backwards from the oldest stored activity using `after_id`.

Each page of ~1000 activities is committed to SQLite immediately via an `OnPage` callback, so progress survives interruptions. Duplicate activities are silently skipped via `INSERT OR IGNORE` on the primary key.

### `audit users` — List licensed users

```bash
# Uses 4-hour cached data if available
audit users

# Force refresh from API
audit users --refresh
```

Flags: `--db`, `--org`, `--api-key`, `--refresh` (same semantics as `fetch`).

### `audit compare <csv>` — Compare API data against CSV export

```bash
audit compare /path/to/audit_logs.csv
audit compare /path/to/audit_logs.zip
```

Loads both datasets for the same date range and reports user overlap, per-user event counts, and event type mappings. Supports both `.csv` and `.zip` files (extracts the largest CSV from the archive).

### `audit projects` — List projects

```bash
# List all projects (uses cached data if fresh)
audit projects

# Force refresh from API
audit projects --refresh

# Filter by creator email
audit projects --user alice@example.com

# Output as JSON
audit projects --json
```

Flags: `--db`, `--org`, `--api-key`, `--refresh`, `--user`, `--json`.

### `audit project <id>` — Show project details

```bash
# Show project details
audit project claude_proj_01abc...

# Output as JSON (includes custom instructions)
audit project --json claude_proj_01abc...
```

### `audit chats` — List chat conversations

```bash
# List chats for a user (--user is required for API fetch)
audit chats --user alice@example.com

# Filter by date range
audit chats --user alice@example.com --since 2026-01-01 --until 2026-02-01

# Filter by project
audit chats --user alice@example.com --project claude_proj_01abc...

# Output as JSON
audit chats --user alice@example.com --json

# Limit results
audit chats --user alice@example.com --limit 50
```

Note: The chats API requires `user_ids[]`, so `--user` is required when fetching from the API. Cached data can be filtered locally.

Flags: `--db`, `--org`, `--api-key`, `--refresh`, `--user`, `--project`, `--since`, `--until`, `--limit`, `--json`.

### `audit chat <id>` — Export chat transcript

```bash
# Export as JSON (default)
audit chat claude_chat_01abc...

# Export as Markdown (human-readable)
audit chat --format=markdown claude_chat_01abc...

# Save to file
audit chat --output=transcript.json claude_chat_01abc...
audit chat --format=markdown --output=transcript.md claude_chat_01abc...
```

Note: Flags must come before the chat ID.

### `audit file <id>` — Download file attachment

```bash
# Download to original filename
audit file claude_file_01abc...

# Download to specific path
audit file --output=/path/to/save.pdf claude_file_01abc...
```

Note: Flags must come before the file ID.

### `audit chatanalysis <email>` — Generate user analysis prompt

Generates a comprehensive prompt for analyzing a user's Claude usage patterns. Gathers activity data from the local database and fetches chat transcripts from the API, then wraps everything with an analysis template suitable for pasting into Claude.

```bash
# Pipe through Claude CLI for immediate analysis
audit chatanalysis --cmd='claude --print' alice@example.com

# Basic usage (outputs prompt to stdout for manual use)
audit chatanalysis alice@example.com

# Save to file
audit chatanalysis --output analysis.md alice@example.com

# Custom analysis prompt via stdin
echo "Why did this user stop using Claude?" | audit chatanalysis --prompt-stdin alice@example.com

# Custom analysis prompt via file
audit chatanalysis --prompt-file my-analysis.txt alice@example.com
```

Flags:
- `--cmd COMMAND` — Shell command to pipe the prompt through (e.g., `claude --print`)
- `--output PATH` — Write to file instead of stdout
- `--prompt-stdin` — Read custom analysis prompt from stdin
- `--prompt-file PATH` — Read custom analysis prompt from file
- `--max-tokens N` — Maximum tokens in output (default 150000, ~4 chars/token)
- `--db`, `--org`, `--api-key` — Same as other commands

The output includes:
- Activity summary (dates, event counts, primary client)
- Event type breakdown
- Full chat transcripts (sampled if necessary to fit token budget)
- Analysis instructions with example output format

When chats exceed the token budget, the command samples intelligently: always includes the first and most recent chat, then randomly samples from the middle.

### `audit rank` — Stack-ranked engagement table

```bash
# Default: last 30 days, auto-fetches if data is stale (>1 hour)
audit rank

# Shorter window
audit rank --days 7

# Force fresh fetch before ranking
audit rank --refresh

# JSON output (pipe to jq)
audit rank --json | jq '.[0]'

# Explicit analytics API key
audit rank --analytics-api-key sk-ant-...
```

Flags: `--db`, `--org`, `--api-key`, `--analytics-api-key`, `--days` (default 30), `--refresh`, `--json`, `--reclaim` (enables seat reclamation mode; see below).

Produces a ranked table of all licensed users sorted by category priority, then recency (most stale first), then event count ascending. The table includes activity breakdown columns: `Proj` (projects created), `Share` (chat snapshots + session shares), and `Files` (files uploaded). When analytics data is available, additional columns are shown: `Conv` (conversations), `Msgs` (messages), and `CC` (Claude Code sessions).

Each user is assigned a category: `ZERO` (no activity in either source), `CODE ONLY` (no compliance activity but analytics shows Claude Code usage), `VIEW ONLY` (events but no chats created), `DORMANT` (last seen >14 days ago), `BARELY TRIED` (≤2 active days, ≤3 chats), `MINIMAL` (≤5 active days, ≤5 chats), `OCCASIONAL` (≤10 active days), or `REGULAR+`. Dormancy takes precedence over low-engagement categories so users who briefly tried Claude and then went silent are flagged as dormant.

Auto-fetches fresh activity data when the cache is >1 hour old (activities) or >4 hours old (users), without requiring `--refresh`. Analytics data is also auto-fetched with a 1-hour TTL. Note that analytics data has a 3-day lag compared to near real-time compliance data.

#### Reclaim mode (`--reclaim`)

Adds seat reclamation safety analysis by merging all available data sources (Compliance API, Analytics API, and optionally a CSV audit log export and Okta SSO logs) to produce a scored, tiered list of licensed users. Each user receives a 0-100 safety score where higher means safer to reclaim.

Reclaim mode requires analytics data and will fail if analytics cannot be fetched or loaded.

```bash
# Seat reclamation safety analysis
audit rank --reclaim

# Cross-reference with CSV audit log export
audit rank --reclaim --csv /path/to/audit_logs.zip

# Cross-reference with Okta SSO login data
audit rank --reclaim --okta

# All data sources combined
audit rank --reclaim --csv /path/to/audit_logs.zip --okta

# JSON output (pipe to jq for analysis)
audit rank --reclaim --json | jq '.[] | select(.tier == "SAFE")'

# Filter to specific tier
audit rank --reclaim --tier safe
audit rank --reclaim --tier investigate
audit rank --reclaim --tier dnr

# Custom grace period and lookback
audit rank --reclaim --days 14 --grace 28
```

Additional flags (require `--reclaim`):
- `--csv PATH` — CSV/ZIP audit log export for cross-reference
- `--okta` — Cross-reference with Okta System Log for Claude SSO events. Only counts successful authentication events (`outcome.result == "SUCCESS"`). Supports at most 90 days of lookback (Okta System Log retention limit); `--okta --days 91` is a fatal error.
- `--okta-api-key KEY` — Explicit Okta API token; if omitted, retrieved from 1Password CLI using `OKTA_OP_ITEM` and `OKTA_OP_FIELD` env vars
- `--grace N` — Grace period for newly provisioned accounts in days (default 21)
- `--tier TIER` — Filter output: `safe`, `investigate`, or `dnr`

The safety score is a composite of five components:

- **Recency (0-40)**: days since last activity across ALL data sources (including Okta SSO when `--okta` is used)
- **Inactivity (0-25)**: volume of activity across all sources. When Okta SSO data is available, users who authenticated to Claude but have no other visible activity score 5 instead of 20-25 — they are logging in, even if the Compliance API cannot see what they did.
- **Shadow channels (0-20)**: penalizes hidden activity invisible to the Compliance API (Claude Code sessions/commits, connector/MCP invocations indicating possible Cowork usage)
- **Integrations (0-10)**: penalizes active GitHub/GDrive integrations whose ongoing usage generates no events
- **Account age (0-5)**: grace period for recently provisioned accounts

Users are classified into three tiers:
- **SAFE** (score ≥75): high confidence the user is inactive across all sources
- **INVESTIGATE** (score 40-74): ambiguous signals requiring human review
- **DO NOT RECLAIM** (score <40): clear evidence of active usage

Hard overrides apply regardless of composite score: users active within 7 days are always DO NOT RECLAIM; accounts within the grace period that have some activity are capped at INVESTIGATE (zero-activity accounts within the grace period can still be SAFE). The Okta session-based override is disabled by default (`CLAUDE_SESSION_DURATION_DAYS=0`); when set to a value >7 and `--okta` is active, a successful Claude SSO within that window triggers a DO NOT RECLAIM override.

Shadow usage channels detected and flagged: Claude Code sessions/commits, connector/MCP invocations (possible Cowork), active GitHub/GDrive integrations, CSV-only events indicating data gaps.

#### Okta app matching

When `--okta` is used, the tool fetches all `user.authentication.sso` events from Okta's System Log and filters for the Claude app. App matching prefers the stable app instance ID (`OKTA_CLAUDE_APP_ID` env var) over display name (`OKTA_CLAUDE_APP_NAME`, default `"Anthropic Claude"`). If neither env var is set, the display name default is used.

### `audit analytics-users` — Fetch per-user daily analytics

Fetches daily per-user engagement metrics from the Analytics API and caches them locally. Uses incremental fetching — only missing dates are fetched on re-runs.

```bash
# Default: last 30 days
audit analytics-users

# Custom range
audit analytics-users --days 14

# Re-fetch all dates
audit analytics-users --refresh

# Explicit API key
audit analytics-users --analytics-api-key sk-ant-...
```

Flags: `--db`, `--analytics-api-key`, `--days` (default 30), `--refresh`.

The Analytics API has a 3-day data lag, so the effective range is `today-N-3` to `today-3`. Metrics include conversation counts, message counts, Claude Code commits/PRs/sessions/lines of code, and web searches.

### `audit analytics-summary` — Org-level DAU/WAU/MAU table

Fetches and displays org-level engagement summaries (daily/weekly/monthly active users, assigned seats, pending invites).

```bash
# Default: last 30 days
audit analytics-summary

# Custom range
audit analytics-summary --days 14

# JSON output
audit analytics-summary --json
```

Flags: `--db`, `--analytics-api-key`, `--days` (default 30), `--json`.

Example output:
```
Date         DAU   WAU   MAU  Seats  Pending
────────────────────────────────────────────────
2026-02-15    23    34    42     85        3
2026-02-14    21    33    41     85        3
```

### `audit user-agents` — List unique user agent strings

Extracts and summarizes unique user agent strings from stored activities, grouped by detected client name. Useful for discovering new client identifiers (e.g., Claude for Chrome, Claude for Excel, Claude for PowerPoint).

```bash
# All unique user agents
audit user-agents

# For a specific user
audit user-agents --user alice@example.com

# JSON output
audit user-agents --json
```

Flags: `--db`, `--user`, `--json`.

The `detectClient` function recognizes: Claude Desktop, Claude Code, Claude for Chrome, Claude for Excel, Claude for PowerPoint, Chrome, Firefox, Safari, and Edge. Unrecognized user agents are grouped as "Browser".

### `audit classify` — Classify chat messages by usage taxonomy

Classifies chat messages according to the taxonomy from "How People Use ChatGPT" (Chatterji et al., 2025). Uses Claude to analyze each user message and categorize it by:
- **Work vs Non-Work** — Is the message work-related? (ChatGPT baseline: 27% work)
- **Intent** — Asking (49%), Doing (40%), or Expressing (11%)
- **Topic** — 24 fine-grained categories grouped into 7 coarse groups

```bash
# Classify using Claude CLI (Sonnet recommended — fast single-word responses)
audit classify --cmd='claude --print --model sonnet' user alice@example.com

# Classify and compare to ChatGPT baseline
audit classify --cmd='claude --print --model sonnet' --compare user alice@example.com

# Limit number of chats
audit classify --cmd='claude --print --model sonnet' --limit=10 user alice@example.com

# Read chat IDs from stdin
cat chat_ids.txt | audit classify --cmd='claude --print --model sonnet' --stdin
```

Flags:
- `--cmd` — Shell command to pipe prompts through (e.g., `claude --print --model sonnet`)
- `--taxonomy` — Which taxonomy: `work`, `intent`, `topic`, or `all` (default: all)
- `--model` — API classifier model: `haiku` or `sonnet` (only used without `--cmd`)
- `--format` — Output format: `summary`, `json`, `csv`, or `sql` (default: summary)
- `--output` — Output file path (default: stdout)
- `--compare` — Show comparison to ChatGPT baseline
- `--no-store` — Don't store results in database
- `--limit N` — Maximum chats to classify (0 = no limit)
- `--stdin` — Read chat IDs from stdin
- `--db`, `--org`, `--api-key` — Same as other commands

Classifications are stored in the SQLite database for later aggregation via `usage-report`.

### `audit usage-report` — Generate usage classification report

Generates aggregated reports from stored classifications.

```bash
# Org-wide usage breakdown
audit usage-report

# Per-user breakdown with ChatGPT baseline comparison
audit usage-report --user alice@example.com --compare

# Last 30 days only
audit usage-report --period 30

# JSON output
audit usage-report --json
```

Flags:
- `--user EMAIL` — Filter to a specific user
- `--period N` — Filter to last N days
- `--compare` — Show comparison to ChatGPT baseline
- `--json` — Output as JSON instead of text
- `--db` — Database path

Example output:
```
=== Claude Usage Classification Report ===

Total messages: 142

## Work vs Non-Work
  Work-related:       98 ( 69.0%)
  Non-work:           44 ( 31.0%)

## User Intent
  Asking:             68 ( 47.9%)
  Doing:              52 ( 36.6%)
  Expressing:         22 ( 15.5%)

## Topic Groups
  technical_help:     45 ( 31.7%)
  writing:            38 ( 26.8%)
  practical_guidance: 28 ( 19.7%)
  ...
```

## Project Structure

```
claude-audit/
├── cmd/audit/main.go         CLI entrypoint with all subcommands
├── analytics/
│   ├── client.go             HTTP client, auth (1Password + explicit key), retry (429/503)
│   ├── types.go              UserMetrics, DailySummary, API response types
│   ├── users.go              FetchUserMetrics with cursor pagination
│   └── summaries.go          FetchSummaries (org-level DAU/WAU/MAU)
├── compliance/
│   ├── client.go             HTTP client, auth (1Password + explicit key), rate limiting
│   ├── types.go              Activity, Actor, User, Chat, Project structs, API response types
│   ├── activity_types.go     Rev I activity type constants and category groupings
│   ├── activities.go         FetchActivities with pagination, SummarizeByUser
│   ├── users.go              FetchUsers with pagination
│   ├── chats.go              FetchChats, GetChat, DownloadFile
│   ├── projects.go           FetchProjects, GetProject
│   └── classify.go           LLM-based message classification (work/intent/topic)
├── okta/
│   ├── client.go             HTTP client, SSWS auth (1Password + explicit key), rate limiting
│   ├── types.go              LogEvent, Actor, Target, Outcome, config helpers
│   └── sso.go                FetchClaudeSSOEvents with bulk query and app filtering
├── store/
│   ├── store.go              SQLite schema, Open/Close/Reset, WAL mode
│   ├── activities.go         Insert, query, aggregation methods, sync state
│   ├── analytics.go          Analytics daily metrics insert, aggregation, org summaries
│   ├── okta.go               Okta SSO event storage and per-user aggregation
│   ├── users.go              User cache with TTL-based staleness
│   ├── chats.go              Chat metadata caching
│   ├── projects.go           Project metadata caching
│   └── classifications.go    Classification storage and aggregation
├── csvaudit/
│   └── parser.go             CSV/ZIP parsing, actor_info extraction
├── scripts/
│   ├── analyze_enhanced.py   CSV audit log analysis with visualizations
│   ├── compliance_api.py     Standalone Compliance API client (Python)
│   ├── okta_prune_users.py   Remove inactive users from Okta SCIM group
│   └── okta_reconcile_orphans.py  Fix SCIM provisioning orphans
└── *_test.go                 Tests in each package
```

## Architecture

### Compliance API Client (`compliance/`)

The `Client` type wraps an `http.Client` with API key auth (`x-api-key` header) and automatic rate-limit handling (retries on HTTP 429 with `Retry-After`). Authentication is either explicit via `--api-key` or automatic via 1Password CLI (`op item get`).

`FetchActivities` paginates through the Activity Feed endpoint, filtering out `compliance_api_accessed` events (our own API footprint). It accepts an `OnPage` callback for per-page persistence; when this callback is set, activities are not accumulated in memory, keeping the heap footprint constant regardless of total result size.

The `Activity` type uses a custom `UnmarshalJSON` that maps known fields to struct fields and captures everything else (e.g., `claude_chat_id`, `filename`) into an `Extra json.RawMessage` field. `MarshalJSON` merges them back, so the full JSON round-trips losslessly through the `raw` column in SQLite.

The `Actor` struct supports multiple actor types with conditional fields: user actors (`email_address`, `user_id`, `ip_address`, `user_agent`), API actors (`api_key_id`), Admin API key actors (`admin_api_key_id`, Rev I), unauthenticated actors (`unauthenticated_email_address`), SCIM directory sync actors (`workos_event_id`, `directory_id`, `idp_connection_type`), and Anthropic actors. Admin API key actors have no email address, so their activities are stored with `actor_email = NULL` and excluded from per-user aggregation queries.

### SQLite Store (`store/`)

Single-file database at `~/.local/share/claude-audit/audit.db` with WAL mode enabled. Tables:

- **`activities`** — One row per API activity. Primary key on `id` enables idempotent upserts. Indexed on `created_at`, `actor_email`, and `type`. The `raw` column stores the full JSON so no data is ever lost.
- **`sync_state`** — Key-value pairs tracking `high_water_mark` (newest activity ID seen), `last_fetched_at`, and `analytics_last_fetched_at`.
- **`users`** — Cached licensed user records with `fetched_at` for TTL-based staleness (4-hour TTL).
- **`projects`** — Cached project metadata.
- **`chats`** — Cached chat metadata.
- **`classifications`** — LLM-generated classifications for user messages (work/non-work, intent, topic).
- **`analytics_user_daily`** — Per-user daily metrics from the Analytics API. Primary key on `(user_email, date)` for idempotent upserts. Tracks conversations, messages, Claude Code commits/PRs/sessions/lines, and web searches.
- **`analytics_org_daily`** — Org-level daily summaries (DAU, WAU, MAU, seats, invites). Primary key on `date`.
- **`okta_sso_events`** — Cached Okta System Log SSO events for the Claude app. Primary key on `event_id` (Okta UUID) for idempotent upserts. Indexed on `actor_email` and `published`. Only populated when `--okta` is used.

Aggregation queries (`UserSummaries`, `EventTypeCounts`, `AnalyticsUserSummaries`, `GetClassificationSummary`, etc.) run entirely in SQL rather than loading all rows into memory.

### Analytics API Client (`analytics/`)

The analytics `Client` mirrors the compliance client's structure but targets the Analytics API (`/v1/organizations/analytics/`). Key differences from the compliance client:

- **No org ID in URLs** — the org is implicit from the API key.
- **Separate API key** — uses a `read:analytics` scoped key stored in 1Password under a different field name (configured via `ANALYTICS_OP_FIELD` env var).
- **Retries on 503** — the Analytics API documents transient 503 errors alongside the standard 429 rate limit.
- **Cursor pagination** — uses a `next_page` / `page` cursor model rather than `before_id`/`after_id`.

`FetchUserMetrics(date)` returns all per-user engagement metrics for a single day. `FetchSummaries(start, end)` returns org-level DAU/WAU/MAU for a date range (max 31 days per request).

The Analytics API has a 3-day data lag — the most recent available date is `today - 3`.

### CSV Parser (`csvaudit/`)

Parses the manually-exported CSV audit logs from the Claude Enterprise admin UI. The `actor_info` column uses Python dict literal syntax with single quotes. Because names can contain apostrophes (e.g., "Reece O'Sullivan"), the parser uses delimiter-aware extraction (`', ` or `'}` as terminators) rather than simple quote replacement.

## API Pagination Notes

The Compliance API returns activities newest-first. The pagination parameters are counterintuitive:

- `before_id` paginates **forward in time** (toward newer activities), using `first_id` from each response as the next cursor
- `after_id` paginates **backward in time** (toward older activities), using `last_id` from each response as the next cursor

Cursor values are opaque (base64-encoded JSON), not raw activity IDs. The API accepts both formats as `before_id`/`after_id`.

The high-water mark tracks the newest activity we've seen (stored as a raw activity ID). For incremental fetches, we pass `before_id=<HWM>` to get only new activities. For backfill, we derive the resume cursor from the oldest stored activity in the database (via `OldestActivity()`) and pass it as `after_id`.

## Event Type Mapping (CSV to API)

Based on Compliance API Rev I (2026-03-29) Appendix B. The full mapping table is in `csvaudit/mappings.go`. Representative mappings shown here; see the source for the complete set.

| CSV Event | API Activity Type |
|---|---|
| `conversation_created` | `claude_chat_created` |
| `conversation_renamed` | `claude_chat_settings_updated` |
| `user_signed_in_sso` | `sso_login_succeeded` |
| `user_requested_magic_link` | `magic_link_login_initiated` |
| `user_name_changed` | `claude_user_settings_updated` |
| `org_user_updated` | `claude_user_role_updated` |
| `project_document_created` | `claude_project_document_uploaded` |
| `integration_org_enabled` | `claude_gdrive_integration_created` / `claude_github_integration_created` |
| `session_share_created` | `session_share_created` |

Three CSV events map to multiple API types (`integration_org_enabled/disabled/config_updated`) because the API distinguishes Google Drive and GitHub integrations while the CSV does not.

The API includes many additional activity types with no CSV equivalent. These are defined as constants in `compliance/activity_types.go` and categorized into: Admin API Keys, Authentication, Billing, Chat Snapshots, Groups (RBAC), Integrations, MCP Servers, Organization Settings, Platform Files, Platform Org Management, Platform Skills, Service Keys, Session Shares (access), and API Keys. API totals will always be higher than CSV totals.

## Data Source Coverage and Blind Spots

Three data sources exist for Claude Enterprise usage: the Compliance API activity feed, the Analytics API, and the manual CSV audit log export from the admin UI. Each has different coverage. Use `audit compare <csv>` to see the exact differences for any export.

### Compliance API blind spots

1. **Claude Code** — the Compliance API has zero visibility into Claude Code sessions, commits, PRs, or lines of code. These are only available via the Analytics API (see `audit rank` and `audit analytics-users`).

2. **Authentication events** — Rev I documents `sso_login_initiated`, `sso_login_failed`, `magic_link_login_initiated/succeeded/failed` and adds CSV-to-API mappings for them. Whether the API reliably emits these in practice has not been verified. Previously `sso_login_succeeded` was documented but never emitted, causing 25 "CSV-only" users whose only activity was auth events.

3. **Claude for Excel and PowerPoint** — these are "research preview" integrations. The `detectClient` function in `cmd/audit/main.go` parses their user agents (`claude-excel`, `claude-powerpoint`), but no matching user agents have been observed in practice. Prompts sent through these clients may not generate `claude_chat_created` events.

4. **Claude for Chrome extension** — `detectClient` supports `claude-chrome` and `crx-claude` user agent patterns, but no matching strings have been observed. The extension likely wraps the claude.ai web UI, so its traffic appears as standard Chrome browser activity.

5. **CSV-only event types** — Rev I resolved most previously CSV-only events (`user_requested_magic_link` → `magic_link_login_initiated`, `user_name_changed` → `claude_user_settings_updated`, `user_revoked_session` → `session_revoked`, `org_user_updated` → `claude_user_role_updated`). Remaining CSV-only types with no API equivalent: `project_renamed`, `conversation_deletion_requested`, `conversation_moved_to_project`, and several WebAuthn/request-signing-key events. The full list is shown by `audit compare`.

6. **Mapped event count gaps** — some mapped event pairs show the CSV total exceeding the API total (notably `project_document_created`, `project_document_deleted`, `conversation_deleted`). Root cause is unclear; may reflect filtering differences or timing. Run `audit compare` to see current gaps.

### CSV export blind spots

1. **View events** — the CSV export does not include `_viewed` events (`claude_file_viewed`, `claude_chat_viewed`, `claude_project_viewed`). These dominate the API's event volume and account for most of the numerical difference between API and CSV totals.

2. **Access failures** — `claude_chat_access_failed`, `claude_file_access_failed`, `claude_artifact_access_failed` are API-only.

3. **Artifacts** — `claude_artifact_created`, `claude_artifact_viewed`, `claude_artifact_sharing_updated` are API-only.

4. **Chat snapshots** — `claude_chat_snapshot_created` and `claude_chat_snapshot_viewed` are API-only.

5. **Groups/RBAC, service keys, API keys** — all API-only event categories.

6. **MCP server management** — `mcp_server_created/deleted/updated` and `mcp_tool_policy_updated` are API-only (Rev I). These are admin-level org configuration events, not individual user MCP tool invocations.

7. **Platform Console/API activity** — Rev I added platform event categories (Platform Files, Platform Org Management, Platform Skills) covering Claude Console and Claude API usage. These are API-only with no CSV equivalent.

### Neither source captures

1. **Claude Code prompt content** — the Analytics API provides aggregate metrics (conversations, messages, commits, PRs, sessions, lines of code) but not the actual prompts or responses. The Compliance API has no Claude Code data at all.

2. **Claude for Excel/PowerPoint prompt content** — if these research preview clients bypass the standard chat creation pipeline, their prompts are invisible to both data sources.

## Python Scripts (`scripts/`)

### `okta_reconcile_orphans.py` — Fix SCIM provisioning orphans

When users are added to the Okta SCIM group while Claude has no available seats, their account creation is rejected but they remain in the Okta group. They cannot re-request the entitlement (already in the group) and cannot log in (account was never created). This script finds those orphans and re-triggers provisioning by removing and re-adding them to the group.

The script compares Okta group membership against the Compliance API's licensed user list to identify orphans, then optionally cycles them out of and back into the group to trigger SCIM re-provisioning.

```bash
cd scripts

# Show orphaned users (dry-run, no changes)
uv run okta_reconcile_orphans.py

# Actually fix them (remove + wait + re-add)
uv run okta_reconcile_orphans.py --execute

# Skip the Compliance API and use a local file of active emails
uv run okta_reconcile_orphans.py --active-users-file active.txt

# Process only specific users from the orphan list
uv run okta_reconcile_orphans.py --execute --only user@example.com
```

Flags:
- `--execute` — Actually remove and re-add orphaned users. Without this flag, the script runs in dry-run mode and only reports what it would do.
- `--seats N` — Total purchased seats. Defaults to `ANTHROPIC_TOTAL_SEATS` env var; one of `--seats` or the env var is required.
- `--active-users-file PATH` — File of active Claude emails (one per line) to use instead of querying the Compliance API.
- `--only EMAIL [EMAIL ...]` — Only process these specific orphans from the detected list.
- `--delay N` — Seconds to wait between the remove and re-add steps (default: 10). Gives SCIM time to process the removal before re-triggering provisioning.

### `okta_prune_users.py` — Remove users from Okta SCIM group

Removes specified users from the Claude Enterprise Okta group. Accepts Okta usernames or email addresses; searches both `profile.login` and `profile.email` to handle mismatches.

```bash
cd scripts

# Dry-run (show what would happen)
uv run okta_prune_users.py user@example.com

# Actually remove
uv run okta_prune_users.py --execute user@example.com

# From a file of identifiers
uv run okta_prune_users.py --file inactive.txt --execute

# List current group members
uv run okta_prune_users.py --list-members
```

Flags:
- `--execute` — Actually remove users (default is dry-run)
- `--file PATH` — File of usernames/emails (one per line)
- `--list-members` — List all current group members and exit
- `--quiet` — Suppress informational output

Both Okta scripts auto-load `.env` from the project root; no manual `source` step is needed.

## Configuration

Copy `.env.example` to `.env` and fill in your org-specific values. The Go CLI and Python scripts both read from these environment variables. The required variables are:

| Variable | Used by | Purpose |
|---|---|---|
| `ANTHROPIC_ORG_ID` | Go CLI | Your Anthropic organization UUID |
| `ANTHROPIC_OP_ITEM` | Go CLI, Python | 1Password item name containing API keys |
| `COMPLIANCE_OP_FIELD` | Go CLI, Python | 1Password field for the Compliance API key |
| `ANALYTICS_OP_FIELD` | Go CLI | 1Password field for the Analytics API key |
| `OKTA_DOMAIN` | Okta scripts | Your Okta admin domain |
| `OKTA_CLAUDE_GROUP_ID` | Okta scripts | Okta group ID for Claude Enterprise |
| `OKTA_CLAUDE_GROUP_NAME` | Okta scripts | Okta group name (for display) |
| `OKTA_OP_ITEM` | Okta scripts, Go CLI | 1Password item for Okta API token |
| `OKTA_OP_FIELD` | Okta scripts, Go CLI | 1Password field for Okta API token |
| `OKTA_CLAUDE_APP_ID` | Go CLI (`--okta`) | Okta Claude app instance ID (preferred for matching) |
| `OKTA_CLAUDE_APP_NAME` | Go CLI (`--okta`) | Okta Claude app display name (default: `Anthropic Claude`) |
| `ANTHROPIC_TOTAL_SEATS` | Python scripts | Total purchased seat count for capacity calculations |
| `CLAUDE_SESSION_DURATION_DAYS` | Go CLI (`--okta`) | Claude session duration in days (default: 0, unlimited/disabled); values >7 enable session-based Okta override |

Other defaults:
- **API Host:** `https://api.anthropic.com`
- **Database Path:** `~/.local/share/claude-audit/audit.db`
- **User Cache TTL:** 4 hours
- **Analytics Data Lag:** 3 days (most recent data is `today - 3`)

## Development

### Testing

```bash
go test ./...
```

### Linting

Uses golangci-lint v2 (config in `.golangci.yml`):

```bash
golangci-lint run
```

### Dependencies

- `modernc.org/sqlite` — Pure Go SQLite driver (no CGo required)
- Standard library for everything else (HTTP, JSON, CSV, zip, CLI flags)
