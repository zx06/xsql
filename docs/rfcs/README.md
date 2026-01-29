# RFCs

本目录用于维护 **xsql** 的设计变更记录（Request For Comments）。

## 何时需要 RFC
当变更会影响以下任一项时，必须先提交/更新 RFC（并同步更新对应 `docs/*`）：
- CLI 命令/flag/默认值/行为
- 输出结构（字段、语义、默认 format）、错误码/退出码
- 配置 schema、参数优先级（CLI/ENV/Config）、profile/secrets 规则
- DB driver 抽象/连接模型/只读策略
- SSH proxy 行为与安全策略（known_hosts、认证方式等）
- 对外可集成接口（`xsql spec`、未来 server/MCP 协议）

## 编号与命名
- 文件命名：`NNNN-title.md`，例如 `0001-ssh-driver-dial.md`
- `NNNN` 从 `0001` 开始递增。

## 流程（最小流程）
1) 新建 RFC（使用 `0000-template.md`）
2) 讨论并定稿（至少包含动机、方案、兼容性与风险）
3) **更新对应 docs**（例如 `docs/cli-spec.md`）
4) 实现代码 + 测试

## 何时只改 docs 不需要 RFC
- 文案修正、示例补充、错别字
- 不改变对外行为/契约/默认值的说明性更新

> 原则：实现不得领先于文档；合并时必须保证“文档与实现一致”。
