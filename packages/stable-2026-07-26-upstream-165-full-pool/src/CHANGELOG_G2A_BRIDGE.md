# Changelog — Sub2API ↔ G2A direct bridge

## stable-2026-07-19-sub2api-direct-bridge (2026-07-19)

### 新增 / 行为
- **A2G 网站直连**：账号工具 →「A2G导入」填写 Grok2API Base URL + Admin Key（`app_key`）。
- 浏览器直连 `GET {g2a}/admin/api/tokens`（不经 Docker 出网），再 `POST /api/v1/admin/accounts/import/a2g`。
- 反向导出：`GET /api/v1/admin/accounts/export/g2a-sso` 供 G2A 服务端拉取。
- 高级选项仍保留：文件 / 粘贴。

### 去重 / 不覆盖
- SSO 命中 → skipped
- Email 命中（含历史无 sso 的 OAuth 号）→ skipped
- 成功导入写入 `credentials.sso`，便于与 G2A 双向去重

### UX 修复
- 弹窗内联状态（不再只靠 toast）
- 拉取 20s 超时
- 8010/8012 自动回退尝试
- 明确提示使用 `app_key` 而非 `api_key`

### 本地部署
- 镜像标签：`local/sub2api:cpa-import`
- 浏览器强刷 `http://localhost:8080`（Ctrl+F5）
- G2A 默认 `http://127.0.0.1:8010`；若 SUB2 反向导入失败，G2A 请改用已验证新版端口（常见 8012）

### 数量差原因
- Sub2 活跃 Grok 账号 ≠ G2A SSO 池全量
- OAuth-only / 无 sso / ConvertFromSSO 失败会减少有效入池数量

