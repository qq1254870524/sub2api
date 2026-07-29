# Fork notes (qq1254870524/sub2api)

This fork tracks upstream [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) and keeps local production customizations:

1. **CPA / CLIProxy xai OAuth import**
   - API: `POST /api/v1/admin/accounts/import/grok-cpa`
   - UI: Admin -> Import Data accepts `xai-*.json` (`type=xai` / `auth_kind=oauth` / refresh+email)
2. **Grok OAuth 429 multi-account failover**
   - Request-local switch budget follows `gateway.max_account_switches` (**0 = full pool**, no artificial cap)
   - Skip rate-limited Grok accounts in scheduler
   - 2026-07-20: removed max=10/3 truncations that stopped after ~5-10 accounts on 800+ pools
3. **Grok2API (G2A) <-> Sub2API account pool bridge**
   - A2G import: `POST /api/v1/admin/accounts/import/a2g` + Admin menu
   - Server-side pull: `POST /api/v1/admin/accounts/fetch/g2a`
   - Export: `GET /api/v1/admin/accounts/export/g2a-sso`
   - SSO dedupe, never overwrite existing accounts
   - Peer G2A runtime failover when local pool exhausted
4. **Full-pool failover (800+ accounts)**
   - `gateway.max_account_switches=0` / `max_account_switches_gemini=0` means unlimited switches
5. **Deploy concurrency wiring**
   - H2C / gateway pool / scheduling / body size env vars injected into compose templates
   - Postgres `max_connections` from `POSTGRES_MAX_CONNECTIONS`

Current baseline: **upstream v0.1.166 / main VERSION 0.1.166** + fork customizations above.

## 2026-07-29 — 0.1.169-upstream-166-full-pool
- Merge upstream tag/release **v0.1.166** (panel API rate limit, WS multi-turn billing, model-mapping usage stats, Antigravity/Codex/Claude/Gemini/Grok fixes).
- Keep all prior fork features (CPA / A2G / fetch-g2a / export-g2a-sso / Peer G2A / full-pool failover / compose concurrency wiring).
- `backend/cmd/server/VERSION` tracks upstream **0.1.166**.
- Root `VERSION` marker: `0.1.169-upstream-166-full-pool`.
- Running production process on :8080 was **not** stopped during this source merge.

## 2026-07-26 — 0.1.168-upstream-165-full-pool
- Merge upstream tag/release **v0.1.165** (ChatGPT Live gateway, claude-opus-5, Ollama request-driven usage, session_id persist, announcement preview, email-alias registration dedup, OpenAI proxy stream circuit, pool-mode temp unschedulable model isolation, postcss security, assorted OpenAI/Grok/Gemini/frontend fixes).
- Keep all prior fork features (CPA / A2G / fetch-g2a / export-g2a-sso / Peer G2A / full-pool failover / compose concurrency wiring).
- `backend/cmd/server/VERSION` tracks upstream **0.1.165**.
- Root `VERSION` marker: `0.1.168-upstream-165-full-pool`.
- Running production process on :8080 was **not** stopped during this source merge.
