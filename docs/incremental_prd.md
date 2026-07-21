# webssh_go 增量 PRD（本次变更范围）

> 本文档为**增量 PRD**，仅描述本次 6 项变更，不重述既有产品全貌。基于仓库代码核实编写（核实结论见 §2）。

## 1. 项目信息

| 项 | 值 |
| --- | --- |
| Language | 中文（与用户需求一致） |
| Programming Language | 后端 Go 1.26（gin + x/crypto/ssh + gorilla/websocket + creack/pty + **modernc.org/sqlite**）；前端 React 19 + Vite 7 + Tailwind v4 + Zustand + xterm.js（沿用现有栈，不引入 MUI） |
| Project Name | `webssh_go` |
| 原始需求复述 | 在已有 go-Term 工程上做 6 项增量：①`transfer.go` 的 `config.Global` 解引用补 nil 守卫；②Web 端用户名+密码登录（登录页/闸门 + JWT）；③保存 SSH 连接并支持可折叠分组；④凭证库（密码/私钥，AES-GCM 加密落库）；⑤大幅扩充设置页可配置项；⑥更新 README 删除占位标注。两项已拍板决策：**SQLite 纯 Go 驱动（modernc.org/sqlite）落盘**；**多用户**，登录用户进 `users` 表、密码 bcrypt 哈希。 |

## 2. 现状核实（已有但未接通 / 需替换的关键事实）

> 以下为读码核实结论，供架构师对齐，避免凭空设计。

1. **`LoginHandler` 当前不校验密码**：`internal/api/handlers.go:65` 接收 `{user,password}` 但从未使用 `Password`；仅用 `serverUserAllowed(req.User)`（即 `SERVER_USER` 白名单，与 Web 登录正交）放行后直接 `security.GenerateToken`。即"白名单用户名 + 任意密码"即可拿 JWT。→ 本次须改为：查 `users` 表 + bcrypt 校验 + 签发 JWT。
2. **现有凭据保险库是文件型且非每用户**：`CredentialsHandler`（`handlers.go:272`）把凭据加密后写入 `<download_dir>/credentials.json`，key 为 `host-port-username`，并挂载 `RequireServerUser()`。这**与 B4 的 SQLite 每用户 `credentials` 表冲突**，应被新凭证库取代（见待确认 Q1）。
3. **`transfer.go` nil 隐患确认**：`transfer.go:89` `runTransferRecv` 在函数顶部 `dir := config.Global.DownloadDir` 提前解引用；若 `config.Global==nil` 会 panic。`runTransferSend` 不引用 `config.Global`，对未知 protocol 干净返回 error。与 B6 一致。
4. **`config` 无 DB/VAULT/引导相关字段**：`config.go` 仅有 `AppSecret`/`JWTSecret` 等，无 `db_path`/`vault_key`/`bootstrap_admin_*`。需新增（见需求池）。
5. **AES-GCM 能力已具备**：`security/crypto.go` 的 `Encrypt/Decrypt` 用 `config.Global.AppSecret` 派生密钥，可直接复用（B4 凭证加密）。
6. **JWT 中间件已就绪且注入 user**：`middleware.go` 的 `JWTAuth()` 在 `ENABLE_AUTH=1` 时校验并 `c.Set("user", claims.User)`；`security.GenerateToken(user, secret, min)` 可复用。每用户数据需从 `c.Get("user")` 取身份。
7. **前端已有登录管线雏形**：`rest.ts` 已有 `rest.login()` 与 `authHeader()`（自动带 Bearer），`token()` 读 `localStorage['webssh_token']`，但 **`login` 未把返回 token 写入 localStorage，且无登录页/闸门组件**。前端工作量主要在登录页 + 闸门 + 凭证/连接/设置三块 UI。
8. **前端设置仅 5 项且纯本地**：`settingStore.ts` 用 zustand `persist` 存 localStorage（theme/fontSize/encoding/cursorBlink/scrollback/webgl），未与后端/用户绑定。B5 要求改为每用户落 `user_settings`。
9. **README 仍有占位**：L18 标注模块路径 `github.com/kaelwang/go-Term` 为占位符；配置表/安全说明描述的是旧"白名单+文件保险库"模型，需整体改写。
10. **依赖缺失**：`go.mod` 无 `modernc.org/sqlite`（需新增，且保持 `CGO_ENABLED=0`）；`golang.org/x/crypto` 已在依赖中，`bcrypt` 可直接 import（无新模块）。

