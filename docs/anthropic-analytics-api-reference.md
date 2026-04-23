# Claude Enterprise Analytics API Reference Guide

## Overview

The Claude Enterprise Analytics API gives your organization programmatic access to engagement data for Claude and Claude Code usage within your Enterprise organization. Whether you're building internal dashboards for user activity or tracking adoption of projects, this API provides the aggregated metrics you need.

---

## Data Aggregation

All data is aggregated per organization, per day. Each endpoint returns a snapshot for a single date that you specify. Data for day (N-1) is run at 10:00:00 UTC time on day N, and is available for querying three days after aggregation, to ensure accuracy of data.

If data is not available within the timeline above, this usually indicates a data pipeline failure that our team will need to investigate internally. We are usually aware of such problems, but please raise this to your CSM if you want a gut check, or suspect something else.

---

## Enabling Access

In order to mint new analytics API keys, you must be a **Primary Owner** within your Enterprise organization. You can do so by navigating to `claude.ai/analytics/api-keys`.

Some more details that might be helpful:

- You can enable/disable access to the public API anytime. If you disable access by toggling the switch off, all requests will be denied.
- You'll need a key with the `read:analytics` scope in order to access the API.
- You can create multiple keys for your organization, but rate limits apply at the organization level, not the key level. See the "Rate limiting" section below.
- As always, we strongly recommend handling API keys securely: never share these keys publicly — they are secret, and should be shared securely.

---

## Base URL

All requests are sent to:
```
https://api.anthropic.com/v1/organizations/analytics/
```

---

## Authentication

Every request requires an API key passed in the `x-api-key` header. Your API key must have the `read:analytics` scope. You can create and manage API keys from the claude.ai admin settings under the **API Keys** section.

**Example request headers:**
```
x-api-key: $YOUR_API_KEY
```

---

## Pagination

Several endpoints return paginated results. Pagination uses a cursor-based approach, where the response includes a `next_page` token you pass back in your next request to retrieve the following page of results.

Two optional parameters control pagination:

- `limit` (integer): Number of records per page. Defaults to 20 for the `/users` endpoint and 100 for all other endpoints. The maximum is 1000.
- `page` (string): The opaque cursor token from the previous response's `next_page` field. Omit this on your first request.

When there are no more results, `next_page` will be `null` in the response.

---

## Error Responses

All endpoints return standard HTTP error codes:

| Code | Meaning |
|------|---------|
| 400  | A query parameter is invalid. Common causes include an invalid date, a date before 1/1/26 (first availability), or a date that is today or in the future. Data availability is delayed by three days. |
| 404  | The API key is missing, invalid, or does not have the `read:analytics` scope. |
| 429  | Rate limit exceeded. Too many requests. |
| 503  | Transient failure, please retry. |

---

## Rate Limiting

We do have default rate limits in place. If that isn't sufficient for your use case, we'd love to understand why. If necessary, we can adjust the rate limits for your organization — please reach out to your CSM.

---

## Endpoints

### 1. List User Activity

`GET /v1/organizations/analytics/users`

Returns per-user engagement metrics for a single day. Each item in the response represents one user and includes their activity counts across Claude (chat) and Claude Code.

#### Query Parameters

| Field  | Type    | Required | Description |
|--------|---------|----------|-------------|
| `date`   | string  | Yes      | The date to retrieve metrics for, in `YYYY-MM-DD` format. |
| `limit`  | integer | No       | Number of records per page (default: 20, max: 1000). |
| `page`   | string  | No       | Cursor token from a previous response's `next_page` field for retrieving the next page. |

#### Response Fields (per user)

