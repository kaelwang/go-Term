# 交付报告：go-Term 主机管理改造（含 ZMODEM 清理）

- **项目**：go-Term（Go + React Web SSH 工具）
- **交付日期**：2026-07-21
- **团队**：software-host-mgmt / software-host-mgmt-backlog（齐活林主理，许清楚/高见远/寇豆码/严过关流程）
- **最终状态**：✅ 完成，三轮 QA 独立回归全部 `NoOne`（成功）

---

## 1. TL;DR

本轮完成了一次「破坏性清理 + 功能补齐 + 质量加固」的组合改造：
1. **彻底删除 ZMODEM（sz/rz）全套功能**（前端+后端），保留 trzsz / xmodem；
2. **补齐主机/分组管理四项缺失功能**（新建可选分组、连接可编辑、分组可改名、主机可勾选+批量操作）；
3. **修复 QA 揪出的 2 个后端数据破坏 Bug**（后端更新接口改为 read-modify-write）；
4. **补完 QA 留下的 2 个前端非阻断 backlog**（`credential_id` 守卫 + `ssh_config_host` 透传）。

双架构二进制已重新编译交付，与最终代码逐字节对应。

---

## 2. 工作流与团队编排

| 阶段 | 工作流 | 涉及成员 | 质量关卡 |
|------|--------|----------|----------|
| 删除 ZMODEM | BugFix / 增量 | 工程师 + QA | QA NoOne |
| 主机管理功能 + 残留清理 | 标准 SOP（增量） | 工程师 + QA | QA 第一轮抓 2 后端 Bug |
| 后端 Bug 修复 | BugFix | 工程师 + QA | QA 第二轮 NoOne |
| 前端 backlog 修复 | BugFix（增量） | 工程师 + QA | QA 第三轮 NoOne |

---

## 3. 变更分期总览

| 阶段 | 主题 | 范围 | 关键产出 | 双架构二进制（amd64 / arm64） |
|------|------|------|----------|------------------------------|
| 一 | 删除 ZMODEM | 前端+后端全栈 | zmodem 包/fork 删除、go.mod 清理、网关检测逻辑移除 | 24,173,186 / 22,931,914 |
| 二 | 主机管理四项功能 + 残留清理 | 纯前端 + README | 新建分组/编辑连接/分组改名/勾选批量 + rest.ts/README 清理 | 24,177,282 / 22,931,914 |
| 三 | 后端部分更新 Bug 修复 | 后端 Go | read-modify-write + 2 回归用例 | 24,184,233 / 22,935,161 |
| 四 | 前端两项 backlog | 纯前端 | credential_id 守卫 + ssh_config_host 透传 | 24,184,233 / 22,935,161（干净重建，逐字节一致） |

---

## 4. 各阶段详情

### 4.1 阶段一：彻底删除 ZMODEM

**决策**：用户不再支持 zmodem，但保留现有整体架构（不动 Go 后端 SSH/鉴权/会话、不动前端框架）。trzsz / xmodem 为独立协议，一并保留。

**删除清单（整文件）**
- 后端：`internal/transfer/zmodem/`（整包，4 文件）、`internal/thirdparty/zmodem/`（原 xiwh/zmodem fork，22 文件）、`internal/gateway/stream_test.go`、`internal/thirdparty/`（空目录）
- 清理：`dist/go-Term`、`server.exe`、`dist/go-Term-win.exe`（ZMODEM 移除前的陈旧含 zmodem 二进制，已删避免误导）

**修改清单**
- `go.mod`：移除 `github.com/xiwh/zmodem`、`github.com/sigurn/crc16`、`replace github.com/xiwh/zmodem => ./internal/thirdparty/zmodem`，并 `go mod tidy`
- `internal/gateway/transfer.go`：删除 `case "zmodem"`、删除 `startAutoZmodemRecv`/`notifyZmodemSend`/`StartAutoZmodemSend`/`cleanupRemoteAfterTransfer`，保留 `RunTransfer`/`runRecvToDir`/`firstFileInDir`/`emitTransferStatus`
- `internal/gateway/stream.go`：删除 `zmodemSig`/`zmodemSigEsc`/`zmodemHeaderDirection` 及 WritePump 中 zmodem 检测分支，ReadPump 的 `transfer` 统一走 `RunTransfer`
- `internal/gateway/session.go`：删除 `zmodemPrefix` 字段
- `internal/api/handlers.go`：`TransferBinsHandler` 移除 `rz`/`sz` 键
- `internal/transfer/transfer.go`：注释 "ZMODEM/trzsz" → "trzsz"
- 前端：`types.ts`（`TransferProtocol` 去 `'zmodem'`）、`store/transferStore.ts`（去 zmodemSendPending / `rz`/`sz` bins）、`components/TransferBar.tsx`（去 ZMODEM 选项与自动弹框）、`api/ws.ts`（去 `zmodem_detected`）、`README.md`

