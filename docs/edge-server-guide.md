# Edge Server 使用指南

## 概述

Edge Server 是部署在生产线边端设备上的服务，负责处理落纱操作、丝锭管理和数据同步。使用SQLite作为本地数据库，支持离线操作。

## 快速开始

### 1. 编译

```bash
# 编译Edge Server
go build -o bin/edge-server cmd/edge-server/main.go

# Windows
go build -o bin/edge-server.exe cmd/edge-server/main.go
```

### 2. 配置

Edge Server 通过环境变量配置：

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|--------|------|
| `EDGE_ID` | 边端设备唯一标识 | `edge-001` | 否 |
| `DATABASE_PATH` | SQLite数据库文件路径 | `./edge.db` | 否 |
| `PORT` | HTTP服务端口 | `8081` | 否 |
| `JWT_SECRET` | JWT签名密钥 | `edge-secret-key-change-in-production` | 否 |
| `CENTER_URL` | 中心端服务地址 | `http://localhost:8080` | 否 |
| `CENTER_TOKEN` | 访问 Center 上传端点的设备凭证 | 无 | **是** |
| `SYNC_INTERVAL` | 兜底重试间隔 | `5m` | 否 |

### 关于 `EDGE_ID` 与 `CENTER_TOKEN`（部署时最容易出错的两项）

**`EDGE_ID` 必须与 Center 注册的 `edge_code` 完全一致。** Center 的设备鉴权第三重会拿
URL 里的 `:code` 与凭证里的 `DeviceID` 比对，二者不一致会直接 403。注册设备用管理员用户
JWT 调用 `POST /v1/edges`。

**`CENTER_TOKEN` 是必填项，缺失时进程会立即退出**（`log.Fatal`）—— 在
`restart: unless-stopped` 下表现为容器崩溃循环。获取方式：

```bash
# 在 Center 上，用管理员用户 JWT（先 POST /v1/login 换取）
curl -X POST http://center:8080/v1/edges/line-01/token \
  -H "Authorization: Bearer $ADMIN_JWT"
```

**该凭证有有效期，且 Edge 不会自动续期。** 有效期由 Center 的 `EDGE_TOKEN_TTL` 控制
（默认 30 天）。过期后上传返回 401，而 401 **不计入重试**，因此记录会静静停在
`pending`、调度器每 `SYNC_INTERVAL` 打一行日志、`/health` 仍然返回 ok —— 也就是
「看起来一切正常，但什么都没在同步」。到期前请重新获取 token 并重启 Edge 容器。

Center 侧相关环境变量：

| 环境变量 | 说明 | 默认值 | 必填 |
|---------|------|--------|------|
| `EDGE_TOKEN_TTL` | 设备凭证有效期 | `720h`（30 天） | 否 |
| `TRUSTED_PROXIES` | 可信反向代理的地址，逗号分隔 | `127.0.0.1/32,::1/128` | **是**（有代理时） |

#### ⚠️ `TRUSTED_PROXIES` 必须填**代理自身的地址**，不是整个网段

这是最容易配错、且配错后**不会报错**的一项。

Center 的设备鉴权第四重校验会把 `ClientIP()` 与设备注册的 `ip_address` 比对。Gin 只有在
请求确实来自**可信代理**时，才会采用 `X-Forwarded-For` 里的值作为 `ClientIP()`。

如果按直觉把 Edge 与代理所在的整个网段写进去（例如 Edge 在 `192.168.2.84`、代理在
`192.168.2.1`，于是填 `192.168.2.0/24`），那么 **Edge 自己的地址也被视为可信**。此时
Edge 直接发来的请求里若带 `X-Forwarded-For`（或经其它路径注入），Gin 会把这个头当作
权威来源，`ClientIP()` 就变成了攻击者可控的值 —— 第四重校验形同虚设。

**正确写法：只填代理自身的地址。**

```bash
# 正确：只信任代理那一跳
TRUSTED_PROXIES=192.168.2.1/32

# 错误：把 Edge 所在网段也算作可信来源
TRUSTED_PROXIES=192.168.2.0/24

# 多个代理时逐个列出
TRUSTED_PROXIES=192.168.2.1/32,192.168.2.2/32
```

未配置时只信任本机回环（`127.0.0.1/32,::1/128`）—— 此时若 Center 在代理之后，
所有 Edge 都会被识别为代理自身的 IP，第四重校验会**全部失败**。

#### ⚠️ Center 的应用端口必须只对代理开放

Center 的第四重校验依赖「请求确实经过了代理」。如果 Edge 能直连 Center 的应用端口，
就可以完全绕过代理：

- 代理通常承担 TLS 终止、访问控制与审计，直连等于把这三项全部跳过；
- 直连时 Center 看到的 `ClientIP()` 是 Edge 的真实地址，而**代理转发时看到的是代理地址**
  —— 两者不可能同时等于设备注册的 `ip_address`，因此「能直连」本身就意味着有人在
  绕过代理，或配置已经处于不一致状态。

