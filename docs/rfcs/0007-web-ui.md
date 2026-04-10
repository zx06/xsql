# RFC 0007: Web 查询界面（serve / web）

Status: Draft

## 摘要
为 xsql 增加一个本地优先的 Web 适配层，提供数据库查询与 schema 浏览能力。新增 `xsql serve` 和 `xsql web` 两个命令，其中 `web` 会在服务就绪后尝试打开默认浏览器。前端采用 Svelte，源码位于仓库子目录，CI/release 阶段先构建前端，再将产物嵌入 Go 二进制中发布。首版 Web 仅支持只读查询，不提供写操作入口。Query 编辑区使用 CodeMirror 6，提供 SQL 高亮、关键字补全、基于 schema API 的表/列补全，以及浏览器内本地 SQL 格式化。常用动作支持编辑器内快捷键：`Mod-Enter` 运行查询，`Shift-Alt-F` / `Mod-Shift-F` 格式化 SQL。Results 区采用紧凑结果网格：单元格默认单行截断，桌面端 hover 快速预览，点击后在底部详情区查看完整值并支持复制。

## 背景 / 动机
- 当前痛点：
  - xsql 目前仅提供 CLI/MCP 入口，交互式查询和 schema 浏览体验较弱。
  - 用户希望在本地以图形化方式浏览 profile、查看 schema、编写查询。
- 目标：
  - 保持 xsql 现有配置、错误码、SSH、只读策略不变，新增一个 Web 入口。
  - 复用 internal 层已有能力，避免 CLI/MCP/Web 三套业务实现。
  - 发布时仍然只交付单个 xsql 二进制。
- 非目标：
  - 不提供写操作能力。
  - 不实现登录系统、查询历史、保存 SQL、多标签页等复杂状态管理。

## 方案（Proposed）
### 用户视角（CLI/配置/输出）
- 新增命令：
  - `xsql serve`：启动 Web 服务
  - `xsql web`：启动 Web 服务并尝试打开浏览器
- 新增配置：
  ```yaml
  web:
    http:
      addr: 127.0.0.1:8788
      auth_token: "keyring:web/http_token"
      allow_plaintext_token: false
  ```
- 新增环境变量：
  - `XSQL_WEB_HTTP_ADDR`
  - `XSQL_WEB_HTTP_AUTH_TOKEN`
- 新增命令参数：
  - `--addr`
  - `--auth-token`
  - `--allow-plaintext`
  - `--ssh-skip-known-hosts-check`
- 默认行为：
  - 默认监听 `127.0.0.1:8788`
  - loopback 地址允许免鉴权访问
  - 非 loopback 地址必须提供 Bearer token
  - Web 查询始终强制只读，不继承 profile 的 `unsafe_allow_write`
- HTTP API 返回继续沿用 xsql JSON 契约：
  - 成功：`{"ok":true,"schema_version":1,"data":{...}}`
  - 失败：`{"ok":false,"schema_version":1,"error":{"code":"...","message":"...","details":{...}}}`

### 技术设计（Architecture）
- 涉及模块：
  - `internal/app`：提炼 profile/query/schema 服务
  - `internal/web`：HTTP server、鉴权、静态资源服务、API handler
  - `cmd/xsql`：`serve` / `web` CLI 命令
  - `webui/`：Svelte 前端源码和嵌入资源
- 数据结构/接口：
  - API 前缀固定 `/api/v1`
  - `GET /api/v1/health`
  - `GET /api/v1/profiles`
  - `GET /api/v1/profiles/{name}`
  - `GET /api/v1/schema/tables?profile=<name>&table=<pattern>&include_system=<bool>`
  - `GET /api/v1/schema/tables/{schema}/{table}?profile=<name>`
  - `POST /api/v1/query`，body 为 `{"profile":"dev","sql":"select 1"}`
  - CLI `xsql schema dump` 对外保持不变，但内部由“表列表 + 单表结构”组合生成
- 前端构建：
  - 使用 Vite 构建到 `webui/dist/`
  - Go 通过 `go:embed` 嵌入 `webui/dist/`
  - Query 编辑区使用 CodeMirror 6，方言按 profile 的 `db` 自动切换 MySQL / PostgreSQL / 通用 SQL
  - schema 补全复用现有 `schema/tables` 与 `schema/tables/{schema}/{table}` 接口，不新增后端 editor 专用 API
  - Query 提供本地 `Format` 动作，按当前 profile 方言对整个 SQL 文本进行格式化，不新增后端格式化接口
  - Results 表格为紧凑网格，长单元格不直接撑高行；hover 显示快速预览，点击后在结果面板内展开底部详情区
- 兼容性策略：
  - 新增能力，不修改现有 CLI/MCP 行为
  - Web API 继续使用现有 schema_version=1 契约
  - 旧 `GET /api/v1/schema` 在引入拆分接口时移除

## 备选方案（Alternatives）
- 方案 A：独立部署前后端
  - 优点：开发模式灵活
  - 缺点：发布复杂，不符合单二进制目标
- 方案 B：基于 MCP streamable HTTP 直接构建前端
  - 优点：减少新增 HTTP 层
  - 缺点：MCP 面向 agent，不适合作为浏览器 UI 的直接 API

## 兼容性与迁移（Compatibility & Migration）
- 是否破坏兼容：否
- 迁移步骤：无需迁移；有需要时补充 `web` 配置即可
- deprecation 计划：无

## 安全与隐私（Security/Privacy）
- 默认安全策略：
  - Web 查询强制只读
  - loopback 地址默认免鉴权
  - 非 loopback 地址强制 Bearer token
  - 错误细节不得泄露密码、私钥、完整 DSN 或 token
- secrets 暴露风险：
  - `auth_token` 支持 keyring，引导优先使用 keyring
  - `profile show` 和 Web profile 详情接口继续脱敏输出

## 测试计划（Test Plan）
- 单元测试：
  - `serve/web` 参数优先级与 remote token 校验
  - Web API 鉴权、错误映射、只读策略
  - 静态资源嵌入和 SPA fallback
- 集成测试：
  - MySQL/PostgreSQL 查询与 schema 浏览
  - SSH profile 下的 Web 查询
- E2E：
  - `xsql serve --format json` 启动和健康检查
  - `xsql web` 浏览器打开逻辑

## 未决问题（Open Questions）
- 首版是否需要提供 schema 结果的搜索/过滤增强
- 后续是否将 MCP 工具实现也统一迁移到新的 app service 层
