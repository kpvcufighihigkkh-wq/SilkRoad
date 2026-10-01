# Edge Server Docker 部署指南

## 快速开始

### 1. 准备配置文件

```bash
cd deployments/edge
cp .env.example .env
```

编辑 `.env` 文件，设置你的配置：

```bash
EDGE_ID=line-01
JWT_SECRET=your-strong-secret-key-here
CENTER_URL=http://center.example.com:8080

# 必填：从 Center 获取设备凭证（缺了容器会立即退出并崩溃循环）
# 在 Center 上以管理员用户 JWT 调用：
#   curl -X POST http://center.example.com:8080/v1/edges/line-01/token \
#        -H "Authorization: Bearer $ADMIN_JWT"
CENTER_TOKEN=<粘贴上一步返回的 token>

# 可选：兜底重试间隔，默认 5m，必须为正数
SYNC_INTERVAL=5m
```

> **`EDGE_ID` 必须与 Center 上注册的 `edge_code` 完全一致**，否则设备鉴权第三重校验会
> 返回 403。注册用管理员用户 JWT 调用 `POST /v1/edges`。

> **`CENTER_TOKEN` 有有效期且不会自动续期**（默认 30 天，由 Center 的 `EDGE_TOKEN_TTL`
> 控制）。到期后上传返回 401，而 401 不计入重试 —— 记录会停在 `pending`、日志每
> `SYNC_INTERVAL` 打一行、`/health` 仍返回 ok，即「看似正常但同步已停」。到期前请重新
> 获取 token 并重启本容器。

### 2. 启动服务

```bash
docker-compose up -d
```

### 3. 查看日志

```bash
docker-compose logs -f edge-server
```

### 4. 检查健康状态

```bash
curl http://localhost:8081/health
```

### 5. 停止服务

```bash
docker-compose down
```

## 构建镜像

### 构建 Edge Server 镜像

```bash
# 从项目根目录执行
docker build -f deployments/edge/Dockerfile -t igh-edge-server:latest .
```

### 构建特定版本

```bash
docker build -f deployments/edge/Dockerfile -t igh-edge-server:v0.1.0 .
```

## 生产部署

### 多边端部署

为每个生产线创建独立的配置：

**line-01/.env**:
```bash
EDGE_ID=line-01
JWT_SECRET=secret-for-line-01
CENTER_URL=http://192.168.1.100:8080

# 必填：该生产线的设备凭证，缺了容器会立即退出并崩溃循环
# 获取：curl -X POST http://192.168.1.100:8080/v1/edges/line-01/token \
#            -H "Authorization: Bearer $ADMIN_JWT"
CENTER_TOKEN=<line-01 的 token>

# 可选：兜底重试间隔，默认 5m，必须为正数
SYNC_INTERVAL=5m
```

**line-01/docker-compose.yml**:
```yaml
version: '3.8'

services:
  edge-server:
    image: igh-edge-server:latest
    container_name: edge-line-01
    restart: unless-stopped
    ports:
      - "8081:8081"
    env_file:
      - .env
    volumes:
      - ./data:/data
      - ./logs:/app/logs
```

启动:
```bash
cd line-01
docker-compose up -d
```

### 数据持久化

数据默认保存在 Docker volume `edge-data` 中。

查看数据卷：
```bash
docker volume ls
docker volume inspect edge_edge-data
```

备份数据：
```bash
# 停止服务
docker-compose down

# 备份数据卷
docker run --rm -v edge_edge-data:/data -v $(pwd)/backup:/backup alpine \
  tar czf /backup/edge-data-$(date +%Y%m%d-%H%M%S).tar.gz -C /data .

# 重启服务
docker-compose up -d
```

恢复数据：
```bash
# 停止服务
docker-compose down

# 恢复数据
docker run --rm -v edge_edge-data:/data -v $(pwd)/backup:/backup alpine \
  tar xzf /backup/edge-data-20260929-100000.tar.gz -C /data

# 重启服务
docker-compose up -d
```

