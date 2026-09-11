# Codex 项目管理规则

本文件供 Codex 使用。通用流程见同目录的 `WORKFLOW.md`；进入代码仓库后，将两者复制到仓库根目录，或在仓库根目录的 `AGENTS.md` 中引用该文件。

开始任务时：

- 使用 Plane MCP 查询项目任务、Bug、当前 Cycle 和阻塞关系。
- 使用终端运行 `veans prime` 或 `veans list` 查询 Vikunja 今日执行项。
- 若用户没有给出编号，先复用已有任务；确需新建时同时建立 Plane 主任务和 Vikunja 执行项。

完成任务时：

- 更新 Plane 和 Vikunja 的状态、评论、测试结果和提交 ID。
- 只移动到 `In Review`，不要移动到 `Done`。
- 把每个 Git 提交与任务编号关联，最后给出未解决问题和下一步。

## 本机连接

- Plane MCP：由用户在完成 Plane 首次注册后运行 `scripts/Configure-Agent-Connections.ps1` 配置。
- Vikunja：在代码仓库运行 `veans init --server http://localhost:3456`，凭据保存在 Windows 凭据管理器或本机配置，不进入 Git。
