# RFC 0004: Proxy Port Forwarding

Status: Draft

## 摘要
新增 `xsql proxy start` 命令，提供本地 TCP 端口转发功能。用户可以通过 xsql 启动一个本地监听端口，所有连接到该端口的流量通过 SSH tunnel 转发到远程数据库。这为开发环境提供了类似 `ssh -L` 的便捷端口转发能力，无需手动配置端口转发即可让本地程序（如数据库客户端、IDE）访问远程数据库。

## 背景 / 动机
- 当前痛点：开发者在本地开发时需要通过 SSH tunnel 访问远程数据库，通常需要手动执行 `ssh -L local_port:remote_host:remote_port user@ssh_host` 命令，且每次重启后需要重新执行
- 目标：提供统一的 CLI 命令管理端口转发，利用 xsql 已有的 SSH 配置和 profile 系统，简化开发流程
- 非目标：不替代现有的 driver dial 方式，这是独立的显式代理功能

## 方案（Proposed）
### 用户视角（CLI/配置/输出）

#### 新增命令

**`xsql proxy start <profile>`**
启动端口转发代理，将本地端口通过 SSH tunnel 转发到指定 profile 的数据库。

```bash
# 使用 profile 启动代理（端口自动分配）
xsql proxy start prod-mysql

# 指定本地端口
xsql proxy start prod-mysql --local-port 13306

# 输出 JSON 格式
xsql proxy start prod-mysql --format json
```

**输出示例（Table）：**
```
✓ Proxy started successfully
  Local:   127.0.0.1:13306
  Remote:  db.internal.example.com:3306 (via bastion.example.com:22)
  Profile: prod-mysql

Press Ctrl+C to stop
```

**输出示例（JSON）：**
```json
{
  "ok": true,
  "schema_version": 1,
  "data": {
    "local_address": "127.0.0.1:13306",
    "remote_address": "db.internal.example.com:3306",
    "ssh_proxy": "bastion.example.com:22",
    "profile": "prod-mysql"
  }
}
```

#### Flags

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--local-port` | 0 | 本地监听端口（0 表示自动分配） |
| `--local-host` | 127.0.0.1 | 本地监听地址 |
| `--format` | auto | 输出格式（table/json/yaml） |

#### 默认行为变化
- 无默认行为变化，这是新增功能

#### 错误码
新增错误码：
- `CodeProxyStartFailed`（退出码 10）：代理启动失败
- `CodeProxyPortInUse`（退出码 10）：端口已被占用

### 技术设计（Architecture）

#### 涉及模块
- 新增 `internal/proxy`：实现 TCP 端口转发逻辑
- 扩展 `cmd/xsql`：添加 `proxy` 子命令
- 复用 `internal/ssh`：SSH 连接管理
- 复用 `internal/config`：profile 和 SSH 配置读取

#### 数据结构/接口

```go
// internal/proxy/proxy.go
type Proxy struct {
    sshClient *ssh.Client
    listener  net.Listener
    ctx       context.Context
    cancel    context.CancelFunc
}

// Start 启动端口转发
func Start(ctx context.Context, opts Options) (*Proxy, error)

type Options struct {
    LocalHost     string
    LocalPort     int
    RemoteHost    string
    RemotePort    int
    SSHClient     *ssh.Client
}
```

#### 工作流程
1. 解析 profile，获取数据库地址和 SSH 配置
2. 建立 SSH 连接（复用现有 `ssh.Connect`）
3. 监听本地端口（`127.0.0.1:local_port`）
4. 对每个本地连接：
   - 通过 SSH client 建立 remote 连接（`sshClient.Dial("tcp", remote_addr)`）
   - 双向复制数据（使用 `io.Copy`）
5. 处理信号（Ctrl+C）优雅关闭

#### 兼容性策略
- 新增功能，不破坏现有兼容性
- 仅在显式调用 `proxy start` 时启动代理

## 备选方案（Alternatives）

**方案 A：使用系统 ssh 命令**
- 调用外部 `ssh -L` 命令
- 优点：利用系统 ssh，无需实现转发逻辑
- 缺点：跨平台兼容性差、难以控制、输出格式不符合 AI-first 原则

**方案 B：仅支持 HTTP API**
- 提供 HTTP REST API 而非 TCP 端口转发
- 优点：更易用、可加认证
- 缺点：无法直接对接数据库客户端、IDE 等工具，不符合用户需求

## 兼容性与迁移（Compatibility & Migration）
- 是否破坏兼容：否，新增功能
- 迁移步骤：无
- deprecation 计划：无

## 安全与隐私（Security/Privacy）
- 默认监听 `127.0.0.1`，仅本地可访问
- 复用 SSH 配置的 known_hosts 校验
- 不在日志中输出连接数据内容
- 密码/passphrase 复用 keyring 机制，不泄露明文

## 测试计划（Test Plan）

#### 单元测试
- `internal/proxy/proxy_test.go`：
  - 测试端口绑定成功/失败
  - 测试 SSH 连接失败场景
  - 测试并发连接处理

#### E2E 测试
- `tests/e2e/proxy_test.go`：
  - 启动代理并验证端口监听
  - 通过代理连接数据库执行简单查询
  - 测试 `--format json` 输出格式
  - 测试端口被占用时的错误处理
  - 测试 Ctrl+C 优雅关闭

## 未决问题
- 是否需要支持多 profile 同时转发？（暂不支持，保持简单）
- 是否需要持久化代理状态？（暂不支持，仅运行时）