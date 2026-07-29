# sub2api stable-2026-07-26-upstream-165-full-pool

## Baseline
- Upstream: Wei-Shaw/sub2api v0.1.165 (server VERSION 0.1.165)
- Fork VERSION: 0.1.168-upstream-165-full-pool
- Commit: 223c141a3

## Upstream highlights
- ChatGPT Live gateway
- Anthropic claude-opus-5
- Ollama Cloud request-driven usage
- Persist usage session_id
- Announcement preview and mobile affiliate copy
- Registration email-alias dedup
- OpenAI proxy stream circuit breaker
- OpenAI pool temp-unschedulable model isolation
- OpenAI/Grok/Gemini/frontend/security fixes

## Kept local customizations
1. CPA/CLIProxy xai OAuth import POST /api/v1/admin/accounts/import/grok-cpa
2. A2G import POST /api/v1/admin/accounts/import/a2g + admin menu
3. G2A server pull POST /api/v1/admin/accounts/fetch/g2a
4. Export GET /api/v1/admin/accounts/export/g2a-sso
5. Peer G2A runtime failover
6. Full-pool failover gateway.max_account_switches=0
7. Compose concurrency env wiring + upstream stream circuit envs
8. Grok OAuth 429 scheduler skip reason=grok_quota_auto_pause

## Verification
- go build ./cmd/server OK
- go test ./internal/handler/admin -run A2G OK
- Production :8080 process NOT stopped

## Notes
- Does not overwrite historical packages/releases
- Rebuild image before deploy; choose maintenance window for hot update
