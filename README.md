# fastmd.dev

Minimalist CLI-first Markdown hosting for developers and AI agents.

`fastmd` lets you push Markdown from terminal to a short URL in milliseconds. The same document can be consumed by humans as HTML and by machines as raw Markdown.

```bash
fastmd login
cat report.md | fastmd
# -> https://fastmd.dev/x7y2
```

---

## Latest Update (v0.6.0)

- GitHub login and one automatically assigned, revealable API token
- CLI browser authorization and remote MCP with six document tools
- Stable document updates, idempotent creation and legacy ownership migration

---

## Install

### Install fastmd Skill

No Python or CLI dependency required. The skill only needs `sh` and `curl`.

```bash
curl -fsSL https://raw.githubusercontent.com/wonfull888/fastmd/main/install-skill.sh | sh
```

Targets installed by default:
- Claude Code: `~/.claude/skills/fastmd`
- OpenCode: `~/.config/opencode/skills/fastmd`
- Codex: `~/.codex/skills/fastmd`

Install only one client:

```bash
curl -fsSL https://raw.githubusercontent.com/wonfull888/fastmd/main/install-skill.sh | sh -s -- --claude
curl -fsSL https://raw.githubusercontent.com/wonfull888/fastmd/main/install-skill.sh | sh -s -- --opencode
curl -fsSL https://raw.githubusercontent.com/wonfull888/fastmd/main/install-skill.sh | sh -s -- --codex
```

### Install fastmd CLI

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

- sign in first with `fastmd login`, or set `FASTMD_API_TOKEN` from your account dashboard
- the skill pushes the Markdown report to fastmd.dev directly, without requiring CLI installation
- the final result is one short URL instead of a huge inline report

Typical flow:

```text
agent finishes report -> fastmd skill loads token -> push succeeds -> URL returned
```

### Dashboard

Open `https://fastmd.dev/dashboard` and sign in with GitHub. Your account receives one permanent API token automatically; it is masked by default, can be revealed again and cannot be reset.

- list published docs
- copy document URLs
- delete documents from the browser

### Pull raw markdown

```bash
fastmd get x7y2
```

### Update an existing document

```bash
fastmd update x7y2 report.md
```

Replaces the complete Markdown while keeping the document ID, URL and creation time.
Sign in to the account that owns the document. Existing local legacy-token documents are claimed automatically by `fastmd login`; the original token file is preserved.
Empty content and unauthorized updates are rejected. There is no version history;
the last successful update wins.

The agent skill supports the same operation without installing the CLI:

```bash
sh scripts/publish.sh --update x7y2 --file report.md
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
  -H "Authorization: Bearer $FASTMD_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"# Hello\nWorld"}'
```

Response:

```json
{"id":"Ab3kL9xQ","url":"https://fastmd.dev/Ab3kL9xQ"}
```

### `GET /v1/docs`

```bash
curl https://fastmd.dev/v1/docs \
  -H "Authorization: Bearer fmd_live_xxxx"
```

Response:

```json
{
  "documents": [
    {
      "id": "Ab3kL9xQ",
      "title": "Weekly Report",
      "created_at": 1710000000,
      "url": "https://fastmd.dev/Ab3kL9xQ"
    }
  ]
}
```

### `GET /:id`

Human-friendly HTML view.

### `PUT /v1/:id`

Replace a document owned by your account:

```bash
curl -X PUT https://fastmd.dev/v1/Ab3kL9xQ \
  -H "Authorization: Bearer fmd_live_xxxx" \
  -H "Content-Type: application/json" \
  -d '{"content":"# Updated\nNew content"}'
```

Returns `{"id":"Ab3kL9xQ","url":"https://fastmd.dev/Ab3kL9xQ.md"}`.
Missing or malformed authorization returns 401; empty or invalid content returns 400;
missing, expired or unauthorized documents return 403 without changing or creating a document.

### `GET /:id.md`

Raw Markdown view for agents and scripts.

