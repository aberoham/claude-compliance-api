#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.13"
# dependencies = [
#     "google-genai>=1.0",
#     "pypdf>=4.0",
#     "python-dotenv>=1.0",
# ]
# ///
"""Convert a large PDF to well-formed Markdown by chunking and dispatching to Gemini.

The Anthropic Compliance API reference PDF is ~54 pages and exceeds Gemini's
per-response output token budget when extracted in a single call. This script
splits the PDF into fixed-size page chunks, sends each chunk to Gemini with a
context-aware prompt (first chunk vs. continuation, including the tail of the
previous chunk's output so tables and lists continue cleanly), and stitches
the resulting markdown into a single coherent file.

Usage:
    uv run pdf_to_markdown.py path/to/document.pdf
    uv run pdf_to_markdown.py path/to/document.pdf -o out.md --pages-per-chunk 8
"""

from __future__ import annotations

import argparse
import io
import os
import sys
import time
from pathlib import Path

from dotenv import load_dotenv
from google import genai
from google.genai import types
from pypdf import PdfReader, PdfWriter

DEFAULT_MODEL = "gemini-3.1-pro-preview"
DEFAULT_PAGES_PER_CHUNK = 10
DEFAULT_TAIL_CONTEXT_CHARS = 1200
MAX_RETRIES = 3

FIRST_CHUNK_PROMPT = """\
Extract all detail from the attached PDF and output well-formed GitHub-flavored Markdown.

This is the FIRST chunk of a larger document: pages {start}-{end} of {total}.

Formatting rules:
- Begin directly with the markdown content. Do NOT add preamble like "Here is the extracted content".
- Start with the document title as a top-level `#` heading and reproduce any Table of Contents as a bulleted list.
- Remove page headers, page numbers, running footers, and watermarks (e.g. "CONFIDENTIAL. DO NOT DISTRIBUTE.", "ANTHROP\\C").
- Render all tables as GitHub-flavored Markdown tables with a header row and separator.
- Preserve code blocks verbatim inside fenced code blocks. Tag the language (```json, ```bash, ```python, ```http) when identifiable.
- Preserve inline `code` spans for identifiers, field names, endpoints, and HTTP verbs.
- Render diagrams as Mermaid if possible; otherwise describe them as a fenced text block an LLM can parse.
- Do NOT add closing remarks or "end of document" markers — more chunks will follow.
- If a section (table, list, code block) appears to be cut off at the end of page {end}, leave it open. The next chunk will continue it.
"""

CONTINUATION_PROMPT = """\
Extract all detail from the attached PDF pages and output well-formed GitHub-flavored Markdown.

This is a CONTINUATION chunk: pages {start}-{end} of {total}. Earlier pages have already been extracted.

Formatting rules:
- Begin directly with the content that appears on page {start}. Do NOT add any preamble, transition phrase ("Continuing from..."), or repeat of the document title or Table of Contents.
- Do NOT repeat section headings that were already emitted in the previous chunk. Only emit a heading if it actually starts on one of these pages.
- Remove page headers, page numbers, running footers, and watermarks (e.g. "CONFIDENTIAL. DO NOT DISTRIBUTE.", "ANTHROP\\C").
- Render tables as GitHub-flavored Markdown tables. If a table was in progress at the end of the previous chunk, continue emitting rows without re-emitting the header — unless the PDF itself repeats the header on page {start}, in which case drop it.
- Preserve code blocks verbatim inside fenced code blocks with language tags when identifiable.
- Render diagrams as Mermaid where possible; otherwise as a fenced text block.
- Do NOT add closing remarks unless this is the final chunk of the document.

For continuity, the tail of the previously emitted markdown was:

<previous_chunk_tail>
{tail}
</previous_chunk_tail>

Continue the document naturally from there.
"""


def split_pdf(pdf_path: Path, pages_per_chunk: int) -> list[tuple[int, int, bytes]]:
    """Split a PDF into chunks of at most `pages_per_chunk` pages.

    Returns a list of `(start_page, end_page, pdf_bytes)` tuples with 1-based,
    inclusive page numbers.
    """
    reader = PdfReader(str(pdf_path))
    total = len(reader.pages)
    chunks: list[tuple[int, int, bytes]] = []
    for zero_based_start in range(0, total, pages_per_chunk):
        zero_based_end = min(zero_based_start + pages_per_chunk, total)
        writer = PdfWriter()
        for i in range(zero_based_start, zero_based_end):
            writer.add_page(reader.pages[i])
        buf = io.BytesIO()
        writer.write(buf)
        chunks.append((zero_based_start + 1, zero_based_end, buf.getvalue()))
    return chunks


