# IGH 部署方案

> **文档编号:** DEPLOYMENT-STRATEGY-001  
> **版本:** 1.0  
> **基于:** 系统需求规格说明书 v1.2 + ADR-19（双机热备）  
> **日期:** 2026-09-28  
> **设计者:** 浮浮酱

---

## 一、部署架构概览

### 1.1 总体架构

```
                    ┌─────────────────────────────────────┐
                    │         中心端服务集群                │
                    │   (双机热备 + 负载均衡)              │
                    └─────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │                               │
            ┌───────▼────────┐            ┌────────▼────────┐
            │  主服务器 (P)   │            │  备服务器 (S)    │
            │  Active         │◄──────────►│  Standby        │
            │                 │   心跳同步   │                 │
            │  PostgreSQL (P) │            │  PostgreSQL (S) │
            │  Redis (P)      │            │  Redis (S)      │
            └─────────────────┘            └─────────────────┘
                    │                               │
                    └───────────────┬───────────────┘
                                    │
                ┌───────────────────┼───────────────────┐
                │                   │                   │
        ┌───────▼────────┐  ┌──────▼───────┐  ┌───────▼────────┐
        │  边端设备 1     │  │  边端设备 2   │  │  边端设备 N     │
        │  (Windows PC)  │  │  (Windows PC)│  │  (Windows PC)  │
        │                │  │              │  │                │
        │  SQLite        │  │  SQLite      │  │  SQLite        │
        │  离线自治       │  │  离线自治     │  │  离线自治       │
        └────────────────┘  └──────────────┘  └────────────────┘
```

### 1.2 部署清单

| 组件 | 数量 | 部署位置 | 说明 |
|------|------|----------|------|
| **中心端服务** | 2 | 主备服务器 | 双机热备 |
| **PostgreSQL** | 2 | 主备服务器 | 流复制 |
| **Redis** | 2 | 主备服务器 | 主从复制 |
| **Nginx** | 2 | 主备服务器 | 负载均衡 |
| **边端服务** | N | 车间 PC | 1-2条线配1台 |
| **SQLite** | N | 边端设备 | 本地存储 |
| **Prometheus** | 1 | 监控服务器 | 指标采集 |
| **Grafana** | 1 | 监控服务器 | 可视化 |

---

## 二、容器化方案

### 2.1 Docker 镜像设计

**目录结构：**

```
deployments/
├── docker/
│   ├── center/
│   │   └── Dockerfile              # 中心端服务镜像
│   ├── edge/
│   │   └── Dockerfile              # 边端服务镜像
│   └── nginx/
│       ├── Dockerfile              # Nginx 镜像
│       └── nginx.conf              # Nginx 配置
├── docker-compose.yml              # 本地开发环境
├── docker-compose.prod.yml         # 生产环境
└── k8s/                            # Kubernetes 配置（可选）
    ├── center-deployment.yaml
    ├── center-service.yaml
    └── ...
```

### 2.2 中心端服务 Dockerfile

**deployments/docker/center/Dockerfile：**

```dockerfile
# Stage 1: 构建阶段
FROM golang:1.23-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make protobuf-dev

WORKDIR /build

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/center-server ./cmd/center

# Stage 2: 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# 创建非 root 用户
RUN addgroup -g 1001 igh && \
    adduser -D -u 1001 -G igh igh

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/center-server .

# 复制配置文件
COPY configs/center.yaml ./configs/

# 切换到非 root 用户
USER igh

# 暴露端口
EXPOSE 8080 50051 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["/app/center-server"]
CMD ["-config", "/app/configs/center.yaml"]
```

### 2.3 边端服务 Dockerfile

**deployments/docker/edge/Dockerfile：**

```dockerfile
# Stage 1: 构建阶段
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 编译（启用 CGO 用于 SQLite）
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /app/edge-server ./cmd/edge

# Stage 2: 运行阶段
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata sqlite

ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN addgroup -g 1001 igh && \
    adduser -D -u 1001 -G igh igh

WORKDIR /app

COPY --from=builder /app/edge-server .
COPY configs/edge.yaml ./configs/

# 创建数据目录
RUN mkdir -p /app/data && chown -R igh:igh /app/data

USER igh

EXPOSE 8081 50052 9091

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

ENTRYPOINT ["/app/edge-server"]
CMD ["-config", "/app/configs/edge.yaml"]
```

