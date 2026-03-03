# fastmd v0.1.2

## Highlights

- SEO page elements upgraded across homepage metadata and social cards
- Homepage UX polished with a cleaner, symmetric Use Cases layout
- Added new scenario: **Instant Skill Report Sharing**
- Privacy/indexation behavior reinforced for document pages

## What Changed

### SEO
- Updated homepage title and meta description for CLI-first + AI-agent positioning
- Added complete Open Graph and Twitter card fields (`og:url`, `og:image`, title/description)
- Added homepage JSON-LD for software application context
- Replaced OG image with minimalist terminal-style visual (`cat report.md | fastmd`)

### Homepage
- Reworked `Use Cases` to a balanced 2x3 card grid on desktop
- Added **Instant Skill Report Sharing** use case with CLI hint
- Updated `Why fastmd.dev` section value pillars for clearer scanability

### Privacy & Crawling
- Kept homepage indexable
- Enforced `X-Robots-Tag: noindex, nofollow, noarchive` on detail/raw/404 responses

### Documentation
- Updated `README.md` and `CHANGELOG.md` for v0.1.2

## Upgrade

```bash
curl -fsSL https://fastmd.dev/install.sh | sh
```

Or, if already installed:

```bash
fastmd upgrade
```
