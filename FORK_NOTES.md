# Fork Notes (qq1254870524/sub2api)

## Current
- Upstream baseline: Wei-Shaw/sub2api **v0.1.168**
- Fork VERSION: **0.1.170-upstream-168-full-pool**
- server VERSION: **0.1.168**

## Kept local customizations
1. CPA import: `POST /api/v1/admin/accounts/import/grok-cpa`
2. A2G import/fetch/export: `import/a2g`, `fetch/g2a`, `export/g2a-sso`
3. Peer G2A failover on pool exhausted (`peer_g2a.go` + chat completions hooks)
4. Full-pool account switch: `gateway.max_account_switches=0` / gemini=0 and `AccountSwitchesExhausted` unlimited semantics
5. Public GHCR publish workflow for this fork
6. A2GImportModal + accountUsageCache frontend helpers

## Upstream absorbed (v0.1.166 -> v0.1.168)
- Passkey / WebAuthn
- Model plaza
- Kimi K3
- SKIP_SETUP / setup bypass
- Codex/Live/store resilience and related security/cache fixes
