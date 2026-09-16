# Vikunja `andy` API Token 直连设计

## 目标

将 Codex 和 Claude Code 的 Vikunja 操作身份从仓库专用账号 `bot-igh-silkroad` 切换为个人账号 `andy`。继续使用现有 `veans prime/list/show/update/api` 命令和项目 Hooks，不在仓库、聊天记录、命令参数或环境变量中保存明文令牌。

## 当前状态

- `.veans.yml` 指向 `http://localhost:3456`，身份为 `bot-igh-silkroad`，用户 ID 为 `2`。
- bot 凭据保存在 Windows 凭据管理器，但当前调用 `/user` 返回 HTTP 401，令牌已经失效。
- Windows 对 `localhost` 的解析存在间歇性异常；Vikunja 已同时绑定 `127.0.0.1:3456` 和 `192.168.2.16:3456`。
- 项目 Hooks 只执行 `veans prime`，无需改变 Hook 命令。

## 设计决定

### 身份与地址

- `.veans.yml` 的服务地址改为 `http://127.0.0.1:3456`，避免依赖 `localhost` DNS 解析。
- `bot.username` 改为 `andy`，`bot.user_id` 改为 `1`。
- 字段名仍保留为 `bot`，因为这是 `veans v2.6.0` 的固定配置结构；其值代表实际执行身份。

### 认证与凭据

- 使用 `veans login` 的 OAuth 2.0 + PKCE 浏览器流程登录 `andy`。
- Vikunja 为 `andy` 创建新的 API Token，`veans` 将其保存到 Windows 凭据管理器。
- 新凭据目标为 `veans:http://127.0.0.1:3456::andy`。
- 不使用已粘贴到聊天中的令牌；该令牌必须在 Vikunja 中撤销。
- 不删除 `bot-igh-silkroad` 用户、历史任务、评论或旧凭据；清理工作需要单独确认。

## 数据流

1. Codex 或 Claude Code 触发 `veans` 命令。
2. `veans` 从 `.veans.yml` 读取服务器、项目、看板和执行身份。
3. `veans` 从 Windows 凭据管理器读取 `andy` 的 API Token。
4. `veans` 通过 `http://127.0.0.1:3456/api/v2` 调用 Vikunja。
5. Vikunja 中新增或修改的任务、评论和状态均记录为 `andy`。

## 迁移步骤

1. 备份 `.veans.yml` 中不含秘密的现有配置。
2. 将服务器地址和执行身份改为 `127.0.0.1`、`andy`、用户 ID `1`。
3. 运行 `veans login`，由用户在浏览器中完成 `andy` 登录授权。
4. 验证凭据只存在于 Windows 凭据管理器，并且目标名称正确。
5. 验证只读操作：`veans api GET /user`、`veans show 1`、`veans list --mine`。
6. 通过一条明确标记为认证迁移验收的任务评论验证写权限。
7. 验证 `veans prime` 和项目 Hooks 仍能加载任务上下文。

## 失败与回滚

- OAuth 未完成或 API Token 创建失败时，停止迁移，不删除旧凭据。
- 新身份验证失败时，将 `.veans.yml` 恢复为原配置；旧 bot 账号和历史不受影响。
- 如果 `andy` 没有项目 `2` 或看板 `12` 的权限，先停止并报告，不自动扩大权限。
- 不把令牌打印到终端、日志、任务评论或 Git 差异中。

## 验收标准

- `veans api GET /user` 返回用户名 `andy`、用户 ID `1`。
- `veans show 1` 和 `veans list --mine` 连续执行成功，不再出现 `localhost` DNS 错误。
- 写入测试评论后，Vikunja 显示操作者为 `andy`。
- `veans prime` 执行成功，SessionStart 和 PreCompact Hooks 无需修改。
- `git status` 不出现任何凭据文件或令牌内容。

## 风险与边界

- 以后所有 Agent 操作与人工操作都显示为 `andy`，无法再通过 Vikunja 操作者区分来源。
- Agent 将继承 `andy` 的全部 Vikunja 权限；令牌泄露的影响范围大于专用 bot 账号。
- 本次不删除 bot 用户、不迁移历史记录、不改变 Plane 身份、不开放 n8n 局域网访问。
