# 双机热备（HA）设计差距分析与实施规格

> **文档性质：** 设计规格（spec），后续可拆解为实施计划
> **日期：** 2026-11-10
> **状态：** 草案 —— 待主人裁定第 3 节的决策点
> **前置：** ADR-19（`docs/design/system-requirements.md:162`）、NFR-001..008（同文件 `:1402`）
> **结论先行：** 本仓库当前**没有任何 HA 代码**。全部 HA 内容停留在设计文档层，且旧设计文档中至少有 5 处与现状不符。建议 v1 只做 PostgreSQL 层，Go 侧改造仅限于加一个真实就绪探针与关闭启动期 DDL。

---

## 一、现状

### 1.1 设计文档层（设计完成，未实现）

| 资产 | 位置 | 内容 | 与现实的关系 |
|------|------|------|--------------|
| ADR-19 | `docs/design/system-requirements.md:162-173` | 双机热备、流复制、双 IP 无 VIP、SilkGuard 独立服务 | 决策有效，命名已过期（见 2.2） |
| NFR-001..008 | `docs/design/system-requirements.md:1404-1413` | 双服务器热备 / 流复制 / 应用心跳 / 自动转移 / 边端双地址 / 边端自治 / 零丢失 / RTO ≤ 30s | 未分配任务，未排期 |
| HA 服务设计 | `docs/design/ha-service-design.md`（857 行，2026-09-28） | IGH-SilkGuard 完整设计：状态机、心跳协议、故障检测、Failover 流程、脑裂与 Fencing、内部 HA API、Prometheus 指标、运维手册 | 架构可用，**细节多处失效** |
| 双机部署方案 | `docs/design/deployment-strategy.md` §三（`:336-580`） | Keepalived + VIP + 流复制配置 | **与 ADR-19 直接冲突** |
| 数据库设计 | `docs/design/database-design.md` | 引用 ADR-19 | 无实现影响 |

### 1.2 代码层（零实现）

以下检索全部返回空，可复核：

```
grep -rni "silkguard|pg_promote|internal/ha|failover|replication" --include=*.go .   → 无命中
grep -rn  "redis|Redis"       --include=*.go .                                        → 无命中
grep -rn  "prometheus"        --include=*.go .                                        → 无命中
grep -rn  "os.Create|os.WriteFile|os.OpenFile|os.MkdirAll" --include=*.go internal/ cmd/ api/  → 无命中
```

`go.mod` 中不存在 Redis 客户端、Prometheus 客户端、gRPC。`configs/` 目录为空。`docs/superpowers/plans/` 下无 HA 相关计划。

**当前 Center 的全部外部依赖只有两个：PostgreSQL 和一个监听端口。**

### 1.3 中心端进程现状

`cmd/center-server/main.go`：

| 环节 | 行号 | 行为 | HA 影响 |
|------|------|------|---------|
| DSN | `:23` | `DATABASE_URL` 环境变量，缺省 `localhost:5432/igh` | 单点单值，无主备切换能力 |
| 连接 | `:24` | `ent.Open(dialect.Postgres, dsn)` | **惰性连接**，不 Ping；连不上也不阻塞 |
| 迁移 | `:33-37` | `ENV == "development"` 时执行 `client.Schema.Create` | **默认值就是 development**（`getEnv("ENV","development")`），生产忘设即为双写 DDL |
| 迁移失败 | `:35` | 仅 `log.Printf` 警告，继续启动 | 迁移失败被静默吞掉 |
| 端口 | `:44-51` | `PORT`，默认 8080 | 两台机器可各自监听 8080，无冲突 |
| JWT 密钥 | `:40` | `JWT_SECRET` 环境变量 | **必须两台一致**，否则切换后 token 全失效 |
| 关闭 | `:69-81` | SIGINT/SIGTERM，5s 超时 | 无 drain 期间的就绪标记翻转 |

`internal/server/center.go`：

