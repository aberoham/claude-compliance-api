# claude-audit

CLI for the [Anthropic Compliance API](https://platform.claude.com/docs/en/manage-claude/compliance-api).

See [CLAUDE.md](CLAUDE.md) for usage.

## Quick Start

```bash
go build -o audit ./cmd/audit/
./audit fetch          # Fetch activities
./audit users          # List licensed users
./audit chats --user alice@example.com   # List user's chats
./audit chatanalysis --cmd='claude --print' alice@example.com  # Analyze a user's usage
./audit classify --cmd='claude --print --model sonnet' --compare user alice@example.com  # Classify usage
./audit compliance list organizations    # Discover organization UUIDs
./audit compliance get settings <org-uuid>  # Effective organization settings
./audit compliance                       # Show the full live-resource command tree
./audit sync-resources --content=all      # Cache all metadata and local content blobs
./audit sync-resources --status           # Show per-resource cache counts
```

## Current Compliance API coverage

The Go client supports all 31 operations found in the documentation snapshot
dated 2026-07-15. The documentation UI's generic "Include beta APIs" setting
was enabled during discovery, but no separate Compliance beta paths were
identified, so these operations are not classified as beta or non-beta here.
Existing commands retain their SQLite-backed workflows; the `audit compliance`
command family provides live JSON access, streaming downloads, and guarded
permanent deletion.

`audit sync-resources` mirrors the newer resource graph into normalized SQLite
tables while retaining each API object's raw JSON. Every list page is committed
before its cursor advances, and interrupted snapshots resume with the same
generation. Metadata is always stored in SQLite. Content modes are:

- `--content=none` (default): metadata and chat transcripts only.
- `--content=text`: also cache project-document text in SQLite.
- `--content=all`: cache document text and stream binary downloads into an
  immutable SHA-256 object tree (default `<database-directory>/objects`).

The object-store boundary is backend-neutral: SQLite records `storage_backend`
and `object_key`, so a future Google Cloud Storage implementation does not
require moving binary payloads into the relational schema. Use
`--max-content-bytes` to impose a per-object limit and `--refresh-content` to
replace existing resource links.

See [the dated gap analysis](docs/compliance-api-gap-analysis-2026-07-15.md)
for the endpoint-by-endpoint before/after matrix and remaining limitations.

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

- [Compliance API guide](https://platform.claude.com/docs/en/manage-claude/compliance-api)
- [Compliance API reference](https://platform.claude.com/docs/en/api/compliance)
- [Claude Enterprise](https://www.anthropic.com/enterprise)
