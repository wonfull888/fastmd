# fastmd.dev v0.3.0

## Release Focus

v0.3 is the Dashboard MVP.

This release adds the first lightweight management layer for fastmd documents without introducing a full account system.

## What Changed

### 1. Document identity and listing

- New documents now use 8-character IDs.
- Added `GET /v1/docs` with Bearer token authentication.
- Response returns document list entries with `id`, `title`, `created_at`, and `url`.

### 2. Dashboard MVP

- Added `/dashboard`.
- Users can paste their `fmd_live_` token into the browser.
- Dashboard lists all documents for that token.
- Supports copying URLs and deleting documents.

### 3. Abuse protection

- Added per-IP rate limiting at 60 requests per minute.

## Compatibility

- Old 4-character document IDs remain readable.
- Old 4-character document IDs remain deletable with the correct token.
- Only newly created documents use 8-character IDs.

## Release Checklist

- [x] New documents create 8-character IDs
- [x] `/v1/docs` returns token-bound document list
- [x] Dashboard login, list, copy, delete flow works
- [x] Old 4-character documents remain compatible
- [x] IP rate limiting added
- [ ] GitHub Release v0.3.0 published
