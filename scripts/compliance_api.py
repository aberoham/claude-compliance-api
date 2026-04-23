#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = ["requests>=2.28.0"]
# ///
"""
Anthropic Compliance API client for fetching organization and user data.

This module provides functions to interact with the Anthropic Compliance API,
with built-in caching to minimize API calls for relatively static data.

Usage:
    from compliance_api import get_active_users_from_api
    users = get_active_users_from_api()  # Returns set of lowercase email addresses
"""

import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

import requests

# API configuration
COMPLIANCE_API_HOST = "https://api.anthropic.com"
DEFAULT_ORG_ID = os.environ.get("ANTHROPIC_ORG_ID", "")

# 1Password configuration (set in .env, loaded via `source .env`)
OP_ITEM = os.environ.get("ANTHROPIC_OP_ITEM", "")
OP_FIELD = os.environ.get("COMPLIANCE_OP_FIELD", "")

# Cache configuration
CACHE_DIR = Path.home() / ".cache" / "claude-audit-logs"
CACHE_FILE = CACHE_DIR / "compliance_cache.json"
USER_CACHE_TTL_HOURS = 4  # 4 hours for user list


def get_compliance_token() -> str:
    """Retrieve Compliance API key from 1Password using op CLI."""
    if not OP_ITEM:
        print("Error: ANTHROPIC_OP_ITEM not set (configure in .env)", file=sys.stderr)
        sys.exit(1)
    if not OP_FIELD:
        print("Error: COMPLIANCE_OP_FIELD not set (configure in .env)", file=sys.stderr)
        sys.exit(1)
    try:
        result = subprocess.run(
            ["op", "item", "get", OP_ITEM, "--field", OP_FIELD, "--reveal"],
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


def compliance_request(method: str, endpoint: str, token: str, params: dict = None) -> dict | None:
    """Make an authenticated request to the Compliance API."""
    url = f"{COMPLIANCE_API_HOST}{endpoint}"
    headers = {
        "x-api-key": token,
        "Accept": "application/json",
        "Content-Type": "application/json"
    }

    response = requests.request(method, url, headers=headers, params=params)

    if response.status_code >= 400:
        print(f"Error {response.status_code} from {method} {endpoint}: {response.text}", file=sys.stderr)
        return None

    return response.json()


def _load_cache() -> dict:
    """Load cache from JSON file, returning empty dict if not found or invalid."""
    if not CACHE_FILE.exists():
        return {}
    try:
        with open(CACHE_FILE, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError):
        return {}


def _save_cache(cache: dict):
    """Save cache to JSON file, creating directory if needed."""
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    with open(CACHE_FILE, 'w', encoding='utf-8') as f:
        json.dump(cache, f, indent=2)


def _is_cache_valid(cache_entry: dict | None, ttl_hours: int) -> bool:
    """Check if cache entry is still valid based on fetched_at timestamp."""
    if not cache_entry or "fetched_at" not in cache_entry:
        return False

    try:
        fetched_at = datetime.fromisoformat(cache_entry["fetched_at"].replace("Z", "+00:00"))
        age_hours = (datetime.now(timezone.utc) - fetched_at).total_seconds() / 3600
        return age_hours < ttl_hours
    except (ValueError, TypeError):
        return False


def list_organization_users(org_id: str, token: str, limit: int = 200) -> list[dict]:
    """
    Fetch all users for a specific organization with pagination.

    Requires read:compliance_org_data scope.
    """
    users = []
    params = {"limit": limit}

    while True:
        result = compliance_request(
            "GET",
            f"/v1/compliance/organizations/{org_id}/users",
            token,
            params=params
        )
        if result is None:
            break

        data = result.get("data", result) if isinstance(result, dict) else result
        if not data:
            break

        users.extend(data)

        # Users endpoint uses page/next_page cursor pagination
        has_more = result.get("has_more", False) if isinstance(result, dict) else False
        if not has_more:
            break

        next_page = result.get("next_page") if isinstance(result, dict) else None
        if not next_page:
            break
        params["page"] = next_page

    return users


def fetch_users_from_activities(token: str, org_id: str = None, limit: int = 1000) -> set[str]:
    """
    Extract unique user emails from the Activity Feed.

    Fallback approach that uses the Activity Feed to discover users who have had
    activity. Works with read:compliance_activities scope.
    """
    all_emails = set()
    params = {"limit": limit}

    # Only add org filter if specified
    if org_id:
        params["organization_ids[]"] = org_id

    page_count = 0
    max_pages = 50  # Safety limit to prevent infinite loops

    while page_count < max_pages:
        result = compliance_request("GET", "/v1/compliance/activities", token, params=params)
        if result is None:
            break

        activities = result.get("data", [])
        if not activities:
            break

        for activity in activities:
            actor = activity.get("actor", {})
            if actor.get("type") == "user_actor":
                email = actor.get("email_address")
                if email:
                    all_emails.add(email.lower())

        # Check for more pages
        has_more = result.get("has_more", False)
        if not has_more:
            break

        # Use after_id for pagination (to get older activities)
        last_id = result.get("last_id")
        if not last_id:
            break

        params["after_id"] = last_id
        page_count += 1

        # Progress indicator for large fetches
        if page_count % 10 == 0:
            print(f"    Processed {page_count} pages, found {len(all_emails)} unique users so far...", file=sys.stderr)

    return all_emails


def get_active_users_from_api(use_cache: bool = True, org_id: str = None) -> set[str]:
    """
    Fetch all users for the organization.

    Tries the direct users endpoint first (requires read:compliance_org_data),
    falls back to scanning Activity Feed if that fails.

    Caches the user list for USER_CACHE_TTL_HOURS (4 hours).
    Returns set of lowercase email addresses.
    """
    cache = _load_cache()

    if use_cache and _is_cache_valid(cache.get("users"), USER_CACHE_TTL_HOURS):
        return set(cache["users"]["data"])

    token = get_compliance_token()
    org_id = org_id or DEFAULT_ORG_ID
    if not org_id:
        print("Error: ANTHROPIC_ORG_ID not set (configure in .env or pass --org-id)", file=sys.stderr)
        sys.exit(1)
    all_emails = set()

    # Try direct users endpoint first
    print(f"  Fetching users for organization {org_id}...", file=sys.stderr)
    users = list_organization_users(org_id, token)

    if users:
        for user in users:
            email = user.get("email_address") or user.get("email")
            if email:
                all_emails.add(email.lower())
        print(f"  Found {len(all_emails)} users", file=sys.stderr)
    else:
        # Fall back to Activity Feed
        print("  Users endpoint unavailable, scanning Activity Feed...", file=sys.stderr)
        all_emails = fetch_users_from_activities(token, org_id)
        if all_emails:
            print(f"  Found {len(all_emails)} unique users in Activity Feed", file=sys.stderr)

    if not all_emails:
        print("Warning: No users found", file=sys.stderr)
        if "users" in cache:
            print("Using stale cached user list", file=sys.stderr)
            return set(cache["users"]["data"])
        return set()

    cache["users"] = {
        "data": sorted(all_emails),
        "fetched_at": datetime.now(timezone.utc).isoformat()
    }
    _save_cache(cache)

    return all_emails


def clear_cache():
    """Clear all cached data."""
    if CACHE_FILE.exists():
        CACHE_FILE.unlink()
        print(f"Cache cleared: {CACHE_FILE}")
    else:
        print("No cache file to clear")


def get_cache_info() -> dict:
    """Get information about the current cache state."""
    cache = _load_cache()
    info = {
        "cache_file": str(CACHE_FILE),
        "exists": CACHE_FILE.exists(),
        "default_org_id": DEFAULT_ORG_ID
    }

    if "users" in cache:
        entry = cache["users"]
        info["users"] = {
            "count": len(entry.get("data", [])),
            "fetched_at": entry.get("fetched_at"),
            "valid": _is_cache_valid(entry, USER_CACHE_TTL_HOURS),
            "ttl_hours": USER_CACHE_TTL_HOURS
        }

    return info


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Anthropic Compliance API client")
    parser.add_argument("--list-users", action="store_true", help="List all active users")
    parser.add_argument("--cache-info", action="store_true", help="Show cache status")
    parser.add_argument("--clear-cache", action="store_true", help="Clear cached data")
    parser.add_argument("--refresh", action="store_true", help="Force refresh (ignore cache)")
    parser.add_argument("--org-id", type=str, help="Organization ID (default: ANTHROPIC_ORG_ID env var)")

    args = parser.parse_args()

    if args.clear_cache:
        clear_cache()
    elif args.cache_info:
        info = get_cache_info()
        print(json.dumps(info, indent=2))
    elif args.list_users:
        users = get_active_users_from_api(use_cache=not args.refresh, org_id=args.org_id)
        for email in sorted(users):
            print(email)
        print(f"\nTotal: {len(users)} users", file=sys.stderr)
    else:
        parser.print_help()