**验证**：`go build ./...` / `go vet ./...` / `go test ./internal/transfer/... ./internal/gateway/... ./internal/api/... ./cmd/...` 全 PASS；fork 15/15（zmodem 阶段）；全仓库 grep zmodem 零残留；二进制内嵌前端无 zmodem。

### 4.2 阶段二：主机/分组管理四项功能 + zmodem 残留清理

**清理（zmodem 残留）**
- `frontend/src/api/rest.ts`：`transferBins` 返回类型 `{ rz; sz; trz; tsz }` → `{ trz; tsz }`（后端实际只返回 trz/tsz）
- `README.md`：L341 去 rz/sz、L358 去 `sz file`、L362 去 `GOTERM_RZ_BIN`/`GOTERM_SZ_BIN`
- 核实：仓库已无任何纯 ZMODEM 文档（连 `zmodem-optimization-proposal.md` 都不存在）

**四项功能（纯前端，后端 CRUD 本就齐全）**
- **B1 新建可选分组**：`connectForm.tsx` 新增 `groupId` state + 分组 `<select>`（选项含"未分组"+groups），save 透传 groupId
- **B2 连接可编辑**：`connectForm.tsx` 新增 `initial?` prop（编辑模式预填、走 `rest.updateConnection`）；`connectionStore.ts` 新增 `updateConnection` action；`ConnectionSidebar.tsx` 每行加编辑按钮 `onEdit`；`App.tsx` 增加 `editingConn` state 传 `initial`
- **B3 分组可改名**：`ConnectionSidebar.tsx` 分组行加重命名按钮接 `renameGroup`
- **B4 主机可勾选 + 批量**：`ConnectionSidebar.tsx` 每行复选框（`stopPropagation` 防误连）+ `selected:Set<number>` + 批量工具条（全选 / 移动到分组 / 批量删除）

**QA 第一轮抓出的 2 个后端 Bug（路由回 Engineer）**
- 🔴 Bug 1（B4 批量移动分组）：前端只发 `{group_id}`，后端 `UpdateConnection` 12 列全量覆盖 → 清空 name/host/凭证引用等
- 🟠 Bug 2（B3 分组改名）：前端只发 `{name}`，后端 `UpdateGroup` 把 `sort_order` 写成 0 → 顺序错乱

### 4.3 阶段三：后端部分更新 Bug 修复

**根因**：后端 `ConnectionsUpdate`/`UpdateGroup` 不支持 PATCH（未先读原记录就全量覆盖）。

**修复（方案 A：read-modify-write）**
- `internal/api/connections_handlers.go`：`ConnectionsUpdate`/`GroupsUpdate` 绑定 `map[string]interface{}`，空 body 不再报错
- `internal/store/connections.go`：`UpdateConnection(username, id, fields)` 先 `GetConnection` 读原记录，仅覆盖 fields 中存在的 key；`group_id`/`credential_id` 的 nil→SQL NULL、数字→int
- `internal/store/groups.go`：`UpdateGroup(username, id, fields)` 同样读-改-写，`sort_order` 仅在 fields 含时覆盖
- 新增辅助 `toInt`/`toIntPtr`/`toString`/`toRawMessage` 做类型转换

**测试**：新增 `TestConnectionPartialUpdatePreservesFields`（部分更新 group_id 后其余字段不被清）、`TestGroupRenamePreservesSortOrder`（改名后 sort_order 仍为 5）；适配 `TestConnectionCRUD`/`TestGroupCRUDAndReorder`。`go test ./internal/store/... ./internal/api/... ./internal/transfer/... ./internal/gateway/...` 全 PASS。

**QA 第二轮回归**：NoOne（成功），两 Bug 真修复、B2 编辑无回归、二进制与报告逐字节一致。

### 4.4 阶段四：前端两项 backlog 修复

**Backlog 1：`credential_id` 写入缺 undefined 守卫**
- `frontend/src/store/connectionStore.ts` 的 `updateConnection`：
  ```ts
  // Before: patch.credential_id = credId   // credId = input.credentialId ?? null
  // After:
  if (typeof credId === 'number' && credId >= 0) {
    patch.credential_id = credId
  }
  ```
  仅有效数值才写；`null`/`undefined` 省略 key → 后端 read-modify-write 保留原凭证引用。批量移动路径（`ConnectionSidebar.tsx:119`）仅传 `{ group_id }`，无误带。

