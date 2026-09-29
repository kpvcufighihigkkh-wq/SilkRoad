# 后端 Service 层与 API 重构设计

> **文档编号:** DESIGN-2026-09-29-001
> **日期:** 2026-09-29
> **状态:** 待评审
> **前置:** V3 数据模型设计（`2026-09-29-v3-data-model-design.md`）

---

## 一、背景与目标

### 1.1 背景

数据库 Schema 已完成重构（commit `a368678`、`2539bc8`），并已在两套数据库上完成迁移验证：

- **Edge SQLite** — 迁移验证通过（2026-09-29）
- **Center PostgreSQL** — 迁移验证通过（2026-09-29，Docker 容器 `igh-postgres`）

但 Service 层与 API 层仍停留在旧模型：

| 问题 | 位置 | 说明 |
|------|------|------|
| 引用已删除实体 | `internal/service/order.go`、`api/center/v1/order.go` | `Order` 实体已从 Schema 移除，代码无法编译 |
| 字段不匹配 | `internal/service/lot.go:26,52` | 仍使用 `order_id`，新 Schema 已改为 `edge_id` + `plc_lot_number` |
| 缺少新实体服务 | — | `Barrel`/`Module`/`Edge`/`Grade`/`Pallet`/`SpinningLine`/`Carton`/`ProductConfig`/`BobbingGrade` 无对应 Service |
| 同步未覆盖新表 | `internal/sync/center/handler.go:71-80` | `handleCreate` 仅支持 `lots`、`bobbins` |

### 1.2 目标

1. 清除所有 `Order` 相关代码，恢复可编译状态
2. 为全部 13 个实体建立 Service 层与 RESTful API
3. 扩展同步机制覆盖新增实体
4. Edge 端具备离线工作能力

### 1.3 非目标

- 不修改已迁移的数据库 Schema（除设计已确认的 Barrel 关系调整）
- 不引入 gRPC / Protobuf（本次保持纯 REST）
- 不实现前端（后续独立子项目）

---

## 二、已确认的核心决策

| # | 决策项 | 选择 | 理由 |
|---|--------|------|------|
| 1 | 迁移策略 | **完全重写** | 新模型与旧模型差异过大，并存会导致长期技术债 |
| 2 | API 风格 | **纯 RESTful HTTP (Gin)** | 内部制造系统，并发压力低；前端对接与调试成本最低 |
| 3 | Service 组织 | **按实体组织** | 职责单一，易于定位与测试 |
| 4 | 部署分离 | **Center / Edge 严格分离** | Edge 需独立于网络运行，边界清晰可避免职责混淆 |
| 5 | 同步机制 | **主动推送 + 批量重试** | 兼顾实时性与离线容错 |
| 6 | Doffing↔Barrel 关系 | **Barrel 归属 Lot** | 见 3.1 |

---

## 三、Schema 调整（重构的前置条件）

### 3.1 Barrel 关系调整

**当前实现**（`internal/database/ent/schema/barrel.go:34-36,80-85`）：

```
Barrel.doffing_id  →  Doffing   （Required, Unique, 一对一）
Barrel.lot_id      →  Lot       （Required, Unique）
```

**问题：** `Doffing` 与 `Barrel` 是双向一对多（`Doffing` 有 `edge.To("barrels")`，`Barrel` 有 `edge.From("doffing")` + `Unique()`），语义矛盾。且一次落纱操作在业务上产生的是「一个桶里的多个丝饼」，而非「一个桶对应一次落纱记录」。

**调整后：**

```
删除：Barrel.doffing_id 字段
删除：Barrel.edges 中的 doffing 关联
删除：Doffing.edges 中的 barrels 关联
保留：Barrel.lot_id → Lot （Required, Unique）

Lot (批次)
  ├─ Barrel[]   落纱桶（容器，容量默认 9）
  │    └─ Bobbin[]  丝饼（桶内锭位，1-9）
  └─ Doffing[]  落纱操作记录（轻量级日志）
```

**影响文件：**
- `internal/database/ent/schema/barrel.go`
- `internal/database/ent/schema/doffing.go`
- 需重新生成 Ent 代码（`go generate ./internal/database/ent`）

### 3.2 同步状态字段

**当前实现：** 所有 Schema 均无同步相关字段。

**需要新增到** `Lot`、`Barrel`、`Bobbin`、`Doffing` **四个实体**（均为 Edge 端产生数据）：

