---
name: chatanalysis
description: Generate an analysis prompt for a user's Claude Enterprise usage patterns. Use when you need to understand why a user stopped using Claude, identify friction points, or analyze engagement patterns.
argument-hint: <user-email>
---

# Chat Analysis Prompt Generator

The `audit chatanalysis` command generates a comprehensive prompt suitable for analyzing a user's Claude Enterprise usage patterns. It gathers activity data and chat transcripts, then wraps them with an analysis template.

## Basic Usage

```bash
# Pipe through Claude CLI for immediate analysis
audit chatanalysis --cmd='claude --print' alice@example.com

# Generate prompt to stdout (for manual use)
audit chatanalysis alice@example.com

# Save to a file
audit chatanalysis --output analysis-prompt.md alice@example.com
```

## Custom Analysis Prompts

You can provide a custom analysis request to replace the default "Analysis Request" section:

```bash
# Via stdin
echo "Why did this user stop using Claude? Focus on friction points." | audit chatanalysis --prompt-stdin alice@example.com

# Via file
audit chatanalysis --prompt-file my-analysis-request.txt alice@example.com
```

The custom prompt replaces only the analysis instructions; the user data sections (activity summary, chat transcripts) are always included.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--cmd` | (none) | Shell command to pipe the prompt through (e.g., `claude --print`) |
| `--output` | stdout | Output file path |
| `--prompt-stdin` | false | Read custom analysis prompt from stdin |
| `--prompt-file` | "" | Read custom analysis prompt from file |
| `--max-tokens` | 150000 | Maximum tokens in output (~4 chars/token) |
| `--db` | ~/.local/share/claude-audit/audit.db | SQLite database path |
| `--org` | THG Ingenuity | Organization ID |
| `--api-key` | 1Password | API key (reads from 1Password if not set) |

## What's Included in the Output

1. **Activity Summary**
   - First/last activity dates
   - Days since last activity
   - Total event count
   - Chats created vs. chats with follow-up messages
   - Primary client (Desktop, Chrome, etc.)

2. **Activity Breakdown by Type**
   - Count of each event type (chat_created, chat_viewed, file_uploaded, etc.)

3. **Activity Velocity (by week)**
   - Weekly breakdown: events, active days, trend (▲/▼/—)
   - Automated pattern detection:
     - "Peak usage near end then stopped — suggests workflow completion"
     - "Single week of activity — tried it briefly"
     - "Steady decline over time — possible loss of interest"
     - "Was ramping up before stopping — external factor likely"
     - "Usage tapered off to minimal — gradual disengagement"

4. **Full Chat Transcripts**
   - All messages from user and assistant
   - Attached file names
   - Chronologically ordered

5. **Analysis Instructions**
   - Default template with example output format
   - Or your custom analysis prompt

## Token Budget & Sampling

To fit within LLM context windows:

- Default budget: 150,000 tokens (~600KB text)
- ~6,000 tokens reserved for template and activity summary
- Remaining budget used for chat transcripts

When chats exceed the budget:
- First chat (user's first experience) is always included
- Most recent chat is always included
- Middle chats are randomly sampled to fill remaining space
- A note indicates how many chats were sampled (e.g., "7 of 45 chats sampled")

## Prerequisites

Before running chatanalysis, ensure you have:

1. **User data cached**: Run `audit users --refresh` to cache licensed users
2. **Activity data fetched**: Run `audit fetch` to populate the activity database

## Example Workflow

```bash
# 1. Ensure data is fresh
audit fetch --days 90
audit users --refresh

# 2a. Pipe through Claude CLI for immediate analysis
audit chatanalysis --cmd='claude --print' dormant.user@example.com

# 2b. Or generate prompt for manual use
audit chatanalysis --output /tmp/user-analysis.md dormant.user@example.com
cat /tmp/user-analysis.md | pbcopy  # macOS
```

## Output Format

The generated prompt follows this structure:

```markdown
You are analyzing Claude Enterprise usage data...

## User: user@example.com

### Activity Summary
- **First seen:** 2025-12-01
- **Last seen:** 2026-01-15 (20 days ago)
- **Total events:** 45
...

### Activity Breakdown by Type
- claude_chat_viewed: 30
- claude_chat_created: 10
...

### Activity Velocity (by week)
| Week Starting | Events | Active Days | Trend |
|---------------|--------|-------------|-------|
| 2025-12-01 | 15 | 3 | — |
| 2025-12-08 | 25 | 5 | ▲ |
| 2025-12-15 | 5 | 2 | ▼ |

**Pattern:** Peak usage near end then stopped — suggests workflow completion

### Chat Transcripts

#### Chat: "Project planning" (2025-12-01 10:30:00)
**User:** How should I structure this project?
**Assistant:** Here are some approaches...
---

## Analysis Request
[Default analysis instructions or your custom prompt]
```
