# Fork notes (qq1254870524/sub2api)

This fork tracks upstream [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) and keeps local production customizations:

1. **CPA / CLIProxy xai OAuth import**
   - API: `POST /api/v1/admin/accounts/import/grok-cpa`
   - UI: Admin → Import Data accepts `xai-*.json` (`type=xai` / `auth_kind=oauth` / refresh+email)
2. **Grok OAuth 429 multi-account failover**
   - Request-local switch budget: 10 (`grokOAuth429MaxAccountSwitches`)
   - Skip rate-limited Grok accounts in scheduler (`shouldAutoPauseGrokAccountByQuota`)
   - Reduces client-visible `exceeded retry limit, last status: 429 Too Many Requests`

Current baseline: **upstream v0.1.161** + fork1 customizations.
