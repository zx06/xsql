# RFC 0001: SSH Proxy 采用 Driver Dial 优先

Status: Implemented

## 摘要
xsql 的 SSH proxy 第一阶段采用 **driver 自定义 dial + sshClient.Dial** 的方式为 MySQL/PostgreSQL 建立到 DB 的网络连接，不开启本地监听端口；当未来扩展到不支持 dial hook 的 driver/场景时，提供本地端口转发作为回退方案。

## 背景 / 动机
- `database/sql` 本身不提供通用“替换 net.Conn”的入口。
- 端口转发通用但会引入本地监听端口管理、生命周期、并发连接等额外复杂度。
- MySQL/PostgreSQL driver 通常提供 dial hook，可直接走 SSH 的 `sshClient.Dial()`，更贴合“代理”的本意。

## 方案（Proposed）
### 用户视角（CLI/配置）
- 当配置了 `ssh` 段时：
  - xsql 建立 SSH client
  - DB 连接通过 driver dial 走 SSH 通道
- 未来如启用回退：对用户透明（除非显式选择/诊断输出）。

### 技术设计（Architecture）
- `internal/ssh` 提供：
  - 构建 `*ssh.Client`（认证、known_hosts 校验）
  - `DialContext(network, addr)` 返回 `net.Conn`
- `internal/db` 的 mysql/pg driver 适配：
  - 将 dial hook 指向 `sshClient.Dial("tcp", target)`

## 备选方案（Alternatives）
- 仅端口转发（ssh -L 语义）：更通用，但第一阶段复杂度更高。

## 兼容性与迁移
- 新增能力，不破坏兼容。
- 回退方案（端口转发）作为可选实现，不影响现有配置。

## 安全与隐私
- 默认启用 known_hosts 校验。
- 禁止在日志/错误中输出明文 secrets。

## 测试计划
- 单测：ssh 配置解析、known_hosts 行为、dial 错误码映射。
- 集成（可选）：ssh server + mysql/pg 容器验证。
