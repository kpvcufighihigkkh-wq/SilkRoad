# Vikunja `andy` API Token Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前仓库的 Vikunja 执行身份从失效的 `bot-igh-silkroad` 凭据迁移到由 Windows 凭据管理器保存的 `andy` API Token，同时保留现有 `veans` 命令和项目 Hooks。

**Architecture:** 继续使用 `veans v2.6.0` 作为 Vikunja 客户端，只修改项目级 `.veans.yml` 的服务器地址与身份字段。通过 `veans login` 的 OAuth 2.0 + PKCE 流程，以 `andy` 登录并让 Vikunja 创建 API Token，令牌仅保存在 Windows 凭据管理器。

**Tech Stack:** Vikunja 2.6.0、veans 2.6.0、Windows Credential Manager、PowerShell、Codex project Hooks。

**Spec:** `docs/superpowers/specs/2026-09-16-vikunja-andy-api-token-design.md`

## Global Constraints

- 不使用、复制或保存已经粘贴到聊天中的令牌。
- 不在仓库、命令参数、日志、任务评论或环境变量中写入明文令牌。
- 不删除 `bot-igh-silkroad` 用户、历史记录或旧凭据。
- 不修改 Plane 身份、n8n 配置或 Docker 端口发布。
- 不创建 Git 提交；保留用户现有的未跟踪 `.codex/` 内容。
- 任务状态最多保持在 `In Review`，不移动到 `Done`。

---

### Task 1: Preflight And Rollback Baseline

**Files:**
- Read: `.veans.yml`
- Read: `.codex/hooks.json`
- Read: `docs/superpowers/specs/2026-09-16-vikunja-andy-api-token-design.md`

**Interfaces:**
- Consumes: 当前 Vikunja 服务器地址、项目 ID、看板 ID、bucket ID 和旧身份。
- Produces: 可核对的迁移前配置基线；不包含任何令牌。

- [x] **Step 1: Record the current non-secret configuration**

Run:

```powershell
Get-Content -LiteralPath '.veans.yml' -Raw
Get-Content -LiteralPath '.codex\hooks.json' -Raw
cmdkey.exe /list | Select-String -Pattern 'veans:' -Context 0,3
```

Expected: `.veans.yml` 显示 `http://localhost:3456`、`bot-igh-silkroad`、用户 ID `2`；只显示凭据目标名称，不显示秘密。

- [x] **Step 2: Verify the target server and identity prerequisites**

Run:

```powershell
(Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:3456/' -TimeoutSec 10).StatusCode
docker inspect agent-pm-vikunja-1 --format '{{.State.Status}} {{json .HostConfig.PortBindings}}'
```

Expected: HTTP `200`，容器状态 `running`，`127.0.0.1:3456` 已发布。

---

### Task 2: Switch The Project Identity Configuration

**Files:**
- Modify: `.veans.yml`

**Interfaces:**
- Consumes: `veans v2.6.0` 固定的 `bot.username` 与 `bot.user_id` 配置结构。
- Produces: 服务器 `http://127.0.0.1:3456`、身份 `andy`、用户 ID `1`；项目和 bucket 映射保持不变。

- [x] **Step 1: Apply the minimal configuration change**

Replace only these values:

```yaml
server: http://127.0.0.1:3456
bot:
    username: andy
    user_id: 1
```

Keep `project_id: 2`、`view_id: 12` and all bucket IDs unchanged.

- [x] **Step 2: Verify that no unrelated configuration changed**

Run:

```powershell
git diff -- .veans.yml
Get-Content -LiteralPath '.veans.yml' -Raw
```

Expected: diff 仅包含服务器、用户名和用户 ID 三项变化。

---

### Task 3: Authorize `andy` And Store The API Token

**Files:**
- Modify indirectly: Windows Credential Manager target `veans:http://127.0.0.1:3456::andy`

**Interfaces:**
- Consumes: `andy` 的浏览器登录会话和 OAuth 授权。
- Produces: Windows 凭据管理器中的 `andy` API Token；仓库中不产生令牌文件。