```go
field.Enum("sync_status").
    Values("pending", "synced", "failed").
    Default("pending").
    Comment("同步状态"),

field.Time("synced_at").
    Optional().
    Comment("同步时间"),

field.Int("sync_retry_count").
    Default(0).
    NonNegative().
    Comment("同步重试次数"),
```

**设计说明：**
- 有同步字段的实体 = 数据在 Edge 端产生、需要上传到 Center 的实体。
- `Edge`/`Grade`/`SpinningLine`/`ProductConfig` 由 Center 下发到 Edge，方向相反，不需要同步字段。
- `Carton`/`Pallet`/`Module` 是否需要同步字段，取决于它们由哪端创建 —— **见 §9 Q1**。

---

## 四、Service 层设计

### 4.1 目录结构

```
internal/service/
├── edge.go            # Edge 设备注册/管理
├── spinning_line.go   # 纺丝线体管理
├── lot.go             # 批次管理（重构）
├── doffing.go         # 落纱记录（重构）
├── barrel.go          # 落纱桶管理
├── bobbin.go          # 丝锭管理（重构）
├── bobbing_grade.go   # 丝锭等级历史
├── pallet.go          # 托盘管理
├── module.go          # 吊车管理
├── carton.go          # 纸箱管理
├── grade.go           # 等级管理
├── product_config.go  # 产品配置
└── user.go            # 用户（保留，仅适配）
```

**约定（每个 Service 统一）：**

```go
type XxxService struct {
    client *ent.Client
}

func NewXxxService(client *ent.Client) *XxxService

// 统一方法集
Create(ctx, *CreateXxxRequest) (*XxxResponse, error)
List(ctx, page, pageSize int, filters...) ([]*XxxResponse, int, error)
Get(ctx, id string) (*XxxResponse, error)
Update(ctx, id string, *UpdateXxxRequest) error
Delete(ctx, id string) error
```

### 4.2 关键 Service 契约

**LotService（重构要点）**

```go
// 变更：移除 OrderID，新增 EdgeID / PLCLotNumber
type CreateLotRequest struct {
    LotNumber       string `json:"lot_number"       binding:"required,max=50"`
    EdgeID          string `json:"edge_id"          binding:"omitempty,uuid"`
    PLCLotNumber    string `json:"plc_lot_number"   binding:"omitempty,max=50"`
    OrderCode       string `json:"order_code"       binding:"omitempty,max=50"` // ERP 集成用，非外键
    ProductType     string `json:"product_type"     binding:"required,oneof=FDY POY DTY"`
    ProductSpec     string `json:"product_spec"     binding:"omitempty,max=100"`
    PlannedQuantity int    `json:"planned_quantity" binding:"required,min=1"`
}

// 列表过滤：order_id → edge_id
ListLots(ctx, page, pageSize int, edgeID, status string) ([]*LotResponse, int, error)
```

**BarrelService（新增）**

```go
CreateBarrel(ctx, *CreateBarrelRequest) (*BarrelResponse, error)
ListBarrels(ctx, page, pageSize int, lotID, status string) ([]*BarrelResponse, int, error)
GetBarrel(ctx, id string) (*BarrelResponse, error)
GetBarrelWithBobbins(ctx, id string) (*BarrelDetailResponse, error)
SealBarrel(ctx, id string) error   // 封桶：校验 current_count == capacity
```

**BobbinService（重构要点）**

```go
// 变更：新增 BarrelID / BarrelPosition
type CreateBobbinRequest struct {
    BobbinNumber    string  `json:"bobbin_number"    binding:"required,max=50"`
    LotID           string  `json:"lot_id"           binding:"required,uuid"`
    BarrelID        string  `json:"barrel_id"        binding:"omitempty,uuid"`
    BarrelPosition  int     `json:"barrel_position"  binding:"omitempty,min=1,max=9"`
    SpinningPosition int    `json:"spinning_position" binding:"required,min=1"`
    GrossWeight     float64 `json:"gross_weight"     binding:"required,gt=0"`
    NetWeight       float64 `json:"net_weight"       binding:"required,gt=0"`
    TareWeight      float64 `json:"tare_weight"      binding:"omitempty,gte=0"`
    GradeID         string  `json:"grade_id"         binding:"omitempty,uuid"`
}

// 列表过滤：新增 barrelID / palletID / cartonID
ListBobbins(ctx, page, pageSize int, filters BobbinFilters) ([]*BobbinResponse, int, error)
UpdateBobbinGrade(ctx, id, gradeID string) error
```

