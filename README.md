# claude-audit

CLI for the [Anthropic Compliance API](https://docs.anthropic.com/en/docs/administration/administration-api/compliance-api-reference).

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

## Links

- [Compliance API Reference](https://docs.anthropic.com/en/docs/administration/administration-api/compliance-api-reference)
- [Claude Enterprise](https://www.anthropic.com/enterprise)