### 日志管理

日志配置已在 docker-compose.yml 中设置：
- 单个日志文件最大 10MB
- 最多保留 3 个日志文件
- 自动轮转

查看日志：
```bash
# 实时日志
docker-compose logs -f

# 最近100行
docker-compose logs --tail 100

# 特定时间
docker-compose logs --since "2026-09-29T10:00:00"
```

### 健康检查

Docker 自动进行健康检查：
- 间隔：30秒
- 超时：5秒
- 重试：3次
- 启动等待：10秒

查看健康状态：
```bash
docker ps
docker inspect --format='{{.State.Health.Status}}' igh-edge-server
```

### 资源限制

生产环境建议添加资源限制，编辑 `docker-compose.yml`：

```yaml
services:
  edge-server:
    # ... 其他配置 ...
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

### 网络配置

#### 连接到已存在的网络

如果 Center Server 在同一个 Docker 网络中：

```yaml
networks:
  igh-network:
    external: true
```

#### 使用主机网络

```yaml
services:
  edge-server:
    network_mode: "host"
    # 端口映射在host模式下无效
```

## 更新部署

### 滚动更新

```bash
# 拉取最新镜像
docker-compose pull

# 重启服务（自动使用新镜像）
docker-compose up -d

# 清理旧镜像
docker image prune -f
```

### 零停机更新

使用蓝绿部署：

1. 启动新版本服务（使用不同端口）
2. 验证新服务正常工作
3. 切换流量到新服务
4. 停止旧版本服务

## 监控和告警

### Prometheus 监控

添加 Prometheus 导出器（未来版本）：

```yaml
services:
  edge-server:
    # ... 现有配置 ...
    
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"
```

### 日志分析

使用 ELK Stack 或 Loki 进行日志聚合（可选）。

## 故障排除

### 容器无法启动

```bash
# 查看容器状态
docker-compose ps

# 查看详细日志
docker-compose logs edge-server

# 检查配置
docker-compose config
```

### 端口冲突

修改 `docker-compose.yml` 中的端口映射：
```yaml
ports:
  - "8082:8081"  # 主机端口:容器端口
```

### 数据库损坏

```bash
# 进入容器
docker-compose exec edge-server sh

# 检查数据库
sqlite3 /data/edge.db "PRAGMA integrity_check;"
```

### 网络连接问题

```bash
# 检查网络
docker network ls
docker network inspect edge_igh-network

# 测试连通性
docker-compose exec edge-server ping center.example.com
```

## 安全建议

1. **使用私有镜像仓库**
   ```bash
   docker tag igh-edge-server:latest registry.example.com/igh-edge-server:latest
   docker push registry.example.com/igh-edge-server:latest
   ```

2. **定期更新基础镜像**
   ```bash
   docker pull alpine:latest
   docker build --no-cache -f deployments/edge/Dockerfile -t igh-edge-server:latest .
   ```

3. **扫描镜像漏洞**
   ```bash
   docker scan igh-edge-server:latest
   ```

4. **限制容器权限**
   ```yaml
   services:
     edge-server:
       cap_drop:
         - ALL
       cap_add:
         - NET_BIND_SERVICE
       read_only: true
       tmpfs:
         - /tmp
   ```

5. **使用 secrets 管理敏感信息**
   ```yaml
   services:
     edge-server:
       secrets:
         - jwt_secret
   
   secrets:
     jwt_secret:
       file: ./secrets/jwt_secret.txt
   ```

## 附录

### 完整的 docker-compose.yml 示例

参考 `docker-compose.yml` 文件。

### 常用命令

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 重启服务
docker-compose restart

# 查看日志
docker-compose logs -f

# 进入容器
docker-compose exec edge-server sh

# 查看资源使用
docker stats igh-edge-server

# 更新服务
docker-compose pull && docker-compose up -d

# 完全清理（包括数据卷）
docker-compose down -v
```

---

**版本**: v0.1.0  
**更新时间**: 2026-09-29  
**作者**: IGH Silkroad Team
