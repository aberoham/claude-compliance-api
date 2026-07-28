#!/usr/bin/env python3
"""Extract all English Compliance API pages from Anthropic's llms-full.txt.

The full corpus uses page blocks containing an authoritative ``**URL:**`` line.
This script keeps blocks whose URL belongs to either the narrative Compliance
section or the Compliance API reference, writes one Markdown file per page,
and creates a deterministic combined document and extraction manifest.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parent
DEFAULT_SOURCE = ROOT / "raw" / "platform.claude.com__llms-full.txt.txt"
DEFAULT_OUTPUT = ROOT / "extracted"
URL_RE = re.compile(
    rb"(?m)^\*\*URL:\*\* "
    rb"(?P<url>https://platform\.claude\.com/docs/en/[^\r\n]+)\r?$"
)
H1_RE = re.compile(r"(?m)^# ([^\n]+)$")
COMPLIANCE_PREFIXES = (
    "https://platform.claude.com/docs/en/manage-claude/compliance",
    "https://platform.claude.com/docs/en/api/compliance",
)
REQUIRED_NARRATIVE_URLS = {
    "https://platform.claude.com/docs/en/manage-claude/compliance-api",
    "https://platform.claude.com/docs/en/manage-claude/compliance-api-access",
    "https://platform.claude.com/docs/en/manage-claude/compliance-activity-feed",
    "https://platform.claude.com/docs/en/manage-claude/compliance-content-data",
    "https://platform.claude.com/docs/en/manage-claude/compliance-org-data",
    "https://platform.claude.com/docs/en/manage-claude/compliance-integration-patterns",
    "https://platform.claude.com/docs/en/manage-claude/compliance-errors",
    "https://platform.claude.com/docs/en/manage-claude/compliance-faq",
}
LINK_RE = re.compile(r"\]\((/docs/en/(?:manage-claude/compliance|api/compliance)[^\s)#]*)(?:#[^)]+)?\)")
OPERATION_RE = re.compile(
    r"(?mi)^\*\*(get|post|put|patch|delete)\*\*\s+`(/v1/compliance/[^`]+)`"
)


@dataclass
class Page:
    url: str
    title: str
    source_line: int
    bytes: int
    sha256: str
    output_path: str


def line_number(data: bytes, position: int) -> int:
    return data.count(b"\n", 0, position) + 1


def page_start(data: bytes, url_start: int) -> int:
    boundary = data.rfind(b"\n---\n", 0, url_start)
    return boundary + len(b"\n---\n") if boundary >= 0 else 0


def page_end(data: bytes, current_url_end: int, next_url_start: int | None) -> int:
    if next_url_start is None:
        return len(data)
    boundary = data.rfind(b"\n---\n", current_url_end, next_url_start)
    return boundary if boundary >= 0 else next_url_start


def output_name(url: str) -> str:
    path = url.removeprefix("https://platform.claude.com/docs/en/").strip("/")
    return re.sub(r"[^A-Za-z0-9._-]+", "__", path) + ".md"


def clean_block(block: bytes) -> bytes:
    cleaned = block.strip()
    while cleaned.endswith(b"\n---"):
        cleaned = cleaned[:-4].rstrip()
    return cleaned + b"\n"


def extract(source: Path, output: Path) -> dict[str, object]:
    data = source.read_bytes()
    corpus_sha256 = hashlib.sha256(data).hexdigest()
    matches = list(URL_RE.finditer(data))
    selected: list[tuple[str, bytes, int]] = []

    for index, match in enumerate(matches):
        url = match.group("url").decode("utf-8")
        if not url.startswith(COMPLIANCE_PREFIXES):
            continue
        next_start = matches[index + 1].start() if index + 1 < len(matches) else None
        start = page_start(data, match.start())
        end = page_end(data, match.end(), next_start)
        selected.append((url, clean_block(data[start:end]), line_number(data, match.start())))

    duplicate_urls = sorted(
        url for url in {item[0] for item in selected} if sum(p[0] == url for p in selected) > 1
    )
    if duplicate_urls:
        raise RuntimeError(f"duplicate Compliance page URLs in corpus: {duplicate_urls}")

    pages_dir = output / "pages"
    pages_dir.mkdir(parents=True, exist_ok=True)
    page_records: list[Page] = []
    combined_parts: list[bytes] = []
    discovered_urls = {item[0] for item in selected}
    discovered_links: set[str] = set()
    operations: set[tuple[str, str]] = set()

    preamble = (
        "# Anthropic Compliance API documentation\n\n"
        "Extracted from Anthropic's unversioned `llms-full.txt` corpus.\n\n"
        f"- Source: <https://platform.claude.com/llms-full.txt>\n"
        f"- Source SHA-256: `{corpus_sha256}`\n"
        "- Snapshot date: 2026-07-15\n"
        "- Scope: all English pages whose canonical URL begins with "
        "`/docs/en/manage-claude/compliance` or `/docs/en/api/compliance`\n\n"
        "---\n"
    ).encode("utf-8")
    combined_parts.append(preamble)

    for url, block, source_line in selected:
        text = block.decode("utf-8")
        title_match = H1_RE.search(text)
        title = title_match.group(1).strip() if title_match else url.rsplit("/", 1)[-1]
        filename = output_name(url)
        destination = pages_dir / filename
        destination.write_bytes(block)
        digest = hashlib.sha256(block).hexdigest()
        page_records.append(
            Page(
                url=url,
                title=title,
                source_line=source_line,
                bytes=len(block),
                sha256=digest,
                output_path=str(destination.relative_to(ROOT)),
            )
        )
        if len(page_records) > 1:
            combined_parts.append(b"\n---\n\n")
        combined_parts.append(block)
        discovered_links.update(
            "https://platform.claude.com" + match.group(1)
            for match in LINK_RE.finditer(text)
        )
        operations.update(
            (match.group(1).upper(), match.group(2))
            for match in OPERATION_RE.finditer(text)
        )

    combined_path = output / "anthropic-compliance-api-all.md"
    combined_path.write_bytes(b"".join(combined_parts))
    source_urls_path = output / "source-urls.txt"
    source_urls_path.write_text(
        "\n".join(page.url for page in page_records) + "\n", encoding="utf-8"
    )

    missing_required = sorted(REQUIRED_NARRATIVE_URLS - discovered_urls)
    unresolved_links = sorted(discovered_links - discovered_urls)
    if missing_required:
        raise RuntimeError(f"missing required narrative pages: {missing_required}")

    result: dict[str, object] = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "source_path": str(source.relative_to(ROOT)),
        "source_url": "https://platform.claude.com/llms-full.txt",
        "source_bytes": len(data),
        "source_sha256": corpus_sha256,
        "page_count": len(page_records),
        "narrative_page_count": sum("/manage-claude/" in page.url for page in page_records),
        "api_reference_page_count": sum("/api/compliance" in page.url for page in page_records),
        "operation_count": len(operations),
        "operations": [
            {"method": method, "path": path} for method, path in sorted(operations)
        ],
        "unresolved_compliance_links": unresolved_links,
        "combined_path": str(combined_path.relative_to(ROOT)),
        "combined_bytes": combined_path.stat().st_size,
        "combined_sha256": hashlib.sha256(combined_path.read_bytes()).hexdigest(),
        "source_urls_path": str(source_urls_path.relative_to(ROOT)),
        "pages": [asdict(page) for page in page_records],
    }
    manifest_path = output / "manifest.json"
    manifest_path.write_text(
        json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return result


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=DEFAULT_SOURCE)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()
    result = extract(args.source.resolve(), args.output.resolve())
    print(
        "Extracted "
        f"{result['page_count']} pages "
        f"({result['narrative_page_count']} narrative, "
        f"{result['api_reference_page_count']} API reference), "
        f"{result['operation_count']} unique operations\n"
        f"Combined: {result['combined_path']} ({result['combined_bytes']} bytes)\n"
        f"SHA-256: {result['combined_sha256']}"
    )
    unresolved = result["unresolved_compliance_links"]
    if unresolved:
        print(f"Warning: {len(unresolved)} linked Compliance URLs were not in the corpus")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
