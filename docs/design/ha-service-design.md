# IGH-SilkGuard 高可用服务设计

> **文档编号:** HA-SERVICE-DESIGN-001  
> **版本:** 1.0  
> **基于:** ADR-19（双机热备）+ NFR-060~NFR-072（HA需求）  
> **日期:** 2026-09-28  
> **设计者:** 浮浮酱

---

## 一、设计概述

### 1.1 服务定位

**IGH-SilkGuard** 是一个独立的高可用（HA）管理服务，负责：

1. ✅ **主备服务器心跳检测** - 持续监控对端存活状态
2. ✅ **PostgreSQL流复制监控** - 检测主备数据库同步状态
3. ✅ **故障自动切换** - 主服务器故障时自动提升备服务器
4. ✅ **脑裂预防** - 确保同一时刻只有一个Active服务器
5. ✅ **通知边端设备** - 切换时通知所有边端设备新的连接地址

**与 igh-center 的关系：**
- IGH-SilkGuard 是**独立进程**，与 igh-center 解耦
- 通过 HTTP API 与 igh-center 通信（查询状态、触发切换）
- igh-center 不感知自己的主备角色，由 SilkGuard 控制

---

## 二、架构设计

### 2.1 部署架构（双IP模式）

```
┌─────────────────────────────────────────────────────┐
│                边端设备（N个）                        │
│  配置: primary_addr + standby_addr                  │
│  逻辑: 优先连primary，失败自动切standby             │
└────────────┬────────────────────────────────────────┘
             │
    ┌────────┼────────┐
    │                 │
┌───▼─────────────┐  │  ┌─────────────────▼────┐
│  主服务器 (P)    │  │  │  备服务器 (S)         │
│  192.168.1.101  │  │  │  192.168.1.102       │
│                 │  │  │                      │
│  [SilkGuard-P]  │◄─┼─►│  [SilkGuard-S]       │
│  role: Active   │  心跳 │  role: Standby       │
│                 │  │  │                      │
│  [igh-center]   │  │  │  [igh-center]        │
│  mode: RW       │  │  │  mode: RO            │
│                 │  │  │                      │
│  [PostgreSQL-P] │  │  │  [PostgreSQL-S]      │
│  role: Primary  │──┼─►│  role: Replica       │
│                 │ 流复制 │                      │
└─────────────────┘  │  └──────────────────────┘
```

**关键点：**
- ✅ **无VIP** - 边端直接配置两个真实IP
- ✅ **双向心跳** - 主备互相检测，防止单向故障
- ✅ **独立进程** - SilkGuard 独立于 igh-center，故障隔离

### 2.2 边端连接策略

**边端配置文件（edge.yaml）：**

```yaml
center:
  addresses:
    - host: 192.168.1.101
      port: 50051
      role: primary
      
    - host: 192.168.1.102
      port: 50051
      role: standby
  
  # 连接策略
  connection:
    strategy: failover            # 故障转移模式
    retry_interval: 5s            # 重连间隔
    health_check_interval: 30s    # 健康检查间隔
    max_retry: 3                  # 最大重试次数
```

**边端连接逻辑：**

```go
// internal/client/center_client.go
type CenterClient struct {
	primaryAddr   string
	standbyAddr   string
	currentConn   *grpc.ClientConn
	currentRole   string  // "primary" or "standby"
}

func (c *CenterClient) Connect(ctx context.Context) error {
	// 1. 优先连接主服务器
	if err := c.tryConnect(ctx, c.primaryAddr, "primary"); err == nil {
		return nil
	}
	
	log.Warn("主服务器连接失败，切换到备服务器")
	
	// 2. 主服务器失败，连接备服务器
	if err := c.tryConnect(ctx, c.standbyAddr, "standby"); err != nil {
		return fmt.Errorf("主备服务器均无法连接: %w", err)
	}
	
	return nil
}

func (c *CenterClient) tryConnect(ctx context.Context, addr string, role string) error {
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return err
	}
	
	// 健康检查
	if !c.healthCheck(conn) {
		conn.Close()
		return fmt.Errorf("健康检查失败")
	}
	
	c.currentConn = conn
	c.currentRole = role
	log.Infof("已连接到%s服务器: %s", role, addr)
	
	return nil
}

// 后台健康检查
func (c *CenterClient) startHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if !c.healthCheck(c.currentConn) {
			log.Error("当前连接不健康，尝试重连")
			c.Reconnect()
		}
	}
}
```

---

## 三、IGH-SilkGuard 服务设计

### 3.1 服务架构

