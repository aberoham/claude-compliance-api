#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["requests>=2.28.0"]
# ///
"""
Reconcile orphaned Claude Enterprise users caused by SCIM provisioning
during seat exhaustion.

When users are added to the Okta SCIM group while Claude has no available
seats, their account creation is rejected but they remain in the Okta group.
They cannot re-request the entitlement (already in the group) and cannot
log in (account was never created). This script finds those orphans and
re-triggers provisioning by removing and re-adding them to the group.

Usage:
  cd claude-compliance-api/scripts && source ../.env

  # Show orphaned users (dry-run, no changes)
  uv run okta_reconcile_orphans.py

  # Actually fix them (remove + wait + re-add)
  uv run okta_reconcile_orphans.py --execute

  # Override seat limit (default: 245)
  uv run okta_reconcile_orphans.py --execute --seats 300

  # Skip the compliance API and use a local file of active emails
  uv run okta_reconcile_orphans.py --active-users-file active.txt

  # Process only specific users from the orphan list
  uv run okta_reconcile_orphans.py --execute --only user@example.com
"""

import argparse
import os
import subprocess
import sys
import time
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
                # Strip surrounding quotes if present
                value = value.strip().strip("'\"")
                os.environ.setdefault(key, value)

TOTAL_SEATS = int(os.environ.get("ANTHROPIC_TOTAL_SEATS", "0"))
REPROVISIONING_DELAY = 10  # seconds between remove and re-add

# Okta configuration (from .env)
OKTA_DOMAIN = os.environ.get("OKTA_DOMAIN", "")
CLAUDE_GROUP_ID = os.environ.get("OKTA_CLAUDE_GROUP_ID", "")
CLAUDE_GROUP_NAME = os.environ.get(
    "OKTA_CLAUDE_GROUP_NAME", "claude_group"
)

# 1Password configuration
OKTA_OP_ITEM = os.environ.get("OKTA_OP_ITEM", "")
OKTA_OP_FIELD = os.environ.get("OKTA_OP_FIELD", "")

# Compliance API configuration
COMPLIANCE_API_HOST = "https://api.anthropic.com"
ANTHROPIC_ORG_ID = os.environ.get("ANTHROPIC_ORG_ID", "")
ANTHROPIC_OP_ITEM = os.environ.get("ANTHROPIC_OP_ITEM", "")
COMPLIANCE_OP_FIELD = os.environ.get("COMPLIANCE_OP_FIELD", "")


def _require_env(name: str) -> str:
    val = os.environ.get(name, "")
    if not val:
        print(
            f"Error: {name} not set (source .env first)",
            file=sys.stderr,
        )
        sys.exit(1)
    return val


def _get_1password_field(item: str, field: str) -> str:
    """Retrieve a field value from 1Password."""
    try:
        result = subprocess.run(
            ["op", "item", "get", item, "--field", field, "--reveal"],
            capture_output=True,
            text=True,
            check=True,
        )
        token = result.stdout.strip()
        if not token:
            raise ValueError("Empty value returned from 1Password")
        return token
    except subprocess.CalledProcessError as e:
        print(
            f"1Password error: {e.stderr.strip()}",
            file=sys.stderr,
        )
        print(
            "Sign in with: op signin", file=sys.stderr
        )
        sys.exit(1)
    except FileNotFoundError:
        print(
            "Error: 1Password CLI (op) not found",
            file=sys.stderr,
        )
        sys.exit(1)


# -- Okta helpers ----------------------------------------------------------


def get_okta_token() -> str:
    if not OKTA_OP_ITEM:
        _require_env("OKTA_OP_ITEM")
    if not OKTA_OP_FIELD:
        _require_env("OKTA_OP_FIELD")
    return _get_1password_field(OKTA_OP_ITEM, OKTA_OP_FIELD)


def okta_request(method, endpoint, token, params=None):
    url = f"https://{OKTA_DOMAIN}{endpoint}"
    headers = {
        "Authorization": f"SSWS {token}",
        "Accept": "application/json",
        "Content-Type": "application/json",
    }
    resp = requests.request(
        method, url, headers=headers, params=params
    )
    if resp.status_code == 204:
        return True
    if resp.status_code >= 400:
        print(
            f"Okta {method} {endpoint} -> {resp.status_code}: "
            f"{resp.text}",
            file=sys.stderr,
        )
        return None
    return resp.json()


def list_okta_group_members(token, limit=200):
    """Fetch all members of the Claude SCIM group with pagination."""
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
        params["after"] = result[-1]["id"]
    return members


