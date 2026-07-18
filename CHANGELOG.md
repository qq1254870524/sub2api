# Changelog (fork qq1254870524/sub2api)

## stable-2026-07-19-sub2api-v0.1.161-fork1 (2026-07-19)

### Base
- Merged upstream Wei-Shaw/sub2api **v0.1.161** tree (security switches default-off, Grok protected video, model-scoped temp cooldown, OpenAI WS lifecycle, Docker cross-compile, etc.).
- `backend/cmd/server/VERSION` set to **0.1.161**.

### Preserved fork customizations
- Native Grok CPA OAuth JSON import: `POST /api/v1/admin/accounts/import/grok-cpa` + `account_grok_cpa_import.go`.
- Admin Import Data UI accepts CLIProxy/CPA `xai-*.json` (type=xai / auth_kind=oauth / refresh+email).
- i18n EN/ZH import hints for CPA/xai JSON.
- `FORK_NOTES.md` retained.

### 429 / exceeded-retry hardening
- Grok OAuth request-local 429 failover budget **10** account switches (`grokOAuth429MaxAccountSwitches`), aligned with default `gateway.max_account_switches`.
- Avoid early stop after only one follow-up account (upstream bound caused pools to exhaust on brief 429 storms → client `exceeded retry limit, last status: 429`).
- Scheduler skips Grok accounts auto-paused by quota / fresh **status_429** snapshots so healthy accounts are selected sooner.
- Grok rate-limit fallback cooldown extended where applicable (**2m → 10m**) to reduce thrashing hot accounts.
- Exhausted upstream 429 client message clarifies *after account failover*.
- Unit/integration tests for multi-account 429 failover retained/updated.

### Notes
- Does **not** overwrite previous GitHub releases/packages (e.g. `stable-2026-07-19-sub2api-v0.1.160-fork1`).
- Deploy: rebuild/restart Sub2API (port 8080) from this tree after pull.

## stable-2026-07-19-sub2api-v0.1.160-fork1 (2026-07-19)

### Base
- Merged upstream Wei-Shaw/sub2api **v0.1.160**.
- Preserved CPA import + Grok 429 multi-failover (budget 10).
