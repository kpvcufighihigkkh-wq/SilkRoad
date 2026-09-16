# Webhook 配置详细指南

**目标：** 配置 GitHub 和 Gitea 的 Webhook，实现 Git 操作自动更新 Plane 任务

**预计时间：** 30-45 分钟

---

## 📋 前置检查清单

在开始配置之前，请确认以下内容：

- [ ] n8n 服务正在运行
- [ ] n8n 可以从外网/内网访问
- [ ] 已知 n8n 的访问地址
- [ ] 有 GitHub 仓库的管理员权限
- [ ] 有 Gitea 仓库的管理员权限
- [ ] 已准备好 Plane API 凭证

---

## 🎯 第一步：确认 n8n 服务

### 1.1 检查 n8n 是否运行

**方法 A：直接访问**
```
打开浏览器访问：http://192.168.2.16:5678
```

**预期结果：**
- ✅ 能看到 n8n 登录页面或工作区界面
- ❌ 如果无法访问，需要先启动 n8n 服务

**方法 B：使用命令检查**
```bash
# 检查 n8n 进程
ps aux | grep n8n

# 或者检查 Docker 容器（如果使用 Docker）
docker ps | grep n8n

# 检查端口占用
netstat -ano | grep 5678
```

### 1.2 确定 n8n 的访问方式

根据你的网络环境，n8n 有两种访问方式：

#### 情况 A：仅内网访问（推荐 Gitea）
```
URL: http://192.168.2.16:5678
适用于: Gitea (因为 Gitea 也在内网)
限制: GitHub 无法访问（GitHub 在公网）
```

#### 情况 B：公网可访问（推荐 GitHub + Gitea）
```
需要配置:
1. 域名 (如 n8n.yourdomain.com)
2. 反向代理 (Nginx/Caddy)
3. HTTPS 证书

URL: https://n8n.yourdomain.com
适用于: GitHub + Gitea 都可以
```

**⚠️ 重要决策点：**

| 场景 | 方案 | 说明 |
|------|------|------|
| 只配置 Gitea | 使用内网地址 | http://192.168.2.16:5678 |
| GitHub + Gitea | 需要公网访问 | 配置域名 + HTTPS |
| 临时测试 | 使用 ngrok/frp | 临时穿透工具 |

**主人的环境：**
- n8n 地址是 `http://192.168.2.16:5678` (内网)
- 建议：**先配置 Gitea Webhook**，GitHub 暂时跳过或使用穿透工具

---

## 🔑 第二步：生成 Webhook Secret

Webhook Secret 是一个密钥，用于验证请求确实来自 GitHub/Gitea，防止伪造攻击。

### 2.1 生成强随机密钥

**方法 A：使用命令行生成**
```bash
# Linux/Mac/Git Bash
openssl rand -hex 32

# 输出示例：
# a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6a7b8c9d0e1f2
```

**方法 B：使用在线工具**
```
访问：https://www.random.org/strings/
配置：
- 字符集: Hex (0-9, a-f)
- 长度: 64
- 数量: 1
```

**方法 C：使用 Node.js**
```bash
node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
```

### 2.2 保存生成的密钥

**生成两个密钥（GitHub 和 Gitea 各用一个）：**

```
GITHUB_WEBHOOK_SECRET=<生成的密钥1>
GITEA_WEBHOOK_SECRET=<生成的密钥2>
```

**保存位置建议：**
1. 密码管理器（推荐）
2. 环境变量文件 `.env` （不要提交到 Git！）
3. 纸质笔记（备份）

---

## 🔵 第三步：在 n8n 中创建 Webhook 工作流

### 3.1 登录 n8n

```
访问：http://192.168.2.16:5678
```

### 3.2 创建新的 Workflow

**步骤：**

1. 点击右上角 **"New Workflow"** 按钮
2. 工作流命名：`Git to Plane - Push Tracking`

### 3.3 添加 Webhook 触发节点

**步骤：**

1. 点击左侧 **"+"** 按钮
2. 搜索并选择 **"Webhook"** 节点
3. 配置 Webhook 节点：

```
HTTP Method: POST
Path: github-plane-push
   （这个路径可以自定义，记住它！）

Authentication: Header Auth (推荐)
   Header Name: X-Hub-Signature-256
   Header Value: 留空（稍后由 GitHub 自动填充）

Respond: Immediately
Response Code: 200
```

4. 点击 **"Test URL"** 按钮，会显示：

```
Webhook URL: http://192.168.2.16:5678/webhook/github-plane-push
              ^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^
              你的 n8n 地址             你设置的 path
```

