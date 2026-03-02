# fastmd v0.1.1

## Highlights

- Redesigned homepage with a clearer CLI-first and AI-native narrative
- Consolidated docs/help content into homepage sections
- Added dedicated `Why fastmd.dev` and refreshed `FAQ`
- Improved terminal demo UX and landing interactions
- Updated README and changelog to reflect current product behavior

## What Changed

### Website
- New section flow: Hero -> Use Cases -> Install/API -> Why -> FAQ
- Refined visual style and terminal simulation behavior
- Removed legacy template files:
  - `web/templates/docs.html`
  - `web/templates/help.html`

### Documentation
- Rewritten `README.md` for current usage and API flow
- Added `v0.1.1` release notes to `CHANGELOG.md`

## Upgrade

```bash
curl -fsSL https://fastmd.dev/install.sh | sh
```

Or, if already installed:

```bash
fastmd upgrade
```
