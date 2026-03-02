# Changelog

All notable changes to fastmd will be documented in this file.

---

## [v0.1.0] - 2026-03

### 🎯 Release Scope

First public MVP. Core pipeline: create → share → retrieve → delete Markdown documents via CLI or API. No registration required.

### ✅ Features

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
- `/` — Homepage with Hero, install command, scenario cards
- `/docs` — Full API & CLI reference
- `/help` — FAQ page
- `/:id` — Document render page (syntax highlighting, responsive)
- `/404` — Friendly 404 page

#### Identity
- Anonymous token: auto-generated `fmd_live_xxxx` format
- Token stored locally at `~/.config/fastmd/token`
- Token hash (SHA-256) used as ownership proof, no password needed

### ⚠️ Not Included in v0.1
- TTL / document expiry (planned for v0.2)
- Document update/edit (planned for v0.2)
- User accounts / dashboard (planned for v0.3)
- Rate limiting / abuse prevention (basic IP hash stored only)