### 4.3 事务边界

跨实体的一致性操作必须使用 Ent 事务。**最关键的一处**是丝饼入桶：

```go
func (s *BobbinService) CreateBobbinWithBarrel(ctx, req) error {
    tx, _ := s.client.Tx(ctx)
    defer tx.Rollback()

    // 1. 桶内位置占用校验（防并发写同一位置）
    taken, _ := tx.Bobbin.Query().
        Where(bobbin.BarrelIDEQ(req.BarrelID), bobbin.BarrelPositionEQ(req.BarrelPosition)).
        Exist(ctx)
    if taken {
        return ErrBarrelPositionTaken
    }

    // 2. 创建丝饼
    tx.Bobbin.Create()...Save(ctx)

    // 3. 桶计数 +1
    tx.Barrel.UpdateOneID(req.BarrelID).AddCurrentCount(1).Exec(ctx)

    // 4. 批次实际数量 +1
    tx.Lot.UpdateOneID(req.LotID).AddActualQuantity(1).Exec(ctx)

    return tx.Commit()
}
```

---

## 五、API 设计

### 5.1 Center Server

**设备管理**
```
GET    /v1/edges                         设备列表
POST   /v1/edges                         注册设备
GET    /v1/edges/:id                     设备详情
PUT    /v1/edges/:id                     更新设备
DELETE /v1/edges/:id                     删除设备
POST   /v1/edges/:id/heartbeat           心跳上报

GET    /v1/spinning-lines                线体列表
POST   /v1/spinning-lines                创建线体
GET    /v1/spinning-lines/:id            线体详情
PUT    /v1/spinning-lines/:id            更新线体
PUT    /v1/spinning-lines/:id/status     更新状态
```

**生产管理**
```
GET    /v1/lots                          批次列表（全局）
POST   /v1/lots                          创建批次
GET    /v1/lots/:id                      批次详情
PUT    /v1/lots/:id/status               更新状态
DELETE /v1/lots/:id                      删除批次

GET    /v1/doffings                      落纱记录（汇总）
GET    /v1/doffings/:id                  落纱详情
```

**载具与丝锭**
```
GET    /v1/barrels                       落纱桶列表
GET    /v1/barrels/:id                   落纱桶详情
GET    /v1/barrels/:id/bobbins           桶内丝饼
POST   /v1/barrels                       创建落纱桶

GET    /v1/bobbins                       丝饼列表（全局）
GET    /v1/bobbins/:id                   丝饼详情
PUT    /v1/bobbins/:id/grade             更新等级

GET    /v1/bobbing-grades                丝饼等级历史
```

**仓储管理**
```
GET    /v1/pallets                       托盘列表
POST   /v1/pallets                       创建托盘
GET    /v1/pallets/:id                   托盘详情
PUT    /v1/pallets/:id                   更新托盘
GET    /v1/pallets/:id/bobbins           托盘上的丝饼

GET    /v1/modules                       吊车列表
POST   /v1/modules                       创建吊车
GET    /v1/modules/:id                   吊车详情
PUT    /v1/modules/:id/status            更新状态

GET    /v1/cartons                       纸箱列表
POST   /v1/cartons                       创建纸箱
GET    /v1/cartons/:id                   纸箱详情
```

**基础数据**
```
GET    /v1/grades                        等级列表
POST   /v1/grades                        创建等级
PUT    /v1/grades/:id                    更新等级
DELETE /v1/grades/:id                    删除等级

GET    /v1/product-configs               产品配置列表
POST   /v1/product-configs               创建配置
PUT    /v1/product-configs/:id           更新配置
```

**同步接口**
```
POST   /v1/sync/upload                   接收 Edge 批量上传
GET    /v1/sync/base-data                下发基础数据（增量）
```

### 5.2 Edge Server

```
POST   /v1/doffings                      执行落纱操作
GET    /v1/doffings                      本地落纱记录
GET    /v1/doffings/:id                  落纱详情

GET    /v1/lots                          本地批次列表
GET    /v1/lots/current                  当前批次
POST   /v1/lots                          创建本地批次

GET    /v1/barrels                       本地落纱桶
GET    /v1/barrels/:id                   落纱桶详情
GET    /v1/barrels/:id/bobbins           桶内丝饼
PUT    /v1/barrels/:id/seal              封桶

GET    /v1/bobbins                       本地丝饼
GET    /v1/bobbins/:id                   丝饼详情

POST   /v1/sync/upload                   手动触发上传
GET    /v1/sync/status                   同步状态与积压量
POST   /v1/sync/refresh                  拉取基础数据
```

