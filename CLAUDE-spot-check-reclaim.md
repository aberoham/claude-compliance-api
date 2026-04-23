# Seat Reclamation: Spot-Check Runbook

Step-by-step process for a human operator or agent to verify that a seat is truly safe to reclaim, and suggested communications for users in the grace/warning period.

## Prerequisites

- Working directory: `claude-compliance-api/`
- Environment configured: `source .env`
- Fresh audit log CSV export from the Anthropic admin console
- A working copy of a companion Okta admin tooling repo (one that provides `user_sso_report.py` and `search_user.py` or equivalents)

## Step 1: Generate the reclaim report

```bash
# Full analysis with CSV cross-reference and Okta SSO
./audit rank --reclaim --csv /path/to/audit_logs.zip --okta

# Without Okta (CSV cross-reference only)
./audit rank --reclaim --csv /path/to/audit_logs.zip

# JSON for programmatic filtering
./audit rank --reclaim --csv /path/to/audit_logs.zip --okta --json > reclaim_report.json

# Show only SAFE users
./audit rank --reclaim --csv /path/to/audit_logs.zip --okta --tier safe

# Adjust grace period (default 21 days)
./audit rank --reclaim --csv /path/to/audit_logs.zip --okta --grace 28
```

Review the summary line at the bottom: `N safe, N investigate, N do not reclaim`.

## Step 2: Verify each SAFE user (mandatory before reclamation)

For each user in the SAFE tier, run through this checklist. Any single failure moves the user to INVESTIGATE.

### 2a. Check Okta login history

If you ran `--okta` in Step 1, Okta SSO events are already factored into the reclaim scoring. The manual check below is only needed for deeper investigation of specific users.

```bash
cd /path/to/okta-api-tools

# Last 90 days of SSO app usage
uv run users/user_sso_report.py USER@example.com --days 30
```

Look for:
- Any SSO events to `claude.ai` or `Anthropic` in the last 30 days
- The `lastLogin` field in the user profile header
- If `lastLogin` is within 30 days, **STOP** — this user is active

### 2b. Check user agent history for shadow clients

```bash
cd /path/to/claude-compliance-api
./audit user-agents --user USER@example.com
```

Look for:
- `claude-excel`, `claude-powerpoint` — research preview Office clients
- `claude-chrome`, `crx-claude` — Chrome extension
- `claude-code`, `claudecode` — Claude Code CLI
- `ClaudeNest/` or ` Claude/` — Claude Desktop app
- Any user agent you don't recognize

If any research preview or non-browser client appears, **STOP** — this user may have shadow usage.

### 2c. Check analytics for Claude Code or connector activity

```bash
sqlite3 ~/.local/share/claude-audit/audit.db \
  "SELECT date, conversations, messages, cc_sessions, cc_commits,
          connectors_used, skills_used, artifacts_created, web_searches
   FROM analytics_user_daily
   WHERE user_email = 'USER@example.com'
     AND (conversations > 0 OR messages > 0 OR cc_sessions > 0
          OR cc_commits > 0 OR connectors_used > 0
          OR skills_used > 0 OR artifacts_created > 0
          OR web_searches > 0)
   ORDER BY date DESC"
```

If any rows are returned, **STOP** — this user has activity outside the Compliance API's visibility.

### 2d. Check the raw compliance activity beyond the lookback window

```bash
sqlite3 ~/.local/share/claude-audit/audit.db \
  "SELECT COUNT(*), MIN(created_at), MAX(created_at)
   FROM activities
   WHERE actor_email = 'USER@example.com'"
```

If the user has historical activity (even outside the 30-day window), note when they were last active. A user whose last activity was 45 days ago is a stronger reclaim candidate than one last active 31 days ago.

### 2e. Cross-reference with CSV export

```bash
# Check if user appears in the CSV with events the API missed
./audit compare /path/to/audit_logs.zip 2>&1 | grep -i 'USER@example.com'
```