### 2.4 docker-compose 开发环境

**deployments/docker-compose.yml：**

```yaml
version: '3.9'

services:
  # PostgreSQL 数据库
  postgres:
    image: postgres:16-alpine
    container_name: igh-postgres
    environment:
      POSTGRES_USER: igh
      POSTGRES_PASSWORD: igh_password
      POSTGRES_DB: igh
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U igh"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - igh-network

  # Redis 缓存
  redis:
    image: redis:7-alpine
    container_name: igh-redis
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
    networks:
      - igh-network

  # 中心端服务
  center-server:
    build:
      context: ../
      dockerfile: deployments/docker/center/Dockerfile
    container_name: igh-center-server
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      - DATABASE_URL=postgres://igh:igh_password@postgres:5432/igh?sslmode=disable
      - REDIS_URL=redis://redis:6379/0
      - LOG_LEVEL=debug
    ports:
      - "8080:8080"    # HTTP
      - "50051:50051"  # gRPC
      - "9090:9090"    # Metrics
    volumes:
      - ./configs/center.yaml:/app/configs/center.yaml:ro
    networks:
      - igh-network
    restart: unless-stopped

  # 边端服务 1
  edge-server-1:
    build:
      context: ../
      dockerfile: deployments/docker/edge/Dockerfile
    container_name: igh-edge-server-1
    depends_on:
      - center-server
    environment:
      - CENTER_URL=center-server:50051
      - EDGE_ID=edge-001
      - LOG_LEVEL=debug
    ports:
      - "8081:8081"    # HTTP
      - "50052:50052"  # gRPC
      - "9091:9091"    # Metrics
    volumes:
      - ./configs/edge.yaml:/app/configs/edge.yaml:ro
      - edge_data_1:/app/data
    networks:
      - igh-network
    restart: unless-stopped

  # Prometheus 监控
  prometheus:
    image: prom/prometheus:latest
    container_name: igh-prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    ports:
      - "9092:9090"
    networks:
      - igh-network
    restart: unless-stopped

  # Grafana 可视化
  grafana:
    image: grafana/grafana:latest
    container_name: igh-grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
      - ./grafana/datasources:/etc/grafana/provisioning/datasources:ro
    ports:
      - "3000:3000"
    networks:
      - igh-network
    restart: unless-stopped

networks:
  igh-network:
    driver: bridge

volumes:
  postgres_data:
  redis_data:
  edge_data_1:
  prometheus_data:
  grafana_data:
```

---

## 三、双机热备方案

### 3.1 架构设计（基于 ADR-19）

**IGH-SilkGuard 双机热备系统：**

```
┌────────────────────────────────────────────────────┐
│              虚拟 IP (VIP)                         │
│            192.168.1.100                           │
└────────────────┬───────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │   Keepalived    │  (VRRP 协议)
        └────────┬────────┘
                 │
    ┌────────────┼────────────┐
    │                         │
┌───▼─────────┐       ┌──────▼──────┐
│  主服务器 (P)│       │  备服务器 (S)│
│  Priority: 100 │       │  Priority: 90  │
│  State: MASTER │       │  State: BACKUP │
│                │       │                │
│  [Center]      │       │  [Center]      │
│  [PostgreSQL]  │◄─────►│  [PostgreSQL]  │
│  [Redis]       │ 同步  │  [Redis]       │
└────────────────┘       └────────────────┘
```

### 3.2 Keepalived 配置

**主服务器配置（/etc/keepalived/keepalived.conf）：**

```conf
global_defs {
    router_id IGH_PRIMARY
    script_user root
    enable_script_security
}

vrrp_script check_center {
    script "/usr/local/bin/check_center.sh"
    interval 5
    weight -10
    fall 2
    rise 2
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    virtual_router_id 51
    priority 100
    advert_int 1
    
    authentication {
        auth_type PASS
        auth_pass igh_secret_2026
    }
    
    virtual_ipaddress {
        192.168.1.100/24 dev eth0
    }
    
    track_script {
        check_center
    }
    
    notify_master "/usr/local/bin/notify_master.sh"
    notify_backup "/usr/local/bin/notify_backup.sh"
    notify_fault "/usr/local/bin/notify_fault.sh"
}
```

**备服务器配置：**