| Field | Description |
|-------|-------------|
| `user.id` | Unique identifier for the user. |
| `user.email_address` | The user's email address. |
| `chat_metrics.distinct_conversation_count` | Number of distinct conversations, specifically within Claude.ai. |
| `chat_metrics.message_count` | Total messages sent, specifically within Claude.ai. |
| `chat_metrics.distinct_projects_created_count` | Number of projects created, specifically within Claude.ai. |
| `chat_metrics.distinct_projects_used_count` | Number of distinct projects used, specifically within Claude.ai. |
| `chat_metrics.distinct_files_uploaded_count` | Number of files uploaded, specifically within Claude.ai. |
| `chat_metrics.distinct_artifacts_created_count` | Number of artifacts created, specifically within Claude.ai. |
| `chat_metrics.thinking_message_count` | Number of thinking (extended) messages, specifically within Claude.ai. |
| `chat_metrics.distinct_skills_used_count` | Number of distinct skills used, specifically within Claude.ai. |
| `chat_metrics.connectors_used_count` | Total number of connectors invoked, specifically within Claude.ai. |
| `claude_code_metrics.core_metrics.commit_count` | Number of git commits made via Claude Code. |
| `claude_code_metrics.core_metrics.pull_request_count` | Number of pull requests created via Claude Code. |
| `claude_code_metrics.core_metrics.lines_of_code.added_count` | Total lines of code added. |
| `claude_code_metrics.core_metrics.lines_of_code.removed_count` | Total lines of code removed. |
| `claude_code_metrics.core_metrics.distinct_session_count` | Number of distinct Claude Code sessions. |
| `claude_code_metrics.tool_actions.edit_tool` | Accepted and rejected counts for the Edit tool. |
| `claude_code_metrics.tool_actions.multi_edit_tool` | Accepted and rejected counts for the Multi-Edit tool. |
| `claude_code_metrics.tool_actions.write_tool` | Accepted and rejected counts for the Write tool. |
| `claude_code_metrics.tool_actions.notebook_edit_tool` | Accepted and rejected counts for the Notebook Edit tool. |
| `web_search_count` | Total of web search tool invocations. This applies to both claude.ai and Claude Code usage within your organization. |

#### Example Request
```bash
curl -X GET "https://api.anthropic.com/v1/organizations/analytics/users?date=2025-01-01&limit=3" \\
  --header "x-api-key: $YOUR_API_KEY"
```

---

### 2. Activity Summary

`GET /v1/organizations/analytics/summaries`

Returns a high-level summary of engagement and seat utilization per-day for your organization for a given date range. The response is a list of days with aggregated counts within the date range.

> **Note:** The maximum difference between `ending_date` and `starting_date` must be 31 days, and there is a three-day delay in data availability.

This is useful for tracking daily active users, weekly and monthly trends, and seat allocation at a glance.

A user is considered **"active"** if any one of the following is true:
- The user sent at least one chat message on Claude (chat).
- The user had at least one Claude Code (local or remote) session associated with the C4E org, with tool use/git activity.

#### Query Parameters

| Field           | Type   | Required | Description |
|----------------|--------|----------|-------------|
| `starting_date` | string | Yes      | The starting date to retrieve data for, in `YYYY-MM-DD` format. There is a three-day delay in data availability, so the most recent data you can access is from three days ago. |
| `ending_date`   | string | No       | The optional ending date to retrieve data for, in `YYYY-MM-DD` format. This is exclusive. |

#### Response Fields

| Field | Description |
|-------|-------------|
| `starting_date` | First day for which metrics are aggregated, interpreted as a UTC date. |
| `ending_date` | Last day (exclusive) for which metrics are aggregated, interpreted as a UTC date. |
| `daily_active_user_count` | Number of users active on the specified date (based on token consumption). |
| `weekly_active_user_count` | Number of users active within the 7-day rolling window ending on the specified date. |
| `monthly_active_user_count` | Number of users active within the 30-day rolling window ending on the specified date. |
| `assigned_seat_count` | Total number of seats currently assigned in your organization. |
| `pending_invite_count` | Number of pending invitations that have not yet been accepted. |

> **Note:** The rolling windows for weekly and monthly counts look backward from the specified date (inclusive). If data is incomplete for some days within the window (for example, if the date is less than 30 days in the past), the monthly count may undercount activity.

#### Example Request
```bash
curl -X GET "https://api.anthropic.com/v1/organizations/analytics/summaries?starting_date=2025-01-01" \\
  --header "x-api-key: $YOUR_API_KEY"
```

---

### 3. Chat Project Usage

`GET /v1/organizations/analytics/apps/chat/projects`