## 3. 增量产品目标（1 段）

把 go-Term 从一个"白名单放行、凭据落本地文件、零持久化用户/连接配置"的工具，升级为"支持多用户用户名密码登录、连接与分组持久化、集中加密凭证库、每用户可深度定制终端与传输偏好"的自托管 Web 终端平台；同时补上两处健壮性与文档短板（`config.Global` nil 守卫、README 去占位），全部以纯 Go 的 SQLite（`modernc.org/sqlite`，不引入 CGO）为落盘底座，保证生产静态二进制 `CGO_ENABLED=0 GOOS=linux` 不受影响。

## 4. 用户故事（按功能）

- **登录(②)**：作为普通用户，我希望用用户名+密码登录 Web 终端并拿到会话令牌，以便只有合法账号能使用；作为管理员，我希望首批部署时通过环境变量引导出首个 admin，以便无需手敲 SQL 即可开局。
- **保存连接+分组(③)**：作为运维，我希望把当前填好的连接配置保存下来并归入某个分组，以便下次一键选用；作为用户，我希望分组在侧边栏可展开/折叠，以便管理大量连接不混乱。
- **凭证库(④)**：作为用户，我希望把常用 SSH 密码/私钥（及口令）集中存进凭证库，并在建连时直接引用，以便不再每次手填且私钥不常驻表单；作为用户，我希望凭证值以加密形式落库，以便磁盘泄露也不暴露明文。
- **设置扩充(⑤)**：作为用户，我希望按自己的喜好配置终端字体/字号/主题/光标/滚动行数、默认协议与认证、传输默认与 known_hosts 策略、连接超时，以便开箱即我的习惯；作为管理员，我希望在设置页统一管理用户（增/删/改密）。
- **nil 守卫(①)**：作为维护者，我希望 transfer 在 `config.Global` 未初始化时不会 panic 而是干净报错，以便极端启动顺序下服务不崩。
- **README(⑥)**：作为使用者/部署者，我希望 README 真实反映 SQLite/多用户/凭证库/分组/扩充设置与新增配置项，以便照着文档即可部署。

## 5. 需求池（P0 / P1 / P2）

### P0 — 必须落地

