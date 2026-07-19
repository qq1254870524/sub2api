# Changelog — A2G 服务端拉取 + Peer G2A

## stable-2026-07-19-a2g-server-pull-peer (2026-07-19)

### 为什么改
旧 A2G 导入在浏览器里直接请求 G2A /admin/api/tokens：
- 常报 Failed to fetch（CORS / Docker 网段 / 本机地址不可达）
- 用户体感「号池连不上」「点了没反应」

现改为 **Sub2 后端代拉 G2A**，与 G2A 到 Sub2 的服务端 SUB2 导入对称。

### API
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/accounts/fetch/g2a | 只拉取 SSO，不入库 |
| POST | /api/v1/admin/accounts/import/a2g | 可带 g2a_base_url + g2a_admin_key 服务端拉取并导入 |

### 密钥
- G2A **Admin Key = app_key**（管理后台登录）
- 不是 G2A api_key，也不是 Sub2 API Key
- Sub2 管理接口需要登录后的 Admin JWT

### 去重
- SSO 规范化后已存在 -> **skipped**
- **永不覆盖**已有账号

### Peer（可选）
见 CHANGELOG_PEER_G2A.md。

### 验证清单
- [x] 二进制含 fetch/g2a / Sub2API-G2A-Bridge
- [x] 未鉴权 POST -> 401（路由存在，不再是 404）
- [x] Admin JWT + app_key -> count 约等于号池规模
- [x] 前端构建通过（fetchG2A 已挂到 export 对象）