### `DELETE /v1/:id`

```bash
curl -X DELETE https://fastmd.dev/v1/Ab3kL9xQ \
  -H "Authorization: Bearer fmd_live_xxxx"
```

### `GET /v1/version`

```bash
curl https://fastmd.dev/v1/version
```

---

## Token & Privacy Model

- GitHub sign-in creates your account and one fixed API token.
- The dashboard masks the token by default; Show token reveals it again. No manual creation or reset is provided.
- The backend keeps a token hash for authentication and an encrypted copy for account-only viewing.
- CLI credentials are stored per server at `~/.config/fastmd/credentials.json`; the skill can reuse them with Python 3, or read `FASTMD_API_TOKEN` directly.
- Document ownership belongs to the account, independent of CLI or MCP authorization credentials.
- Public reading needs no login. Document pages include no-index directives; anyone with a link can read them.

---

## Skill Source

- Skill source: [`skills/fastmd`](./skills/fastmd)
- Installer: [`install-skill.sh`](./install-skill.sh)
- Main skill entry: [`skills/fastmd/SKILL.md`](./skills/fastmd/SKILL.md)
- Runtime helper: [`skills/fastmd/scripts/publish.sh`](./skills/fastmd/scripts/publish.sh)

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
- [RELEASE_v0.2.1.md](./RELEASE_v0.2.1.md)
- [RELEASE_v0.2.0.md](./RELEASE_v0.2.0.md)
- [RELEASE_v0.1.2.md](./RELEASE_v0.1.2.md)

---

## License

MIT

## GitHub account configuration

The server reads `.env` from its working directory. Existing process environment variables take priority. Never commit this file.

```dotenv
FASTMD_PUBLIC_URL=https://fastmd.dev
GITHUB_CLIENT_ID=your-client-id
GITHUB_CLIENT_SECRET=your-client-secret
GITHUB_REDIRECT_URI=https://fastmd.dev/auth/github/callback
```

The GitHub OAuth app redirect URI must match exactly. No email service or repository permissions are required. GitHub credentials are used only to obtain a public identity; provider access and refresh tokens are not persisted.

The server encrypts the fixed API token using a separate key at `<database-path>.key`, created automatically with owner-only permissions. Back up **both** the SQLite database and this key. Losing the key prevents token recovery; changing GitHub credentials does not change API tokens.

## CLI authentication

```sh
fastmd login                 # GitHub sign-in, browser authorization for this CLI
fastmd login --no-browser    # print a URL to open on another device
fastmd login --token-stdin   # read the account API token securely from stdin
fastmd whoami
fastmd list --limit 50
fastmd usage
fastmd logout
```

CLI login credentials are stored per server in `~/.config/fastmd/credentials.json` with mode 0600. Noninteractive agents may instead use `FASTMD_API_TOKEN`. `--server <origin>` selects a different server. `--json` provides machine-readable output. `--request-id <stable-id>` makes a publish retry safe: reusing it with different content returns a conflict, and deleting its document does not allow a retry to recreate it.

For manual legacy migration, use `fastmd claim --token-file ~/.config/fastmd/token` from a CLI browser login. Original document IDs, links and creation dates are preserved. Old anonymous credentials cannot create new documents.

## Remote MCP

Use a single Streamable HTTP endpoint:

```text
https://fastmd.dev/mcp
```

Clients can connect through OAuth discovery and PKCE, or send `Authorization: Bearer <your-api-token>`. The six tools are `create_document`, `update_document`, `get_document`, `list_documents`, `delete_document`, and `get_usage`. OAuth connections have separate short-lived access tokens and rotating refresh tokens; disconnect them in the dashboard without changing the fixed API token. This project does not provide a local stdio or legacy SSE server.

For browser-based MCP clients with a different origin, set `FASTMD_MCP_ORIGINS` to a space-separated allowlist. CLI clients do not need this setting. Document read links remain public; mutation and account listings require account ownership.