```conf
global_defs {
    router_id IGH_BACKUP
    script_user root
    enable_script_security
}

vrrp_script check_center {
    script "/usr/local/bin/check_center.sh"
    interval 5
    weight -10
    fall 2
    rise 2
}

vrrp_instance VI_1 {
    state BACKUP
    interface eth0
    virtual_router_id 51
    priority 90  # 备服务器优先级低于主服务器
    advert_int 1
    
    authentication {
        auth_type PASS
        auth_pass igh_secret_2026
    }
    
    virtual_ipaddress {
        192.168.1.100/24 dev eth0
    }
    
    track_script {
        check_center
    }
    
    notify_master "/usr/local/bin/notify_master.sh"
    notify_backup "/usr/local/bin/notify_backup.sh"
    notify_fault "/usr/local/bin/notify_fault.sh"
}
```

### 3.3 健康检查脚本

**/usr/local/bin/check_center.sh：**

```bash
#!/bin/bash

# 检查中心端服务健康状态
# 返回 0 表示健康，非 0 表示故障

# 检查端口是否监听
if ! netstat -tuln | grep -q ":8080 "; then
    echo "HTTP port 8080 not listening"
    exit 1
fi

if ! netstat -tuln | grep -q ":50051 "; then
    echo "gRPC port 50051 not listening"
    exit 1
fi

# 检查 HTTP 健康端点
if ! curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo "HTTP health check failed"
    exit 1
fi

# 检查数据库连接
if ! PGPASSWORD=igh_password psql -h localhost -U igh -d igh -c "SELECT 1" > /dev/null 2>&1; then
    echo "PostgreSQL connection failed"
    exit 1
fi

# 检查 Redis 连接
if ! redis-cli ping > /dev/null 2>&1; then
    echo "Redis connection failed"
    exit 1
fi

echo "All checks passed"
exit 0
```

### 3.4 状态切换脚本

**/usr/local/bin/notify_master.sh：**

```bash
#!/bin/bash

# 切换为 MASTER 时执行
echo "$(date): Switched to MASTER" >> /var/log/keepalived-state.log

# 启动服务（如果未运行）
systemctl start igh-center || true
systemctl start postgresql || true
systemctl start redis || true

# 发送通知
curl -X POST "http://monitoring-server/api/alerts" \
    -H "Content-Type: application/json" \
    -d '{"level":"info","message":"IGH服务器切换为主服务器","host":"'$(hostname)'"}'
```

**/usr/local/bin/notify_backup.sh：**

```bash
#!/bin/bash

# 切换为 BACKUP 时执行
echo "$(date): Switched to BACKUP" >> /var/log/keepalived-state.log

# 发送通知
curl -X POST "http://monitoring-server/api/alerts" \
    -H "Content-Type: application/json" \
    -d '{"level":"info","message":"IGH服务器切换为备服务器","host":"'$(hostname)'"}'
```

### 3.5 PostgreSQL 流复制

**主服务器 postgresql.conf：**

```conf
# 复制设置
wal_level = replica
max_wal_senders = 5
wal_keep_size = 128MB
synchronous_commit = on
synchronous_standby_names = 'standby1'
```

**主服务器 pg_hba.conf：**

```conf
# 允许备服务器复制连接
host    replication     replicator      192.168.1.101/32      md5
```

**备服务器配置：**

```bash
# 创建 standby.signal 文件
touch /var/lib/postgresql/data/standby.signal

# 配置 primary_conninfo
cat >> /var/lib/postgresql/data/postgresql.auto.conf << EOF
primary_conninfo = 'host=192.168.1.100 port=5432 user=replicator password=repl_password application_name=standby1'
restore_command = 'cp /var/lib/postgresql/archive/%f %p'
EOF
```

### 3.6 Redis 主从复制

**备服务器 redis.conf：**

```conf
# 配置主服务器地址
replicaof 192.168.1.100 6379

# 如果主服务器设置了密码
masterauth igh_redis_password

# 只读模式（备服务器）
replica-read-only yes
```

---

## 四、生产环境部署

### 4.1 生产环境 docker-compose

**deployments/docker-compose.prod.yml：**