def extract_chunk(
    client: genai.Client,
    model: str,
    pdf_bytes: bytes,
    prompt: str,
) -> str:
    """Send a chunk to Gemini and return the extracted markdown text.

    Retries on transient errors with exponential backoff. Raises if the model
    stops due to hitting the output token limit, since that indicates the
    caller should reduce `--pages-per-chunk`.
    """
    last_err: Exception | None = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = client.models.generate_content(
                model=model,
                contents=[
                    types.Part.from_bytes(data=pdf_bytes, mime_type="application/pdf"),
                    prompt,
                ],
                config=types.GenerateContentConfig(
                    temperature=0.3,
                    thinking_config=types.ThinkingConfig(thinking_level="HIGH"),
                ),
            )
            finish_reason = ""
            if response.candidates:
                raw = response.candidates[0].finish_reason
                finish_reason = raw.name if hasattr(raw, "name") else str(raw or "")
            if finish_reason == "MAX_TOKENS":
                raise RuntimeError(
                    "Gemini stopped at MAX_TOKENS — reduce --pages-per-chunk and rerun."
                )
            text = response.text
            if not text:
                raise RuntimeError(
                    f"Gemini returned empty text (finish_reason={finish_reason or 'unknown'})."
                )
            return text
        except RuntimeError:
            raise
        except Exception as exc:
            last_err = exc
            wait = 2**attempt
            print(
                f"  attempt {attempt}/{MAX_RETRIES} failed: {exc}; retrying in {wait}s",
                file=sys.stderr,
            )
            time.sleep(wait)
    raise RuntimeError(f"chunk extraction failed after {MAX_RETRIES} attempts: {last_err}")


def strip_fences(markdown: str) -> str:
    """Strip a wrapping ```markdown ... ``` fence if the model emitted one."""
    stripped = markdown.strip()
    if stripped.startswith("```"):
        first_newline = stripped.find("\n")
        if first_newline != -1 and stripped.endswith("```"):
            return stripped[first_newline + 1 : -3].strip()
    return stripped


def tail_context(text: str, max_chars: int) -> str:
    """Return the last `max_chars` characters of `text`, trimmed to a line boundary."""
    if len(text) <= max_chars:
        return text
    snippet = text[-max_chars:]
    first_newline = snippet.find("\n")
    if first_newline != -1 and first_newline < max_chars // 4:
        snippet = snippet[first_newline + 1 :]
    return snippet


def resolve_api_key(explicit: str | None, env_file: Path) -> str | None:
    if explicit:
        return explicit
    if env_file.exists():
        load_dotenv(env_file)
    return (
        os.environ.get("GEMINI_API_KEY")
        or os.environ.get("GEMINI_API")
        or os.environ.get("GOOGLE_API_KEY")
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Convert a PDF to Markdown by chunking through Gemini.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("pdf", type=Path, help="Input PDF file")
    parser.add_argument(
        "-o", "--output", type=Path, help="Output markdown file (default: <pdf>.md)"
    )
    parser.add_argument(
        "--pages-per-chunk",
        type=int,
        default=DEFAULT_PAGES_PER_CHUNK,
        help=f"Pages per chunk (default: {DEFAULT_PAGES_PER_CHUNK}). "
        "Reduce if Gemini hits MAX_TOKENS.",
    )
    parser.add_argument(
        "--model",
        default=DEFAULT_MODEL,
        help=f"Gemini model (default: {DEFAULT_MODEL})",
    )
    parser.add_argument(
        "--env",
        type=Path,
        default=Path(__file__).parent / ".env",
        help="Path to .env file (default: alongside this script)",
    )
    parser.add_argument("--api-key", help="Gemini API key (overrides env)")
    parser.add_argument(
        "--keep-chunks",
        action="store_true",
        help="Also write each chunk's raw output as <output>.chunkNN.md for debugging",
    )
    parser.add_argument(
        "--tail-context",
        type=int,
        default=DEFAULT_TAIL_CONTEXT_CHARS,
        help=f"Characters of prior output fed to the next chunk (default: {DEFAULT_TAIL_CONTEXT_CHARS})",
    )
    args = parser.parse_args()

    if not args.pdf.exists():
        print(f"error: PDF not found: {args.pdf}", file=sys.stderr)
        return 1

    api_key = resolve_api_key(args.api_key, args.env)
    if not api_key:
        print(
            "error: no Gemini API key found. Set GEMINI_API_KEY / GEMINI_API in "
            f"{args.env} or pass --api-key.",
            file=sys.stderr,
        )
        return 1

    output_path = args.output or args.pdf.with_suffix(".md")

    print(f"splitting {args.pdf.name} into chunks of {args.pages_per_chunk} pages...")
    chunks = split_pdf(args.pdf, args.pages_per_chunk)
    total_pages = chunks[-1][1]
    print(f"  {len(chunks)} chunks covering {total_pages} pages")

    client = genai.Client(api_key=api_key)

    parts: list[str] = []
    prior_tail = ""
    for idx, (start, end, pdf_bytes) in enumerate(chunks, start=1):
        is_first = idx == 1
        template = FIRST_CHUNK_PROMPT if is_first else CONTINUATION_PROMPT
        prompt = template.format(
            start=start, end=end, total=total_pages, tail=prior_tail
        )
        print(f"[{idx}/{len(chunks)}] extracting pages {start}-{end}...", flush=True)
        raw = extract_chunk(client, args.model, pdf_bytes, prompt)
        markdown = strip_fences(raw)
        parts.append(markdown)
        prior_tail = tail_context(markdown, args.tail_context)
        if args.keep_chunks:
            chunk_path = output_path.with_name(
                f"{output_path.stem}.chunk{idx:02d}.md"
            )
            chunk_path.write_text(raw)
            print(f"  saved raw chunk to {chunk_path.name}")

    stitched = "\n\n".join(p.strip() for p in parts) + "\n"
    output_path.write_text(stitched)
    print(f"done: {output_path} ({len(stitched):,} bytes, {len(parts)} chunks)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
