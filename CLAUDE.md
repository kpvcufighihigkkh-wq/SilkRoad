# Claude Code 项目管理规则

通用流程见同目录的 `WORKFLOW.md`。Claude Code 会在会话开始和压缩前运行 `veans prime`，因此当天 Vikunja 清单会进入上下文。

## 任务管理

- 用 Plane MCP 查询或创建项目任务和 Bug。
- 用 `veans` 创建、认领、更新当天的执行项；项目项标题保留 Plane 编号。
- 不重复创建任务；不把人工验收前的任务标记为 `Done`。
- 每个任务使用包含 Plane 编号的分支和提交信息。
- 每个提交后回写任务评论、测试结果和提交 ID。
- **无任务不编码：** 如果 SessionStart 报告没有活跃任务，先创建 Vikunja 任务再开始工作。

若 Plane MCP 不可用，先报告连接错误，不要凭记忆修改任务状态；Vikunja 仍可通过 `veans` 使用。

## Git 安全规则

- **频繁提交：** 每完成一个逻辑单元的工作就 commit，不积累大量未提交变更。
- **提交前确认：** commit 前先 `git diff --stat` 确认变更范围。
- **禁止 force push：** 永远不要对 `main` 分支 `git push -f`。
- **分支工作：** 使用 feature 分支开发，完成后 PR 合并。分支格式：`fix/PLN-123-short-name`、`feat/PLN-128-short-name`。
- **回退安全：** 执行危险 git 操作前，记录当前 HEAD hash，确保可回退。
- **检查点：** 当 hook 提醒未提交变更过多时，立即 commit 或 stash。

## 文档同步规则

- **代码变文档跟：** 修改代码逻辑后，必须检查并更新相关设计文档（`docs/` 下的设计文档、`WORKFLOW.md`、`README.md` 等）。
- **设计先行：** 对于架构级变更，先更新设计文档再写代码。
- **前后一致：** 文档描述必须与代码实际行为一致；发现不一致时，确认哪个是正确的再统一修改。
- **归档摘要：** 会话结束时由 Stop hook 自动检查文档同步情况。

## Hooks 配置

所有 hook 脚本存放在 `.claude/hooks/` 目录，由 `.claude/settings.json` 调用：

| 触发点 | 脚本 | 作用 |
|--------|------|------|
| SessionStart | `veans prime` | 加载 Vikunja 任务清单 |
| SessionStart | `session-init.sh` | 检查项目任务状态，无任务时引导创建 |
| SessionStart | `git-session-check.sh` | 报告 git 工作区状态和最近提交 |
| PreCompact | `veans prime` | 压缩前刷新任务清单 |
| PreToolUse | `git-danger-warn.sh` | 危险 git 操作前输出安全锚点 |
| PreToolUse | `git-dirty-remind.sh` | 未提交变更超过 5 个文件时提醒 commit |
| Stop | `session-archive.sh` | 会话结束归档摘要 + 文档同步检查 |
