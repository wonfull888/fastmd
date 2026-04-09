# fastmd.dev

Minimalist CLI-first Markdown hosting for developers and AI agents.

`fastmd` lets you push Markdown from terminal to a short URL in milliseconds. The same document can be consumed by humans as HTML and by machines as raw Markdown.

```bash
cat report.md | fastmd
# -> https://fastmd.dev/x7y2
```

---

## Latest Update (v0.2.0)

- Added homepage `AI Integration` section for agent workflows
- Added `fastmd` skill usage docs and token lifecycle notes
- Prepared v0.2 release docs around the Antigravity Skill flow

---

## Install

```bash
curl -fsSL https://fastmd.dev/install.sh | sh
```

Supported platforms:
- macOS: `arm64`, `amd64`
- Linux: `arm64`, `amd64`

---

## Quick Start

### Publish

```bash
# Pipe mode
cat report.md | fastmd

# File mode
fastmd push report.md
```

### Agent skill

If you use the `fastmd` skill in an agent environment, the skill name is just `fastmd`.

Expected behavior:

- first run auto-generates a local token like `fmd_live_xxxx`
- the skill pushes the Markdown report to fastmd.dev
- the final result is one short URL instead of a huge inline report

Typical flow:

```text
agent finishes report -> fastmd skill loads token -> push succeeds -> URL returned
```

### Pull raw markdown

```bash
fastmd get x7y2
```

### Delete

```bash
fastmd delete x7y2
```

### Upgrade

```bash
fastmd upgrade
```

---

## Core Use Cases

1. **AI External Display**
Large agent output is published to a clean web view instead of flooding terminal history.

2. **Seamless Agent Handoff**
Use `/:id.md` to pass structured context between local and remote agents.

3. **Stealth Doc Sharing**
Share internal docs quickly while keeping distribution controlled.

4. **CI/CD Debug Snapshots**
Pipe failed build/test logs to a short URL for team debugging.

5. **Remote Prompt Control**
Host prompt/state markdown centrally and let remote workers pull updates.

6. **Instant Skill Report Sharing**
Pipe AI skill outputs (SEO audits, content drafts, reports) to fastmd and share a clean private link in milliseconds.

---

## REST API

### `POST /v1/push`

```bash
curl -X POST https://fastmd.dev/v1/push \
  -H "Content-Type: application/json" \
  -d '{"content":"# Hello\nWorld","token":"fmd_live_xxxx"}'
```

Response:

```json
{"id":"x7y2","url":"https://fastmd.dev/x7y2"}
```

### `GET /:id`

Human-friendly HTML view.

### `GET /:id.md`

Raw Markdown view for agents and scripts.

### `DELETE /v1/:id`

```bash
curl -X DELETE https://fastmd.dev/v1/x7y2 \
  -H "Authorization: Bearer fmd_live_xxxx"
```

### `GET /v1/version`

```bash
curl https://fastmd.dev/v1/version
```

---

## Token & Privacy Model

- First run generates token like `fmd_live_xxxx`
- Token is saved locally at `~/.config/fastmd/token`
- Server stores only token hash (`SHA-256`), never raw token
- Token ownership controls delete permission
- Detail/raw pages return no-index directives for privacy-first sharing

Skill flow follows the same model: first use bootstraps the token locally, and later pushes reuse the same token automatically.

---

## Self-Hosting

```bash
git clone https://github.com/wonfull888/fastmd.git
cd fastmd
make build-server
./dist/fastmd-server --port 8080 --db ./data/fastmd.db
```

Reverse proxy example (Caddy):

```caddy
fastmd.yourdomain.com {
    reverse_proxy localhost:8080
}
```

---

## Release

See:
- [CHANGELOG.md](./CHANGELOG.md)
- [RELEASE_v0.2.0.md](./RELEASE_v0.2.0.md)
- [RELEASE_v0.1.2.md](./RELEASE_v0.1.2.md)

---

## License

MIT