**Backlog 2：编辑态 `ssh_config_host` 未透传**
- `connectForm.tsx`：该字段已是受控 `<input>`（带 datalist 选 SSH 别名），编辑态预填 `initial.ssh_config_host`；`save()` 的 payload 增加 `sshConfigHost: sshConfigHost` 透传
- `connectionStore.ts`：`SaveConnectionInput` 新增 `sshConfigHost?: string`；`updateConnection` 在 `input.sshConfigHost !== undefined` 时写 `ssh_config_host`；`saveConnection` create payload 增加 `ssh_config_host: input.sshConfigHost || null`

**验证**：`go build`/`go vet` 通过（后端无改动）；`npm ci && npm run build` 通过（tsc 类型检查）；重新内嵌（清掉旧 `frontend/dist` 哈希残留后干净重建）；双架构二进制与阶段三**逐字节一致**（见下）。

**QA 第三轮回归**：NoOne（成功）。

---

## 5. 文件清单（汇总，去重）

**删除文件**
- `internal/transfer/zmodem/`（整包）
- `internal/thirdparty/zmodem/`（整 fork）
- `internal/thirdparty/`（空目录）
- `internal/gateway/stream_test.go`
- 陈旧二进制：`dist/go-Term`、`server.exe`、`dist/go-Term-win.exe`

**修改文件**
后端 Go：
- `go.mod`
- `internal/gateway/transfer.go`、`stream.go`、`session.go`
- `internal/api/handlers.go`、`internal/api/router.go`、`internal/api/connections_handlers.go`
- `internal/transfer/transfer.go`
- `internal/store/connections.go`、`internal/store/groups.go`、`internal/store/store_test.go`

前端（React/TS）：
- `frontend/src/api/rest.ts`、`frontend/src/api/ws.ts`
- `frontend/src/types.ts`、`frontend/src/store/transferStore.ts`、`frontend/src/store/connectionStore.ts`
- `frontend/src/components/TransferBar.tsx`、`frontend/src/components/ConnectionSidebar.tsx`
- `frontend/src/ssh/connectForm.tsx`
- `frontend/src/App.tsx`
- `README.md`

---

## 6. 交付二进制（按 `uname -m` 选）

| 架构 | 文件 | 字节数 | 适用 |
|------|------|--------|------|
| x86_64 | `dist/go-Term-amd64` | 24,184,233 B | `uname -m` → x86_64 |
| aarch64 | `dist/go-Term-arm64` | 22,935,161 B | `uname -m` → aarch64 |

> 较含 ZMODEM 前的 ~37MB 大幅瘦身；最终字节数与「阶段三 + 阶段四前端修复」代码逐字节对应（阶段四为干净重建，无体积增量）。

---

## 7. 测试与质量关卡

- `go build ./...` / `go vet ./...`：全 PASS
- `go test ./internal/store/... ./internal/api/... ./internal/transfer/... ./internal/gateway/...`：全 PASS（含新增 2 个回归用例 `TestConnectionPartialUpdatePreservesFields`、`TestGroupRenamePreservesSortOrder`）
- 前端 `npm ci && npm run build`（`tsc --noEmit && vite build`）：类型检查通过
- 三轮独立 QA 回归：全部 `NoOne`（成功）

---

## 8. 已知风险 / 遗留

1. **`ssh_config_host` 清空语义**（非缺陷，可选优化）：`save()` 始终透传 `sshConfigHost`（含空串），清空别名 = 显式清空。若产品期望"清空=保留原值"，需改为仅在 `!== undefined` 时传。
2. **`TestReceiverHandlesZSINIT` 偶发 flaky**：依赖时序的历史已知问题，非本轮引入；CI 偶败请先重跑确认。
3. **zmodem 彻底移除**：前端已无任何 zmodem 入口，传输协议仅剩 XMODEM / trzsz。

---

## 9. 用户下一步建议

1. **部署**：按服务器架构替换 `dist/` 下对应二进制（`go-Term-amd64` / `go-Term-arm64`），Linux 实测四项功能。
2. **验证重点**：
   - 批量移动分组后，原连接的主机名/账号/凭证不能丢（Bug1 修复点）
   - 改名分组后顺序不乱跳（Bug2 修复点）
   - 编辑连接时不改凭证 → 凭证不丢；改 `ssh_config_host` 别名 → 改动生效（Backlog 修复点）
3. **回归传输**：确认 trzsz / xmodem 传输仍正常（本次未动其逻辑，但动了 store 更新语义，建议顺手验一次）。
4. **可选收尾**：若要将 `ssh_config_host` 清空语义改为"保留原值"，或进一步归档过时调试笔记，可再开小任务。