| 环节 | 行号 | 行为 |
|------|------|------|
| 健康检查 | `:110-115` | 恒返回 `200` + 硬编码 `"status":"ok"`，**不查数据库** |
| JWT 中间件 | `:267-288` | 仅验签，无状态 |
| `requireUserPrincipal` | `:297-315` | 拒绝设备凭证，无状态 |
| `requireActiveUser` | `user_state_middleware.go:144-192` | 每请求查库刷新角色/激活态；**失败即 fail closed** |
| 设备鉴权 | `:402-443` | 四重校验：签名 → 已注册 → code 匹配 → **源 IP 匹配** |
| 上传处理 | `:495` | 委托 `center.UploadHandler` |
| 心跳写库 | `:502` | `UpdateHeartbeat`，失败仅记日志 |

**关键结论：Center 进程内除 `uploadMu` 之外没有任何业务状态，且 `uploadMu` 在 Edge 侧。** 所有授权状态、设备注册、业务数据都在 PostgreSQL 中。Center 本身足够无状态，可以直接在提升后的副本上重启 —— 唯一障碍是启动期的 schema 迁移（见 4.5）。

### 1.4 边端进程现状

`cmd/edge-server/main.go`：

| 配置 | 行号 | 说明 |
|------|------|------|
| `EDGE_ID` | `:53-56` | 设备编码，须与 Center 注册的 `edge_code` 一致 |
| `DATABASE_PATH` | `:58-61` | SQLite 本地库，默认 `./edge.db` |
| `CENTER_URL` | `:73-76` | **单一地址**，默认 `http://localhost:8080` |
| `CENTER_TOKEN` | `:85-89` | 设备凭证，缺失直接 `log.Fatal` |
| `SYNC_INTERVAL` | `:92` | 兜底重试间隔，默认 5m |

`internal/sync/edge/uploader.go`：

| 行为 | 行号 | 说明 |
|------|------|------|
| 构造 | `:69-76` | `centerURL` 单值，`http.Client{Timeout: 30s}` |
| 并发保护 | `:71` + `:529-532` | `TryLock`，重入返回 `ErrUploadInProgress` |
| URL 拼接 | `:571` | 每次上传现拼 `{centerURL}/v1/edges/{edgeID}/upload` |
| 网络失败 | `:585-592` | 传输层错误 → 整批 `markRetried` |
| 非 200 | `:602-608` | 一律 `markRetried` |
| 401/403 | `:596-601` | 不计重试，保持 pending |
| 重试上限 | `:29` | `maxRetryCount = 5` |
| 超限翻牌 | `:862-910` | `markExhausted` → `status = failed` |
| 回收 | `resync.go` | 需人工 `POST /v1/sync/resync` |

`internal/sync/edge/scheduler.go:34-65`：每 `interval` 触发一次 `Upload`，是网络故障后的唯一兜底。

### 1.5 同步协议现状

`internal/sync/center/handler.go`：

- `HandleUpload`（`:71`）逐条处理，无事务，单条失败仅累加 `rejected` 并继续（`:99-107`）。
- `handleCreate`（`:136-149`）只接受 `lots` / `bobbins` / `doffings` / `bobbin_grades` 四张表。
- 全部为 **upsert 语义**：先按 ID 查，未命中再按业务编号查（`upsertLot:151-229`、`upsertBobbin:307-370`），编号命中则“认领” Center 侧既有行并保留其 ID。
- `handleUpdate` / `handleDelete`（`:604-613`）**显式返回错误**，不静默成功。
- `models.UploadResponse.NewCursor`（`models/models.go:40`）声明了但**从未赋值**；`UploadRequest.Cursor`（`:32`）被接收但**从未使用**。游标协议是空壳。

**结论：应用层 RPO 事实上为零。** 重传、重试、网络重发天然幂等，Edge 侧 `synced` 状态不会被 Center 的重复写入破坏。

---

## 二、设计资产盘点

### 2.1 `ha-service-design.md` 中可直接复用的部分

