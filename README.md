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

## Links

- [Compliance API Reference](https://support.claude.com/en/articles/13015708-access-the-compliance-api)
- [Claude Enterprise](https://www.anthropic.com/enterprise)