部署要求：**Center 的应用端口只对代理开放**（防火墙/安全组层面），代理再对外提供服务。

```nginx
# 代理侧
location /v1/ {
    proxy_pass http://center:8080;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

### 3. 启动

**Linux/Mac**:
```bash
export EDGE_ID="line-01"
export DATABASE_PATH="/data/edge.db"
export PORT="8081"
export JWT_SECRET="your-secret-key"
export CENTER_URL="http://center.example.com:8080"

./bin/edge-server
```

**Windows**:
```powershell
$env:EDGE_ID="line-01"
$env:DATABASE_PATH="C:\data\edge.db"
$env:PORT="8081"
$env:JWT_SECRET="your-secret-key"
$env:CENTER_URL="http://center.example.com:8080"

.\bin\edge-server.exe
```

**Docker** (推荐):
```bash
docker run -d \
  --name edge-server \
  -p 8081:8081 \
  -v /data/edge:/data \
  -e EDGE_ID="line-01" \
  -e DATABASE_PATH="/data/edge.db" \
  -e JWT_SECRET="your-secret-key" \
  -e CENTER_URL="http://center.example.com:8080" \
  igh-edge-server:latest
```

### 4. 健康检查

```bash
curl http://localhost:8081/health
```

响应:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ok",
    "edge_id": "line-01",
    "time": "2026-09-29T08:00:00+08:00"
  }
}
```

## 用户认证

### 登录

Edge Server 内置测试账号用于开发和测试：

```bash
curl -X POST http://localhost:8081/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "edge",
    "password": "edge123"
  }'
```

响应:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "username": "edge",
      "edge_id": "line-01",
      "role": "operator"
    }
  }
}
```

**生产环境建议**:
- 修改默认用户名和密码
- 使用强密码策略
- 定期轮换JWT密钥
- 集成企业SSO/LDAP认证

### 使用Token

所有认证API都需要在请求头中携带JWT token：

```bash
curl -X GET http://localhost:8081/v1/doffing \
  -H "Authorization: Bearer <your-token>"
```

## API使用示例

### 落纱操作

#### 1. 创建落纱

当操作员在某个位号上执行落纱操作时调用：

```bash
curl -X POST http://localhost:8081/v1/doffing \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "spinning_line_id": "uuid-of-spinning-line",
    "spinning_position": 15,
    "lot_id": "uuid-of-current-lot",
    "operator_id": "uuid-of-operator"
  }'
```

响应:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "doffing-uuid",
    "spinning_line_id": "line-uuid",
    "spinning_position": 15,
    "lot_id": "lot-uuid",
    "operator_id": "operator-uuid",
    "status": "pending",
    "doffing_time": "2026-09-29T10:30:00+08:00",
    "created_at": "2026-09-29T10:30:00+08:00",
    "updated_at": "2026-09-29T10:30:00+08:00"
  }
}
```

#### 2. 确认落纱

落纱完成后，记录实际数据：

```bash
curl -X PUT http://localhost:8081/v1/doffing/{id}/confirm \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "actual_weight": 5.2,
    "bobbin_number": "BOB-LINE01-20260929-001",
    "grade": "A"
  }'
```

#### 3. 取消落纱

如果落纱操作出现问题需要取消：

```bash
curl -X PUT http://localhost:8081/v1/doffing/{id}/cancel \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "设备故障，丝锭未成型"
  }'
```

#### 4. 查询落纱记录

```bash
# 查询所有落纱
curl "http://localhost:8081/v1/doffing?page=1&page_size=20" \
  -H "Authorization: Bearer <token>"

# 按状态筛选
curl "http://localhost:8081/v1/doffing?status=pending" \
  -H "Authorization: Bearer <token>"

# 按线体筛选
curl "http://localhost:8081/v1/doffing?spinning_line_id=<line-uuid>" \
  -H "Authorization: Bearer <token>"
```

### 丝锭管理

#### 1. 查询丝锭

```bash
# 查询所有丝锭
curl "http://localhost:8081/v1/bobbins?page=1&page_size=20" \
  -H "Authorization: Bearer <token>"

# 按批次筛选
curl "http://localhost:8081/v1/bobbins?lot_id=<lot-uuid>" \
  -H "Authorization: Bearer <token>"

# 按状态筛选
curl "http://localhost:8081/v1/bobbins?status=completed" \
  -H "Authorization: Bearer <token>"
```

#### 2. 丝锭称重

在落纱后对丝锭进行称重：

```bash
curl -X PUT http://localhost:8081/v1/bobbins/{id}/weigh \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "gross_weight": 5.8,
    "tare_weight": 0.6,
    "net_weight": 5.2
  }'
```