```yaml
version: '3.9'

services:
  # PostgreSQL 主库
  postgres-primary:
    image: postgres:16-alpine
    container_name: igh-postgres-primary
    environment:
      POSTGRES_USER: igh
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: igh
      POSTGRES_INITDB_ARGS: "-E UTF8 --locale=C"
    volumes:
      - /data/postgres/primary:/var/lib/postgresql/data
      - /data/postgres/archive:/var/lib/postgresql/archive
      - ./configs/postgres/primary/postgresql.conf:/etc/postgresql/postgresql.conf:ro
      - ./configs/postgres/primary/pg_hba.conf:/etc/postgresql/pg_hba.conf:ro
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
    ports:
      - "5432:5432"
    networks:
      - igh-network
    restart: always
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"

  # Redis 主库
  redis-primary:
    image: redis:7-alpine
    container_name: igh-redis-primary
    command: >
      redis-server
      --appendonly yes
      --requirepass ${REDIS_PASSWORD}
      --maxmemory 2gb
      --maxmemory-policy allkeys-lru
    volumes:
      - /data/redis/primary:/data
    ports:
      - "6379:6379"
    networks:
      - igh-network
    restart: always
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"

  # 中心端服务
  center-server:
    image: ghcr.io/yourorg/igh-center:${VERSION}
    container_name: igh-center-server
    depends_on:
      - postgres-primary
      - redis-primary
    environment:
      - ENV=production
      - DATABASE_URL=postgres://igh:${POSTGRES_PASSWORD}@postgres-primary:5432/igh?sslmode=require
      - REDIS_URL=redis://:${REDIS_PASSWORD}@redis-primary:6379/0
      - LOG_LEVEL=info
      - JAEGER_AGENT_HOST=jaeger
      - JAEGER_AGENT_PORT=6831
    ports:
      - "8080:8080"
      - "50051:50051"
      - "9090:9090"
    volumes:
      - /data/center/logs:/app/logs
      - /data/center/uploads:/app/uploads
      - ./configs/center.prod.yaml:/app/configs/center.yaml:ro
    networks:
      - igh-network
    restart: always
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"

  # Nginx 负载均衡
  nginx:
    image: nginx:alpine
    container_name: igh-nginx
    depends_on:
      - center-server
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
      - /data/nginx/logs:/var/log/nginx
    ports:
      - "80:80"
      - "443:443"
    networks:
      - igh-network
    restart: always
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "5"

networks:
  igh-network:
    driver: bridge
```

### 4.2 Nginx 配置

**deployments/nginx/nginx.conf：**

```nginx
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 4096;
    use epoll;
    multi_accept on;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for" '
                    'rt=$request_time uct="$upstream_connect_time" '
                    'uht="$upstream_header_time" urt="$upstream_response_time"';

    access_log /var/log/nginx/access.log main;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    client_max_body_size 100M;

    # Gzip 压缩
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript 
               application/json application/javascript application/xml+rss 
               application/rss+xml font/truetype font/opentype 
               application/vnd.ms-fontobject image/svg+xml;

    # 上游服务器（中心端）
    upstream center_http {
        least_conn;
        server center-server:8080 max_fails=3 fail_timeout=30s;
        keepalive 32;
    }

    upstream center_grpc {
        least_conn;
        server center-server:50051 max_fails=3 fail_timeout=30s;
    }

    # HTTP 服务
    server {
        listen 80;
        server_name igh.example.com;

        # 重定向到 HTTPS
        location / {
            return 301 https://$server_name$request_uri;
        }
    }

    # HTTPS 服务
    server {
        listen 443 ssl http2;
        server_name igh.example.com;

        # SSL 证书
        ssl_certificate /etc/nginx/ssl/cert.pem;
        ssl_certificate_key /etc/nginx/ssl/key.pem;
        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers on;
        ssl_session_cache shared:SSL:10m;
        ssl_session_timeout 10m;

        # API 路由
        location /api/ {
            proxy_pass http://center_http;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            
            proxy_connect_timeout 60s;
            proxy_send_timeout 60s;
            proxy_read_timeout 60s;
            proxy_buffering off;
        }

        # WebSocket 路由
        location /ws/ {
            proxy_pass http://center_http;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            
            proxy_connect_timeout 7d;
            proxy_send_timeout 7d;
            proxy_read_timeout 7d;
        }

        # 静态文件
        location /static/ {
            alias /usr/share/nginx/html/static/;
            expires 30d;
            add_header Cache-Control "public, immutable";
        }

        # 健康检查
        location /health {
            proxy_pass http://center_http/health;
            access_log off;
        }
    }

    # gRPC 服务
    server {
        listen 50051 http2;
        server_name _;

        location / {
            grpc_pass grpc://center_grpc;
            grpc_connect_timeout 60s;
            grpc_send_timeout 60s;
            grpc_read_timeout 60s;
            
            error_page 502 = /error502grpc;
        }

        location = /error502grpc {
            internal;
            default_type application/grpc;
            add_header grpc-status 14;
            add_header grpc-message "upstream unavailable";
            return 204;
        }
    }
}
```

