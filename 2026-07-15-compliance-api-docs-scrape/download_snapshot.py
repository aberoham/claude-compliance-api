#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "beautifulsoup4>=4.12",
#   "markdownify>=0.13",
# ]
# ///
"""Download and hash the public Anthropic Compliance API documentation.

The public pages are not versioned. This script turns the fetch time and
SHA-256 digest into the local version identifier. It deliberately saves the
raw response before attempting any HTML-to-Markdown conversion.

Usage:
    uv run download_snapshot.py
    uv run download_snapshot.py --timeout 60 --overwrite
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path

from bs4 import BeautifulSoup
from markdownify import markdownify


ROOT = Path(__file__).resolve().parent
SOURCE_LIST = ROOT / "source-urls.txt"
USER_AGENT = "claude-compliance-api-docs-snapshot/2026-07-15"


@dataclass
class Result:
    url: str
    fetched_at: str
    ok: bool
    status: int | None = None
    final_url: str | None = None
    content_type: str | None = None
    etag: str | None = None
    last_modified: str | None = None
    bytes: int | None = None
    sha256: str | None = None
    raw_path: str | None = None
    markdown_path: str | None = None
    warning: str | None = None
    error: str | None = None


def load_urls() -> list[str]:
    urls: list[str] = []
    for line in SOURCE_LIST.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            urls.append(line)
    return urls


def slug_for(url: str) -> str:
    parsed = urllib.parse.urlparse(url)
    path = parsed.path.strip("/") or "index"
    slug = f"{parsed.netloc}__{path}"
    return re.sub(r"[^A-Za-z0-9._-]+", "__", slug)


def extension(content_type: str, url: str) -> str:
    media_type = content_type.split(";", 1)[0].strip().lower()
    if media_type == "text/html":
        return ".html"
    if media_type in {"application/json", "application/openapi+json"}:
        return ".json"
    if media_type.startswith("text/"):
        suffix = Path(urllib.parse.urlparse(url).path).suffix
        return suffix if suffix else ".txt"
    return ".bin"


def useful_body(soup: BeautifulSoup):
    for selector in ("main", "article", "[role=main]"):
        node = soup.select_one(selector)
        if node is not None:
            return node
    return soup.body or soup


def html_to_markdown(data: bytes) -> tuple[str, str | None]:
    soup = BeautifulSoup(data, "html.parser")
    for tag in soup.select("script, style, noscript, nav, header, footer"):
        tag.decompose()
    body = useful_body(soup)
    text = body.get_text(" ", strip=True)
    warning = None
    if text.count("Loading...") >= 4 or len(text) < 200:
        warning = (
            "The raw HTML appears to be a JavaScript shell; use llms-full.txt "
            "or a rendered-browser capture for the complete page."
        )
    rendered = markdownify(str(body), heading_style="ATX").strip() + "\n"
    return rendered, warning


def fetch(url: str, raw_dir: Path, md_dir: Path, timeout: float, overwrite: bool) -> Result:
    fetched_at = datetime.now(timezone.utc).isoformat()
    request = urllib.request.Request(
        url,
        headers={
            "User-Agent": USER_AGENT,
            "Accept": "text/html,text/plain,application/json,application/yaml;q=0.9,*/*;q=0.5",
            "Accept-Encoding": "identity",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            data = response.read()
            status = response.status
            final_url = response.geturl()
            content_type = response.headers.get("Content-Type", "application/octet-stream")
            digest = hashlib.sha256(data).hexdigest()
            basename = slug_for(url)
            raw_path = raw_dir / f"{basename}{extension(content_type, final_url)}"
            if overwrite or not raw_path.exists():
                raw_path.write_bytes(data)

            markdown_path: Path | None = None
            warning: str | None = None
            if content_type.lower().startswith("text/html"):
                converted, warning = html_to_markdown(data)
                markdown_path = md_dir / f"{basename}.md"
                if overwrite or not markdown_path.exists():
                    markdown_path.write_text(converted, encoding="utf-8")
            elif content_type.lower().startswith("text/plain"):
                markdown_path = md_dir / f"{basename}.md"
                if overwrite or not markdown_path.exists():
                    markdown_path.write_bytes(data)

            return Result(
                url=url,
                fetched_at=fetched_at,
                ok=200 <= status < 300,
                status=status,
                final_url=final_url,
                content_type=content_type,
                etag=response.headers.get("ETag"),
                last_modified=response.headers.get("Last-Modified"),
                bytes=len(data),
                sha256=digest,
                raw_path=str(raw_path.relative_to(ROOT)),
                markdown_path=(
                    str(markdown_path.relative_to(ROOT)) if markdown_path else None
                ),
                warning=warning,
            )
    except urllib.error.HTTPError as exc:
        body = exc.read()
        basename = slug_for(url)
        raw_path = raw_dir / f"{basename}.http-{exc.code}.bin"
        if body and (overwrite or not raw_path.exists()):
            raw_path.write_bytes(body)
        return Result(
            url=url,
            fetched_at=fetched_at,
            ok=False,
            status=exc.code,
            final_url=exc.geturl(),
            content_type=exc.headers.get("Content-Type"),
            bytes=len(body),
            sha256=hashlib.sha256(body).hexdigest() if body else None,
            raw_path=str(raw_path.relative_to(ROOT)) if body else None,
            error=str(exc),
        )
    except Exception as exc:  # preserve per-URL failure and continue the capture
        return Result(url=url, fetched_at=fetched_at, ok=False, error=repr(exc))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument("--overwrite", action="store_true")
    parser.add_argument("--delay", type=float, default=0.1)
    args = parser.parse_args()

    raw_dir = ROOT / "raw"
    md_dir = ROOT / "markdown"
    raw_dir.mkdir(exist_ok=True)
    md_dir.mkdir(exist_ok=True)

    results: list[Result] = []
    urls = load_urls()
    for index, url in enumerate(urls, start=1):
        print(f"[{index:02d}/{len(urls):02d}] {url}", file=sys.stderr)
        result = fetch(url, raw_dir, md_dir, args.timeout, args.overwrite)
        results.append(result)
        if result.error:
            print(f"  ERROR: {result.error}", file=sys.stderr)
        elif result.warning:
            print(f"  WARNING: {result.warning}", file=sys.stderr)
        if args.delay and index != len(urls):
            time.sleep(args.delay)

    manifest = {
        "snapshot_name": "2026-07-15-compliance-api-docs-scrape",
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "user_agent": USER_AGENT,
        "results": [asdict(result) for result in results],
    }
    (ROOT / "manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )

    failures = sum(not result.ok for result in results)
    shells = sum(bool(result.warning) for result in results)
    print(
        f"Wrote manifest.json: {len(results)} URLs, {failures} failures, "
        f"{shells} likely JavaScript shells",
        file=sys.stderr,
    )
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
