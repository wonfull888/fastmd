# fastmd.dev v0.2.1

## Release Focus

v0.2.1 ships the first standalone `fastmd` skill.

The skill installs directly from GitHub, does not require the CLI, shares the existing local token model, and works across Claude Code, OpenCode, and Codex.

## What Changed

### 1. Standalone skill source

- Added `skills/fastmd/SKILL.md`
- Added `skills/fastmd/scripts/publish.sh`
- Skill publishes Markdown directly to `fastmd.dev` and returns a short URL

### 2. Multi-client installer

- Added `install-skill.sh`
- Installs by default to:
  - `~/.claude/skills/fastmd`
  - `~/.config/opencode/skills/fastmd`
  - `~/.codex/skills/fastmd`
- Supports client-specific installation flags

### 3. GitHub discoverability

- README now exposes skill installation near the top of the page
- README explains skill vs CLI clearly
- README links directly to skill source and installer files

## Runtime Model

- No Python dependency
- No CLI dependency
- Requires only `sh` and `curl`
- Shares token file with CLI at `~/.config/fastmd/token`

## Release Checklist

- [x] Standalone skill source added
- [x] Multi-client installer added
- [x] README install and quick-start docs added
- [x] Shell-only publish flow validated
- [ ] GitHub Release v0.2.1 published