- [x] **Step 1: Start the secure OAuth login**

Run interactively:

```powershell
veans login
```

Expected: `veans` 输出 OAuth 授权地址并等待回调；命令中不包含 `--token`。

- [x] **Step 2: Complete authorization as `andy`**

Open the generated authorization URL, sign in as `andy`, approve authorization, and return the callback URL exactly as requested by `veans`.

Expected: `veans` reports successful login/token storage. If the browser shows another account, stop without approving.

- [x] **Step 3: Verify the credential target without revealing its secret**

Run:

```powershell
cmdkey.exe /list | Select-String -Pattern 'veans:http://127.0.0.1:3456::andy' -Context 0,3
```

Expected: one locally persistent generic credential for the exact target; output contains no token value.

---

### Task 4: Verify Read Identity And Workflow

**Files:**
- Read: `.veans.yml`
- Read: `.codex/hooks.json`

**Interfaces:**
- Consumes: the credential created in Task 3.
- Produces: evidence that all read operations execute as `andy` and avoid `localhost` DNS.

- [x] **Step 1: Verify the authenticated identity**

Run:

```powershell
veans api GET /token/test
veans api GET /tasks/1/comments --query per_page=100
```

Expected: token test returns `message: ok`; the migration comment author is `andy`, user ID `1`. The scoped token does not grant the management-style `/user` route, so `/user` returning 401 is not used as the validity check.

- [x] **Step 2: Verify project reads twice**

Run:

```powershell
veans show 1
veans list --mine
veans show 1
veans list --mine
```

Expected: all four calls exit `0`; no DNS or HTTP 401 error.

- [x] **Step 3: Verify task-context injection**

Run:

```powershell
veans prime
```

Expected: prompt names `andy` as the configured execution identity and includes project/bucket context. The Hooks file remains unchanged because it still calls `veans prime`.

---

### Task 5: Verify Write Access And Record The Migration

**Files:**
- No local file changes.
- Modify remotely: Vikunja task `#1` comment.
- Modify remotely: Plane `SILKROAD-1` comment.

**Interfaces:**
- Consumes: verified `andy` credential and existing task mappings.
- Produces: one audit comment in each system; task status remains `In Review` / `Code Review`.

- [x] **Step 1: Add a Vikunja verification comment**

Run:

```powershell
veans update 1 --comment '<h3>认证迁移验收</h3><p>Vikunja 执行身份已切换为 <code>andy</code>，API Token 仅保存在 Windows 凭据管理器；读取、写入和 <code>veans prime</code> 验证通过。本次未创建 Git 提交。</p>'
```

Expected: exit `0`; task stays in bucket `In Review` and response shows a new update timestamp.

- [x] **Step 2: Add the matching Plane verification comment**

Use Plane MCP `workitem_comment create` on `SILKROAD-1` with the same facts, without credentials or token text.

Expected: comment creation succeeds; Plane task remains `Code Review`.

- [x] **Step 3: Run final secret and worktree checks**

Run:

```powershell
rg -n 'tk_[A-Za-z0-9]+' . -g '!docs/superpowers/**'
git status --short --branch
veans api GET /token/test
veans show 1
```

Expected: repository secret scan returns no token; `git status` contains only intended configuration/docs plus pre-existing `.codex/`; token test succeeds and the audited comment author is `andy`; task remains not done.

---

### Task 6: Roll Back Only If Migration Fails

**Files:**
- Modify only on failure: `.veans.yml`

**Interfaces:**
- Consumes: a failed OAuth, identity, read, write, or Hook check.
- Produces: restored non-secret project configuration; no credential deletion.

- [ ] **Step 1: Restore the old project configuration after a failed migration**

Restore:

```yaml
server: http://localhost:3456
bot:
    username: bot-igh-silkroad
    user_id: 2
```

Keep all project/view/bucket IDs unchanged.

- [ ] **Step 2: Report the exact failing check**

Record which command failed, its exit code, and the non-secret error message. Do not retry more than once without a new hypothesis, do not delete either credential, and do not claim completion.