```
┌─────────────────────────────────────────┐
│         IGH-SilkGuard Service           │
├─────────────────────────────────────────┤
│                                         │
│  ┌──────────────────────────────────┐  │
│  │     Heartbeat Manager            │  │
│  │  - 发送心跳到对端                 │  │
│  │  - 接收对端心跳                   │  │
│  │  - 心跳超时检测                   │  │
│  └──────────────────────────────────┘  │
│                                         │
│  ┌──────────────────────────────────┐  │
│  │   PostgreSQL Monitor             │  │
│  │  - 监控流复制状态                 │  │
│  │  - 检测主备延迟                   │  │
│  │  - 执行Promote操作                │  │
│  └──────────────────────────────────┘  │
│                                         │
│  ┌──────────────────────────────────┐  │
│  │   Failover Controller            │  │
│  │  - 故障检测决策                   │  │
│  │  - 切换流程编排                   │  │
│  │  - 脑裂预防                       │  │
│  └──────────────────────────────────┘  │
│                                         │
│  ┌──────────────────────────────────┐  │
│  │   Center Controller              │  │
│  │  - 通知igh-center切换角色         │  │
│  │  - 查询igh-center健康状态         │  │
│  └──────────────────────────────────┘  │
│                                         │
│  ┌──────────────────────────────────┐  │
│  │   State Machine                  │  │
│  │  - Active / Standby / Transition │  │
│  │  - 状态持久化                     │  │
│  └──────────────────────────────────┘  │
│                                         │
└─────────────────────────────────────────┘
```

### 3.2 状态机设计

```
                  ┌──────────┐
                  │  INIT    │
                  └────┬─────┘
                       │
              ┌────────┴────────┐
              │                 │
         ┌────▼─────┐     ┌────▼──────┐
         │ ACTIVE   │     │ STANDBY   │
         │ (主服务器) │     │ (备服务器) │
         └────┬─────┘     └────┬──────┘
              │                 │
              │  对端故障        │  主服务器故障
              │                 │
         ┌────▼─────┐     ┌────▼──────┐
         │ DEGRADED │     │ PROMOTING │
         │ (降级模式) │     │ (提升中)   │
         └────┬─────┘     └────┬──────┘
              │                 │
              │  对端恢复        │  提升完成
              │                 │
         ┌────▼─────┐     ┌────▼──────┐
         │ ACTIVE   │◄────┤ ACTIVE    │
         └──────────┘     └───────────┘
```

**状态说明：**

| 状态 | 说明 | PostgreSQL | igh-center | 行为 |
|------|------|------------|------------|------|
| **ACTIVE** | 主服务器模式 | Primary | RW | 接受边端连接 |
| **STANDBY** | 备服务器模式 | Replica | RO | 热备状态 |
| **PROMOTING** | 提升中 | Promoting | RO→RW | 正在切换 |
| **DEGRADED** | 降级模式 | Primary | RW | 对端故障，单机运行 |

### 3.3 配置文件

**silkguard.yaml：**

```yaml
# 服务器配置
server:
  host: 192.168.1.101
  role: primary  # primary | standby
  
  # 对端服务器地址
  peer:
    host: 192.168.1.102
    port: 8888

# 心跳配置
heartbeat:
  interval: 2s            # 发送心跳间隔
  timeout: 10s            # 心跳超时时间（5个心跳周期）
  max_miss: 5             # 最大丢失心跳数
  
# PostgreSQL监控
postgres:
  primary_dsn: "postgres://igh:password@localhost:5432/igh"
  replica_dsn: "postgres://igh:password@192.168.1.102:5432/igh"
  
  # 流复制监控
  replication:
    check_interval: 5s
    max_lag_bytes: 10485760  # 10MB
    
  # Promote配置
  promote:
    command: "pg_ctl promote -D /var/lib/postgresql/data"
    timeout: 30s

# igh-center控制
center:
  api_url: "http://localhost:8080"
  health_check_path: "/health"
  
  # 角色切换通知
  notify:
    switch_to_active_path: "/internal/ha/activate"
    switch_to_standby_path: "/internal/ha/standby"

# 脑裂预防
split_brain:
  method: fencing          # fencing | quorum
  
  # Fencing（隔离）策略
  fencing:
    fence_peer: true       # 是否主动隔离对端
    fence_command: "ssh root@192.168.1.102 'systemctl stop igh-center'"

# 日志
logging:
  level: info
  file: /var/log/igh-silkguard.log
```

### 3.4 心跳协议

**心跳消息格式（JSON over HTTP）：**

