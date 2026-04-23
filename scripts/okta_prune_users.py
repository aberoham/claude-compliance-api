#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["requests>=2.28.0"]
# ///
"""
Okta Group Membership Pruner - Remove inactive users from Claude Enterprise group.

Accepts Okta usernames (profile.login) or email addresses. When given an
email, searches both profile.login and profile.email to handle cases where
the two differ.

Usage:
  uv run okta_prune_claude_users.py [users...]                     # Dry-run with specific users
  uv run okta_prune_claude_users.py --file inactive.txt            # Dry-run from file
  uv run okta_prune_claude_users.py --list-members                 # List current group members
  uv run okta_prune_claude_users.py --execute user@example.com     # Actually remove user

Examples:
  uv run okta_prune_claude_users.py alice@example.com bob@example.com
  uv run okta_prune_claude_users.py jsmith                         # Okta username (login)
  uv run okta_prune_claude_users.py --file users_to_remove.txt --execute
  uv run okta_prune_claude_users.py --list-members | grep -i "inactive"
"""

import argparse
import os
import subprocess
import sys
from pathlib import Path

import requests

# Auto-load .env from the parent directory so `uv run` works without
# manually exporting variables first.
_ENV_FILE = Path(__file__).resolve().parent.parent / ".env"
if _ENV_FILE.exists():
    with open(_ENV_FILE, encoding="utf-8") as _f:
        for _line in _f:
            _line = _line.strip()
            if not _line or _line.startswith("#"):
                continue
            _line = _line.removeprefix("export ").strip()
            key, _, value = _line.partition("=")
            if key and value:
                value = value.strip().strip("'\"")
                os.environ.setdefault(key, value)


def _require_env(name: str) -> str:
    """Return an environment variable or exit with an error."""
    val = os.environ.get(name, "")
    if not val:
        print(f"Error: {name} not set (configure in .env)", file=sys.stderr)
        sys.exit(1)
    return val


# Okta configuration (from .env, auto-loaded above)
OKTA_DOMAIN = os.environ.get("OKTA_DOMAIN", "")
CLAUDE_GROUP_ID = os.environ.get("OKTA_CLAUDE_GROUP_ID", "")
CLAUDE_GROUP_NAME = os.environ.get("OKTA_CLAUDE_GROUP_NAME", "claude_group")

# 1Password configuration
OP_ITEM = os.environ.get("OKTA_OP_ITEM", "")
OP_FIELD = os.environ.get("OKTA_OP_FIELD", "")


def get_okta_token():
    """Retrieve API token from 1Password using op CLI."""
    if not OP_ITEM:
        _require_env("OKTA_OP_ITEM")
    if not OP_FIELD:
        _require_env("OKTA_OP_FIELD")
    try:
        result = subprocess.run(
            ["op", "item", "get", OP_ITEM, "--field", OP_FIELD],
            capture_output=True,
            text=True,
            check=True
        )
        token = result.stdout.strip()
        if not token:
            raise ValueError("Empty token returned from 1Password")
        return token
    except subprocess.CalledProcessError as e:
        print(f"Error retrieving token from 1Password: {e.stderr}", file=sys.stderr)
        print("Make sure you're signed into 1Password CLI: op signin", file=sys.stderr)
        sys.exit(1)
    except FileNotFoundError:
        print("Error: 1Password CLI (op) not found. Install from https://1password.com/downloads/command-line/", file=sys.stderr)
        sys.exit(1)


def okta_request(method, endpoint, token, params=None, expected_status=None):
    """Make an authenticated request to the Okta API."""
    url = f"https://{OKTA_DOMAIN}{endpoint}"
    headers = {
        "Authorization": f"SSWS {token}",
        "Accept": "application/json",
        "Content-Type": "application/json"
    }

    response = requests.request(method, url, headers=headers, params=params)

    if expected_status and response.status_code != expected_status:
        print(f"Unexpected status {response.status_code} from {method} {endpoint}", file=sys.stderr)
        print(f"Response: {response.text}", file=sys.stderr)
        return None

    if response.status_code == 204:
        return True

    if response.status_code >= 400:
        print(f"Error {response.status_code} from {method} {endpoint}: {response.text}", file=sys.stderr)
        return None

    return response.json()