**⚠️ 重要：复制这个 URL！**
```
Gitea Webhook URL: http://192.168.2.16:5678/webhook/github-plane-push
GitHub Webhook URL: (需要公网地址，暂时跳过)
```

### 3.4 添加解析节点（Function 节点）

**步骤：**

1. 点击 Webhook 节点右侧的 **"+"**
2. 搜索并选择 **"Code"** 节点（或 "Function" 节点）
3. 输入以下代码：

```javascript
// 提取 Git Push 事件信息
const body = $input.item.json.body;
const commits = body.commits || [];
const results = [];

// 遍历所有提交
for (const commit of commits) {
  // 提取任务编号 (匹配 SILKROAD-XX 或 [SILKROAD-XX])
  const match = commit.message.match(/\[?(SILKROAD-\d+)\]?/i);
  
  if (match) {
    const taskId = match[1].toUpperCase();
    
    // 提取提交信息
    const commitMsg = commit.message.split('[')[0].trim();
    const commitHash = commit.id.substring(0, 7);
    
    // 获取文件变更列表
    const filesChanged = [];
    if (commit.added) filesChanged.push(...commit.added.map(f => `➕ ${f}`));
    if (commit.modified) filesChanged.push(...commit.modified.map(f => `✏️ ${f}`));
    if (commit.removed) filesChanged.push(...commit.removed.map(f => `➖ ${f}`));
    
    results.push({
      json: {
        taskId: taskId,
        commitHash: commitHash,
        commitMessage: commitMsg,
        commitUrl: commit.url,
        author: commit.author.name || commit.author.username,
        timestamp: commit.timestamp,
        branch: body.ref.replace('refs/heads/', ''),
        filesChanged: filesChanged,
        repository: body.repository.full_name
      }
    });
  }
}

// 如果没有找到任务编号，返回空数组
if (results.length === 0) {
  console.log('No SILKROAD task IDs found in commits');
  return [];
}

return results;
```

4. 点击 **"Test step"** 测试（需要有真实的 Webhook 数据）

### 3.5 添加 HTTP Request 节点（调用 Plane API）

**步骤：**

1. 点击 Code 节点右侧的 **"+"**
2. 搜索并选择 **"HTTP Request"** 节点
3. 配置：

```
Method: POST
URL: {{$env.PLANE_BASE_URL}}/api/v1/workspaces/{{$env.PLANE_WORKSPACE_SLUG}}/projects/2f1e469d-f629-4fb2-884c-fc3347363060/issues/{{ $json.taskId }}/comments/

Authentication: Generic Credential Type
   Credential for: Header Auth
   Name: X-API-Key
   Value: {{$env.PLANE_API_KEY}}

Headers:
   Content-Type: application/json

Body Content Type: JSON
Body: 点击 "Add Expression" 并输入：
```

```json
{
  "comment_html": "<h3>💾 代码提交</h3><table><tr><td><strong>提交哈希</strong></td><td><code>{{ $json.commitHash }}</code></td></tr><tr><td><strong>提交信息</strong></td><td>{{ $json.commitMessage }}</td></tr><tr><td><strong>作者</strong></td><td>{{ $json.author }}</td></tr><tr><td><strong>分支</strong></td><td>{{ $json.branch }}</td></tr><tr><td><strong>时间</strong></td><td>{{ $json.timestamp }}</td></tr></table><h4>📝 修改文件</h4><ul>{{ $json.filesChanged.map(f => '<li>' + f + '</li>').join('') }}</ul><h4>🔗 关联链接</h4><p><a href='{{ $json.commitUrl }}'>查看完整 Diff</a></p>"
}
```

### 3.6 保存并激活 Workflow

1. 点击右上角 **"Save"** 按钮
2. 切换右上角开关为 **"Active"** (绿色)

**✅ Webhook URL 已就绪：**
```
http://192.168.2.16:5678/webhook/github-plane-push
```

---

## 🟦 第四步：配置 Gitea Webhook

### 4.1 访问 Gitea 仓库设置

```
1. 访问：http://192.168.2.84/andy/SilkRoad
2. 点击右上角 "设置" (Settings)
3. 左侧菜单选择 "Web 钩子" (Webhooks)
4. 点击 "添加 Web 钩子" → 选择 "Gitea"
```

### 4.2 配置 Webhook

**填写表单：**

