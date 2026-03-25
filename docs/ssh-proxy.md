# SSH Proxy

## 关键说明：为什么不能"通用替换 net.Conn"
`database/sql` 本身并不暴露一个通用的 "dialer" 入口；**是否能用 SSH 的 `sshClient.Dial()` 替换网络连接，取决于具体 driver 是否提供自定义 Dial hook**。
因此正确做法是：
- **优先：Driver Dial（推荐）**：对 mysql/pg 分别用 driver 支持的 dial hook，把连接建立过程替换为 `sshClient.Dial("tcp", "dbhost:dbport")`。
- **回退：本地端口转发（ssh -L 语义）**：如果未来扩展到不支持自定义 dial 的数据库/driver，则启用本地监听端口转发方案保证通用性。

## 已实现能力（Driver Dial）
- Go 内建立 SSH client（`x/crypto/ssh`）。
- 为 MySQL / PostgreSQL driver 配置自定义 dial：
  - 由 driver 回调触发时，通过 `sshClient.Dial("tcp", target)` 建立到 DB 的连接。
  - 支持连接池：dial 可并发、安全复用 SSH client。

## SSH Keepalive 与自动重连

### 问题
长生命周期的 SSH 连接（如 `xsql proxy`）可能因网络中断而变为"死连接"。此时通过 proxy 的新连接请求会超时失败，且用户无法感知 proxy 已不可用。

### 解决方案
xsql 内置了 SSH 连接健康检测和自动重连机制：

1. **SSH Keepalive**：周期性发送 `keepalive@openssh.com` 请求探测连接存活状态
   - 默认间隔：30 秒
   - 默认最大失败次数：3（连续 3 次失败判定为死连接）

2. **自动重连（ReconnectDialer）**：
   - 当 keepalive 检测到连接死亡时，自动触发重连
   - 当 dial 操作失败时，自动尝试重连并重试
   - 带指数退避的重试策略
   - 重连过程中输出状态日志：
     ```
     [proxy] SSH connection lost: <error>
     [proxy] reconnecting to SSH server...
     [proxy] SSH reconnected successfully
     ```

### 适用范围
- **`xsql proxy`**：使用 ReconnectDialer，支持自动重连（长生命周期连接）
- **`xsql query` / `xsql schema dump`**：每次命令创建新连接，不需要重连

## 配置方式

### SSH Proxy 复用（推荐）

多个数据库可以共享同一个 SSH 代理配置：

```yaml
ssh_proxies:
  bastion:
    host: bastion.example.com
    port: 22
    user: admin
    identity_file: ~/.ssh/id_ed25519
    passphrase: "keyring:ssh/passphrase"
    known_hosts_file: ~/.ssh/known_hosts

profiles:
  prod-db1:
    db: mysql
    host: db1.internal
    ssh_proxy: bastion  # 引用预定义的代理

  prod-db2:
    db: pg
    host: db2.internal
    ssh_proxy: bastion  # 复用同一个代理
```

### SSH Proxy 配置项

| 字段 | 类型 | 说明 |
|------|------|------|
| `host` | string | SSH 跳板机地址 |
| `port` | int | SSH 端口（默认 22） |
| `user` | string | SSH 用户名 |
| `identity_file` | string | SSH 私钥路径 |
| `passphrase` | string | 私钥密码（支持 `keyring:` 引用） |
| `known_hosts_file` | string | known_hosts 文件路径 |
| `skip_host_key` | bool | 跳过主机密钥验证（危险） |

## 认证与安全
- 已支持：私钥（含 passphrase）
- 计划中：SSH agent 支持
- 默认启用 `known_hosts` 校验；允许显式关闭（`--ssh-skip-known-hosts-check`）。

## ssh_config（计划中）
> 当前版本不支持自动解析 ssh_config，需显式配置 SSH 参数。

- 计划支持解析：`Host`, `HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`。
- 解析失败时回退到显式参数（config/cli/env）。

## 回退方案：本地端口转发（已实现）
当需要传统的 `ssh -L` 行为或 driver 不支持 dial hook 时，可使用 `xsql proxy` 命令启用本地端口转发：
- 监听 `127.0.0.1:0`（或指定端口）分配端口
- 将 DB 连接指向本地端口
- 输出支持 JSON/YAML 或终端表格（详见 `docs/cli-spec.md`）
- 内置 SSH 自动重连：网络中断后自动恢复代理服务