If the user appears in "CSV-only users" with activity, **STOP** — this indicates a data gap in the Compliance API.

### 2f. Check Okta provisioning date and lifecycle

```bash
cd /path/to/okta-api-tools
uv run users/search_user.py 'profile.email eq "USER@example.com"'
```

Verify:
- `created` date — when the account was provisioned
- `status` — should be ACTIVE (if SUSPENDED, they may return)
- `lastLogin` — Okta's independent record of last authentication

If the account was created within the last 21 days and has never logged in, this is a **grace period** user (see Step 4 below), not a reclaim candidate.

## Step 3: Decision tree

```
Is the user active in ANY source within the last 7 days?
(Sources: Compliance API, Analytics API, CSV export, Okta SSO)
  YES --> DO NOT RECLAIM
  NO  --> Continue

Was the account provisioned within the last 21 days?
  YES --> Has the user ever logged in or generated any activity?
    NO  --> SAFE TO RECLAIM (zero-activity new account)
    YES --> Send onboarding nudge (Step 5a), check again in 7 days
  NO  --> Continue

Does the user have ANY of these shadow signals?
  - Claude Code sessions or commits (analytics)
  - Connector/MCP invocations (analytics)
  - Research preview user agent (Excel/PowerPoint/Chrome)
  - Active GitHub, GDrive, or GitHub Enterprise integration
  - Recent Okta SSO to Claude (even without Compliance API activity)
  YES --> INVESTIGATE — verify with chatanalysis or direct outreach
  NO  --> Continue

Is there ANY activity across ALL sources in the last 30 days?
  YES --> Is it only view/auth events (no chats, no messages)?
    YES --> Send engagement nudge (Step 5b), reclaim after 14 days if no response
    NO  --> DO NOT RECLAIM (genuine user with recent activity)
  NO  --> Continue

Is there ANY activity across ALL sources in the last 60 days?
  YES --> Send final warning (Step 5c), reclaim after 7 days if no response
  NO  --> SAFE TO RECLAIM
```

## Step 4: Batch verification script

For bulk verification, pipe the SAFE list through this loop:

```bash
# Generate SAFE email list (include --okta so SSO is factored into scoring)
./audit rank --reclaim --csv /path/to/audit_logs.zip --okta --json --tier safe \
  | jq -r '.[].email' > safe_candidates.txt

# Spot-check each against Okta for deeper investigation
cd /path/to/okta-api-tools
while read email; do
  echo "=== $email ==="
  uv run users/user_sso_report.py "$email" --days 90 2>&1 | head -5
  echo
done < /path/to/claude-compliance-api/safe_candidates.txt
```

When `--okta` is used, users with recent Claude SSO are already excluded from the SAFE tier. The manual loop above is a secondary verification for auditing purposes.

## Step 5: Communication templates

All communications should be sent from the IT/Security team, not automated. Tone should be helpful, not threatening.

### 5a. Onboarding nudge (new accounts with no login)

Send after 14 days of no activity post-provisioning.

> **Subject:** Your Claude Enterprise access is ready
>
> Hi {first_name},
>
> You were granted a Claude Enterprise seat on {provisioning_date}, but we haven't seen you log in yet. We want to make sure you can access it and know what's available.
>
> **How to access Claude:**
> - Go to claude.ai and sign in with your company SSO via Okta
> - Or open `Anthropic Claude` from your Okta dashboard
>
> **Did you know you can also use Claude via:**
> - **Claude Desktop** — download from claude.ai/download for a native app experience
> - **Claude for Chrome** — browser extension for using Claude alongside your work
> - **Claude for Excel/PowerPoint** — research preview integrations for Office
> - **Claude Code** — CLI tool for software development (ask your engineering team)
>
> If you're having trouble accessing Claude or don't need this seat, please reply to this email. Unused seats will be reassigned after {grace_deadline} to ensure our licenses are being used effectively.
>
> Thanks,
> {sender_name}

