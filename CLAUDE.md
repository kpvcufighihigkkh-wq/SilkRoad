# Claude Code 项目管理规则

通用流程见同目录的 `WORKFLOW.md`。Claude Code 会在会话开始和压缩前运行 `veans prime`，因此当天 Vikunja 清单会进入上下文。

- 用 Plane MCP 查询或创建项目任务和 Bug。
- 用 `veans` 创建、认领、更新当天的执行项；项目项标题保留 Plane 编号。
- 不重复创建任务；不把人工验收前的任务标记为 `Done`。
- 每个任务使用包含 Plane 编号的分支和提交信息。
- 每个提交后回写任务评论、测试结果和提交 ID。

若 Plane MCP 不可用，先报告连接错误，不要凭记忆修改任务状态；Vikunja 仍可通过 `veans` 使用。
