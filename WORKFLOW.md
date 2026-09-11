# Agent 工作流（Plane + Vikunja）

## 任务来源

1. 开始工作前查询 Plane 的 `Ready` 和 `In Progress`，再查询 Vikunja 今日到期、未完成和被阻塞任务。
2. 用户明确给出任务编号时，优先使用该编号；按编号去重，禁止仅按标题猜测创建重复任务。
3. Plane 是项目主任务和 Bug 的唯一来源；Vikunja 是当天的可执行清单。项目任务在 Vikunja 标题中保留 Plane 编号，例如 `[PLN-123] 修复登录失败`。
4. 用户在 Vikunja 新建的项目类任务没有 Plane 编号时，先在 Plane 建立主任务并回写编号；纯个人事务只留在 Vikunja。

## 状态

- Plane：`Backlog -> Ready -> In Progress -> Code Review -> QA -> Done`
- Vikunja：`Todo -> In Progress -> In Review -> Done`
- Agent 只能推进到 `In Review`，不能自行标记 `Done`。合并、测试和人工验收后由用户完成 Done。

## 执行与并行

- 先按 P0/P1、当前 Cycle、阻塞关系、截止时间排序，再决定并行任务。
- 每个并行任务使用独立分支或 worktree；Codex 和 Claude 不得同时编辑同一个 worktree。
- 分支格式：`fix/PLN-123-short-name`、`feat/PLN-128-short-name`。
- 开始执行前把任务标记为 In Progress，并在任务评论写入计划；遇到阻塞就写明原因和下一步。

## 交付记录

- 每次提交信息必须包含 `PLN-123` 或 Vikunja 任务 ID，例如 `fix(auth): handle invalid response [PLN-123]`。
- 提交前运行与改动相关的测试；提交后说明测试结果、提交 ID、任务状态和剩余风险。
- 不执行破坏性命令、不删除数据、不提交凭据；需要人工确认时停在 In Review。

## 每日收尾

- 汇总 Plane 新增/关闭/逾期/P0-P1 未解决/In Review。
- 汇总 Vikunja 已完成/未完成/阻塞。
- 汇总 Git 提交数、关联任务数和测试结果。n8n 日报会读取这些数据并写回。