### 5b. Engagement nudge (view-only or minimal users)

Send to users who logged in but have minimal productive activity.

> **Subject:** Getting more from Claude Enterprise
>
> Hi {first_name},
>
> We noticed you've logged into Claude but haven't used it much yet. We'd love to help you get started — here are some things people in your role commonly use it for:
>
> - Drafting and editing documents, emails, and presentations
> - Summarizing long documents or meeting notes
> - Analyzing data and creating reports
> - Brainstorming and research
>
> **Quick start:** Try pasting a document into Claude and asking it to summarize the key points, or describe a task you need help with.
>
> We have a limited number of Enterprise seats, so if Claude isn't a good fit for your workflow, no worries — just let us know and we'll reallocate your seat to someone on the waiting list.
>
> Is there anything blocking you from using Claude? We're happy to set up a quick walkthrough.
>
> Thanks,
> {sender_name}

### 5c. Final reclamation warning (dormant users, 60+ days inactive)

Send 7 days before reclaiming the seat.

> **Subject:** Your Claude Enterprise seat will be reassigned on {reclaim_date}
>
> Hi {first_name},
>
> Your Claude Enterprise seat has been inactive since {last_active_date}. To make sure our licenses are being used effectively, we'll be reassigning unused seats on **{reclaim_date}** (7 days from now).
>
> **If you still need Claude access:**
> Simply log in to claude.ai before {reclaim_date} and your seat will be retained. If you need help accessing it, reply to this email.
>
> **If you no longer need it:**
> No action needed — your seat will be reassigned automatically. You can request access again in the future if your needs change.
>
> Thanks,
> {sender_name}

## Step 6: Post-reclamation

After reclaiming seats via Okta:

1. Run `./audit rank --reclaim` again to verify the users no longer appear as licensed
2. Monitor for re-provisioning requests over the next 2 weeks
3. If a reclaimed user requests access back, fast-track their reprovisioning
4. Update the SCIM group in Okta to reflect the changes
5. For Okta-based reclamation, use `okta_reconcile_orphans.py` to clean up any provisioning orphans

## Appendix: Shadow usage channels reference

| Channel | Where to check | Risk level |
|---------|---------------|------------|
| Claude Code | `analytics_user_daily.cc_sessions`, `cc_commits` | HIGH — invisible to Compliance API |
| Claude Code Desktop | `org_claude_code_desktop_enabled` activity (org-level) | HIGH — invisible to Compliance API |
| Claude Cowork | `analytics_user_daily.connectors_used` | HIGH — invisible to Compliance API |
| Cowork Agent | `org_cowork_agent_enabled/disabled` activity (org-level) | HIGH — invisible to Compliance API |
| Claude for Excel | User agent strings (`claude-excel`) | MEDIUM — no UAs observed yet |
| Claude for PowerPoint | User agent strings (`claude-powerpoint`) | MEDIUM — no UAs observed yet |
| Claude for Chrome | User agent strings (`claude-chrome`, `crx-claude`) | LOW — likely appears as Chrome |
| MCP Servers | `mcp_server_created/updated/deleted`, `mcp_tool_policy_updated` activities | MEDIUM — org-level config, not per-user invocations |
| Connectors/Skills | `analytics_user_daily.connectors_used`, `skills_used` | MEDIUM — org must have Cowork enabled |
| GitHub integration | `integration_user_connected` activity type | LOW-MEDIUM — usage after connect is invisible |
| GitHub Enterprise | `ghe_user_connected/disconnected` activity type | LOW-MEDIUM — separate from standard GitHub integration |
| GDrive integration | `integration_user_connected` activity type | LOW-MEDIUM — usage after connect is invisible |
| Okta SSO | `--okta` flag or manual Okta System Log check | MEDIUM — user may hold active session |
| iOS/Mobile | CSV `client_platform: ios` or Safari UA | LOW — generates standard events |
