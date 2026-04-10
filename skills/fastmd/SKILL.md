---
name: fastmd
version: "0.2.1"
description: Publish long Markdown content as a fastmd.dev URL. Use when the user wants a shareable link instead of a large inline Markdown response.
---

# fastmd

Publish Markdown to fastmd.dev and return a short URL.

## When To Use

- The user wants a shareable link for a long Markdown report.
- The result is too large to paste directly into chat.
- The user explicitly asks to publish, upload, share, or generate a URL for Markdown content.

## When Not To Use

- The user only wants Markdown editing help.
- The content is short enough to reply inline.
- The user wants to manage an existing document. This version does not support delete, get, or update.

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

## Helper Script Lookup

Use the first existing path from this list:

- `~/.claude/skills/fastmd/scripts/publish.sh`
- `~/.config/opencode/skills/fastmd/scripts/publish.sh`
- `~/.codex/skills/fastmd/scripts/publish.sh`

## Notes

- The skill shares the token file used by fastmd CLI: `~/.config/fastmd/token`.
- If no token exists, the helper generates one automatically.
- This skill talks to `https://fastmd.dev` directly and does not require the CLI.
