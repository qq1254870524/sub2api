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

Current baseline: **upstream v0.1.162** + fork customizations (CPA + A2G + peer G2A + full-pool failover).

## 2026-07-20 — v0.1.165-upstream-162-full-pool
- Merge upstream tag `v0.1.162` (perf responses/SSE, Grok/OpenAI/sticky/quota/WS/security/frontend fixes).
- Keep all prior fork features listed above.
- Default full-pool account switches (0 = unlimited) for large account pools.
- Concurrency defaults with upstream: DB 256/128, Redis pool 1024+, gateway max_idle_conns 2560, WS pool/ttft weights.

