---
name: classify
description: Classify Claude Enterprise chat messages by usage taxonomy (work/non-work, intent, topic). Based on "How People Use ChatGPT" (Chatterji et al., 2025). Use to understand how users engage with Claude and compare to ChatGPT baseline.
argument-hint: user <email> or chat <chat-id>
---

# Message Classification Tool

The `audit classify` command classifies Claude Enterprise chat messages according to the taxonomy from "How People Use ChatGPT" (Chatterji et al., 2025). It uses Claude (Haiku or Sonnet) to analyze each user message and categorize it by:

- **Work vs Non-Work** — Is the message work-related? (ChatGPT baseline: 27% work)
- **Intent** — Asking (49%), Doing (40%), or Expressing (11%)
- **Topic** — 24 fine-grained categories grouped into 7 coarse groups

## Basic Usage

```bash
# Classify all chats for a user
audit classify user alice@example.com

# Classify a single chat
audit classify chat claude_chat_01abc...

# Classify and compare to ChatGPT baseline
audit classify --compare user alice@example.com
```

## Output Formats

```bash
# Default: text summary to stderr
audit classify user alice@example.com

# JSON output (for programmatic use)
audit classify --format=json user alice@example.com

# CSV format (matches database schema)
audit classify --format=csv --output=results.csv user alice@example.com

# SQL INSERT statements
audit classify --format=sql --output=inserts.sql user alice@example.com
```

## Taxonomy Selection

```bash
# Only classify work/non-work
audit classify --taxonomy=work user alice@example.com

# Only classify intent (asking/doing/expressing)
audit classify --taxonomy=intent user alice@example.com

# Only classify topic
audit classify --taxonomy=topic user alice@example.com

# All taxonomies (default)
audit classify --taxonomy=all user alice@example.com
```

## Classifier Model

```bash
# Fast/cheap (default): Claude 3.5 Haiku
audit classify --model=haiku user alice@example.com

# More accurate: Claude Sonnet
audit classify --model=sonnet user alice@example.com

# Use Claude CLI (Sonnet recommended for speed)
audit classify --cmd='claude --print --model sonnet' user alice@example.com
```

The `--cmd` flag pipes classification prompts through any shell command instead of calling the API directly. This is useful when:
- You have Claude CLI installed and configured
- You want to use your existing Claude authentication
- The Compliance API key doesn't have Messages API access

## Batch Classification

```bash
# Classify chat IDs from stdin
cat chat_ids.txt | audit classify --stdin

# Limit number of chats
audit classify --limit=10 user alice@example.com
```

## Dry Run Mode

```bash
# Classify without storing to database
audit classify --no-store --format=json user alice@example.com
```

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--taxonomy` | all | Which taxonomy: `work`, `intent`, `topic`, or `all` |
| `--model` | haiku | Classifier model: `haiku` (fast) or `sonnet` (accurate) |
| `--cmd` | (none) | Shell command to pipe prompts through (e.g., `claude --print --model sonnet`) |
| `--format` | summary | Output format: `summary`, `json`, `csv`, or `sql` |
| `--output` | stdout | Output file path |
| `--limit` | 0 | Max chats to classify (0 = unlimited) |
| `--no-store` | false | Don't store results in database |
| `--compare` | false | Show comparison to ChatGPT baseline |
| `--stdin` | false | Read chat IDs from stdin |
| `--db` | ~/.local/share/claude-audit/audit.db | Database path |
| `--org` | THG Ingenuity | Organization ID |
| `--api-key` | 1Password | API key (only needed if not using `--cmd`) |

## Viewing Results

After classification, use `usage-report` to see aggregated statistics:

```bash
# Org-wide usage breakdown
audit usage-report

# Per-user breakdown
audit usage-report --user alice@example.com

# Last 30 days
audit usage-report --period 30

# JSON output
audit usage-report --json
```

## ChatGPT Baseline Comparison

The `--compare` flag shows how usage compares to ChatGPT's aggregate statistics:

```
=== Comparison to ChatGPT Baseline ===
(From "How People Use ChatGPT", Chatterji et al., 2025)

## Work vs Non-Work
  Work-related:         69.0% vs  27.0% (ChatGPT)  +42.0%
  Non-work:             31.0% vs  73.0% (ChatGPT)  -42.0%

## User Intent
  asking:               47.9% vs  49.0% (ChatGPT)   -1.1%
  doing:                36.6% vs  40.0% (ChatGPT)   -3.4%
  expressing:           15.5% vs  11.0% (ChatGPT)   +4.5%

## Topic Groups
  technical_help:       31.7% vs   7.0% (ChatGPT)  +24.7%
  writing:              26.8% vs  24.0% (ChatGPT)   +2.8%
  ...
```

## Database Schema

Classifications are stored in the `classifications` table:

```sql
CREATE TABLE classifications (
    message_id       TEXT PRIMARY KEY,
    chat_id          TEXT NOT NULL,
    user_email       TEXT NOT NULL,
    message_created  TEXT NOT NULL,
    work_related     INTEGER,        -- 0 or 1
    intent           TEXT,           -- asking, doing, expressing
    topic_fine       TEXT,           -- 24 categories
    topic_coarse     TEXT,           -- 7 groups
    classified_at    TEXT NOT NULL,
    classifier_model TEXT NOT NULL
);
```

## Topic Categories

### Coarse Groups (7)
- **practical_guidance** (29%): how-to advice, tutoring, creative ideation, self-care
- **writing** (24%): editing, personal writing, translation, summaries, fiction
- **seeking_information** (24%): specific info, products, recipes
- **technical_help** (7%): programming, math, data analysis
- **multimedia** (7%): image creation/analysis, other media
- **self_expression** (2.4%): chitchat, relationships, games
- **other** (~6%): asking about the model, unclear

### Fine-Grained Categories (24)
`how_to_advice`, `tutoring_or_teaching`, `creative_ideation`, `health_fitness_beauty_or_self_care`, `edit_or_critique_provided_text`, `personal_writing_or_communication`, `translation`, `argument_or_summary_generation`, `write_fiction`, `specific_info`, `purchasable_products`, `cooking_and_recipes`, `computer_programming`, `mathematical_calculation`, `data_analysis`, `create_an_image`, `analyze_an_image`, `generate_or_retrieve_other_media`, `greetings_and_chitchat`, `relationships_and_personal_reflection`, `games_and_role_play`, `asking_about_the_model`, `other`, `unclear`

## Prerequisites

Before running classify, ensure you have:

1. **User data cached**: Run `audit users --refresh` to cache licensed users
2. **For user classification**: The user must exist in the cached users table

## Example Workflow

```bash
# 1. Ensure user data is cached
audit users --refresh

# 2. Classify a user's chats with ChatGPT comparison
audit classify --compare user alice@example.com

# 3. View aggregated report
audit usage-report --user alice@example.com

# 4. Export classifications as CSV
audit classify --format=csv --output=alice-classifications.csv user alice@example.com
```
