# Browser-harness and page-source findings

Observed on 2026-07-15.

## Chrome/CDP status

Chrome remote debugging was available at `127.0.0.1:9222` for the matching
profile. A retry with the local `~/Desktop/work/claude/browser-harness`
successfully attached and inspected the rendered documentation.

The rendered Compliance navigation and network activity did not reveal a
Compliance-specific OpenAPI, Swagger, or JSON Schema download. The generic
"Include beta APIs" UI control was enabled, but it exposed no additional
Compliance paths beyond the 31 operations in the extracted corpus.

## What the downloaded page source shows

The downloaded API-reference HTML is a Next.js response rather than a small
empty bootstrap document. For example, the Activity List response is 639,661
bytes and contains 61 inline `self.__next_f` records plus hashed
`/_next/static/` JavaScript and CSS assets.

Across all 27 captured Compliance HTML responses, literal searches found no
references to:

- `openapi`
- `swagger`
- `json-schema`
- `stainless`
- `speakeasy`
- `mintlify`

This does not prove that Anthropic lacks an internal schema. It does establish
that the public page responses neither name nor link a conventional raw schema
artifact under those common identifiers.

The three direct public schema probes captured by `download_snapshot.py` all
returned HTTP 404:

```text
https://platform.claude.com/openapi.json
https://platform.claude.com/docs/openapi.json
https://api.anthropic.com/openapi.json
```

## Practical conclusion

No public, versioned Compliance OpenAPI/Swagger artifact was found. The best
downloadable raw source currently exposed by the documentation site is
`https://platform.claude.com/llms-full.txt`. It is unversioned and site-wide,
but it contains the complete rendered Markdown for all 60 current Compliance
pages in this capture.

For later work, pin the source by snapshot date and SHA-256 rather than treating
the live URL as a version. See `extracted/manifest.json` for the source,
combined-document, per-page, and operation inventories.