### 5.3 端点职责对照

| 资源 | Center | Edge | 说明 |
|------|--------|------|------|
| Edge 设备 | 完整 CRUD | 只读自身 | Edge 上报心跳，Center 管理注册 |
| SpinningLine | 完整 CRUD | 只读 | Center 下发 |
| Lot | 完整 CRUD | 创建/查询本地 | Edge 创建，上传至 Center |
| Doffing | 只读汇总 | **核心写操作** | Edge 产生，Center 汇总展示 |
| Barrel | 只读查询 | 创建/封桶 | Edge 产生 |
| Bobbin | 只读 + 改等级 | 创建/查询 | Edge 产生，Center 质检改等级 |
| Pallet / Module / Carton | 完整 CRUD | 只读 | **待确认创建端**（§9） |
| Grade / ProductConfig | 完整 CRUD | 只读 | Center 下发基础数据 |

---

## 六、同步机制设计

### 6.1 复用现有模型

`internal/sync/models/models.go` 已定义完整结构（`UploadRequest`/`UploadEntry`/`UploadResponse`/`BaseDataPullRequest`/`BaseDataPullResponse`/`SyncCursor`/`ConflictResolution`），**本次直接复用，不重新定义**。

### 6.2 上传流程（Edge → Center）

```
┌─────────────────────────────────────────────────┐
│  触发点                                          │
│  1. 实时：封桶 / 丝饼落筒完成后异步触发           │
│  2. 定时：每 5 分钟批量扫描 pending 记录          │
└─────────────────────────────────────────────────┘
                     ↓
        查询 sync_status=pending
        AND sync_retry_count < 5
        ORDER BY created_at ASC
        LIMIT 100
                     ↓
        构建 models.UploadRequest
        （携带 EdgeID + Entries + Cursor）
                     ↓
        POST /v1/sync/upload
                     ↓
        ┌────────────┴────────────┐
        ↓                         ↓
     成功                       失败
        ↓                         ↓
  sync_status=synced      sync_retry_count++
  synced_at=now()         (≥5 次 → sync_status=failed)
```

**幂等性：** Edge 生成的 UUID 在 Center 端作为主键直接使用，`handler` 在创建前检查 `ID` 是否已存在，已存在则跳过。这保证重传不会产生重复数据。

**排序规则：** Edge 侧的扫描按 `created_at ASC` 排序，保证 `Lot` → `Barrel` → `Bobbin` 的先后顺序。这不能只靠时间戳碰巧正确 —— 上传前需按「批次 → 落纱 → 桶 → 丝饼」显式排序 `Entries`，避免外键引用尚未到达的记录。

### 6.3 同步配置项

以下参数需要在 Edge 端可配置（写在配置文件，不硬编码）：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `batch_size` | 100 | 单次上传的最大记录数 |
| `flush_interval` | 5s | 实时上传的攒批窗口 |
| `retry_interval` | 5m | 定时扫描间隔 |
| `max_retry_count` | 5 | 超过后标记 `failed` |

**`flush_interval` 的作用：** 封桶是「9 个丝饼写入 → 一个桶封口」的连续动作，紧接着上传会切出大量小请求。5 秒的攒批窗口把这一串操作合并成一次上传，同时保留接近实时的体感（用户封完桶走到下一个工位时，数据已经发出）。

### 6.4 下发流程（Center → Edge）

**触发时机：**
- Edge 启动时全量拉取
- 每 30 分钟增量拉取（基于 `updated_at > last_sync_time`）

**下发内容：**
```
Grade         等级定义
ProductConfig 产品配置
SpinningLine  线体信息
Edge          设备信息（含自身配置）
```

**`handler.go` 需要扩展的表：**

```go
func (h *UploadHandler) handleCreate(ctx, entry) error {
    switch entry.Table {
    case "lots":     return h.createLot(ctx, entry)
    case "barrels":  return h.createBarrel(ctx, entry)   // 新增
    case "bobbins":  return h.createBobbin(ctx, entry)
    case "doffings": return h.createDoffing(ctx, entry)  // 新增
    default:         return fmt.Errorf("unknown table: %s", entry.Table)
    }
}
```

---

## 七、错误处理

### 7.1 错误码扩展

`api/response.go` 已有分层错误码体系，业务错误段（3xxxx）需补充：

