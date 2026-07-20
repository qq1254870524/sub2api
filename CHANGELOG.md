## 0.1.165-upstream-162-full-pool (2026-07-20)

### Upstream
- Merge [Wei-Shaw/sub2api v0.1.162](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.162)
- Absorb upstream TTFT/concurrency-related fixes (responses image intent, SSE type) plus Grok/OpenAI/sticky/quota/WS/security/frontend fixes

### Keep fork customizations
- CPA/CLIProxy xai OAuth import: `POST /api/v1/admin/accounts/import/grok-cpa` + Import Data UI
- A2G import / server-side G2A fetch / export g2a-sso / Peer G2A runtime forward
- A2G dedupe + max_convert + SSO backfill (fix UI hang)
- Full-pool failover: `max_account_switches=0` (800+ accounts no longer hard-capped at ~5/10)
- Grok OAuth 429 switch budget aligned to `gateway.max_account_switches` (0=full pool)

### Performance / deploy
- Default pools: DB max_open=256 max_idle=128; Redis pool_size=1024; gateway max_idle_conns=2560
- Deploy image still local build `local/sub2api:cpa-import` (`pull_policy: never`)
- Production: set `GATEWAY_MAX_ACCOUNT_SWITCHES=0` or `gateway.max_account_switches: 0`

### Version markers
- VERSION: `0.1.165-upstream-162-full-pool`
- backend/cmd/server/VERSION: `0.1.162` (upstream baseline)

## 0.1.164-full-pool-failover (2026-07-20)

### 账号全池 Failover
- **根因**：单次请求切换账号有硬顶 `gateway.max_account_switches`（历史默认 10；OpenAI handler 无配置时甚至 fallback 到 3），Grok OAuth 429 还有额外硬顶 10。因此 800+ 号池也只会试少量账号后返回失败。
- **改动**：
  - `max_account_switches` / `max_account_switches_gemini` 默认改为 **0 = 不限制**，失败时继续切换直到号池无可选账号。
  - 所有 handler 去掉 `<=0 强制改成 3/10` 的错误 fallback。
  - Grok OAuth 429 请求内切换预算与 `gateway.max_account_switches` 对齐（0=全池）。
  - `AccountSwitchesExhausted` / `AccountSwitchesWithinBudget` 统一语义。
- **配置**：`gateway.max_account_switches: 0`（推荐大号池）；若要硬顶可设正数如 `5`/`10`。
- **注意**：全池切换可能拉长单次请求耗时；仍受客户端超时与上游超时约束。

﻿
## 0.1.163-a2g-dedup-maxconvert (2026-07-19)

### A2G 导入加固
- 邮箱/SSO 去重，已有账号不覆盖
- 邮箱冲突时回填 credentials.sso（便于下次秒跳过）
- 默认 max_convert=40，超额 deferred，请求分钟级结束（修复「A2G 导入中」假死）
- 前端超时按转换预算计算（上限 45 分钟）
- 日志：import_plan → import_progress → import_done

详见 CHANGELOG_A2G_DEDUP_MAXCONVERT.md。
# Changelog (fork qq1254870524/sub2api)

## stable-2026-07-19-a2g-server-pull-peer (2026-07-19)

### Highlights
- **A2G 导入改为服务端拉取 G2A 号池**（彻底绕过浏览器 CORS / Failed to fetch）。
- **Sub2 与 G2A 运行时 peer 自动调配**：本地 Grok 号池耗尽 / failover 失败后，自动转发到宿主机 Grok2API。
- 与既有 A2G 导入 / G2A SUB2 导入互补：导入是静态合池，peer 是运行时灵活调配。
- 版本标记：VERSION=0.1.162-a2g-server-pull。

### A2G 服务端拉取（账号池互通）
- 新增 API：POST /api/v1/admin/accounts/fetch/g2a
  - 请求体：g2a_base_url + g2a_admin_key
  - **Admin Key 必须是 G2A app_key（管理后台登录密钥），不是 api_key**
- POST /api/v1/admin/accounts/import/a2g 支持同一套 bridge 字段，后端代拉后入库
- 后端候选地址自动尝试（Docker 友好）：
  - 用户填写地址
  - 127.0.0.1:8010 / :8012
  - 172.24.80.1:8010（WSL/Docker 宿主机常见网关）
  - host.docker.internal:8010
  - 环境变量 GATEWAY_PEER_G2A_BASE_URL / G2A_HOST_REACHABLE
- 去重：规范化 SSO 已存在一律 skipped，永不覆盖
- User-Agent：Sub2API-G2A-Bridge/1.0

### Frontend
- A2GImportModal.vue：浏览器直拉改为 adminAPI.accounts.fetchG2A / 一键 importA2G 服务端桥接
- accounts.ts：导出对象补挂 fetchG2A（修复 Docker 前端构建 TS 失败）
- 清理未使用的浏览器 candidateBaseUrls / fetchJsonWithTimeout 残留

### Peer G2A（运行时自动调配）
- 新文件：backend/internal/handler/peer_g2a.go
- 接入：openai_chat_completions.go、gateway_handler_chat_completions.go（本地耗尽 hook）
- 环境变量：
  - GATEWAY_PEER_G2A_ENABLED=true
  - GATEWAY_PEER_G2A_BASE_URL=http://172.24.80.1:8010（容器内勿写 127.0.0.1）
  - GATEWAY_PEER_G2A_API_KEY=<G2A api_key>
  - GATEWAY_PEER_G2A_TIMEOUT_SECONDS=90
  - GATEWAY_PEER_G2A_MODELS=grok-4.5,grok-4,grok-3
- 防递归头：X-Peer-Failover / X-G2A-Peer-Failover
- 半截 SSE（streamStarted）不 peer，避免破坏流
- 成功可观察日志 peer.g2a.*，响应头 X-Peer-Source: g2a

### 本地验证（本机实测）
- G2A http://127.0.0.1:8010/health OK；号池约 230 SSO
- Sub2 Docker http://127.0.0.1:8080/health OK
- Admin JWT + POST .../fetch/g2a -> count=230，base_url_used=http://172.24.80.1:8010
- 镜像标签：local/sub2api:cpa-import / local/sub2api:a2g-server-pull

### 部署
1. git pull
2. 重建镜像并重启容器
3. Sub2 管理页 -> 账号 -> A2G导入 -> 填 http://127.0.0.1:8010 + G2A app_key -> 拉取/导入
4. 可选 peer：在 compose 注入 GATEWAY_PEER_G2A_* 后重启

### Notes
- 不覆盖既有 GitHub tag/release。
- 新 tag：stable-2026-07-19-a2g-server-pull-peer
- 详见 CHANGELOG_PEER_G2A.md 与 CHANGELOG_A2G_SERVER_PULL.md

## 2026-07-19 — batch-test-forbidden-fix

详见 CHANGELOG_BATCH_TEST.md。

- 批量测试连接（SSE，并发 3）
- 修复测试/恢复成功后 UI 仍显示 forbidden（前后端 usage 缓存失效）

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