def find_user(identifier, token):
    """Look up an Okta user by login or email.

    Tries profile.login first (more reliable, canonical casing), then
    falls back to profile.email. This handles cases where a user's Okta
    login and email address differ.
    """
    for field in ("login", "email"):
        search_query = f'profile.{field} eq "{identifier}"'
        result = okta_request(
            "GET", "/api/v1/users", token,
            params={"search": search_query},
        )
        if result is None:
            continue
        if len(result) == 0:
            continue
        if len(result) > 1:
            print(
                f"Warning: Multiple users matched profile.{field}"
                f" for {identifier}, using first match",
                file=sys.stderr,
            )
        return result[0]
    return None


def list_group_members(token, limit=200):
    """List all members of the Claude Enterprise group with pagination."""
    members = []
    endpoint = f"/api/v1/groups/{CLAUDE_GROUP_ID}/users"
    params = {"limit": limit}

    while True:
        result = okta_request("GET", endpoint, token, params=params)
        if result is None:
            break

        members.extend(result)

        if len(result) < limit:
            break

        # Get next page cursor from last user ID
        params["after"] = result[-1]["id"]

    return members


def get_group_member_ids(token):
    """Get a set of user IDs that are members of the Claude group."""
    members = list_group_members(token)
    return {m["id"] for m in members}


def remove_user_from_group(user_id, user_email, token, dry_run=True):
    """Remove a user from the Claude Enterprise group."""
    if dry_run:
        print(f"  [DRY-RUN] Would remove {user_email} (ID: {user_id}) from {CLAUDE_GROUP_NAME}")
        return True

    endpoint = f"/api/v1/groups/{CLAUDE_GROUP_ID}/users/{user_id}"
    result = okta_request("DELETE", endpoint, token, expected_status=204)

    if result:
        print(f"  [REMOVED] {user_email} (ID: {user_id}) from {CLAUDE_GROUP_NAME}")
        return True
    else:
        print(f"  [FAILED] Could not remove {user_email} (ID: {user_id})", file=sys.stderr)
        return False


def load_identifiers_from_file(filepath):
    """Load Okta usernames or email addresses from a file (one per line)."""
    identifiers = []
    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#'):
                identifiers.append(line)
    return identifiers


def print_group_members(members):
    """Print group members in a formatted table."""
    print(f"\n{'='*80}")
    print(f"Members of {CLAUDE_GROUP_NAME} (Group ID: {CLAUDE_GROUP_ID})")
    print(f"{'='*80}")
    print(f"{'Name':<30} {'Email':<40} {'Status':<10}")
    print(f"{'-'*30} {'-'*40} {'-'*10}")

    for member in sorted(members, key=lambda m: m.get("profile", {}).get("email", "").lower()):
        profile = member.get("profile", {})
        name = f"{profile.get('firstName', '')} {profile.get('lastName', '')}".strip() or "Unknown"
        email = profile.get("email", "Unknown")
        status = member.get("status", "Unknown")
        print(f"{name:<30} {email:<40} {status:<10}")

    print(f"\nTotal members: {len(members)}")