Returns usage data broken down by chat project for a given date. Projects are specific to Claude (chat), so this endpoint focuses on that surface. Each item shows the project name, how many unique users interacted with it, and the total number of conversations held in that project.

#### Query Parameters

| Field   | Type    | Required | Description |
|---------|---------|----------|-------------|
| `date`    | string  | Yes      | The date to retrieve metrics for, in `YYYY-MM-DD` format. There is a three-day delay in data availability, so the most recent data you can access is from three days ago. |
| `limit`   | integer | No       | Number of records per page (default: 100, max: 1000). |
| `page`    | string  | No       | Cursor token from a previous response's `next_page` field for retrieving the next page. |

#### Response Fields (per project)

| Field | Description |
|-------|-------------|
| `project_name` | The name of the project. |
| `project_id` | The tagged project id, i.e. `"claude_proj_{ID}"`. |
| `distinct_user_count` | Number of unique users who used this project on the given date. |
| `distinct_conversation_count` | Number of conversations in this project on the given date. |
| `message_count` | Total number of messages sent within this project on the given date. |

#### Example Request
```bash
curl -X GET "https://api.anthropic.com/v1/organizations/analytics/apps/chat/projects?date=2025-01-01&limit=50" \\
  --header "x-api-key: $YOUR_API_KEY"
```

---

### 4. Skill Usage

`GET /v1/organizations/analytics/skills`

Returns skill usage data across both Claude (chat) and Claude Code within your organization for a given date. Each item represents a skill and shows how many unique users used it.

#### Query Parameters

| Field   | Type    | Required | Description |
|---------|---------|----------|-------------|
| `date`    | string  | Yes      | The date to retrieve metrics for, in `YYYY-MM-DD` format. There is a three-day delay in data availability, so the most recent data you can access is from three days ago. |
| `limit`   | integer | No       | Number of records per page (default: 100, max: 1000). |
| `page`    | string  | No       | Cursor token from a previous response's `next_page` field for retrieving the next page. |

#### Response Fields (per skill)

| Field | Description |
|-------|-------------|
| `skill_name` | The name of the skill. |
| `distinct_user_count` | Number of unique users who used this skill on the given date. |
| `chat_metrics.distinct_conversation_skill_used_count` | Number of distinct conversations in which the skill was used at least once, in chat. |
| `claude_code_metrics.distinct_session_skill_used_count` | Number of distinct remote sessions in which the skill was used at least once, in Claude Code. |

#### Example Request
```bash
curl -X GET "https://api.anthropic.com/v1/organizations/analytics/skills?date=2025-01-01" \\
  --header "x-api-key: $YOUR_API_KEY"
```

---

### 5. Connector Usage

`GET /v1/organizations/analytics/connectors`

Returns MCP/connector usage data across both Claude (chat) and Claude Code within your organization for a given date. Each item represents a connector and shows how many unique users used it.

> **Note:** Connector names are normalized from various sources — for example, "Atlassian MCP server," "mcp-atlassian," and "atlassian_MCP" would all appear as `"atlassian"`.

#### Query Parameters

| Field   | Type    | Required | Description |
|---------|---------|----------|-------------|
| `date`    | string  | Yes      | The date to retrieve metrics for, in `YYYY-MM-DD` format. There is a three-day delay in data availability, so the most recent data you can access is from three days ago. |
| `limit`   | integer | No       | Number of records per page (default: 100, max: 1000). |
| `page`    | string  | No       | Cursor token from a previous response's `next_page` field for retrieving the next page. |

#### Response Fields (per connector)

| Field | Description |
|-------|-------------|
| `connector_name` | The normalized name of the connector. |
| `distinct_user_count` | Number of unique users who used this connector on the given date. |
| `chat_metrics.distinct_conversation_connector_used_count` | Number of distinct conversations in which the connector was used at least once, in chat. |
| `claude_code_metrics.distinct_session_connector_used_count` | Number of distinct remote sessions in which the connector was used at least once, in Claude Code. |

#### Example Request
```bash
curl -X GET "https://api.anthropic.com/v1/organizations/analytics/connectors?date=2025-01-01" \\
  --header "x-api-key: $YOUR_API_KEY"
```