- **双 IP、无 VIP 的架构取向**（`§2.1`）—— 与 ADR-19 一致，选型正确，回避了 VRRP 在工厂网段的组播问题。
- **双中心心跳而非单向探测**（`§2.1`、`§3.4`）—— JSON over HTTP，字段含 `role` / `sequence` / `health` / `signature`，比 ICMP 可靠。
- **状态机四态划分**（`§3.2`）—— `ACTIVE` / `STANDBY` / `PROMOTING` / `DEGRADED`，语义清晰，`DEGRADED` 覆盖“对端挂了但我还在跑”的中间态，这是很多设计漏掉的。
- **提升前置条件**（`§3.5`）—— 复制延迟阈值（10MB）、`pg_is_in_recovery()` 轮询确认、防重复提升。
- **Fencing 先于 Promote 的顺序**（`§3.6` 步骤 3→4）—— 顺序正确。
- **Prometheus 指标骨架**（`§6.1`）—— `role` / `heartbeat_missed_total` / `failover_total` / `failover_duration_seconds` 四个指标覆盖了观测面。

### 2.2 设计中已不成立的前提

| # | 位置 | 旧假设 | 现状 |
|---|------|--------|------|
| 1 | `ha-service-design.md:5` | “NFR-060~NFR-072（HA需求）” | 实际编号是 **NFR-001..008**（`system-requirements.md:1404-1413`）。引用失效 |
| 2 | `:24-26,:50,228,271,288,580` | 服务名 **`igh-center`** | 本仓库是 **`cmd/center-server`**，二进制 `center-server`。全篇需改名 |
| 3 | `:72,:76,:90-118` | 边端以 **gRPC + `edge.yaml` 多地址**连 Center（`internal/client/center_client.go`） | 实际是 **HTTP/JSON + 单个 `CENTER_URL` 环境变量**（`cmd/edge-server/main.go:73`），无 yaml 配置层，`internal/client/` 不存在 |
| 4 | `:339` | 故障判定第二条件：“igh-center 健康检查失败（HTTP 503）” | Center 的 `/health` **恒返 200**（`internal/server/center.go:110-115`）。该条件**永不成立**，四条件 AND 退化为仅 ICMP |
| 5 | `:619-621` | 调用 `database.EnableWrites()` / `scheduler.StartAll()` | 两者**均不存在**。Center 无后台调度器，无读写模式门 |
| 6 | `:643` | `Health()` 中 ping **Redis** | 仓库**无 Redis 依赖**（`go.mod` 无 redis 客户端） |
| 7 | `:827` | “编辑 `/var/lib/postgresql/data/recovery.conf`” 恢复为副本 | **PostgreSQL ≤ 11 的做法**。目标版本是 **PG 16**（`deployment-strategy.md:203,:593`），必须改用 `standby.signal` + `primary_conninfo`（同文件 §3.5 `:531-562` 已写对） |
| 8 | `:288` | `systemctl stop igh-center` | 服务单元名随改名失效 |
| 9 | `§8.2`（`:847-856`） | 要求 deployment-strategy 移除 VIP 改用双 IP | **从未执行**。`deployment-strategy.md:338-452` 仍是 Keepalived + VIP + `192.168.1.100` |

> 第 9 条最危险：两份设计文档互相矛盾，而运维会照 `deployment-strategy.md` 执行，因为它有完整可复制的配置文件。

### 2.3 设计留白（未规定）

- **Center 在 Standby 态该做什么**：文档只说“RO 模式”，未定义只读副本上 `requireActiveUser` 每请求查库的成本、以及 `/health` 应返回什么。
- **失败回切（failback）**：文档只覆盖“主机故障后修复”，未规定修好的旧主重新加入时是自动降级还是需人工。
- **切换期间的 Edge 行为**：文档假设“边端自动重连”，但未定义重连窗口内 `retry_count` 增长会不会耗尽 5 次预算。
- **认证**：`/internal/ha/activate` 无鉴权方案（`§5.1:603-641` 直接裸奔）。
- **时钟**：心跳 `sequence` 与 `timestamp` 都未规定单调时钟要求。

