---
name: fastmd
description: Publish Markdown as a fastmd.dev URL, or update an existing fastmd document while keeping its link. Use when the user asks to publish Markdown or replace a previously published document.
metadata:
  version: "0.5.0"
allowed-tools:
  - Bash
  - Read
  - Write
---

# fastmd

Publish Markdown to fastmd.dev, or update an existing document without changing its URL.

## When To Use

- The user wants a shareable link for a long Markdown report.
- The result is too large to paste directly into chat.
- The user explicitly asks to publish, upload, share, or generate a URL for Markdown content.
- The user asks to update an existing fastmd document and provides its ID or URL.

## When Not To Use

- The user only wants Markdown editing help without publishing the changes.
- The content is short enough to reply inline.
- The user wants to manage existing documents via browser. Use https://fastmd.dev/dashboard instead.

## Input Contract

- Accept Markdown content.
- If the content is empty after trimming whitespace, stop and tell the user.
- Do not publish partial or placeholder content.

## Output Contract

- Return the final short URL.
- Keep the response brief.
- Do not paste the full Markdown back to the user once publishing succeeds.

## Publishing Workflow

1. Write the Markdown content to a temporary `.md` file.
2. Run the installed `publish.sh` helper with `--file <tempfile>`.
3. Return the URL from stdout.
4. If publishing fails, report the stderr message clearly.

## Updating Workflow

1. Identify the existing document ID. For a fastmd URL, use its path without the leading `/` or trailing `.md`.
2. Write the complete replacement Markdown to a temporary `.md` file. For a partial edit, first read the current document with `Accept: text/plain` and apply the requested change to that content.
3. Run `publish.sh --update <ID> --file <tempfile>` using the helper lookup below.
4. Return the original URL printed on stdout. If updating fails, report the error; do not fall back to creating a new document.

Updates require the original publishing token. The helper never generates a new token in update mode. Replacements have no version history; the last successful save wins.

## Helper Script Lookup

Use the first existing path from this list:

- `~/.claude/skills/fastmd/scripts/publish.sh`
- `~/.config/opencode/skills/fastmd/scripts/publish.sh`
- `~/.codex/skills/fastmd/scripts/publish.sh`

## Notes

- The skill shares the token file used by fastmd CLI: `~/.config/fastmd/token`.
- Publishing generates a token if none exists. Updating requires the original token.
- This skill talks to `https://fastmd.dev` directly and does not require the CLI.
