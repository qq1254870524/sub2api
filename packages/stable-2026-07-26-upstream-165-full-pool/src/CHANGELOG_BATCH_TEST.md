# Changelog — Sub2API 批量测试连接 + Forbidden 粘性修复

## stable-2026-07-19-batch-test-forbidden-fix (2026-07-19)

### 问题背景
1. 单账号「测试连接」已成功，列表 usage 列仍显示 **forbidden**
2. 没有批量「测试连接」入口（仅有「批量重置状态」，不会真正探测上游）
3. 批量重置状态 / 恢复状态后，usage 徽章有时仍粘住旧 forbidden

### 根因
- **前端** `AccountUsageCell` 模块级 usage 缓存 TTL = 5 分钟，`is_forbidden` 被粘住
- **后端** antigravity/api/grok probe 等 usage 缓存约 3 分钟，成功测试后未主动清除
- 单测成功只关闭弹窗，不 invalidate 缓存、不 reload 列表
- SSE `test_complete` 事件在服务端 `RecoverAccountAfterSuccessfulTest` / `InvalidateAccountUsageCache` **之前**发出；若在此时刷新 usage，会读到旧 forbidden

### 后端改动
- `AccountUsageService.InvalidateAccountUsageCache(s)`：清除 api / windowStats / antigravity / openAIProbe / grokProbe 缓存
- `usageHealthyForErrorClear`：仅当 usage 无 `IsForbidden` / `IsBanned` / `NeedsVerify` / `NeedsReauth` / `errorCodeForbidden` / 非空 `Error` 时才允许清 sticky StatusError
- `tryClearRecoverableAccountError`：可恢复关键字扩大至 `403` / `forbidden` / `access denied` / `permission`
- 成功路径挂 invalidate：
  - `POST /admin/accounts/:id/test`（测试成功后）
  - `POST /admin/accounts/:id/clear-error`
  - `POST /admin/accounts/:id/recover-state`
  - `POST /admin/accounts/batch-clear-error`

### 前端改动
- 新增 `frontend/src/utils/accountUsageCache.ts`：共享 usage 缓存 + `invalidateAccountUsageCache`
- `AccountTestModal`：SSE 流 **完整结束后** 再 `emit('tested')`，避免抢在服务端 recover 之前刷新
- `AccountsView`：
  - 监听 `tested` → 清缓存 + `usageManualRefreshToken++` + 成功时 `reload()`
  - **批量测试连接**：对选中账号并发 3 调用 SSE `testAccountStream`，汇总 toast，成功后清缓存并 reload
  - 批量重置状态 / 单账号恢复状态 同样清 usage 缓存
- `AccountBulkActionsBar`：新增按钮「批量测试连接」
- `accounts.ts`：`testAccountStream(id)` 解析 SSE 至 `test_complete|error`，等 HTTP 流结束后返回
- 手动/强制刷新 usage 时带 `force=true`，避免只绕过前端缓存仍吃后端旧缓存
- i18n zh/en：`bulkActions.testConnection` / `testConnectionSuccess` / `testConnectionPartial` / `testConnectionRunning`

### 使用说明
1. 浏览器 **Ctrl+F5** 强刷管理后台
2. 单账号菜单 → 测试连接 → 成功后 forbidden 应立即消失
3. 勾选多个账号 → 顶部批量栏 → **批量测试连接** → 看成功/失败数 toast
4. 若上游 **实时仍返回 403**，清缓存后仍会显示 forbidden（正确信号，不是粘性缓存）

### 本地部署
- 源码：`C:\Users\zhang\sub2api-src` → 同步 WSL `/home/baoge/sub2api-src`
- 镜像：`local/sub2api:cpa-import`
- 重建脚本：`C:\Users\zhang\Desktop\rebuild_sub2_batch_test.sh`
- 容器端口：`http://127.0.0.1:8080`

### 相关文件
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/service/account_usage_service.go`
- `frontend/src/utils/accountUsageCache.ts`
- `frontend/src/components/account/AccountUsageCell.vue`
- `frontend/src/components/admin/account/AccountTestModal.vue`
- `frontend/src/components/admin/account/AccountBulkActionsBar.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/i18n/locales/{zh,en}/admin/accounts.ts`