---

## 三、必须解决的决策点（按影响排序）

### D1. SilkGuard 是独立二进制，还是 center-server 的一个模式？【最高优先级】

| 选项 | 优点 | 缺点 |
|------|------|------|
| A. 独立二进制 `cmd/silkguard` | 完全故障隔离，符合 ADR-19；切换逻辑崩了不影响业务进程 | 多一个部署单元、多一套日志/升级/监控；两进程需共享配置 |
| B. `center-server --ha-role` 模式 | 部署单元不变，运维面最小；天然共享 DSN 与 JWT 配置 | SilkGuard 故障会连带业务进程；进程需同时监听业务端口与心跳端口 |
| C. 先 B 后 A | 用最小代价验证状态机与 PG 提升，跑通后再拆 | 走两遍 |

**建议：C。** 理由：D2 若选“仅 PG 层”，A 的独立性毫无价值——没有需要隔离的业务进程逻辑。状态机、心跳、提升、`/internal/ha/activate` 全部先在一个进程内验证，跑通后再拆出 `cmd/silkguard`，届时是纯搬运，不改逻辑。

### D2. 最小可用 HA（仅 PG 层）还是完整 HA？【决定工期】

| 选项 | 覆盖 NFR | 需要改 Go 吗 | 工期量级 |
|------|---------|-------------|---------|
| A. 仅 PG 流复制 + 手动 promote + 运维手册 | NFR-002、NFR-007 | **否** | 天 |
| B. A + Center 就绪探针 + 关闭启动期 DDL | + 为 NFR-004 铺路 | 是，约 30 行 | 天 |
| C. B + SilkGuard 自动检测与提升 | + NFR-003、NFR-004、NFR-008 | 是，一个包 | 周 |
| D. C + 边端双地址 | + NFR-005 | 是，改 uploader/scheduler | 周 |

**建议：先 B，再评估 C。** 关键论据是时限：网络失败后 Edge 会在 **约 25 分钟**（`maxRetryCount=5` × 默认 `SYNC_INTERVAL=5m`）内耗尽预算把记录翻成 `failed`（`uploader.go:29`、`:862-910`），而 `failed` 只有人工调 `POST /v1/sync/resync` 才回收（`resync.go`）。所以**任何自动方案，其切换总时长必须显著小于 25 分钟**，否则“零丢失”在 Edge 侧不成立——数据没丢，但自动化链条断了。NFR-008 的 RTO ≤ 30s 连 PG 提升（文档自己给 30s 超时，`:506-524`）都只是勉强够，加上检测窗口（`max_miss: 5` × `interval: 2s` = 10s）就已超 30s。**建议主人重新裁定 NFR-008 的数值**，或把它降为 P2。

### D3. Fencing 机制选型

| 选项 | 说明 | 前提 |
|------|------|------|
| A. SSH 停止对端服务（文档现状，`:288,:562-591`） | 简单有效 | 需 root 免密互通；仓库无任何密钥分发方案 |
| B. Docker API | 容器场景下禁用对端容器 | 生产是否容器化未定；`deployments/` 目前只有 edge |
| C. 人工确认 | 提升前要求运维确认 | 失去自动化，但**不引入新的信任面** |
| D. 无 fencing | 仅靠“不满足提升条件就不提升” | 脑裂风险直接暴露 |

**建议：v1 选 C，并把 D 作为降级明确记录。** 理由：A 要求两机 root 免密 SSH，这本身是一个比脑裂更难审计的安全面；而本仓库目前**没有容器化生产部署**（`deployments/` 下只有 `edge/`），B 无落点。人工确认可以配合“提升前先 `SELECT pg_is_in_recovery()` + 人工断网”的 runbook 走，先把流程跑顺。等 B 的条件具备再切。

### D4. Center 是否必须做到“可在副本上直接重启”？

