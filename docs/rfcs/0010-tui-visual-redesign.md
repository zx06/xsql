# RFC 0010: TUI Visual and Interactive Experience Redesign

Status: Proposed

## 摘要
本 RFC 提出对 `xsql-ai` 终端 TUI 进行视觉与交互体验的重构。参考 Catppuccin / Tokyo Night 现代调色盘以及 Charm (Bubbletea / Lipgloss) 社区开源工具（如 `mods`、`glow`、`gh-dash`）的设计精髓，全面优化 Pill 胶囊标签、✦ Prompt 输入框、全主题自适应调色盘与按键指引。

## 背景 / 动机
- **当前问题**：
  - Header 与提示文本在白色/浅色终端主题下对比度不理想。
  - 缺少 SQL 执行的耗时与元信息状态展示。
  - 底部快捷键文本较为平淡，缺乏现代终端工具的按键 Badge 指示器。

## 方案（Proposed）

### 视觉与交互规范
1. **Pill 胶囊 Header 导航**：
   - 使用 rounded 内边距与不同主题色背景塑造 `xsql AI`、Profile、DB、只读/读写模式与自动/手动执行模式。
2. **运行指标 (Metrics Bar)**：
   - 执行 SQL 后输出包含执行耗时（如 `⏱️ 14ms`）、行数（如 `📊 10 rows`）与 LLM 模型（如 `🤖 gpt-4o`）。
3. **✦ Prompt 输入框与 Focus 指示器**：
   - 输入框增加 `✦ Ask AI:` 品牌提示前缀，并根据获得焦点状态显示鲜明边框。
4. **按键 Badge 底部栏**：
   - 底部提示升级为形如 `[Enter] 发送` `[Tab] 聚焦表格` `[e] 编辑SQL` `[Shift+Tab] 模式切换` `[Esc] 退出` 的精致键盘 Badge。
5. **Catppuccin / Tokyo Night 调色盘**：
   - 全量采用 `lipgloss.AdaptiveColor` 确保在任何深色/浅色背景终端中均具有符合 WCAG 的高对比度。

### 测试计划
- 单元测试：`internal/tui/components_test.go` 与 `model_test.go`。
- E2E 测试：`tests/e2e/ai_test.go`。