```
目标 URL (Target URL):
   http://192.168.2.16:5678/webhook/github-plane-push
   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
   从 n8n Webhook 节点复制的 URL

HTTP 方法 (HTTP Method):
   POST

POST Content Type:
   application/json

密钥 (Secret):
   <粘贴之前生成的 GITEA_WEBHOOK_SECRET>
   例如：a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6...

触发条件 (Trigger On):
   ✅ 推送 (Push Events)
   ✅ 拉取请求 (Pull Request)
   ❌ 其他事件暂时不勾选

分支过滤 (Branch Filter):
   留空（或填 main 只监听主分支）

激活 (Active):
   ✅ 勾选
```

### 4.3 保存并测试

1. 点击底部 **"添加 Web 钩子"** 按钮
2. 保存后，点击新创建的 Webhook 右侧的 **"测试推送"** 按钮
3. 观察测试结果：

**✅ 成功的标志：**
```
HTTP 状态码: 200
响应时间: < 2000ms
最近推送记录: 绿色勾号 ✓
```

**❌ 失败的排查：**
```
连接超时: 检查 n8n 服务是否启动
404: 检查 Webhook URL 路径是否正确
401/403: 检查密钥配置
500: 查看 n8n Workflow 执行日志
```

### 4.4 查看执行日志

**在 Gitea 中：**
```
Webhook 详情页 → 最近推送记录 → 点击查看详情
可以看到：
- 请求 URL
- 请求头
- 请求体（Push 事件的 JSON）
- 响应状态
```

**在 n8n 中：**
```
Workflow 页面 → 左侧 "Executions" 标签
可以看到：
- 每次触发的记录
- 执行时间
- 成功/失败状态
- 每个节点的输入输出数据
```

---

## 🟩 第五步：配置 GitHub Webhook（可选）

**⚠️ 前提条件：n8n 必须有公网访问地址**

### 5.1 配置方案选择

#### 方案 A：使用域名 + HTTPS（推荐生产环境）

**步骤：**
1. 购买域名（如 n8n.yourdomain.com）
2. 配置 DNS 解析到服务器 IP
3. 配置 Nginx 反向代理：

```nginx
server {
    listen 443 ssl;
    server_name n8n.yourdomain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://192.168.2.16:5678;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

4. GitHub Webhook URL 使用：
   ```
   https://n8n.yourdomain.com/webhook/github-plane-push
   ```

#### 方案 B：使用 ngrok 临时穿透（测试用）

**步骤：**

1. 下载 ngrok：https://ngrok.com/download
2. 启动穿透：
   ```bash
   ngrok http 192.168.2.16:5678
   ```
3. 会显示一个临时 URL：
   ```
   Forwarding: https://abc123.ngrok.io -> http://192.168.2.16:5678
   ```
4. GitHub Webhook URL 使用：
   ```
   https://abc123.ngrok.io/webhook/github-plane-push
   ```

**⚠️ 注意：ngrok 免费版每次重启 URL 会变，需要重新配置 Webhook**

#### 方案 C：暂时跳过 GitHub

如果不需要公网访问：
- 仅使用 Gitea Webhook（内网）
- GitHub 仓库作为备份/展示用途
- 主要开发工作在 Gitea 进行

### 5.2 配置 GitHub Webhook（有公网地址后）

```
1. 访问：https://github.com/kpvcufighihigkkh-wq/SilkRoad/settings/hooks
2. 点击 "Add webhook" 按钮
3. 填写表单：

Payload URL:
   https://your-public-n8n-url.com/webhook/github-plane-push

Content type:
   application/json

Secret:
   <粘贴之前生成的 GITHUB_WEBHOOK_SECRET>

Which events would you like to trigger this webhook?
   ☑ Just the push event (推送事件)
   ☐ Send me everything (暂时不勾选)

Active:
   ✅ 勾选

4. 点击 "Add webhook"
5. 测试：GitHub 会自动发送一个 ping 事件
```

---

## 🧪 第六步：测试完整流程

### 6.1 准备测试提交

```bash
# 创建测试文件
echo "# Test Webhook" > test-webhook.md

# 提交（包含任务编号）
git add test-webhook.md
git commit -m "test: webhook integration [SILKROAD-5]"

# 推送到 Gitea
git push gitea main
```

### 6.2 验证 Gitea Webhook 触发

**步骤：**
1. 访问 Gitea Webhook 设置页
2. 查看 "最近推送记录"
3. 确认状态为 ✓ (绿色勾号)

### 6.3 验证 n8n 执行

**步骤：**
1. 打开 n8n Workflow
2. 点击左侧 "Executions" 标签
3. 查看最新的执行记录
4. 确认状态为 "Success" (绿色)
5. 点击查看详情，检查每个节点的数据

### 6.4 验证 Plane 更新

**步骤：**
1. 访问 Plane 项目
2. 打开任务 SILKROAD-5
3. 查看活动/评论区域
4. 应该看到新增的评论：

```
💾 代码提交
提交哈希: abc1234
提交信息: test: webhook integration
作者: Andy Yuan
分支: main
时间: 2026-09-16T14:50:00Z

