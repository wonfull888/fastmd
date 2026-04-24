# AGENTS

## Deployment Awareness

The production service automatically syncs files from GitHub.

When developing and shipping changes, always evaluate whether the updated files can affect the live service.

## Required Check Before Ship

For every change, classify it into one of these groups:

- `No runtime impact`: files like release notes, README-only changes, or local-only tooling that do not change the running server behavior.
- `Potential runtime impact`: server code, embedded web assets, install scripts used by users, deployment scripts, config files, or anything the production service reads during build or runtime.
- `Confirmed runtime impact`: API behavior changes, database/schema changes, rendered page changes, auth/token logic changes, startup/build changes, or dependency changes.

## Required Response

If a change has potential or confirmed runtime impact, do not stay silent.

You must explicitly raise:

1. What part of the live service may be affected.
2. What failure or regression risk exists.
3. What the mitigation or rollout plan is.
4. Whether restart, rebuild, migration, or smoke test is required.

## Default Mitigation Template

Use this structure when a live-impacting change is present:

1. Impact: what may change online.
2. Risk: what could break.
3. Rollout: `git pull`, rebuild, restart, verify.
4. Validation: homepage/API/smoke checks to run after deploy.

## Examples

- Changing `cmd/server/*`, `internal/*`, `assets.go`, or `web/*` can affect the live service.
- Changing `install.sh` or `install-skill.sh` can affect user installation flows.
- Changing only `README.md` or release notes usually does not affect the running service.

## Rule Summary

Because production auto-syncs from GitHub, every shipped change must be checked for live impact first. If there is impact, raise the problem and provide the solution path before or during rollout.