```json
POST http://192.168.1.102:8888/heartbeat

{
  "node_id": "primary-node-001",
  "timestamp": "2026-09-28T10:15:30Z",
  "role": "ACTIVE",
  "sequence": 12345,
  
  "health": {
    "postgres_status": "healthy",
    "postgres_lag_bytes": 0,
    "center_status": "running",
    "cpu_usage": 45.2,
    "memory_usage": 60.5
  },
  
  "signature": "sha256:abcdef..."
}
```

**心跳响应：**

```json
{
  "received": true,
  "my_role": "STANDBY",
  "my_sequence": 12346,
  "timestamp": "2026-09-28T10:15:30.500Z"
}
```

### 3.5 故障检测算法

**检测条件（AND逻辑）：**

```yaml
主服务器故障判定:
  1. 心跳超时（连续5次未收到）           AND
  2. igh-center健康检查失败（HTTP 503）  AND
  3. PostgreSQL连接失败                  AND
  4. 网络Ping失败（ICMP）

备服务器提升条件:
  1. 检测到主服务器故障                  AND
  2. 本机PostgreSQL Replica健康         AND
  3. 流复制延迟 < 10MB                   AND
  4. 本机未处于PROMOTING状态（防止重复提升）
```

**故障检测代码：**

```go
// internal/failover/detector.go
type FailoverDetector struct {
	lastHeartbeat    time.Time
	missedHeartbeats int
	mu               sync.RWMutex
}

func (d *FailoverDetector) CheckPeerHealth(ctx context.Context) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	// 1. 心跳检测
	if d.missedHeartbeats >= 5 {
		log.Warn("对端心跳超时，连续丢失5次心跳")
		
		// 2. igh-center健康检查
		if !d.checkCenterHealth() {
			log.Error("对端igh-center不健康")
			
			// 3. PostgreSQL连接检查
			if !d.checkPostgresHealth() {
				log.Error("对端PostgreSQL不可达")
				
				// 4. 网络Ping检查（最终确认）
				if !d.pingPeer() {
					log.Error("对端网络不可达（ICMP失败）")
					return false, fmt.Errorf("对端服务器完全故障")
				}
			}
		}
	}
	
	return true, nil
}

func (d *FailoverDetector) ShouldPromote() bool {
	// 1. 检测到主服务器故障
	healthy, _ := d.CheckPeerHealth(context.Background())
	if healthy {
		return false
	}
	
	// 2. 本机PostgreSQL健康
	if !d.checkLocalPostgres() {
		log.Error("本机PostgreSQL不健康，无法提升")
		return false
	}
	
	// 3. 流复制延迟检查
	lag := d.getReplicationLag()
	if lag > 10*1024*1024 { // 10MB
		log.Warnf("流复制延迟过大: %d bytes，暂不提升", lag)
		return false
	}
	
	// 4. 状态检查
	if d.currentState == StatePromoting {
		log.Warn("已在提升过程中，跳过")
		return false
	}
	
	return true
}
```

### 3.6 Failover流程

**自动故障切换步骤：**

```
1. 【检测阶段】
   ├─ 备服务器检测到主服务器故障
   ├─ 验证故障条件（心跳+健康检查+网络）
   └─ 记录故障时间和原因

2. 【决策阶段】
   ├─ 判断是否满足提升条件
   ├─ 检查流复制延迟
   └─ 确认无脑裂风险

3. 【隔离阶段】（可选，防止脑裂）
   ├─ SSH到主服务器执行fence命令
   └─ 停止主服务器的igh-center服务

4. 【提升PostgreSQL】
   ├─ 执行 pg_ctl promote
   ├─ 等待提升完成（最多30秒）
   └─ 验证提升成功（SELECT pg_is_in_recovery()）

5. 【切换igh-center】
   ├─ 调用本机igh-center API: POST /internal/ha/activate
   ├─ igh-center切换为RW模式
   └─ 等待igh-center就绪

6. 【通知边端】（可选，边端自动重连）
   ├─ 向监控系统发送告警
   └─ 记录切换日志

7. 【完成】
   ├─ 更新状态为ACTIVE
   └─ 记录切换完成时间
```

**代码实现：**