| 编号 | 描述 | 验收标准 |
| --- | --- | --- |
| R-AUTH-1 | 新增 SQLite 存储层（`modernc.org/sqlite`，`CGO_ENABLED=0` 兼容）。DB 路径由 `GOTERM_DB_PATH` 控制，默认 `./go-Term.db`；`config.Load()` 后打开并按 B1 `AutoMigrate`/建表 `users`/`connection_groups`/`connections`/`credentials`/`user_settings`。 | 启动生成 db 文件；`CGO_ENABLED=0 GOOS=linux go build` 通过；表结构与 B1 一致。 |
| R-AUTH-2 | `users` 表 + 多用户登录。重写 `LoginHandler`：校验 `users` 表 username + bcrypt 比对 `password_hash`；成功复用 `security.GenerateToken(user, JWTSecret, JWTExpireMinutes)` 签发，返回 `{token, user, role}`；失败返回 `CodeAuthFail`。 | 错误密码被拒；正确密码拿 token；`api_test.go` 中旧"任意密码"用例须改写。 |
| R-AUTH-3 | 引导首个 admin。启动时若 `users` 表为空且 `GOTERM_BOOTSTRAP_ADMIN_USER`/`GOTERM_BOOTSTRAP_ADMIN_PASS` 均设置，则插入 role=admin（bcrypt）。 | 首次启动后可用该账号登录；重复启动不重复插入。 |
| R-AUTH-4 | `GET /api/me` 返回当前用户 `{user, role}`（JWT claims 取）；`ENABLE_AUTH=1` 时所有 `/api/*`（除 `/api/login`）与 `/ws` 经 `JWTAuth()`（已有）。 | 带有效 token 返回身份；无 token 返回 `CodeAuthFail`。 |
| R-AUTH-5 | 前端登录闸门 + 登录页。新增公开端点返回 `{auth_enabled}` 让前端决定是否显示登录页；前端新增 `Login` 组件（用户名+密码、错误态），成功后 `localStorage.setItem('webssh_token', token)`；App 顶层在 `auth_enabled && 无 token` 时渲染登录页。 | `ENABLE_AUTH=1` 未登录见登录页；登录后入主界面；刷新凭 token 保持。 |
| R-VAULT-1 | `credentials` 表 + 凭证库后端 CRUD（B1 字段）。`value` 用 `security.Encrypt`（密钥取 `GOTERM_VAULT_KEY`，缺省回退 `AppSecret`）AES-GCM 加密；所有查询按 `user_id` 隔离（JWT claims 取）。新增 `GET/POST/PUT/DELETE /api/credentials`（或 `/api/vault/*`，见 Q1）。 | password 明文不落库；private_key+可选口令加密；列表不返明文，按需解密；越权返 `CodePermissionDenied`。 |
| R-VAULT-2 | 连接引用凭证。`connections` 存 `credential_id`（不存密码/私钥）；WS connect 时后端据 `credential_id` 解密出密码/私钥填入 `protocol.Connection.Credential`。 | 保存连接后 `connections` 表无明文凭据；连接时正确复用凭证。 |
| R-CONN-1 | `connection_groups`/`connections` 表 + 保存连接后端（B1 字段：`group_id` 可空、`credential_id` 可空、`ssh_config_host`/`proxy`/`hops`/`options(JSON)` 等）。新增 `POST/GET/PUT/DELETE /api/connections`、`POST/GET/PUT/DELETE /api/connection-groups`（单层 `parent_id` 可空、`sort_order`）。 | 保存后可查到；分组可建/改名/删/排序；凭证以 `credential_id` 引用。 |
| R-CONN-2 | 前端连接管理侧边栏 + 分组折叠。新增"连接"导航，列出当前用户分组（可展开/折叠，状态本地 `useState`/zustand）与组内连接；支持新建/重命名/删除/排序分组；点击连接→回填 `ConnectForm` 并连接；`ConnectForm` 增加"保存"按钮。 | 保存连接出现在侧边栏；折叠态本地保持；点击可一键连接。 |
| R-README | 更新 README。删除 L18 占位标注说明；新增/改写：SQLite 存储、多用户登录与用户管理、连接保存与分组、凭证库、扩充设置，以及 `GOTERM_DB_PATH`/`GOTERM_VAULT_KEY`/`GOTERM_BOOTSTRAP_ADMIN_*` 配置项；移除"文件型 credentials.json 保险库"与"仅白名单登录"旧描述（保留 `SERVER_USER` 与 Web 登录正交说明）。 | README 无"占位/PLACEHOLDER"标注；功能描述与实现一致。 |

### P1 — 重要