def remove_from_group(user_id, token):
    endpoint = (
        f"/api/v1/groups/{CLAUDE_GROUP_ID}/users/{user_id}"
    )
    return okta_request("DELETE", endpoint, token) is not None


def add_to_group(user_id, token):
    endpoint = (
        f"/api/v1/groups/{CLAUDE_GROUP_ID}/users/{user_id}"
    )
    return okta_request("PUT", endpoint, token) is not None


# -- Claude Compliance API helpers -----------------------------------------


def get_compliance_token() -> str:
    if not ANTHROPIC_OP_ITEM:
        _require_env("ANTHROPIC_OP_ITEM")
    if not COMPLIANCE_OP_FIELD:
        _require_env("COMPLIANCE_OP_FIELD")
    return _get_1password_field(ANTHROPIC_OP_ITEM, COMPLIANCE_OP_FIELD)


def fetch_claude_users(token: str, org_id: str) -> set[str]:
    """Fetch all licensed Claude users (returns lowercase emails)."""
    emails = set()
    params: dict = {"limit": 500}
    while True:
        url = (
            f"{COMPLIANCE_API_HOST}"
            f"/v1/compliance/organizations/{org_id}/users"
        )
        headers = {
            "x-api-key": token,
            "Accept": "application/json",
        }
        resp = requests.get(url, headers=headers, params=params)
        if resp.status_code >= 400:
            print(
                f"Compliance API error {resp.status_code}: "
                f"{resp.text}",
                file=sys.stderr,
            )
            break
        body = resp.json()
        data = body.get("data", body) if isinstance(body, dict) else body
        if not data:
            break
        for user in data:
            email = user.get("email_address") or user.get("email")
            if email:
                emails.add(email.lower())
        has_more = (
            body.get("has_more", False)
            if isinstance(body, dict)
            else False
        )
        if not has_more:
            break
        next_page = (
            body.get("next_page") if isinstance(body, dict) else None
        )
        if not next_page:
            break
        params["page"] = next_page
    return emails


def load_emails_from_file(path: str) -> set[str]:
    emails = set()
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#"):
                emails.add(line.lower())
    return emails