**建议：是，且这是 v1 的必做项。** 今天 Center 已经足够无状态（1.3 节），唯一两个障碍都是小改动：

- `/health` 不查库，无法表达就绪（`center.go:110-115`）；
- `ENV` 默认 `development`，`Schema.Create` 会在提升后的副本上跑 DDL（`main.go:33`）。

第一条不修，任何自动检测都退化成 ICMP；第二条不修，两台 Center 在 development 下并发 DDL。**这两条合计不超过 30 行，是整份规格里性价比最高的改动。**

### D5. 边端双地址是否进 v1？

**建议：不进。** 当前 Edge 只有单 `CENTER_URL`（`main.go:73`），引入多地址需要：配置格式（仓库无 yaml 配置层）、`Uploader` 持有地址列表并记录当前活跃地址（`uploader.go:69-76` 的 `centerURL` 是值类型）、失败切换策略、以及 `deployments/edge/docker-compose.yml:18` 的环境变量扩展。这是一个独立特性，与 Center 侧 HA 正交。**在 Center 侧尚未稳定前先做 Edge 侧，会让两个变量同时动。**

### D6. 两台 Center 的 `JWT_SECRET` 与 `EDGE_TOKEN_TTL` 是否需要强制一致检查？

**建议：是，且应写入启动自检。** 设备凭证默认 30 天（`api/middleware/jwt.go:41`），且**没有任何续期路径**（同文件 `:27-35` 的注释已明确）。切换后若备用机的 `JWT_SECRET` 与主机不同，所有 Edge 立即 401，而 401 **不计重试**（`uploader.go:596-601`）—— 上传静默停止，`/health` 仍返回 ok。这是最容易在切换当晚发生、也最难排查的故障。

### D7. 提升后的旧主如何回切？

**建议：v1 明确为“不回切，人工重建为副本”。** 文档 `§7.3`（`:820-833`）已描述该流程，只需把 `recovery.conf` 换成 PG 16 的 `standby.signal`（见 2.2 第 7 条）。自动回切需要数据一致性校验，超出 v1。

---

## 四、与当前代码的差距

### 4.1 边端寻址（差距：无任何故障转移）

- **现状：** `CENTER_URL` 单值（`cmd/edge-server/main.go:73-76`），`Uploader` 在 `:69-76` 固化该值，`:571` 每次拼 URL 只用它。
- **差距：** 无地址列表、无健康探测、无切换。主机宕机后 Edge 只会不断重试同一地址，直到 5 次耗尽。
- **v1 建议：不改。** 若要改，最小形态是在 `Uploader` 上加 `centerURLs []string` + `activeIdx`，并在 `:585-592` 的传输层错误分支里推进下标。契约形状：

```go
// 仅示意形状，非实现
type Uploader struct {
    centerURLs []string   // 有序，首个为 primary
    activeIdx  int        // 受 uploadMu 保护
    // ...
}
```

### 4.2 Center 就绪端点（差距：`/health` 无意义）

- **现状：** `center.go:110-115` 恒返 `200`，不触碰数据库。
- **差距：** 无法表达“我已连上可写的主库”。旧设计的检测条件 2（`ha-service-design.md:339`）因此恒不成立。
- **建议：** 新增 `/health/ready`，内部执行一次 `SELECT 1`（并可选检查 `pg_is_in_recovery()`），失败返 `503`。`/health` 保持现状不动——它被 `deployments/edge/docker-compose.yml:31` 的 healthcheck 与运维脚本依赖，改变语义会破坏既有用法。

```go
// 仅示意形状
func (s *CenterServer) handleReady(c *gin.Context) {
    if err := s.client.Driver.(*entsql.Driver).DB().PingContext(c.Request.Context()); err != nil {
        c.JSON(http.StatusServiceUnavailable, api.Error(api.CodeServerError, "数据库不可用"))
        return
    }
    c.JSON(http.StatusOK, api.Success(gin.H{"status": "ready"}))
}
```

