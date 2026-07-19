# Fork notes (qq1254870524/sub2api)

This fork tracks upstream [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) and keeps local production customizations:

1. **CPA / CLIProxy xai OAuth import**
   - API: `POST /api/v1/admin/accounts/import/grok-cpa`
   - UI: Admin → Import Data accepts `xai-*.json` (`type=xai` / `auth_kind=oauth` / refresh+email)
2. **Grok OAuth 429 multi-account failover**
   - Request-local switch budget: 10 (`grokOAuth429MaxAccountSwitches`)
   - Skip rate-limited Grok accounts in scheduler (`shouldAutoPauseGrokAccountByQuota`)
   - Reduces client-visible `exceeded retry limit, last status: 429 Too Many Requests`


3. **Grok2API (G2A) ↔ Sub2API 账号池互通**
   - Sub2API **A2G导入**: `POST /api/v1/admin/accounts/import/a2g` + Admin 菜单「A2G导入」
   - 解析 G2A txt/JSON pool；按规范化 SSO 去重，**不覆盖**已有账号
   - 导入后持久化 `credentials.sso` 便于双向对账
   - 反向方向在 Grok2API 侧为 **SUB2导入**（见 grok2api 仓库）

Current baseline: **upstream v0.1.161** + fork2 customizations (CPA + 429 failover + A2G import).