def main():
    parser = argparse.ArgumentParser(
        description="Manage Claude Enterprise Okta group membership.",
        epilog="""
Examples:
  uv run okta_prune_claude_users.py user@example.com           # Dry-run single user
  uv run okta_prune_claude_users.py jsmith                     # Dry-run by Okta username
  uv run okta_prune_claude_users.py --file inactive.txt        # Dry-run from file
  uv run okta_prune_claude_users.py --list-members             # List all members
  uv run okta_prune_claude_users.py --execute user@example.com # Actually remove
        """,
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    parser.add_argument('users', nargs='*', help='Okta usernames or email addresses to remove from group')
    parser.add_argument('--file', '-f', type=str, help='File containing usernames or emails (one per line)')
    parser.add_argument('--execute', action='store_true',
                       help='Actually remove users (default is dry-run)')
    parser.add_argument('--list-members', action='store_true',
                       help='List all current group members and exit')
    parser.add_argument('--quiet', '-q', action='store_true',
                       help='Suppress informational output')

    args = parser.parse_args()

    # Validate required env vars
    if not OKTA_DOMAIN:
        _require_env("OKTA_DOMAIN")
    if not CLAUDE_GROUP_ID:
        _require_env("OKTA_CLAUDE_GROUP_ID")

    # Get API token
    if not args.quiet:
        print("Retrieving Okta API token from 1Password...")
    token = get_okta_token()
    if not args.quiet:
        print("Token retrieved successfully.\n")

    # List members mode
    if args.list_members:
        if not args.quiet:
            print("Fetching group members...")
        members = list_group_members(token)
        print_group_members(members)
        return

    # Collect identifiers to process
    identifiers = []
    if args.file:
        identifiers.extend(load_identifiers_from_file(args.file))
    if args.users:
        identifiers.extend(args.users)

    if not identifiers:
        print("No users provided. Use --help for usage information.", file=sys.stderr)
        sys.exit(1)

    # Remove duplicates while preserving order
    identifiers = list(dict.fromkeys(identifiers))

    dry_run = not args.execute
    mode = "DRY-RUN" if dry_run else "EXECUTE"

    print(f"\n{'='*80}")
    print(f"Okta Group Membership Pruner - {mode} MODE")
    print(f"{'='*80}")
    print(f"Target group: {CLAUDE_GROUP_NAME} (ID: {CLAUDE_GROUP_ID})")
    print(f"Users to process: {len(identifiers)}")
    if dry_run:
        print(f"\n⚠️  DRY-RUN: No changes will be made. Use --execute to actually remove users.\n")
    else:
        print(f"\n🔴 EXECUTE MODE: Users WILL be removed from the group!\n")

    # Get current group members for validation
    if not args.quiet:
        print("Fetching current group membership...")
    group_member_ids = get_group_member_ids(token)
    if not args.quiet:
        print(f"Found {len(group_member_ids)} members in group.\n")

    # Process each identifier
    results = {
        'removed': [],
        'not_found': [],
        'not_in_group': [],
        'failed': []
    }

    print(f"Processing {len(identifiers)} user(s)...\n")

    for identifier in identifiers:
        print(f"Looking up: {identifier}")

        user = find_user(identifier, token)
        if user is None:
            print(f"  [NOT FOUND] No Okta user found for {identifier}")
            results['not_found'].append(identifier)
            continue

        user_id = user["id"]
        profile = user.get("profile", {})
        user_name = f"{profile.get('firstName', '')} {profile.get('lastName', '')}".strip()
        user_email = profile.get("email", identifier)

        if user_id not in group_member_ids:
            print(f"  [NOT IN GROUP] {identifier} ({user_name}) is not a member of {CLAUDE_GROUP_NAME}")
            results['not_in_group'].append(identifier)
            continue

        success = remove_user_from_group(user_id, user_email, token, dry_run=dry_run)
        if success:
            results['removed'].append(identifier)
        else:
            results['failed'].append(identifier)

    # Print summary
    print(f"\n{'='*80}")
    print("Summary")
    print(f"{'='*80}")
    print(f"{'Would remove' if dry_run else 'Removed'}: {len(results['removed'])}")
    print(f"Not found in Okta: {len(results['not_found'])}")
    print(f"Not in group: {len(results['not_in_group'])}")
    print(f"Failed: {len(results['failed'])}")

    if results['not_found']:
        print(f"\nUsers not found in Okta:")
        for ident in results['not_found']:
            print(f"  - {ident}")

    if results['not_in_group']:
        print(f"\nUsers not in {CLAUDE_GROUP_NAME}:")
        for ident in results['not_in_group']:
            print(f"  - {ident}")

    if dry_run and results['removed']:
        print(f"\nTo actually remove these {len(results['removed'])} users, run with --execute")


if __name__ == "__main__":
    main()