---

## 五、监控方案

### 5.1 Prometheus 配置

**deployments/prometheus/prometheus.yml：**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'igh-production'
    region: 'china-east'

# 告警规则文件
rule_files:
  - 'alerts/*.yml'

# Alertmanager 配置
alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

# 抓取配置
scrape_configs:
  # 中心端服务 - 主服务器
  - job_name: 'center-primary'
    static_configs:
      - targets: ['192.168.1.100:9090']
        labels:
          role: 'center'
          instance: 'primary'

  # 中心端服务 - 备服务器
  - job_name: 'center-backup'
    static_configs:
      - targets: ['192.168.1.101:9090']
        labels:
          role: 'center'
          instance: 'backup'

  # 边端服务
  - job_name: 'edge'
    static_configs:
      - targets:
          - 'edge-001:9091'
          - 'edge-002:9091'
          - 'edge-003:9091'
        labels:
          role: 'edge'

  # PostgreSQL Exporter
  - job_name: 'postgres'
    static_configs:
      - targets:
          - 'postgres-exporter-primary:9187'
          - 'postgres-exporter-backup:9187'

  # Redis Exporter
  - job_name: 'redis'
    static_configs:
      - targets:
          - 'redis-exporter-primary:9121'
          - 'redis-exporter-backup:9121'

  # Node Exporter
  - job_name: 'node'
    static_configs:
      - targets:
          - '192.168.1.100:9100'  # 主服务器
          - '192.168.1.101:9100'  # 备服务器