```go
const (
    CodeLotNotFound          = 30001
    CodeLotCompleted         = 30002  // 批次已完成，不可继续操作
    CodeBarrelFull           = 30003  // 桶已满
    CodeBarrelPositionTaken  = 30004  // 桶内位置已被占用
    CodeInvalidWeight        = 30005  // 重量数据非法（净重 > 毛重等）
    CodeEdgeOffline          = 30006  // 设备离线
    CodeSyncConflict         = 30007  // 同步冲突
)
```

### 7.2 分层错误转换

Service 层返回领域错误，Handler 层映射为 HTTP 状态码与错误码，避免 Service 层依赖 `gin`：

```go
// Service 层
var ErrBarrelPositionTaken = errors.New("barrel position already taken")

// Handler 层统一映射
func mapServiceError(err error) (int, int) {
    switch {
    case errors.Is(err, service.ErrBarrelPositionTaken):
        return http.StatusConflict, api.CodeBarrelPositionTaken
    case ent.IsNotFound(err):
        return http.StatusNotFound, api.CodeNotFound
    default:
        return http.StatusInternalServerError, api.CodeServerError
    }
}
```

---

## 八、测试策略

| 层次 | 范围 | 工具 |
|------|------|------|
| 单元测试 | Service 层业务规则（封桶校验、位置占用、事务回滚） | `enttest` + SQLite 内存库 |
| 集成测试 | API 端到端（请求 → 响应 → 数据库状态） | `httptest` + `gin` |
| 同步测试 | 幂等性、顺序性、重试、断网恢复 | mock Center 服务 |

**测试数据库：** `enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=1")` —— 注意需启用外键。

**关键测试用例：**
1. 同一桶位写入两个丝饼 → 第二个必须返回 `CodeBarrelPositionTaken`
2. 桶未满时封桶 → 返回 `CodeBarrelFull` 之外的业务错误
3. 同一 `UploadRequest` 提交两次 → Center 端记录数不变（幂等）
4. `Bobbin` 上传先于其 `Barrel` → 应重试而非永久失败（顺序保证）
5. 事务中途失败 → `Barrel.current_count` 与 `Lot.actual_quantity` 均回滚

---

## 九、待确认事项

以下问题在设计中**尚未确定**，需要在实施计划前明确：

**Q1. `Pallet` / `Module` / `Carton` 由哪端创建？**

这三个实体在 API 表中暂标为「Center 完整 CRUD，Edge 只读」，但依据 V2 业务分析，`modules`（吊车）是物流载具、`pallets`（栈板）由码垛机生成 —— 若它们实际在 Edge 产生，则需要同步字段与上传逻辑，且 API 归属需要反转。

**Q2. `sync_status` 字段的迁移策略？**

新增字段需要二次迁移。当前两套数据库均为空表（除 `SpinningLines` 4 条、`Users` 4 条），可直接迁移。但需确认是否保留现有 8 条数据。

**Q3. `BobbingGrade`（丝饼等级历史）的业务语义？**

Schema 中存在该实体，但尚未明确：是等级变更的审计日志，还是每个丝饼可对应多个等级评定（人工 + 机器）？

---

## 十、实施顺序

```
Phase 0  删除 Order 相关代码，恢复编译          ← 最小风险，先做
Phase 1  Schema 调整（Barrel 关系 + 同步字段）
         + 重新生成 Ent 代码 + 二次迁移
Phase 2  重建核心 Service：Edge / SpinningLine / Lot / Grade
Phase 3  重建生产 Service：Doffing / Barrel / Bobbin / BobbingGrade
Phase 4  重建仓储 Service：Pallet / Module / Carton / ProductConfig
Phase 5  注册路由，接通 API 层
Phase 6  扩展同步机制（handler + uploader + scheduler）
Phase 7  测试补齐（单元 + 集成 + 同步）
```

**每个 Phase 独立提交**，保证可回退。

---

## 十一、风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| Barrel 关系调整导致二次迁移失败 | 数据库结构不一致 | 迁移前备份；两库均为空表，风险低 |
| 同步幂等性实现有误 | Center 数据重复 | 上传前按 UUID 查重；集成测试覆盖 |
| Edge 离线时间过长 | 积压数据量过大 | 批量上传每批 100 条；重试上限 5 次后标记 failed 供人工处理 |
| 事务边界遗漏 | 数据不一致 | 跨实体写操作统一走事务；测试覆盖回滚路径 |