```go
// internal/failover/controller.go
func (c *FailoverController) ExecuteFailover(ctx context.Context) error {
	log.Info("========== 开始故障切换 ==========")
	
	// 1. 更新状态为PROMOTING
	c.setState(StatePromoting)
	
	// 2. 隔离对端（可选）
	if c.config.SplitBrain.Fencing.FencePeer {
		if err := c.fencePeer(ctx); err != nil {
			log.Warnf("隔离对端失败: %v，继续切换", err)
		}
	}
	
	// 3. 提升PostgreSQL
	log.Info("提升PostgreSQL为Primary...")
	if err := c.promotePostgres(ctx); err != nil {
		c.setState(StateStandby)
		return fmt.Errorf("PostgreSQL提升失败: %w", err)
	}
	
	// 4. 切换igh-center为Active
	log.Info("激活igh-center...")
	if err := c.activateCenter(ctx); err != nil {
		c.setState(StateStandby)
		return fmt.Errorf("igh-center激活失败: %w", err)
	}
	
	// 5. 更新状态为ACTIVE
	c.setState(StateActive)
	
	// 6. 发送告警通知
	c.sendAlert("故障切换完成", "备服务器已提升为主服务器")
	
	log.Info("========== 故障切换完成 ==========")
	return nil
}

func (c *FailoverController) promotePostgres(ctx context.Context) error {
	// 执行promote命令
	cmd := exec.CommandContext(ctx, "pg_ctl", "promote", "-D", "/var/lib/postgresql/data")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_ctl promote失败: %w, output: %s", err, output)
	}
	
	// 等待提升完成（轮询pg_is_in_recovery）
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			return fmt.Errorf("PostgreSQL提升超时")
		case <-ticker.C:
			if !c.isPostgresInRecovery() {
				log.Info("PostgreSQL已成功提升为Primary")
				return nil
			}
		}
	}
}

func (c *FailoverController) activateCenter(ctx context.Context) error {
	url := c.config.Center.APIURL + "/internal/ha/activate"
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("调用igh-center激活API失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("igh-center激活失败，状态码: %d", resp.StatusCode)
	}
	
	return nil
}
```

---

## 四、脑裂预防

### 4.1 脑裂场景

**什么是脑裂：**
- 主备服务器网络分区，互相认为对方故障
- 两台服务器同时以Primary角色运行
- **后果：** 数据不一致、边端连接混乱

**脑裂预防策略：**

| 策略 | 原理 | 优点 | 缺点 |
|------|------|------|------|
| **Fencing（隔离）** | 提升前先SSH停止对端服务 | 简单有效 | 需要SSH免密 |
| **Quorum（仲裁）** | 需要第三方仲裁节点投票 | 严格保证 | 需要额外节点 |
| **STONITH** | 物理断电对端服务器 | 最彻底 | 硬件要求高 |

**IGH推荐方案：Fencing（隔离）**

### 4.2 Fencing实现

```go
// internal/splitbrain/fencing.go
func (f *FencingController) FencePeer(ctx context.Context, peerHost string) error {
	log.Warnf("开始隔离对端服务器: %s", peerHost)
	
	// 1. SSH连接对端
	client, err := f.sshConnect(peerHost)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()
	
	// 2. 停止igh-center服务
	session, _ := client.NewSession()
	defer session.Close()
	
	if err := session.Run("systemctl stop igh-center"); err != nil {
		log.Errorf("停止对端igh-center失败: %v", err)
	}
	
	// 3. 停止PostgreSQL（可选，更激进）
	// session2, _ := client.NewSession()
	// session2.Run("systemctl stop postgresql")
	
	log.Info("对端服务器已隔离")
	return nil
}
```

---

## 五、igh-center HA适配

### 5.1 内部HA API

**igh-center需要提供的内部API：**

```go
// internal/api/ha_controller.go
type HAController struct {
	mode string  // "RW" or "RO"
	mu   sync.RWMutex
}

// POST /internal/ha/activate - 切换为Active模式
func (c *HAController) Activate(ctx *gin.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	log.Info("收到SilkGuard激活请求")
	
	// 1. 切换为RW模式
	c.mode = "RW"
	
	// 2. 启用写操作
	database.EnableWrites()
	
	// 3. 启用后台任务
	scheduler.StartAll()
	
	ctx.JSON(200, gin.H{"success": true, "mode": "RW"})
}

// POST /internal/ha/standby - 切换为Standby模式
func (c *HAController) Standby(ctx *gin.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	log.Info("收到SilkGuard待机请求")
	
	// 1. 切换为RO模式
	c.mode = "RO"
	
	// 2. 禁用写操作
	database.DisableWrites()
	
	// 3. 停止后台任务
	scheduler.StopAll()
	
	ctx.JSON(200, gin.H{"success": true, "mode": "RO"})
}

// GET /health - 健康检查（SilkGuard调用）
func (c *HAController) Health(ctx *gin.Context) {
	healthy := true
	
	// 检查数据库连接
	if err := database.Ping(); err != nil {
		healthy = false
	}
	
	// 检查Redis连接
	if err := redis.Ping(); err != nil {
		healthy = false
	}
	
	status := "healthy"
	code := 200
	if !healthy {
		status = "unhealthy"
		code = 503
	}
	
	ctx.JSON(code, gin.H{
		"status": status,
		"mode":   c.mode,
	})
}
```

