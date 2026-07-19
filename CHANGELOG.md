# Changelog (fork qq1254870524/sub2api)

## stable-2026-07-19-sub2api-v0.1.161-fork2 (2026-07-19)

### Highlights
- **Grok2API ↔ Sub2API 账号池互通（手动双向导入）**
  - Sub2API 管理端新增 **「A2G导入」**：从 Grok2API 号池导出（txt 每行 SSO / G2A JSON `{pool:[tokens]}`）导入为 Grok OAuth 账号。
  - 去重键为规范化 SSO；**已存在 SSO 一律 skipped，永不覆盖**。
  - 导入成功后写入 `credentials.sso`，便于与 G2A 再导出/再导入对账。
  - API：`POST /api/v1/admin/accounts/import/a2g`
  - UI：账户页 → 更多操作 → 数据操作 → **A2G导入**（`A2GImportModal.vue`）

### Backend
- 新增 `backend/internal/handler/admin/account_grok_a2g_import.go` + 解析/去重单测。
- 路由：`accounts.POST("/import/a2g", ...)`。
- Grok OAuth 创建链路持久化 `credentials["sso"]`；`CreateAccountsFromSSO` 批量前扫描已有 SSO 跳过。
- CPA 身份键补充 `sso:`，避免跨导入通道身份不一致。

### Frontend
- `A2GImportModal.vue`：文件/粘贴、结果摘要（created/skipped/failed）。
- `AccountsView.vue` 菜单接线 + i18n zh/en（`a2gImport*`）。
- `accountsAPI.importA2G`。

### Notes
- 不做服务间自动同步；两侧均为管理员手动操作。
- 旧账号若无 `credentials.sso`，无法按 SSO 与 G2A 对账去重。
- A2G 会走 SSO→OAuth 上游转换，超时随批量规模放大。
- **不覆盖**既有 GitHub tag/release（新 tag：`stable-2026-07-19-sub2api-v0.1.161-fork2`）。
- 部署：pull 后重建/重启 Sub2API（默认 8080）。

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
