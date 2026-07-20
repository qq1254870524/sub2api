# Stable version

- Tag intent: `0.1.165-upstream-162-full-pool`
- Upstream baseline: `v0.1.162`
- Date: 2026-07-20
- Keep: CPA import, A2G import/fetch/peer, full-pool failover (max_account_switches=0)

## Deploy verified (2026-07-20)
- Image: local/sub2api:cpa-import (pull_policy: never)
- Runtime: GATEWAY_MAX_ACCOUNT_SWITCHES=0, GATEWAY_MAX_ACCOUNT_SWITCHES_GEMINI=0
- Health: http://127.0.0.1:8080/health => ok
- Binary markers: import/grok-cpa, import/a2g, fetch/g2a, export/g2a-sso, PeerG2A, AccountSwitchesExhausted
- Binary reports: Sub2API 0.1.162 (upstream baseline) / VERSION file 0.1.165-upstream-162-full-pool
- Perf env wired live: H2C streams=128, gateway max_conns_per_host=2048, max_idle=8192/4096, postgres max_connections=1024