### 4.3 数据库访问模式（差距：无读写门）

- **现状：** 单一 `*ent.Client`（`main.go:24`），无读写区分。
- **差距：** 副本上的 Center 无法拒绝写请求；`requireActiveUser`（`user_state_middleware.go:167`）每请求查库，在只读副本上也只能读，不受影响，但任何写路径都会以 PG 的 `cannot execute INSERT in a read-only transaction` 形式暴露为 500。
- **建议：** v1 不做写门。副本上的 Center 由运维**不启动**来保证不自相矛盾；只启动 PG 副本。这是 D2 选 B 的直接推论。若将来需要，应在中间件层加一个 `requireWritable`，而不是在 DB 层拦 SQL（后者无法可靠识别写语句）。

### 4.4 同步幂等（差距：已满足，但有一个隐患）

- **现状：** upsert + 业务编号认领（`handler.go:151-229,:307-370`），`update`/`delete` 显式拒绝（`:604-613`）。**重传安全。**
- **隐患：** `HandleUpload` 逐条无事务（`:99-107`）。一批评审中途失败会产生“部分已应用、部分未应用”的中间态。这对 HA 并非致命（Edge 重传会补齐），但会让“applied=N”的计数在切换窗口内不可用于判定。**建议 v1 接受，不引入事务**——引入事务会与现有的“逐条 rejected 不中断”语义冲突。

### 4.5 启动期 DDL（差距：默认开启）

- **现状：** `main.go:33-37`，`ENV` 缺省 `development`，`Schema.Create` 在 `/health` 之前跑。
- **差距：** 提升后的副本处于可写状态，两台 Center 同时启动即为并发 DDL。而迁移失败只 `log.Printf`（`:35`），会被忽略。
- **建议：** 反转默认值 —— HA 场景下 `ENV` 未显式设置时**不迁移**；或新增 `AUTO_MIGRATE` 显式开关。迁移交给 `cmd/migrate`（`cmd/migrate/main.go:64`）在部署步骤中执行。

### 4.6 指标（差距：零）

- **现状：** 无 Prometheus 依赖，无 `/metrics`。
- **差距：** 旧设计 `§6.1`（`:715-751`）的四个指标无落点。
- **建议：** v1 不加。HA 的观测用运维脚本 + `pg_stat_replication` 足够（`ha-service-design.md:794-810` 的检查命令仍有效）。指标属于 D2 选项 C 的配套。

### 4.7 配置（差距：无配置文件层）

- **现状：** `configs/` 空；全部配置走环境变量。
- **差距：** 旧设计假设的 `silkguard.yaml`、`edge.yaml` 均不存在。
- **建议：** 保持环境变量风格，与现有代码一致。SilkGuard 的配置同样用环境变量，避免引入 yaml 依赖（`go.mod` 中只有 `gopkg.in/yaml.v3` 作为 indirect）。

---

## 五、验收方式

以下场景按“无需改 Go 代码”到“需要新代码”排序。前三个是 v1 的可交付验收。

### S1. PG 流复制建立（无需 Go）

```bash
# 主机上
psql -U igh -d igh -c "SELECT client_addr, state, sent_lsn, replay_lsn,
  pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes FROM pg_stat_replication;"
# 期望：一行，state=streaming，lag_bytes 接近 0

# 备机上
psql -U igh -d igh -c "SELECT pg_is_in_recovery();"
# 期望：t
```

### S2. 只读副本拒绝写（无需 Go）

```bash
# 备机上
psql -U igh -d igh -c "INSERT INTO edges(edge_code) VALUES ('x');"
# 期望：ERROR: cannot execute INSERT in a read-only transaction
```

### S3. 手动提升 + Center 重连（无需 Go，验证 D4 的必要性）