| 编号 | 描述 | 验收标准 |
| --- | --- | --- |
| R-SET-1 | 每用户设置。`user_settings(user_id, data JSON)`；新增 `GET/PUT /api/settings` 读写当前用户 JSON 设置；默认值的全局来源由 config 或代码常量给出（见 Q5），用户值覆盖。 | 设置可保存并跨会话/刷新恢复；不同用户互不影响。 |
| R-SET-2 | 设置页扩充可配置项（至少）：终端字体族/字号/主题(dark\|light)/光标样式(block\|bar\|underline)/滚动缓冲行数；默认连接协议、默认认证方式；传输默认协议、recv 是否自动下载；known_hosts 策略（StrictHostKeyChecking 默认 on/off）；连接超时（秒）。前端设置页由现有 5 项扩充，并改为从 `/api/settings` 读写（不再纯 localStorage）。 | 上述项均可在 UI 配置并持久化；新建连接时默认协议/认证/known_hosts 策略取自用户设置。 |
| R-AUTH-6 | 用户管理（管理员）。设置页"用户管理"区：`GET /api/users`、`POST /api/users`（新增，bcrypt 存哈希，role admin/user）、`DELETE /api/users/:id`、`POST /api/users/:id/reset-password`；仅 admin 可操作（role 取自 JWT）。 | admin 可增删用户/改密；非 admin 调用返 `CodePermissionDenied`。 |
| R-NIL-1 | `transfer.go` nil 守卫。`runTransferRecv` 在解引用 `config.Global` 前加 `if config.Global == nil { return "", fmt.Errorf("config not loaded") }`，或把 `dir := config.Global.DownloadDir` 推迟到真正需要（`MkdirAll`）前并做 nil 检查（B6）。 | `config.Global==nil` 时 recv 返回 error 而非 panic；send 路径不变；补充单测覆盖 nil 场景。 |

### P2 — 可选增强

| 编号 | 描述 | 验收标准 |
| --- | --- | --- |
| R-SET-3 | 设置页进阶项：终端行高/字间距、配色主题自选、WebGL 渲染开关（已有 `webgl` 字段未接）、recv 自动下载目录策略。 | 进阶项可配置并持久化。 |
| R-CONN-3 | 连接/凭证批量导出导入（加密归档）；分组拖拽排序（替代 P1 上/下按钮）。 | 可导出加密归档并导入恢复。 |
| R-VAULT-3 | 凭证库审计日志（谁在何时引用了哪条凭证）；私钥内容脱敏预览。 | 引用可审计；私钥不泄露明文。 |
| R-AUTH-7 | 登录失败限流（防爆破）；"记住我"延长 JWT 有效期。 | 连续失败被限流。 |

## 6. UI 设计稿（文字描述）

### 登录页 / 闸门（Login gate）
- 全屏居中卡片（dark 主题）：标题"go-Term 登录"，用户名 + 密码（`type=password`）两输入框，主按钮"登录"，下方红字错误提示（如"用户名或密码错误"）。
- 行为：提交 → `rest.login` → 成功 `localStorage.setItem('webssh_token', token)` 并进入主界面；失败显示错误。
- 闸门逻辑：App 启动先请求公开配置（`auth_enabled`）；若开启且无 token → 渲染登录页；否则直接主界面。注销清 token 回登录页。
- 仅 `ENABLE_AUTH=1` 启用；`ENABLE_AUTH=0` 后端对所有 API/WS pass-through，前端不显示登录页。

### 连接管理侧边栏 + 分组折叠（Connection sidebar）
- 左侧导航新增"连接"入口（或在现有面板加"连接管理" tab）。面板内自上而下：顶部"＋ 新建分组"、"＋ 保存当前连接"；下方分组树。
- 分组项：行首折叠箭头 ▸/▾（点击切换，状态存 zustand 本地，不落库）+ 分组名 + 操作（重命名 ✎ / 删除 🗑 / 上移↓上移↑）；展开后列出该组连接卡片（连接名、协议徽标、host:port）。
- 连接卡片：点击 → 回调 `onConnect` 用保存的 `ConnectionSpec`（含 `credential_id` 引用，运行时解密）回填并连接；hover 显示"删除"。
- 未分组连接归入"默认/未分组"虚拟分组。