### 5.2 数据库连接控制

```go
// internal/database/connection.go
type DBConnectionPool struct {
	readOnly bool
	mu       sync.RWMutex
}

func (p *DBConnectionPool) EnableWrites() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.readOnly = false
	log.Info("数据库写操作已启用")
}

func (p *DBConnectionPool) DisableWrites() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.readOnly = true
	log.Warn("数据库写操作已禁用（只读模式）")
}

func (p *DBConnectionPool) Exec(query string, args ...interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if p.readOnly && isWriteQuery(query) {
		return fmt.Errorf("当前为Standby模式，禁止写操作")
	}
	
	return p.db.Exec(query, args...)
}
```

---

## 六、监控与告警

### 6.1 Prometheus指标

```go
// internal/metrics/ha_metrics.go
var (
	haRole = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "silkguard_role",
			Help: "当前HA角色 (1=ACTIVE, 0=STANDBY)",
		},
		[]string{"node"},
	)
	
	haHeartbeatMissed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "silkguard_heartbeat_missed_total",
			Help: "丢失的心跳数",
		},
		[]string{"peer"},
	)
	
	haFailoverTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "silkguard_failover_total",
			Help: "故障切换次数",
		},
	)
	
	haFailoverDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "silkguard_failover_duration_seconds",
			Help:    "故障切换耗时",
			Buckets: []float64{1, 5, 10, 30, 60},
		},
	)
)
```

### 6.2 告警规则

```yaml
# prometheus/alerts/ha.yml
groups:
  - name: ha_alerts
    rules:
      # 心跳丢失告警
      - alert: HAHeartbeatLost
        expr: rate(silkguard_heartbeat_missed_total[1m]) > 0.5
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "IGH-SilkGuard心跳丢失"
          description: "对端服务器心跳丢失率超过50%"
      
      # 故障切换告警
      - alert: HAFailoverOccurred
        expr: increase(silkguard_failover_total[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "IGH系统发生故障切换"
          description: "主备服务器已切换，请检查原主服务器"
      
      # 单机运行告警
      - alert: HARunningAlone
        expr: silkguard_role{node="primary"} == 1 AND silkguard_heartbeat_missed_total > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "IGH系统单机运行"
          description: "备服务器长时间无响应，系统处于降级模式"
```

---

## 七、运维手册

### 7.1 日常检查

```bash
# 检查SilkGuard服务状态
systemctl status igh-silkguard

# 查看当前HA角色
curl http://localhost:8888/status

# 查看心跳状态
journalctl -u igh-silkguard -f | grep heartbeat

# 检查PostgreSQL流复制
psql -U igh -d igh -c "SELECT * FROM pg_stat_replication;"
```

### 7.2 手动切换

```bash
# 主服务器主动让出（优雅切换）
curl -X POST http://localhost:8888/switchover

# 强制提升备服务器为主服务器
curl -X POST http://192.168.1.102:8888/promote --data '{"force": true}'
```

### 7.3 故障恢复

**场景1：主服务器故障后恢复**

```bash
# 1. 修复主服务器
# 2. 重新配置为Replica
vi /var/lib/postgresql/data/recovery.conf
# 3. 重启PostgreSQL
systemctl restart postgresql
# 4. 启动SilkGuard（自动进入Standby模式）
systemctl start igh-silkguard
```

---

## 八、总结

### 8.1 设计特点

✅ **双IP模式** - 边端配置主备IP，自主切换  
✅ **独立进程** - SilkGuard与igh-center解耦  
✅ **自动切换** - 故障检测+自动Failover  
✅ **脑裂预防** - Fencing隔离机制  
✅ **PostgreSQL集成** - 流复制监控+Promote  
✅ **监控完善** - Prometheus指标+告警

### 8.2 与部署方案的集成

deployment-strategy.md中需要补充：
- 删除Keepalived + VIP方案
- 使用本文档的双IP模式
- 添加IGH-SilkGuard服务部署

---

> **文档状态：** ✅ HA服务设计完成！  
> **下一步：** 更新deployment-strategy.md，移除VIP，采用双IP模式