```bash
# 备机上
pg_ctl promote -D /var/lib/postgresql/data
psql -U igh -d igh -c "SELECT pg_is_in_recovery();"   # 期望 f

# 启动 Center 指向本机
DATABASE_URL="postgres://igh:...@localhost:5432/igh?sslmode=disable" \
  ./bin/center-server.exe
curl -s localhost:8080/health           # 现状：恒 200，**此断言在今天就恒成立，无信息量**
curl -s -X POST localhost:8080/v1/login -d '{"username":"admin","password":"..."}' -H 'Content-Type: application/json'
```

**这一场景本身就是 4.2 的证据：** 在引入 `/health/ready` 之前，S3 无法区分“提升成功且 Center 可用”与“PG 没起来但 Center 照样报 ok”。

### S4. 应用层零丢失验证（无需 Go，利用现有 upsert）

```bash
# 1. Edge 造数据，记下 ID
# 2. 停 Center（模拟故障）
# 3. 循环 6 次触发上传 → 耗尽 5 次预算，记录翻 failed（uploader.go:29,:862）
curl -X POST localhost:8081/v1/sync/upload -H "Authorization: Bearer $EDGE_JWT"
# 4. 提升副本并启动 Center
# 5. 回收
curl -X POST localhost:8081/v1/sync/resync -d '{"mode":"failed"}' -H "Authorization: Bearer $EDGE_JWT"
# 6. 验证 Center 侧数据完整，且**无重复行**（upsert 的编号认领生效）
psql -U igh -d igh -c "SELECT count(*) FROM lots WHERE lot_number='<测试编号>';"
# 期望：1
```

这条同时证明 4.4 的幂等性与 D2 里“25 分钟预算”的真实性。

### S5. 就绪端点（需 D4 的最小实现）

```bash
# PG 正常
curl -s -o /dev/null -w '%{http_code}' localhost:8080/health/ready   # 期望 200
# 停 PG
systemctl stop postgresql
curl -s -o /dev/null -w '%{http_code}' localhost:8080/health/ready   # 期望 503
# 对照：/health 仍返回 200 —— 这正是它今天不能用于故障判定的原因
curl -s -o /dev/null -w '%{http_code}' localhost:8080/health          # 期望 200
```

### S6. 自动检测与提升（需 D1+D3 的代码，v1 范围外）

需构造：心跳丢失 → 提升 → 调用 `/internal/ha/activate` → 状态变 ACTIVE。**在没有 S5 的前提下无法可靠判定“对端真的挂了还是我网络分区了”**，因此 S6 必须排在 S5 之后。

### S7. 回归保护（现状即可跑）

```bash
go test ./internal/sync/... ./internal/server/...
```

既有的 `upload_auth_test.go`、`edge_upload_test.go`、`sync_test.go`（1707 行）覆盖了上传鉴权与 upsert 语义，**任何 HA 改动都不得让它们变红**。这是 v1 的硬门槛。

---

## 六、范围外

明确不做，避免实施时被误纳入：

1. **边端双地址故障转移**（NFR-005）—— 见 D5。
2. **Center 写模式门（RO/RW 切换）** —— 见 4.3。v1 用“副本上不启动 Center”达成同样效果。
3. **Prometheus 指标与告警规则** —— 见 4.6。
4. **自动回切（failback）** —— 见 D7，v1 为人工。
5. **第三方仲裁 / Quorum / STONITH** —— 需要额外节点或硬件，工厂站点不具备。
6. **Keepalived + VIP** —— ADR-19 已否决；`deployment-strategy.md` §3.1-3.2 的该部分应在实施时删除（本次不改动该文件）。
7. **Center 多活 / 读写分离** —— 与热备语义冲突，不做。
8. **Edge 侧 SQLite 的 HA** —— NFR-006 的“边端自治”已由本地库满足，无需改造。
9. **`configs/` 引入 yaml 配置层** —— 见 4.7。
10. **修改 `docs/design/ha-service-design.md` 与 `deployment-strategy.md`** —— 本次交付仅新增本文档；两份旧文档的修正（改名、`recovery.conf`→`standby.signal`、删 VIP）应作为独立任务，在实施前完成，否则运维仍会按错的文档操作。