```

### 5.2 告警规则

**deployments/prometheus/alerts/center.yml：**

```yaml
groups:
  - name: center_alerts
    interval: 30s
    rules:
      # 服务宕机
      - alert: CenterServiceDown
        expr: up{job="center-primary"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "中心端服务宕机"
          description: "主服务器中心端服务已宕机超过 1 分钟"

      # CPU 使用率高
      - alert: HighCPUUsage
        expr: process_cpu_seconds_total{job=~"center.*"} > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "CPU 使用率过高"
          description: "{{ $labels.instance }} CPU 使用率超过 80% 持续 5 分钟"

      # 内存使用率高
      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes{job=~"center.*"} / (1024*1024*1024) > 3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "内存使用率过高"
          description: "{{ $labels.instance }} 内存使用超过 3GB"

      # HTTP 错误率高
      - alert: HighHTTPErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "HTTP 5xx 错误率过高"
          description: "HTTP 5xx 错误率超过 5%"

      # gRPC 错误率高
      - alert: HighGRPCErrorRate
        expr: rate(grpc_server_handled_total{grpc_code!="OK"}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "gRPC 错误率过高"
          description: "gRPC 非 OK 响应率超过 5%"

      # 数据库连接池耗尽
      - alert: DatabasePoolExhausted
        expr: database_connections_in_use / database_connections_max > 0.9
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "数据库连接池即将耗尽"
          description: "数据库连接池使用率超过 90%"

      # Redis 内存使用高
      - alert: HighRedisMemory
        expr: redis_memory_used_bytes / redis_memory_max_bytes > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Redis 内存使用率过高"
          description: "Redis 内存使用率超过 80%"
```

**deployments/prometheus/alerts/edge.yml：**

```yaml
groups:
  - name: edge_alerts
    interval: 30s
    rules:
      # 边端服务离线
      - alert: EdgeServiceOffline
        expr: up{job="edge"} == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "边端服务离线"
          description: "边端设备 {{ $labels.instance }} 离线超过 5 分钟"

      # 边端同步队列堆积
      - alert: EdgeSyncQueueBacklog
        expr: edge_sync_queue_length > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "边端同步队列堆积"
          description: "边端设备 {{ $labels.instance }} 同步队列长度超过 1000"

      # SQLite 数据库大小
      - alert: EdgeDatabaseTooLarge
        expr: edge_sqlite_size_bytes / (1024*1024*1024) > 5
        for: 1h
        labels:
          severity: info
        annotations:
          summary: "边端数据库过大"
          description: "边端设备 {{ $labels.instance }} SQLite 数据库超过 5GB"
```

### 5.3 Grafana 仪表盘

**部署 Grafana 仪表盘配置：**

```yaml
# deployments/grafana/dashboards/dashboard.yaml
apiVersion: 1

providers:
  - name: 'IGH Dashboards'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
```

**关键指标面板：**

1. **系统概览面板**
   - 在线边端设备数量
   - 今日生产数量（锭）
   - 实时合格率
   - 系统健康状态

2. **服务性能面板**
   - HTTP/gRPC 请求 QPS
   - P50/P95/P99 延迟
   - 错误率
   - 数据库连接池状态

3. **业务指标面板**
   - 落纱操作统计
   - 质检操作统计
   - 打包操作统计
   - 入库操作统计

4. **基础设施面板**
   - CPU/内存使用率
   - 磁盘 I/O
   - 网络流量
   - 数据库性能

---

## 六、备份与恢复

### 6.1 数据库备份策略

**自动备份脚本（/usr/local/bin/backup_postgres.sh）：**

```bash
#!/bin/bash

# PostgreSQL 自动备份脚本
BACKUP_DIR="/data/backups/postgres"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="igh_backup_${DATE}.sql.gz"
RETENTION_DAYS=30

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 执行备份
PGPASSWORD=igh_password pg_dump -h localhost -U igh -d igh | gzip > "$BACKUP_DIR/$BACKUP_FILE"

if [ $? -eq 0 ]; then
    echo "$(date): Backup successful: $BACKUP_FILE" >> /var/log/postgres-backup.log
    
    # 清理旧备份（保留最近 30 天）
    find "$BACKUP_DIR" -name "igh_backup_*.sql.gz" -mtime +$RETENTION_DAYS -delete
    
    # 上传到远程存储（可选）
    # aws s3 cp "$BACKUP_DIR/$BACKUP_FILE" s3://igh-backups/postgres/
else
    echo "$(date): Backup failed!" >> /var/log/postgres-backup.log
    exit 1
fi
```

**定时任务（crontab）：**

```cron
# 每天凌晨 2 点备份数据库
0 2 * * * /usr/local/bin/backup_postgres.sh

# 每周日凌晨 3 点备份 SQLite（边端）
0 3 * * 0 /usr/local/bin/backup_edge_sqlite.sh
```

### 6.2 数据恢复流程

**PostgreSQL 恢复：**

```bash
#!/bin/bash

# 恢复 PostgreSQL 数据库
BACKUP_FILE=$1

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    exit 1
fi

# 解压并恢复
gunzip -c "$BACKUP_FILE" | PGPASSWORD=igh_password psql -h localhost -U igh -d igh

if [ $? -eq 0 ]; then
    echo "Database restored successfully"
else
    echo "Database restore failed!"
    exit 1
fi
```

---

## 七、部署清单

### 7.1 部署前检查清单

**硬件要求：**

| 组件 | CPU | 内存 | 磁盘 | 网络 |
|------|-----|------|------|------|
| **中心端（主）** | 8核 | 16GB | 500GB SSD | 千兆 |
| **中心端（备）** | 8核 | 16GB | 500GB SSD | 千兆 |
| **边端设备** | 4核 | 8GB | 256GB SSD | 千兆 |

**软件要求：**

- [x] Docker Engine 24.0+
- [x] Docker Compose 2.20+
- [x] PostgreSQL 16+
- [x] Redis 7+
- [x] Nginx 1.24+
- [x] Keepalived 2.2+

### 7.2 部署步骤

**步骤1：准备服务器环境**

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装 Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 安装 Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# 安装 Keepalived
sudo apt install -y keepalived
```

**步骤2：配置网络和防火墙**

```bash
# 开放端口
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 5432/tcp
sudo ufw allow 6379/tcp
sudo ufw allow 50051/tcp
sudo ufw enable

# 配置内核参数（允许绑定 VIP）
echo "net.ipv4.ip_nonlocal_bind = 1" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

**步骤3：部署主服务器**

```bash
# 克隆代码
git clone https://github.com/yourorg/igh-silkroad.git
cd igh-silkroad

# 配置环境变量
cp .env.example .env
vim .env  # 编辑配置

# 启动服务
cd deployments
docker-compose -f docker-compose.prod.yml up -d

# 检查服务状态
docker-compose ps
docker-compose logs -f center-server
```

**步骤4：配置 PostgreSQL 主从复制**

```bash
# 在主服务器创建复制用户
psql -U igh -d igh -c "CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'repl_password';"

# 在备服务器配置从库
# （参见 3.5 PostgreSQL 流复制）
```

**步骤5：配置 Keepalived**

```bash
# 复制配置文件
sudo cp configs/keepalived/primary.conf /etc/keepalived/keepalived.conf
sudo cp scripts/check_center.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/check_center.sh

# 启动 Keepalived
sudo systemctl enable keepalived
sudo systemctl start keepalived

# 检查 VIP 绑定
ip addr show | grep 192.168.1.100
```

**步骤6：部署边端服务**

```bash
# Windows 环境使用 Docker Desktop
# 或直接运行二进制文件

# 配置边端
cp configs/edge.example.yaml configs/edge.yaml
vim configs/edge.yaml  # 修改中心端地址

# 启动边端服务
./edge-server.exe -config configs/edge.yaml
```

**步骤7：验证部署**

```bash
# 检查服务健康
curl http://192.168.1.100:8080/health

# 检查数据库连接
docker exec igh-postgres-primary psql -U igh -d igh -c "SELECT version();"

# 检查 Redis
docker exec igh-redis-primary redis-cli ping

# 测试 gRPC
grpcurl -plaintext 192.168.1.100:50051 list
```

---

## 八、运维手册

### 8.1 日常运维任务

| 任务 | 频率 | 负责人 |
|------|------|--------|
| **检查服务状态** | 每天 | 运维 |
| **查看监控告警** | 每天 | 运维 |
| **数据库备份验证** | 每周 | DBA |
| **磁盘空间检查** | 每周 | 运维 |
| **日志清理** | 每月 | 运维 |
| **系统更新** | 每月 | 运维 |

### 8.2 故障处理流程

**场景1：主服务器宕机**

1. Keepalived 自动切换到备服务器
2. VIP 漂移到备服务器
3. 边端设备自动连接到新的主服务器
4. 修复原主服务器后，手动切回或保持当前状态

**场景2：数据库故障**

1. 检查 PostgreSQL 日志
2. 如果主库故障，手动提升备库为主库
3. 修复原主库后，重新配置为备库
4. 恢复主从复制

**场景3：边端设备离线**

1. 边端自动进入离线自治模式
2. 本地 SQLite 保存所有操作
3. 网络恢复后自动同步到中心端
4. 检查同步队列状态

### 8.3 升级流程

**滚动升级（零停机）：**

```bash
# 1. 升级备服务器
ssh backup-server
cd /opt/igh-silkroad
git pull origin main
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d

# 2. 验证备服务器
curl http://192.168.1.101:8080/health

# 3. 切换主备（Keepalived 自动切换）
ssh primary-server
sudo systemctl stop keepalived

# 4. 升级原主服务器（现在是备服务器）
git pull origin main
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d

# 5. 恢复 Keepalived
sudo systemctl start keepalived

# 6. 边端服务升级（分批进行）
# 每次升级 1-2 台，避免影响生产
```

---

## 九、总结

### 9.1 部署架构特点

✅ **双机热备** - 主备自动切换，RTO < 1分钟  
✅ **容器化部署** - Docker Compose，易于管理  
✅ **数据库高可用** - PostgreSQL 流复制 + Redis 主从  
✅ **负载均衡** - Nginx 反向代理  
✅ **边端自治** - SQLite 本地存储，离线可用  
✅ **监控告警** - Prometheus + Grafana + Alertmanager  
✅ **自动备份** - 每日数据库备份，保留 30 天  
✅ **滚动升级** - 零停机升级

### 9.2 部署完成度

```
✅ 系统需求 (100%)
✅ 数据库设计 (100%)
✅ API设计 (100%)
✅ Protobuf定义 (100%)
✅ 服务实现 (100%)
✅ PLC通信 (100%)
✅ 打印机驱动 (100%)
✅ 前端设计 (100%)
✅ 测试方案 (100%)
✅ 部署方案 (100%)
```

**设计文档完成度：100% (10/10)** 🎉

---

> **文档状态：** ✅ 部署方案完成！  
> **下一步：** 开始实现
