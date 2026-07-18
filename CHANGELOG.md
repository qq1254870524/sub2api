# CHANGELOG

## 2026-07-19 — stable-2026-07-19-docs-sync-18r28i

- Grok OAuth **429 failover** 加宽：同请求内最多切换 `grokOAuth429MaxAccountSwitches=5` 个账号（原 storm 路径过窄导致号池有号却失败）。
- `OpenAIOAuth429FailoverState` 字段改为 `grokOAuth429FailoverArmed`；混合池在 Grok 429 武装后继续计数。
- 补充 grok import probe / quota / scheduling 相关测试与调度小修正。
- 配套 grok-regkit **18r28i** 文档同步还原点；**不覆盖**旧 tags/releases。

﻿# CHANGELOG

## 2026-07-18 — restore point #4 stable-2026-07-18-pending-18r3

See RESTORE_POINT_2026-07-18-pending-18r3.md

## 2026-07-18 — restore point #3 `stable-2026-07-18-matrix-uifallback`

- Keep CPA/CLIProxy OAuth JSON import path and SSO-to-OAuth pool ingest compatibility.
- Tagged together with grok-regkit matrix/UI-fallback restore point #3.
- Does not overwrite previous tags `stable-2026-07-18` / `stable-2026-07-18-sso-mainflow`.

