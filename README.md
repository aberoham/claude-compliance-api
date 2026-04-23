# claude-audit

CLI for the [Anthropic Compliance API](https://support.claude.com/en/articles/13015708-access-the-compliance-api).

See [CLAUDE.md](CLAUDE.md) for usage.

## Quick Start

```bash
go build -o audit ./cmd/audit/
./audit fetch          # Fetch activities
./audit users          # List licensed users
./audit chats --user alice@example.com   # List user's chats
./audit chatanalysis --cmd='claude --print' alice@example.com  # Analyze a user's usage
./audit classify --cmd='claude --print --model sonnet' --compare user alice@example.com  # Classify usage
```

## Claude Code Skills

Two Claude Code skills ship alongside the CLI in [`docs/skills/`](docs/skills/):

- **`chatanalysis`** — generates an analysis prompt for a user's engagement
  patterns (see [`docs/skills/chatanalysis/SKILL.md`](docs/skills/chatanalysis/SKILL.md))
- **`classify`** — classifies chat messages by the "How People Use ChatGPT"
  taxonomy (see [`docs/skills/classify/SKILL.md`](docs/skills/classify/SKILL.md))

Claude Code discovers skills under `.claude/skills/` relative to the project
root or `~/.claude/skills/` globally. To install, symlink the skill
directories into whichever location you prefer:

```bash
# Project-scoped (recommended — available only when working in this repo)
mkdir -p .claude/skills
ln -s "$(pwd)/docs/skills/chatanalysis" .claude/skills/chatanalysis
ln -s "$(pwd)/docs/skills/classify"     .claude/skills/classify

# Or user-scoped (available in every repo)
ln -s "$(pwd)/docs/skills/chatanalysis" ~/.claude/skills/chatanalysis
ln -s "$(pwd)/docs/skills/classify"     ~/.claude/skills/classify
```

Symlinks keep the tracked copy in `docs/skills/` as the source of truth;
updating the skill here immediately updates the installed version. If you
prefer a copy over a symlink, use `cp -r` instead.

## Links

- [Compliance API Reference](https://support.claude.com/en/articles/13015708-access-the-compliance-api)
- [Claude Enterprise](https://www.anthropic.com/enterprise)
