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
