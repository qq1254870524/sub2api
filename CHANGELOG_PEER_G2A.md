# CHANGELOG — Peer G2A Runtime Failover

## 2026-07-19 — Sub2 ↔ G2A 运行时自动调配（peer）

### 功能
- Sub2API 本地 Grok 账号选号失败 / failover 耗尽时，自动转发到宿主机 Grok2API（G2A）。
- 与 A2G/SUB2 账号导入互补：导入合池 + 运行时 peer 双保险。

### 环境变量
```
GATEWAY_PEER_G2A_ENABLED=true
GATEWAY_PEER_G2A_BASE_URL=http://host.docker.internal:8012
GATEWAY_PEER_G2A_API_KEY=<G2A app.api_key>
GATEWAY_PEER_G2A_TIMEOUT_SECONDS=90
GATEWAY_PEER_G2A_MODELS=grok-4.5,grok-4,grok-3
```

### 行为
- 仅在本地耗尽/无号路径触发；本地成功时不 peer。
- 半截 SSE（`streamStarted`）不 peer，避免破坏流。
- 防递归：`X-Peer-Failover` + `X-G2A-Peer-Failover`。
- 日志：`peer.g2a.forward_start` / `peer.g2a.forward_status`。
- 成功响应头：`X-Peer-Source: g2a`。

### 代码
- `backend/internal/handler/peer_g2a.go`（包级转发）
- `openai_chat_completions.go`（Grok 主路径耗尽 hook）
- `gateway_handler_chat_completions.go`（通用 CC 耗尽 hook）
- `deploy/docker-compose.yml`：peer env + `extra_hosts: host.docker.internal`

### Docker 注意
- G2A 在宿主机时，容器内必须用 `host.docker.internal`，不能用 `127.0.0.1`。