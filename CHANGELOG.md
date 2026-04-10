# Changelog

All notable changes to fastmd are documented in this file.

---

## [v0.2.1] - 2026-04-10

### Release Summary

v0.2.1 adds a standalone `fastmd` skill that can be installed directly from GitHub and used across Claude Code, OpenCode, and Codex without depending on the CLI.

### Added

- New `skills/fastmd` standalone skill source.
- New `install-skill.sh` installer for Claude Code, OpenCode, and Codex.
- README install and quick-start docs for the standalone skill.

### Changed

- Skill and CLI are now documented as separate products with a shared token file.

---

## [v0.2.0] - 2026-04-09

### Release Summary

v0.2 packages fastmd more clearly for agent workflows by formalizing the `fastmd` skill release and tightening product messaging around AI integration.

### Added

- New homepage `AI Integration` section focused on agent-side publishing flow.
- README documentation for the `fastmd` skill flow and token bootstrap behavior.
- New release notes document: `RELEASE_v0.2.0.md`.

### Changed

- Homepage navigation now links directly to the AI integration section.
- Product docs now describe the `fastmd` skill as the canonical agent integration name.

---

## [v0.1.2] - 2026-03-03

### Release Summary

v0.1.2 focuses on SEO page-element hardening and homepage information architecture polish.

### Added

- Homepage JSON-LD enhancements aligned with software application positioning.
- New homepage use case: **Instant Skill Report Sharing**.
- New release notes document: `RELEASE_v0.1.2.md`.

### Changed

- Homepage metadata rewritten for SEO targeting:
  - Title tag
  - Meta description
  - Open Graph title/description
  - Twitter title/description
- Added canonical/robots-aware head rendering support in shared template.
- Replaced OG preview asset with minimalist terminal-style social image.
- Refined `Use Cases` section to a symmetric card layout with consistent visual rhythm.
- Expanded `Why fastmd.dev` section with explicit value pillars.
- Updated README with v0.1.2 highlights and use-case coverage.

### Security / Indexation

- Enforced `X-Robots-Tag: noindex, nofollow, noarchive` for detail/raw/404 responses.
- Kept homepage indexable while preserving privacy posture for shared document pages.

---

## [v0.1.1] - 2026-03-02

### Release Summary

v0.1.1 focuses on homepage redesign, information architecture cleanup, and documentation refresh.

### Added

- New homepage narrative sections:
  - Hero
  - Use Cases
  - Install/API
  - Why fastmd.dev
  - FAQ
- New terminal demo behavior and interaction refinements on landing page.
- Updated release documentation for current product positioning.

### Changed

- Homepage fully redesigned to match the current brand and CLI-first messaging.
- Homepage copy and section structure updated for AI + developer workflows.
- `/docs` and `/help` standalone page approach removed from template layer; content consolidated into homepage.
- Navigation/footer links aligned to homepage section anchors.
- Document page action styling and frontend interaction behavior improved.
- README rewritten to reflect current product behavior, use cases, and API usage.

### Removed

- Legacy `docs.html` and `help.html` templates.

---

## [v0.1.0] - 2026-03

### Release Scope

First public MVP. Core pipeline: create -> share -> retrieve -> delete Markdown documents via CLI or API. No registration required.

### Features

#### Backend API
- `POST /v1/push` — Create a document, returns short ID (e.g. `x7y2`)
- `GET /:id` — Human mode: returns rendered HTML page
- `GET /:id.md` — Machine mode: returns raw Markdown
- `DELETE /v1/:id` — Delete document (requires Token)
- `GET /v1/version` — Returns latest CLI version info

#### CLI
- `cat file.md | fastmd` — Pipe push
- `fastmd push <file>` — Push local file
- `fastmd get <ID>` — Pull document to local, auto-named by H1 heading
- `fastmd delete <ID>` — Delete document
- `fastmd upgrade` — Self-upgrade via install script

#### Website
- `/` — Homepage
- `/:id` — Document render page
- `/404` — Friendly 404 page

#### Identity
- Anonymous token: auto-generated `fmd_live_xxxx` format
- Token stored locally at `~/.config/fastmd/token`
- Token hash (`SHA-256`) used as ownership proof

### Not Included in v0.1
- TTL / document expiry (planned for v0.2)
- Document update/edit (planned for v0.2)
- User accounts / dashboard (planned for v0.3)
- Rate limiting / abuse prevention (basic IP hash stored only)
