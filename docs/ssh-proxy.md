# SSH Proxy

## 关键说明：为什么不能“通用替换 net.Conn”
`database/sql` 本身并不暴露一个通用的 "dialer" 入口；**是否能用 SSH 的 `sshClient.Dial()` 替换网络连接，取决于具体 driver 是否提供自定义 Dial hook**。
因此正确做法是：
- **优先：Driver Dial（推荐）**：对 mysql/pg 分别用 driver 支持的 dial hook，把连接建立过程替换为 `sshClient.Dial("tcp", "dbhost:dbport")`。
- **回退：本地端口转发（ssh -L 语义）**：如果未来扩展到不支持自定义 dial 的数据库/driver，则启用本地监听端口转发方案保证通用性。

## 必须能力（第一阶段：采用 Driver Dial）
- Go 内建立 SSH client（`x/crypto/ssh`）。
- 为 MySQL / PostgreSQL driver 配置自定义 dial：
  - 由 driver 回调触发时，通过 `sshClient.Dial("tcp", target)` 建立到 DB 的连接。
  - 仍需支持连接池：dial 必须可并发、安全复用 SSH client。

## 认证与安全
- 支持：私钥（含 passphrase）、SSH agent。
- 默认启用 `known_hosts` 校验；允许显式关闭（不推荐，需提供“很吓人”的开关名，例如 `--ssh-skip-known-hosts-check`）。

## ssh_config（可选增强）
- 尽量支持解析：`Host`, `HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`。
- 解析失败时回退到显式参数（config/cli/env）。

## 回退方案：本地端口转发（后续）
- 仅在 driver 不支持 dial hook / 特殊场景时启用：
  - 监听 `127.0.0.1:0` 分配端口
  - 将 DB 连接指向本地端口
