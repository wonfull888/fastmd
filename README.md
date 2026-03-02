# fastmd

**A Markdown fast-transfer pipeline for AI Agents and developers.**

Push Markdown from your terminal or agent. Get a shareable link in milliseconds. No sign-up required.

```bash
cat report.md | fastmd
# → https://fastmd.dev/x7y2
```

---

## Why fastmd?

When an AI Agent finishes a task, where does the output go? Pasting it into a chat context bloats the conversation. Uploading to a cloud drive requires authentication. fastmd is the missing piece: a single `curl` call to publish structured Markdown, and a short URL to share it anywhere.

**The same URL works for both humans and machines:**
- `fastmd.dev/x7y2` → Beautiful HTML page (for humans)
- `fastmd.dev/x7y2.md` → Raw Markdown (for agents and scripts)

---

## Install

```bash
curl -fsSL https://fastmd.dev/install.sh | sh
```

Supports macOS (arm64, amd64) and Linux (arm64, amd64).

---

## Quick Start

### Push

```bash
# From a file
fastmd push report.md

# From a pipe
cat report.md | fastmd
echo "# Quick note" | fastmd

# Output: ✓ Published → https://fastmd.dev/x7y2
```

### Get (pull back to local)

```bash
fastmd get x7y2
# Saves to: report.md (extracted from H1 heading)
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

## REST API

Built for AI Agent integration. No authentication setup — just include your token in the request body.

### POST /v1/push — Create document

```bash
curl -X POST https://fastmd.dev/v1/push \
  -H "Content-Type: application/json" \
  -d '{
    "content": "# Agent Report\n\nTask completed successfully.",
    "token": "fmd_live_xxxx"
  }'

# Response
{"id":"x7y2","url":"https://fastmd.dev/x7y2"}
```

### GET /:id — View document (HTML)

```bash
curl https://fastmd.dev/x7y2
```

### GET /:id.md — View document (Raw Markdown)

```bash
curl https://fastmd.dev/x7y2.md
```

Also works with `Accept: text/plain` header.

### DELETE /v1/:id — Delete document

```bash
curl -X DELETE https://fastmd.dev/v1/x7y2 \
  -H "Authorization: Bearer fmd_live_xxxx"

# Response
{"ok":true}
```

### GET /v1/version — CLI version info

```bash
curl https://fastmd.dev/v1/version
```

---

## How Tokens Work

On first run, the CLI auto-generates a token (`fmd_live_xxxx`) and saves it to `~/.config/fastmd/token`. No registration, no email — just use it.

The token is **hashed (SHA-256) before storage**. We never store the raw token. It acts as your document ownership proof: only you can delete your documents.

> **Lost your token?** Generate a new one by deleting `~/.config/fastmd/token`. Previously created documents can no longer be deleted, but remain accessible.

---

## AI Agent Integration

### Example: LangChain / Python

```python
import requests

def publish_report(markdown: str, token: str) -> str:
    resp = requests.post("https://fastmd.dev/v1/push", json={
        "content": markdown,
        "token": token,
    })
    return resp.json()["url"]

url = publish_report("# Task Complete\n\nAll steps finished.", "fmd_live_xxxx")
print(f"Report: {url}")
```

### Example: curl in shell scripts

```bash
REPORT=$(cat <<'EOF'
# Daily Summary

- Tasks completed: 12
- Errors: 0
EOF
)

URL=$(curl -s -X POST https://fastmd.dev/v1/push \
  -H "Content-Type: application/json" \
  -d "{\"content\": $(echo "$REPORT" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))'), \"token\": \"fmd_live_xxxx\"}" \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["url"])')

echo "Published: $URL"
```

---

## Self-Hosting

fastmd is open source. Run your own instance in minutes.

### Requirements
- Go 1.22+
- Any Linux VPS

### Build & Run

```bash
git clone https://github.com/wonfull888/fastmd.git
cd fastmd

# Build server
make build-server

# Run
./dist/fastmd-server --port 8080 --db ./data/fastmd.db
```

### With Caddy (recommended)

```
fastmd.yourdomain.com {
    reverse_proxy localhost:8080
}
```

### With Nginx

```nginx
server {
    listen 443 ssl;
    server_name fastmd.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Point CLI to your instance

```bash
# Set env var (or edit ~/.config/fastmd/config)
FASTMD_HOST=https://fastmd.yourdomain.com fastmd push file.md
```

> Note: `FASTMD_HOST` env var support is on the v0.2 roadmap. For now, rebuild the CLI with a custom `baseURL`.

---

## Roadmap

- [x] v0.1 — Core pipeline: push, view, delete, raw API
- [ ] v0.2 — TTL options (24h / 7d / 30d), document update, `FASTMD_HOST` env var
- [ ] v0.3 — Named tokens, document list, webhook notifications

---

## License

MIT © 2026 [wonfull888](https://github.com/wonfull888)
