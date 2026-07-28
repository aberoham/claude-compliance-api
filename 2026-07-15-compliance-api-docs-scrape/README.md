# Anthropic Compliance API documentation snapshot

Snapshot date: **2026-07-15** (Europe/London)

Entry point: <https://platform.claude.com/docs/en/manage-claude/compliance-api>

## Result

No public, Compliance-specific, versioned OpenAPI/Swagger document was found.
The current documentation is split across eight narrative pages and a
JavaScript-rendered API-reference tree. The pages do not carry a document
revision comparable to the old PDF's `2026-04-20 (Rev J)` marker, and the API
reference changes in place.

The raw-document sources are:

- <https://platform.claude.com/llms.txt> — a 204,482-byte documentation index.
- <https://platform.claude.com/llms-full.txt> — a 90,697,636-byte site-wide
  Markdown corpus containing full rendered content for 1,893 English pages.
  The captured SHA-256 is
  `41b5031bcf9a6c1aa5b32623b61e65c626e7436732402fce05cd1eef42ea7859`.

The corpus contains **60 current Compliance pages**: all eight narrative pages
and 52 API-reference pages. The deterministic combined document and its hash
are recorded in `extracted/manifest.json`.

Obvious OpenAPI locations (`/openapi.json`, `/docs/openapi.json`, and
`https://api.anthropic.com/openapi.json`) all returned HTTP 404. Searches of
Anthropic's public SDK repositories also did not reveal a checked-in Compliance
API OpenAPI source.

## Files

- [`compliance-api-snapshot.md`](compliance-api-snapshot.md) — current
  documentation map, endpoint inventory, important contracts, and differences
  from the old versioned PDF model.
- [`source-urls.txt`](source-urls.txt) — seed URLs for the eight narrative
  pages, API-reference groups, raw-document candidates, and spec probes.
- [`download_snapshot.py`](download_snapshot.py) — reproducible downloader.
  It saves raw response bytes, converts useful HTML bodies to Markdown, and
  writes a SHA-256 manifest so a later run can be pinned even if Anthropic
  continues to mutate the pages in place.
- [`extract_compliance_docs.py`](extract_compliance_docs.py) — extracts every
  English Compliance narrative and API-reference page from the downloaded
  `llms-full.txt` corpus into individual Markdown files and a combined file.
- [`browser-harness-findings.md`](browser-harness-findings.md) — records the
  CDP retry, the sandbox-level loopback blocker, and offline inspection of the
  captured Next.js page sources.

## Capture notes

The individual HTML responses are JavaScript application shells, so their
automatic HTML-to-Markdown files are not useful documentation. The authoritative
content for this snapshot is the captured `llms-full.txt` plus the 60 files
under `extracted/pages/`. The extraction also verified that every Compliance
link between those pages resolves to another captured page.

Chrome remote debugging was made available on port 9222. After reconnecting
with the matching Chrome profile, the local browser harness successfully
inspected the rendered documentation and its network activity. It found no
linked or fetched Compliance-specific OpenAPI/Swagger document. See
`browser-harness-findings.md`. The rendered UI also exposed only the same 31
Compliance operations extracted from `llms-full.txt`.

The existing last Anthropic-versioned full document remains at
[`../docs/Anthropic_Compliance_API_v2026-04-20_rev_J.md`](../docs/Anthropic_Compliance_API_v2026-04-20_rev_J.md).

## Reproduce the raw capture

From this directory, in an environment with outbound HTTPS access:

```bash
uv run download_snapshot.py
python3 extract_compliance_docs.py
```

The script creates:

```text
raw/                 Exact response bodies
markdown/            Best-effort Markdown extraction for HTML pages
manifest.json        URL, status, content type, byte length, SHA-256, timestamp
extracted/pages/      One complete Markdown file per Compliance documentation URL
extracted/anthropic-compliance-api-all.md
                      Combined current Compliance documentation
extracted/manifest.json
                      Per-page hashes, source lines, endpoint inventory
extracted/source-urls.txt
                      All 60 canonical Compliance documentation URLs
```

Keep `manifest.json` with the capture. Since Anthropic does not expose a
revision identifier, the fetch timestamp plus content hash is the usable local
version.

The checked-in snapshot intentionally excludes `raw/` and `markdown/`. Those
directories contain reproducible JavaScript shells and an 86 MB site-wide
`llms-full.txt` duplicate. The pinned Compliance-only material is retained
under `extracted/`.

## Recommended source strategy

1. Treat `llms-full.txt` as the convenient raw documentation bundle; this
   capture confirms that it contains the complete Compliance pages.
2. Do not treat it as an API contract: it is site-wide and unversioned.
3. Pin every downstream task to the dated directory and SHA-256 manifest.
4. Retain the rendered API-reference pages too; they currently contain schema
   detail (especially the Activity union) that the narrative pages omit.
5. Diff future captures against this directory and the old Rev J Markdown,
   rather than assuming the public URLs are stable contracts.
