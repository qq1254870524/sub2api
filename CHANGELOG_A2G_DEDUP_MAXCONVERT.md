# CHANGELOG — A2G 导入加固（2026-07-19）

## 标题
A2G 导入：邮箱/SSO 去重、SSO 回填、单次转换上限，修复「A2G 导入中」长时间无响应

## 问题根因
1. Sub2API 历史 Grok 账号多数只有 OAuth 凭据，**`credentials.sso` 为空**（约 20/350+ 有 SSO）。
2. 旧 A2G 逻辑只用 **SSO 集合**做预过滤，导致从 G2A 拉到的 ~350+ token 几乎全部进入 `SSO→OAuth`。
3. 并发仅 3，单 token 换票慢；前端 one-shot 超时按 `timeout(300)` 估约 **2.5 小时**，UI 一直停在「A2G 导入中…」。
4. 日志出现 `grok_a2g_server_pull count=357` 后长期没有 `grok_a2g_import_done`，不是「没工作」，而是 **全量换票卡住 HTTP 请求**。

## 本次改动

### 后端 `account_grok_a2g_import.go`
- 正确接收 `listExistingGrokIdentitySets` 的 **emailSet**（此前 `existingSSO, _, err` 丢弃了邮箱集）。
- 预过滤：已有 SSO **跳过且不覆盖**。
- 新增 **`max_convert`（默认 40）**：超出部分标记 `deferred`，请求可在分钟级结束；再次点击导入继续缺口。
- 新增结果字段：`deferred` / `convert_attempted` / `existing_sso_skipped` / `existing_email_skipped` / `backfilled_sso` / `max_convert`。
- 日志：`grok_a2g_import_plan` → `grok_a2g_import_progress` → **`grok_a2g_import_done`**（必须结束）。
- 请求字段：`only_missing`、`max_convert`（负数=不限制，不推荐全池）。

### 后端 `grok_oauth_handler.go`
- 邮箱冲突时：**不覆盖账号**；若该行无 SSO，则 **回填 `credentials.sso`**，便于下次 A2G 按 SSO 秒级跳过。
- worker 结果增加 `backfilled` 标志。

### 前端
- `getGrokA2GImportTimeout`：按 **转换预算（默认 40）** 计算超时，上限 45 分钟，下限 2 分钟；**不再**对 300 全池硬编码超长等待。
- 导入默认带 `only_missing=true`、`max_convert=40`。
- 结果面板展示 convert/deferred/sso_skip/email_skip/sso_backfill 明细。

## 使用说明
1. **先刷新** Sub2 管理页，关掉卡住的「A2G 导入中」弹窗。
2. G2A Base URL：Docker 内建议 `http://172.24.80.1:8010` 或 `http://host.docker.internal:8010`（本机 Windows G2A `:8010`）。
3. **Grok2API Admin Key = G2A `app_key`（管理后台登录密钥）**，不是 `api_key`。
4. **Sub2API Admin Token = 登录后 JWT**，不是管理员密码；页面内操作已带会话，一般无需手填。
5. 点 **G2A 导入**：应快速返回大量 `skipped`/`deferred`，仅对上限内缺口做 SSO 换票；有 deferred 再点一次继续。
6. **不会覆盖** 已有 SSO / 邮箱对应账号。

## 验证
- `go test ./internal/handler/admin/` 通过。
- 镜像：`local/sub2api:cpa-import`（`pull_policy: never`）。
- 期望日志：`grok_a2g_server_pull` → `grok_a2g_import_plan` → `..._progress` → `grok_a2g_import_done`。

## 号池说明（非丢号）
- G2A / Sub2 数量差通常来自：G2A 含 expired、Sub2 侧 SSO 过期未入、或 A2G 全量卡死未完成。
- 本修复后同步策略为 **缺口导入 + 去重**，不是互相覆盖。