📝 修改文件
- ➕ test-webhook.md

🔗 关联链接
查看完整 Diff
```

### 6.5 测试失败场景

**测试 1：提交信息不含任务编号**
```bash
git commit -m "test: without task id"
git push gitea main

预期: n8n 执行成功但不调用 Plane API（日志显示 "No task IDs found"）
```

**测试 2：任务编号不存在**
```bash
git commit -m "test: invalid task [SILKROAD-9999]"
git push gitea main

预期: n8n 执行，Plane API 返回 404 错误，但 Workflow 不应崩溃
```

---

## 📊 第七步：监控与维护

### 7.1 设置告警（可选）

在 n8n 中添加错误处理节点：

1. 在 HTTP Request 节点后添加 **"IF"** 节点
2. 条件：`{{ $json.statusCode }} !== 200`
3. True 分支添加 **"Send Email"** 或 **"Slack"** 节点
4. 发送告警通知

### 7.2 查看 Webhook 统计

**Gitea 统计：**
```
Webhook 设置页 → 查看所有推送记录
可以看到：
- 成功/失败次数
- 平均响应时间
- 最近错误信息
```

**n8n 统计：**
```
Workflow → Executions
可以筛选：
- 时间范围
- 成功/失败状态
- 执行时长
```

### 7.3 定期检查

**每周检查：**
- [ ] Webhook 成功率 > 95%
- [ ] n8n Workflow 无积压失败记录
- [ ] Plane 任务评论同步正常
- [ ] 没有重复或遗漏的评论

---

## 🐛 常见问题排查

### Q1: Gitea Webhook 测试失败 "连接超时"

**原因：** n8n 服务未启动或端口不通

**解决：**
```bash
# 检查 n8n 进程
ps aux | grep n8n

# 检查端口
netstat -ano | grep 5678

# 重启 n8n (Docker)
docker restart n8n

# 或者手动启动
n8n start
```

### Q2: n8n 收到请求但 Code 节点报错

**原因：** Push 事件结构不符合预期

**解决：**
1. 在 n8n Executions 查看原始数据
2. 检查 `$input.item.json.body.commits` 是否存在
3. 调整 Code 节点的数据提取逻辑

### Q3: Plane API 返回 404

**原因：** 任务 ID 不存在或格式错误

**解决：**
1. 确认 Plane 中存在 SILKROAD-5 任务
2. 检查 API URL 中的 project_id 是否正确
3. 使用 Plane MCP 查询任务：
   ```
   mcp__plane__workitem retrieve --project_id "..." --workitem_id "SILKROAD-5"
   ```

### Q4: Plane 评论乱码或格式错误

**原因：** HTML 编码问题

**解决：**
1. 检查 comment_html 字段的 HTML 是否合法
2. 特殊字符需要转义：`<` → `&lt;`, `>` → `&gt;`
3. 使用在线 HTML 验证器测试

### Q5: GitHub Webhook 无法访问 n8n

**原因：** n8n 在内网，GitHub 无法访问

**解决：**
- 方案 A: 配置公网域名 + HTTPS
- 方案 B: 使用 ngrok 临时穿透
- 方案 C: 仅使用 Gitea Webhook

---

## ✅ 配置完成检查清单

完成以下所有项后，Webhook 配置即完成：

- [ ] n8n 服务正常运行
- [ ] n8n Webhook Workflow 已创建并激活
- [ ] Webhook URL 已复制：`http://192.168.2.16:5678/webhook/github-plane-push`
- [ ] 已生成并保存 Webhook Secret
- [ ] Gitea Webhook 已配置并测试成功
- [ ] (可选) GitHub Webhook 已配置
- [ ] 测试提交已推送并触发 Webhook
- [ ] n8n Workflow 执行成功
- [ ] Plane 任务评论已自动添加
- [ ] 告警监控已配置（可选）

---

## 📚 相关文档

- [n8n 官方文档](https://docs.n8n.io/)
- [Gitea Webhook 文档](https://docs.gitea.io/en-us/webhooks/)
- [GitHub Webhook 文档](https://docs.github.com/en/webhooks)
- [Plane API 文档](../automation/n8n-plane-integration.md)

---

**创建时间：** 2026-09-16  
**作者：** 浮浮酱  
**版本：** v1.0