# -- Main ------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Find and fix orphaned Claude users caused by SCIM "
            "provisioning during seat exhaustion."
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("Usage:")[1] if "Usage:" in __doc__ else "",
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Actually remove and re-add orphaned users (default: dry-run)",
    )
    parser.add_argument(
        "--seats",
        type=int,
        default=TOTAL_SEATS or None,
        help="Total purchased seats (env: ANTHROPIC_TOTAL_SEATS)",
    )
    parser.add_argument(
        "--active-users-file",
        type=str,
        help="File of active Claude emails (one per line) instead of API",
    )
    parser.add_argument(
        "--only",
        nargs="*",
        metavar="EMAIL",
        help="Only process these specific orphans",
    )
    parser.add_argument(
        "--delay",
        type=int,
        default=REPROVISIONING_DELAY,
        help=f"Seconds between remove and re-add (default: {REPROVISIONING_DELAY})",
    )

    args = parser.parse_args()

    # Validate required env vars
    if not OKTA_DOMAIN:
        _require_env("OKTA_DOMAIN")
    if not CLAUDE_GROUP_ID:
        _require_env("OKTA_CLAUDE_GROUP_ID")
    if not args.seats:
        print("ERROR: --seats or ANTHROPIC_TOTAL_SEATS is required")
        sys.exit(1)

    # -- Step 1: Get Okta group members ------------------------------------
    print("Retrieving Okta API token from 1Password...")
    okta_token = get_okta_token()

    print(
        f"Fetching members of {CLAUDE_GROUP_NAME} "
        f"({CLAUDE_GROUP_ID})..."
    )
    okta_members = list_okta_group_members(okta_token)
    okta_by_email = {}
    for m in okta_members:
        email = m.get("profile", {}).get("email", "").lower()
        if email:
            okta_by_email[email] = m
    print(f"  Okta group members: {len(okta_by_email)}")

    # -- Step 2: Get Claude licensed users ---------------------------------
    if args.active_users_file:
        print(f"Loading active users from {args.active_users_file}...")
        claude_emails = load_emails_from_file(args.active_users_file)
    else:
        if not ANTHROPIC_ORG_ID:
            _require_env("ANTHROPIC_ORG_ID")
        print("Retrieving Compliance API token from 1Password...")
        compliance_token = get_compliance_token()
        print(
            f"Fetching licensed Claude users for org "
            f"{ANTHROPIC_ORG_ID}..."
        )
        claude_emails = fetch_claude_users(
            compliance_token, ANTHROPIC_ORG_ID
        )
    print(f"  Licensed Claude users: {len(claude_emails)}")

    # -- Step 3: Compute delta (orphans) -----------------------------------
    orphan_emails = sorted(
        set(okta_by_email.keys()) - claude_emails
    )

    if args.only:
        only_set = {e.lower() for e in args.only}
        orphan_emails = [e for e in orphan_emails if e in only_set]

    available_seats = args.seats - len(claude_emails)

    # -- Step 4: Report ----------------------------------------------------
    dry_run = not args.execute
    mode = "DRY-RUN" if dry_run else "EXECUTE"

    print(f"\n{'=' * 72}")
    print(f"Claude SCIM Orphan Reconciliation - {mode}")
    print(f"{'=' * 72}")
    print(f"Okta group members:    {len(okta_by_email)}")
    print(f"Licensed Claude users: {len(claude_emails)}")
    print(f"Purchased seats:       {args.seats}")
    print(f"Available seats:       {available_seats}")
    print(f"Orphaned users:        {len(orphan_emails)}")

    if not orphan_emails:
        print("\nNo orphaned users found. Nothing to do.")
        return

    print(f"\n{'Name':<35} {'Email':<40}")
    print(f"{'-' * 35} {'-' * 40}")
    for email in orphan_emails:
        m = okta_by_email[email]
        profile = m.get("profile", {})
        name = (
            f"{profile.get('firstName', '')} "
            f"{profile.get('lastName', '')}"
        ).strip() or "Unknown"
        print(f"{name:<35} {email:<40}")

    if available_seats <= 0:
        print(
            f"\nNo available seats ({len(claude_emails)}/{args.seats})."
            f" Cannot re-provision any users."
        )
        print(
            "Free up seats first, then re-run this script."
        )
        return

    can_fix = min(len(orphan_emails), available_seats)
    if can_fix < len(orphan_emails):
        print(
            f"\nOnly {available_seats} seats available; "
            f"will process first {can_fix} of "
            f"{len(orphan_emails)} orphans."
        )
        orphan_emails = orphan_emails[:can_fix]

    if dry_run:
        print(
            f"\nDry-run: {can_fix} user(s) would be re-provisioned."
            f" Run with --execute to apply."
        )
        return

    # -- Step 5: Re-provision orphans --------------------------------------
    print(f"\nRe-provisioning {can_fix} user(s)...\n")

    results = {"success": [], "failed": []}

    for i, email in enumerate(orphan_emails, 1):
        m = okta_by_email[email]
        okta_id = m["id"]
        profile = m.get("profile", {})
        name = (
            f"{profile.get('firstName', '')} "
            f"{profile.get('lastName', '')}"
        ).strip()

        print(f"[{i}/{can_fix}] {name} ({email})")

        print(f"  Removing from {CLAUDE_GROUP_NAME}...")
        if not remove_from_group(okta_id, okta_token):
            print(f"  FAILED to remove. Skipping.", file=sys.stderr)
            results["failed"].append(email)
            continue

        print(f"  Waiting {args.delay}s for SCIM sync...")
        time.sleep(args.delay)

        print(f"  Re-adding to {CLAUDE_GROUP_NAME}...")
        if not add_to_group(okta_id, okta_token):
            print(
                f"  FAILED to re-add! User is now OUTSIDE the group.",
                file=sys.stderr,
            )
            print(
                f"  Manually add {email} back to {CLAUDE_GROUP_NAME}.",
                file=sys.stderr,
            )
            results["failed"].append(email)
            continue

        print(f"  Done.")
        results["success"].append(email)

    # -- Summary -----------------------------------------------------------
    print(f"\n{'=' * 72}")
    print("Summary")
    print(f"{'=' * 72}")
    print(f"Re-provisioned: {len(results['success'])}")
    print(f"Failed:         {len(results['failed'])}")

    if results["failed"]:
        print("\nFailed users (may need manual group re-add):")
        for email in results["failed"]:
            print(f"  - {email}")

    if results["success"]:
        print(
            "\nSCIM sync will now attempt to create accounts for "
            "re-added users. Verify in the Claude admin console "
            "that new accounts appear within a few minutes."
        )


if __name__ == "__main__":
    main()
