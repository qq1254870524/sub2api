# Fork notes (qq1254870524/sub2api)

## 2026-07-19 — v0.1.160-fork1
- Base: upstream tag `v0.1.160`.
- Keep: Grok CPA import API + ImportDataModal CPA/xai JSON + i18n.
- 429: `grokOAuth429MaxAccountSwitches=10`, status_429 auto-pause in scheduler path, grok fallback cooldown 10m.

# sub2api (qq1254870524 fork)

Based on upstream [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api).

## Local additions

### Grok CPA / CLIProxy OAuth JSON import

- Handler: `backend/internal/handler/admin/account_grok_cpa_import.go`
- Route: `POST /api/v1/admin/accounts/import/grok-cpa`
- Accepts Desktop/Grok/cpa style `xai-*.json` payloads (`type=xai`, `auth_kind=oauth`) and creates/updates Sub2API Grok OAuth accounts.

### Build / deploy notes

Official `weishaw/sub2api:latest` image does **not** include this route.
Rebuild backend and replace the running container/image for the native endpoint to take effect.

Client-side import via `POST /api/v1/admin/accounts` (used by grok-regkit `sub2api_client.py`) still works against stock Sub2API.

### Companion projects

- https://github.com/qq1254870524/grok-regkit
- https://github.com/qq1254870524/grok-regkit-services

Do not commit real admin passwords, OAuth tokens, or production `config.yaml` secrets.

## Web UI: Import Data accepts CPA xai JSON

`frontend/src/components/admin/account/ImportDataModal.vue` now auto-detects CLIProxy/CPA `xai-*.json` (`type=xai`, `auth_kind=oauth`) and converts them into Sub2API backup payload (`type=sub2api-data`) before calling `POST /api/v1/admin/accounts/data`.

This removes the false error when users upload CPA OAuth files into the admin Import Data dialog.

Still true:
- stock `weishaw/sub2api:latest` image does not include this UI patch until you rebuild frontend or replace image.
- without rebuild, use grok-regkit client import: `python -B scripts/import_cpa_to_sub2api.py --dir <cpa_dir>`