**重量验证规则**:
- 净重必须等于毛重减去皮重（允许0.01kg误差）
- 所有重量必须大于0

#### 3. 丝锭质检

质检员对丝锭进行质量检验：

```bash
curl -X PUT http://localhost:8081/v1/bobbins/{id}/inspect \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "grade": "A",
    "inspector_id": "inspector-uuid",
    "defect_note": ""
  }'
```

**等级标准**:
- `A`: 优等品
- `B`: 一等品
- `C`: 合格品
- `D`: 不合格品

### 数据同步

#### 1. 上传数据到中心端

```bash
curl -X POST http://localhost:8081/v1/sync/upload \
  -H "Authorization: Bearer <token>"
```

#### 2. 从中心端下载数据

```bash
curl -X POST http://localhost:8081/v1/sync/download \
  -H "Authorization: Bearer <token>"
```

**注意**: 同步功能目前为stub实现，完整功能开发中。

## 响应格式

所有API使用统一的响应格式：

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 分页响应

```json
{
  "code": 0,
  "message": "success",
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

### 错误响应

```json
{
  "code": 10001,
  "message": "参数错误: lot_id无效"
}
```

## 错误码

| 错误码 | 说明 |
|-------|------|
| 0 | 成功 |
| 10001 | 参数错误 |
| 20001 | 业务逻辑错误 |
| 40001 | 资源不存在 |
| 40101 | 未授权 |
| 40301 | 无权限 |
| 50001 | 服务器内部错误 |

## 数据库管理

### 备份

```bash
# 停止服务
systemctl stop edge-server

# 备份数据库
cp /data/edge.db /backup/edge-$(date +%Y%m%d-%H%M%S).db

# 启动服务
systemctl start edge-server
```

### 恢复

```bash
# 停止服务
systemctl stop edge-server

# 恢复数据库
cp /backup/edge-20260929-100000.db /data/edge.db

# 启动服务
systemctl start edge-server
```

### 清理

如果需要重置数据库：

```bash
# 停止服务
systemctl stop edge-server

# 删除数据库（⚠️ 所有数据将丢失）
rm /data/edge.db

# 启动服务（会自动创建新数据库）
systemctl start edge-server
```

## 生产部署建议

### 1. 使用Systemd服务

创建 `/etc/systemd/system/edge-server.service`:

```ini
[Unit]
Description=IGH Edge Server
After=network.target

[Service]
Type=simple
User=igh
Group=igh
WorkingDirectory=/opt/igh-edge
Environment="EDGE_ID=line-01"
Environment="DATABASE_PATH=/data/edge/edge.db"
Environment="PORT=8081"
Environment="JWT_SECRET=your-production-secret"
Environment="CENTER_URL=http://center.example.com:8080"
ExecStart=/opt/igh-edge/bin/edge-server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启用服务:
```bash
sudo systemctl daemon-reload
sudo systemctl enable edge-server
sudo systemctl start edge-server
sudo systemctl status edge-server
```

### 2. 日志管理

```bash
# 查看实时日志
sudo journalctl -u edge-server -f

# 查看最近日志
sudo journalctl -u edge-server -n 100

# 按时间查询
sudo journalctl -u edge-server --since "2026-09-29 10:00:00"
```

### 3. 监控

建议监控以下指标：
- 服务运行状态
- HTTP响应时间
- 数据库大小
- 磁盘空间使用
- 内存使用
- 与中心端的同步状态

### 4. 安全建议

1. **网络隔离**: 将Edge Server部署在工业网络中，限制外部访问
2. **HTTPS**: 生产环境使用HTTPS（可通过Nginx反向代理）
3. **强密码**: 修改默认用户密码
4. **密钥管理**: 定期轮换JWT密钥
5. **访问控制**: 基于角色的权限控制
6. **日志审计**: 记录所有关键操作

## 故障排除

### 服务无法启动

检查日志:
```bash
sudo journalctl -u edge-server -n 50
```

常见问题:
1. 端口被占用: 修改`PORT`环境变量
2. 数据库文件权限: 确保服务用户有读写权限
3. SQLite驱动问题: 确认使用modernc.org/sqlite

### 无法连接到中心端

1. 检查网络连通性: `ping center.example.com`
2. 检查防火墙规则
3. 验证`CENTER_URL`配置正确
4. 查看中心端服务状态

### JWT认证失败

1. 检查`JWT_SECRET`是否正确
2. Token是否过期（2小时有效期）
3. 重新登录获取新token

## 技术支持

- **文档**: [docs/edge-api-test-report.md](docs/edge-api-test-report.md)
- **API文档**: Swagger UI (开发中)
- **问题反馈**: GitHub Issues

---

**版本**: v0.1.0  
**更新时间**: 2026-09-29  
**作者**: IGH Silkroad Team