### 凭证库页（Credential vault）
- 设置区新增"凭证库"标签页（或独立 nav）。列表行：名称、类型徽标（password=🔑 / private_key=🗝️）、meta（用户名/创建时间）、操作（编辑/删除/复制）。
- "新增/编辑"弹窗：名称、类型（password\|private_key 单选）；password→密码输入；private_key→私钥文本域 + 可选"私钥口令"；保存时前端仅把明文发给后端（后端 AES-GCM 加密落库，前端不持久明文）。
- "复制"：调用后端解密接口返回明文到剪贴板（仅按需解密）。
- 建连时引用：`ConnectForm` 认证区增加"从凭证库选择"下拉（按当前用户列出凭证），选定后填充用户名/密码/私钥/口令（UI 显示，提交连接时不落 `connections`）。

### 扩充后的设置页（Settings）
- 现有 5 项保留并迁移到后端；新增分区：
  - **终端外观**：字体族（下拉）、字号（数字）、主题（dark/light 切换）、光标样式（block/bar/underline）、滚动缓冲行数（数字）。
  - **默认连接**：默认协议（ssh/telnet/vnc/localshell）、默认认证方式（password/publickey/...）。
  - **传输默认**：默认传输协议（sftp/ftp）、recv 是否自动下载（开关）。
  - **安全**：known_hosts 策略（StrictHostKeyChecking 默认 开/关）、连接超时（秒）。
  - **用户管理**（仅 admin 可见）：用户列表（用户名/角色/操作）、新增用户、删除、重置密码。
- 交互：改动即时 `PUT /api/settings`；进入页面 `GET /api/settings` 初始化；用户管理区独立调用 `/api/users`。

## 7. 待确认问题

1. **旧文件型 `/api/credentials` 如何处置**：现有 `/api/credentials`（host-port-username 键、`RequireServerUser` 网关、`credentials.json`）与 B4 的 SQLite 每用户凭证库冲突。建议：用新 SQLite 实现**取代**旧端点（保留 `/api/credentials` 路径或改名 `/api/vault/*`）。需定路径命名 + 是否保留兼容期。
2. **引导 admin 兜底行为**：若 `users` 表为空且未设 `GOTERM_BOOTSTRAP_ADMIN_*`，登录将无人可用。建议"fail-closed"（拒绝登录并日志告警），还是自动创建默认 admin + 随机密码打印日志？倾向 fail-closed。
3. **加密密钥来源**：B4 说"由 `config.AppSecret` 或新增 `GOTERM_VAULT_KEY` 派生"。建议新增独立 `GOTERM_VAULT_KEY`，缺省回退 `APP_SECRET`，与 JWT 密钥解耦。需确认默认回退策略。
4. **`SERVER_USER` 白名单 与 Web 登录 的关系**：B7 说二者正交。确认：Web 登录只用 `users` 表（任何在表用户均可登录，与 `SERVER_USER` 无关）；`SERVER_USER` 仍仅控制 localshell 与 hostkey/credentials 写权限。是否需在 Web 登录层也叠加 `SERVER_USER` 限制？倾向不叠加（完全正交）。
5. **每用户设置默认值来源**：B5 说"全局默认值从 config 取"，但 config 目前无这些终端/传输默认值键。建议：代码内常量作默认、用户值覆盖，不新增 config 键（避免膨胀）；或新增少量 config 键。需定。
6. **分组排序 UI**：单层分组排序用上/下按钮（简单）还是拖拽？倾向按钮（P1），拖拽为 P2。
7. **公开配置端点**：前端需感知 `ENABLE_AUTH`。建议新增 `GET /api/public/config` 返回 `{auth_enabled, version}`（免 token）。是否复用某现有端点？需定。
8. **测试回归影响**：`api_test.go`/`feature_f3_test.go` 中基于"白名单+任意密码"的 `LoginHandler` 用例需随登录重写而更新；旧 `credentials.json` 相关行为移除。需评估是否保留本地文件保险库代码或彻底删除。
