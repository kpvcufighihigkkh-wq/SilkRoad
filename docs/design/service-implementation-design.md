# IGH 服务实现设计

> **文档编号:** SERVICE-IMPL-001  
> **版本:** 1.0  
> **基于:** 数据库设计 v1.0 + API设计 v1.0 + Protobuf定义 v1.0  
> **日期:** 2026-09-24  
> **设计者:** 浮浮酱

---

## 一、整体架构

### 1.1 技术栈（ADR-05）

| 层次 | 技术选型 | 理由 |
|------|----------|------|
| **Web框架** | go-kratos v2 | 微服务框架，gRPC+HTTP双协议 |
| **ORM** | ent | 类型安全，代码生成，性能优秀 |
| **数据库** | PostgreSQL (中心) + SQLite (边端) | ADR-09 |
| **缓存** | Redis (中心) + 内存缓存 (边端) | 高频查询优化 |
| **日志** | zap | 高性能结构化日志 |
| **监控** | Prometheus + Grafana | 指标采集和可视化 |
| **配置** | Viper | 支持多种配置源 |

### 1.2 项目结构

```
igh-silkroad/
├── cmd/
│   ├── igh-center/          # 中心端服务入口
│   │   └── main.go
│   ├── igh-edge/            # 边端服务入口
│   │   └── main.go
│   └── tools/               # 工具脚本
│       ├── migrate/         # 数据库迁移工具
│       └── seed/            # 测试数据生成
│
├── internal/
│   ├── generated/           # Protobuf 生成代码
│   │   ├── common/v1/
│   │   ├── center/v1/
│   │   └── edge/v1/
│   │
│   ├── ent/                 # Ent ORM 生成代码
│   │   ├── schema/          # 数据库 Schema 定义
│   │   └── ...              # 自动生成的代码
│   │
│   ├── domain/              # 领域模型（DDD）
│   │   ├── order/           # 订单聚合
│   │   ├── bobbin/          # 丝锭聚合
│   │   ├── warehouse/       # 仓储聚合
│   │   └── ...
│   │
│   ├── service/             # 业务服务层
│   │   ├── center/          # 中心端服务
│   │   │   ├── sync/        # 同步服务
│   │   │   ├── ota/         # OTA 服务
│   │   │   └── ...
│   │   └── edge/            # 边端服务
│   │       ├── doffing/     # 落纱服务
│   │       ├── inspection/  # 质检服务
│   │       └── ...
│   │
│   ├── repository/          # 数据访问层
│   │   ├── postgres/        # PostgreSQL 仓储
│   │   └── sqlite/          # SQLite 仓储
│   │
│   ├── client/              # gRPC 客户端
│   │   └── center/          # 边端调用中心端客户端
│   │
│   ├── middleware/          # 中间件
│   │   ├── auth/            # 认证中间件
│   │   ├── logging/         # 日志中间件
│   │   └── recovery/        # 恢复中间件
│   │
│   ├── pkg/                 # 公共包
│   │   ├── cache/           # 缓存封装
│   │   ├── plc/             # PLC 通信
│   │   ├── printer/         # 打印机驱动
│   │   └── utils/           # 工具函数
│   │
│   └── config/              # 配置管理
│       ├── center.yaml      # 中心端配置
│       └── edge.yaml        # 边端配置
│
├── api/
│   └── proto/               # Protobuf 定义（已完成）
│
├── docs/
│   └── design/              # 设计文档（已完成）
│
├── test/
│   ├── integration/         # 集成测试
│   ├── e2e/                 # 端到端测试
│   └── benchmark/           # 性能测试
│
├── scripts/                 # 脚本工具
│   ├── gen-proto.sh         # 生成 Protobuf 代码
│   └── gen-ent.sh           # 生成 Ent 代码
│
├── migrations/              # 数据库迁移文件
│   ├── postgres/            # 中心端迁移
│   └── sqlite/              # 边端迁移
│
├── go.mod
├── go.sum
└── Makefile
```

### 1.3 分层架构（DDD + 清晰架构）

```
┌─────────────────────────────────────────────────┐
│  API 层 (cmd/igh-center, cmd/igh-edge)         │
│  - gRPC Server / HTTP Server                    │
│  - 请求验证、响应转换                            │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│  Service 层 (internal/service)                  │
│  - 业务逻辑编排                                  │
│  - 事务管理                                      │
│  - 领域服务调用                                  │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│  Domain 层 (internal/domain)                    │
│  - 领域模型（实体、值对象）                       │
│  - 业务规则验证                                  │
│  - 领域事件                                      │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│  Repository 层 (internal/repository)            │
│  - 数据持久化                                    │
│  - 查询构建                                      │
│  - 缓存管理                                      │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│  Infrastructure 层 (internal/pkg)               │
│  - 数据库、缓存、消息队列                         │
│  - PLC 通信、打印机驱动                          │
│  - 日志、监控、配置                               │
└─────────────────────────────────────────────────┘
```

**依赖规则：**
- 上层依赖下层，下层不依赖上层
- Domain 层不依赖任何外部库（纯业务逻辑）
- 依赖注入解耦（使用 Wire）

---

## 二、核心服务实现

### 2.1 落纱服务（DoffingService）

#### 2.1.1 业务流程

```
用户操作 → 扫描批次二维码 → 选择机台 → 确认落纱
                                          ↓
                            [事务开始] 边端 SQLite
                                          ↓
                    1. 创建 doffing 记录（doffing_number）
                    2. 批量创建 24 个 bobbin 记录（编码规则）
                    3. 更新 lot 的 actual_quantity
                    4. 记录到 sync_queue（待上传）
                                          ↓
                            [事务提交] → 返回成功
                                          ↓
                            异步上传到中心端（后台协程）
```

#### 2.1.2 代码实现

**Service 层（internal/service/edge/doffing/service.go）：**

```go
package doffing

import (
    "context"
    "fmt"
    "time"
    
    "github.com/igh/internal/domain/bobbin"
    "github.com/igh/internal/domain/doffing"
    "github.com/igh/internal/repository/sqlite"
    pb "github.com/igh/internal/generated/edge/v1"
)

type Service struct {
    pb.UnimplementedDoffingServiceServer
    
    repo       *sqlite.Repository
    syncQueue  *sqlite.SyncQueue
    codeGen    *bobbin.CodeGenerator
}

func NewService(repo *sqlite.Repository, queue *sqlite.SyncQueue) *Service {
    return &Service{
        repo:      repo,
        syncQueue: queue,
        codeGen:   bobbin.NewCodeGenerator(),
    }
}

func (s *Service) ExecuteDoffing(
    ctx context.Context,
    req *pb.DoffingRequest,
) (*pb.DoffingResponse, error) {
    // 1. 参数验证
    if err := s.validateRequest(req); err != nil {
        return nil, err
    }
    
    // 2. 查询批次信息
    lot, err := s.repo.GetLot(ctx, req.LotId)
    if err != nil {
        return nil, fmt.Errorf("批次不存在: %w", err)
    }
    
    // 3. 查询机台信息
    module, err := s.repo.GetModule(ctx, req.ModuleId)
    if err != nil {
        return nil, fmt.Errorf("机台不存在: %w", err)
    }
    
    // 4. 业务规则验证
    if err := s.validateBusinessRules(lot, module); err != nil {
        return nil, err
    }
    
    // 5. 事务执行
    tx, err := s.repo.BeginTx(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // 5.1 创建落纱记录
    doffingEntity := &doffing.Doffing{
        LotID:           req.LotId,
        ModuleID:        req.ModuleId,
        DoffingSequence: req.DoffingSequence,
        OperatorID:      req.OperatorId,
        ShiftID:         req.ShiftId,
        DoffedAt:        time.Now(),
    }
    doffingEntity.GenerateNumber(lot.LotNumber, req.DoffingSequence)
    
    doffingID, err := tx.CreateDoffing(ctx, doffingEntity)
    if err != nil {
        return nil, fmt.Errorf("创建落纱记录失败: %w", err)
    }
    
    // 5.2 批量创建丝锭记录（24 个）
    bobbins := s.generateBobbins(lot, module, doffingID, 24)
    bobbin_codes, err := tx.CreateBobbinsBatch(ctx, bobbins)
    if err != nil {
        return nil, fmt.Errorf("创建丝锭记录失败: %w", err)
    }
    
    // 5.3 更新批次的实际产量
    err = tx.UpdateLotQuantity(ctx, req.LotId, 24)
    if err != nil {
        return nil, fmt.Errorf("更新批次产量失败: %w", err)
    }
    
    // 5.4 记录到同步队列
    for _, b := range bobbins {
        s.syncQueue.Enqueue(ctx, tx, "BOBBIN", b.ID, "INSERT", b)
    }
    s.syncQueue.Enqueue(ctx, tx, "DOFFING", doffingID, "INSERT", doffingEntity)
    
    // 5.5 提交事务
    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("事务提交失败: %w", err)
    }
    
    // 6. 返回响应
    return &pb.DoffingResponse{
        DoffingId:      doffingID,
        DoffingNumber:  doffingEntity.DoffingNumber,
        BobbinsCreated: 24,
        BobhinCodes:    bobbin_codes,
        DoffedAt:       timestampPB(doffingEntity.DoffedAt),
    }, nil
}

// generateBobbins 生成丝锭记录
func (s *Service) generateBobbins(
    lot *domain.Lot,
    module *domain.Module,
    doffingID string,
    count int,
) []*bobbin.Bobbin {
    bobbins := make([]*bobbin.Bobbin, count)
    
    for i := 0; i < count; i++ {
        code := s.codeGen.Generate(lot.LotNumber, i+1)
        bobbins[i] = &bobbin.Bobbin{
            LotID:        lot.ID,
            ModuleID:     module.ID,
            DoffingID:    doffingID,
            BobbinCode:   code,
            Position:     i + 1,
            Lifecycle:    bobbin.LifecycleProduced,
            ProducedAt:   time.Now(),
        }
    }
    
    return bobbins
}

// validateBusinessRules 业务规则验证
func (s *Service) validateBusinessRules(
    lot *domain.Lot,
    module *domain.Module,
) error {
    // 规则1：批次必须是进行中状态
    if lot.Status != "in_progress" {
        return fmt.Errorf("批次状态不允许落纱: %s", lot.Status)
    }
    
    // 规则2：批次未锁定
    if lot.IsLocked {
        return fmt.Errorf("批次已锁定，不允许落纱")
    }
    
    // 规则3：机台产品类型匹配
    if module.ProductType != lot.ProductType {
        return fmt.Errorf("机台产品类型不匹配")
    }
    
    // 规则4：不能超过计划产量
    if lot.ActualQuantity + 24 > lot.PlannedQuantity {
        return fmt.Errorf("超过计划产量")
    }
    
    return nil
}
```

**丝锭编码生成器（internal/domain/bobbin/code_generator.go）：**

```go
package bobbin

import "fmt"

type CodeGenerator struct {
    // 编码规则：批次号-位置号
    // 示例：FDY2026001A001-001
}

func NewCodeGenerator() *CodeGenerator {
    return &CodeGenerator{}
}

func (g *CodeGenerator) Generate(lotNumber string, position int) string {
    return fmt.Sprintf("%s-%03d", lotNumber, position)
}
```

**同步队列（internal/repository/sqlite/sync_queue.go）：**

```go
package sqlite

import (
    "context"
    "encoding/json"
)

type SyncQueue struct {
    db *DB
}

func (q *SyncQueue) Enqueue(
    ctx context.Context,
    tx *Tx,
    entityType string,
    entityID string,
    operation string,
    data interface{},
) error {
    payload, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    query := `
        INSERT INTO sync_queue (entity_type, entity_id, operation, payload)
        VALUES (?, ?, ?, ?)
    `
    _, err = tx.Exec(query, entityType, entityID, operation, string(payload))
    return err
}

// ProcessQueue 后台协程定期处理
func (q *SyncQueue) ProcessQueue(ctx context.Context) error {
    // 查询 PENDING 状态的记录
    query := `
        SELECT id, entity_type, entity_id, operation, payload
        FROM sync_queue
        WHERE status = 'PENDING'
        ORDER BY created_at
        LIMIT 100
    `
    // 批量上传到中心端
    // 成功后标记为 SUCCESS
    // 失败则重试（max_retries 检查）
    return nil
}
```

---

### 2.2 质检服务（InspectionService）

#### 2.2.1 业务流程

```
质检员扫码 → 获取丝锭信息 → 显示已有质检记录
                                      ↓
                          录入质检结果（维度、等级、缺陷）
                                      ↓
                          [事务] 创建 bobbin_grades 记录
                                      ↓
                          计算 final_grade（5 维度都完成时）
                                      ↓
                          更新 bobbins.final_grade
                                      ↓
                          记录到 sync_queue
```

#### 2.2.2 代码实现

**Service 层（internal/service/edge/inspection/service.go）：**

```go
package inspection

import (
    "context"
    "fmt"
    
    "github.com/igh/internal/domain/bobbin"
    pb "github.com/igh/internal/generated/edge/v1"
)

type Service struct {
    pb.UnimplementedInspectionServiceServer
    
    repo         *sqlite.Repository
    syncQueue    *sqlite.SyncQueue
    gradeCalc    *bobbin.GradeCalculator
}

func (s *Service) ScanBobbin(
    ctx context.Context,
    req *pb.ScanBobbinRequest,
) (*pb.ScanBobbinResponse, error) {
    // 1. 查询丝锭信息
    bobbin, err := s.repo.GetBobbinByCode(ctx, req.BobbinCode)
    if err != nil {
        return nil, fmt.Errorf("丝锭不存在: %w", err)
    }
    
    // 2. 查询已有质检记录
    grades, err := s.repo.GetBobbinGrades(ctx, bobbin.ID)
    if err != nil {
        return nil, err
    }
    
    // 3. 转换为响应
    return &pb.ScanBobbinResponse{
        Bobbin: &pb.BobbinScanInfo{
            Id:             bobbin.ID,
            BobbinCode:     bobbin.BobbinCode,
            LotNumber:      bobbin.LotNumber,
            ProductType:    pb.ProductType(bobbin.ProductType),
            Position:       int32(bobbin.Position),
            Lifecycle:      pb.BobbinLifecycle(bobbin.Lifecycle),
            GrossWeightG:   bobbin.GrossWeightG,
            ExistingGrades: convertGrades(grades),
        },
    }, nil
}

func (s *Service) SubmitInspection(
    ctx context.Context,
    req *pb.InspectionRequest,
) (*pb.InspectionResponse, error) {
    // 1. 参数验证
    if err := s.validateInspection(req); err != nil {
        return nil, err
    }
    
    // 2. 查询丝锭
    bobbin, err := s.repo.GetBobbin(ctx, req.BobbinId)
    if err != nil {
        return nil, err
    }
    
    // 3. 业务规则验证
    if bobbin.Lifecycle != bobbin.LifecycleProduced {
        return nil, fmt.Errorf("丝锭状态不允许质检: %s", bobbin.Lifecycle)
    }
    
    // 4. 事务执行
    tx, err := s.repo.BeginTx(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // 4.1 创建质检记录
    grade := &bobbin.Grade{
        BobbinID:      req.BobbinId,
        Dimension:     req.Dimension,
        GradeValue:    req.GradeValue,
        InspectorID:   req.InspectorId,
        EquipmentCode: req.EquipmentCode,
        InspectedAt:   time.Now(),
    }
    
    // weight 维度记录重量
    if req.Dimension == "weight" {
        grade.GrossWeightG = req.GrossWeightG
        grade.NetWeightG = req.NetWeightG
        grade.TubeWeightG = req.TubeWeightG
        
        // 更新丝锭重量
        tx.UpdateBobbinWeight(ctx, req.BobbinId, req.GrossWeightG, req.NetWeightG)
    }
    
    gradeID, err := tx.CreateGrade(ctx, grade)
    if err != nil {
        return nil, fmt.Errorf("创建质检记录失败: %w", err)
    }
    
    // 4.2 计算 final_grade（如果 5 个维度都完成）
    allGrades, err := tx.GetBobbinGrades(ctx, req.BobbinId)
    if err != nil {
        return nil, err
    }
    
    var finalGrade string
    var isQualified bool
    
    if len(allGrades) == 5 {
        finalGrade = s.gradeCalc.CalculateFinalGrade(allGrades)
        isQualified = finalGrade != "D"
        
        // 更新丝锭
        tx.UpdateBobbinGrade(ctx, req.BobbinId, finalGrade, isQualified)
    }
    
    // 4.3 记录到同步队列
    s.syncQueue.Enqueue(ctx, tx, "GRADE", gradeID, "INSERT", grade)
    if finalGrade != "" {
        s.syncQueue.Enqueue(ctx, tx, "BOBBIN", req.BobbinId, "UPDATE", bobbin)
    }
    
    // 4.4 提交事务
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    return &pb.InspectionResponse{
        GradeId:     gradeID,
        IsQualified: isQualified,
        FinalGrade:  finalGrade,
    }, nil
}
```

**等级计算器（internal/domain/bobbin/grade_calculator.go）：**

```go
package bobbin

type GradeCalculator struct {
    // 等级规则：取 5 个维度的最低等级
    // 优先级：AA > B > C > D
}

func (c *GradeCalculator) CalculateFinalGrade(grades []*Grade) string {
    priority := map[string]int{
        "AA": 4,
        "B":  3,
        "C":  2,
        "D":  1,
    }
    
    minGrade := "AA"
    minPriority := 4
    
    for _, grade := range grades {
        if p, ok := priority[grade.GradeValue]; ok {
            if p < minPriority {
                minGrade = grade.GradeValue
                minPriority = p
            }
        }
    }
    
    return minGrade
}
```

---

### 2.3 中心端同步服务（SyncService）

#### 2.3.1 批量上传实现

**Service 层（internal/service/center/sync/service.go）：**

```go
package sync

import (
    "context"
    "encoding/json"
    
    pb "github.com/igh/internal/generated/center/v1"
)

type Service struct {
    pb.UnimplementedSyncServiceServer
    
    repo  *postgres.Repository
    cache *cache.RedisCache
}

func (s *Service) BatchUpload(
    ctx context.Context,
    req *pb.BatchUploadRequest,
) (*pb.BatchUploadResponse, error) {
    var errors []*pb.SyncError
    successCount := 0
    
    // 并发处理（控制并发数）
    sem := make(chan struct{}, 10)  // 最多 10 个并发
    errCh := make(chan *pb.SyncError, len(req.Entities))
    
    for _, entity := range req.Entities {
        sem <- struct{}{}
        
        go func(e *pb.SyncEntity) {
            defer func() { <-sem }()
            
            if err := s.processEntity(ctx, e); err != nil {
                errCh <- &pb.SyncError{
                    EntityId:     e.EntityId,
                    EntityType:   e.EntityType,
                    ErrorCode:    "PROCESSING_FAILED",
                    ErrorMessage: err.Error(),
                }
            }
        }(entity)
    }
    
    // 等待所有协程完成
    for i := 0; i < cap(sem); i++ {
        sem <- struct{}{}
    }
    close(errCh)
    
    // 收集错误
    for err := range errCh {
        errors = append(errors, err)
    }
    
    successCount = len(req.Entities) - len(errors)
    
    return &pb.BatchUploadResponse{
        SuccessCount: int32(successCount),
        FailedCount:  int32(len(errors)),
        Errors:       errors,
    }, nil
}

func (s *Service) processEntity(ctx context.Context, entity *pb.SyncEntity) error {
    switch entity.EntityType {
    case "BOBBIN":
        return s.syncBobbin(ctx, entity)
    case "GRADE":
        return s.syncGrade(ctx, entity)
    case "DOFFING":
        return s.syncDoffing(ctx, entity)
    default:
        return fmt.Errorf("未知实体类型: %s", entity.EntityType)
    }
}

func (s *Service) syncBobbin(ctx context.Context, entity *pb.SyncEntity) error {
    var bobbin domain.Bobbin
    if err := json.Unmarshal(entity.Data, &bobbin); err != nil {
        return err
    }
    
    switch entity.Operation {
    case "INSERT":
        // 检查是否已存在（幂等性）
        exists, err := s.repo.BobbinExists(ctx, bobbin.ID)
        if err != nil {
            return err
        }
        if exists {
            return nil  // 已存在，跳过
        }
        
        return s.repo.CreateBobbin(ctx, &bobbin)
        
    case "UPDATE":
        return s.repo.UpdateBobbin(ctx, &bobbin)
        
    case "DELETE":
        return s.repo.DeleteBobbin(ctx, bobbin.ID)
        
    default:
        return fmt.Errorf("未知操作: %s", entity.Operation)
    }
}
```

#### 2.3.2 双向流式同步

```go
func (s *Service) StreamSync(stream pb.SyncService_StreamSyncServer) error {
    ctx := stream.Context()
    deviceID := "" // 从第一个消息中提取
    
    // 启动心跳检测
    heartbeatTicker := time.NewTicker(10 * time.Second)
    defer heartbeatTicker.Stop()
    
    // 启动接收协程
    recvCh := make(chan *pb.SyncMessage)
    go func() {
        for {
            msg, err := stream.Recv()
            if err != nil {
                close(recvCh)
                return
            }
            recvCh <- msg
        }
    }()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
            
        case msg, ok := <-recvCh:
            if !ok {
                return nil  // 客户端关闭
            }
            
            switch payload := msg.Payload.(type) {
            case *pb.SyncMessage_Heartbeat:
                deviceID = payload.Heartbeat.DeviceId
                
                // 回复心跳
                ack := &pb.SyncMessage{
                    Payload: &pb.SyncMessage_HeartbeatAck{
                        HeartbeatAck: &pb.HeartbeatResponse{
                            ServerTime: timestampNow(),
                        },
                    },
                }
                stream.Send(ack)
                
                // 更新设备状态
                s.updateDeviceStatus(ctx, deviceID, payload.Heartbeat.Health)
                
            case *pb.SyncMessage_IncrementalData:
                // 处理增量数据
                s.processIncrementalData(ctx, deviceID, payload.IncrementalData)
            }
            
        case <-heartbeatTicker.C:
            // 心跳超时检测
            if deviceID != "" && s.isHeartbeatTimeout(deviceID) {
                return fmt.Errorf("心跳超时")
            }
        }
    }
}
```

---

## 三、数据访问层（Repository）

### 3.1 PostgreSQL Repository

**接口定义（internal/repository/postgres/repository.go）：**

```go
package postgres

import (
    "context"
    "github.com/igh/internal/ent"
)

type Repository struct {
    client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
    return &Repository{client: client}
}

// Order 相关
func (r *Repository) CreateOrder(ctx context.Context, order *domain.Order) (string, error) {
    return r.client.Order.Create().
        SetOrderNumber(order.OrderNumber).
        SetProductType(order.ProductType).
        SetProductName(order.ProductName).
        SetSpecification(order.Specification).
        SetPlannedQuantity(order.PlannedQuantity).
        SetTargetWeightKg(order.TargetWeightKg).
        Save(ctx)
}

func (r *Repository) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
    order, err := r.client.Order.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    return convertOrderFromEnt(order), nil
}

// Bobbin 批量查询（避免 N+1）
func (r *Repository) GetBobinsBatch(ctx context.Context, codes []string) ([]*domain.Bobbin, error) {
    bobbins, err := r.client.Bobbin.Query().
        Where(bobbin.BobbinCodeIn(codes...)).
        WithLot().       // 预加载关联
        WithModule().
        WithDoffing().
        All(ctx)
    
    if err != nil {
        return nil, err
    }
    
    return convertBobbinsFromEnt(bobbins), nil
}

// 使用 Ent 的 QueryBuilder
func (r *Repository) GetOrdersWithPagination(
    ctx context.Context,
    page int,
    pageSize int,
    filter *domain.OrderFilter,
) ([]*domain.Order, int, error) {
    query := r.client.Order.Query()
    
    // 应用过滤条件
    if filter.Status != "" {
        query = query.Where(order.StatusEQ(filter.Status))
    }
    if filter.ProductType != "" {
        query = query.Where(order.ProductTypeEQ(filter.ProductType))
    }
    
    // 分页查询（使用窗口函数优化）
    total, err := query.Count(ctx)
    if err != nil {
        return nil, 0, err
    }
    
    orders, err := query.
        Offset((page - 1) * pageSize).
        Limit(pageSize).
        Order(ent.Desc(order.FieldCreatedAt)).
        All(ctx)
    
    if err != nil {
        return nil, 0, err
    }
    
    return convertOrdersFromEnt(orders), total, nil
}
```

### 3.2 SQLite Repository（边端）

**接口定义（internal/repository/sqlite/repository.go）：**

```go
package sqlite

import (
    "context"
    "database/sql"
)

type Repository struct {
    db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{db: db}
}

// 事务支持
func (r *Repository) BeginTx(ctx context.Context) (*Tx, error) {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    return &Tx{tx: tx}, nil
}

type Tx struct {
    tx *sql.Tx
}

func (tx *Tx) Commit() error {
    return tx.tx.Commit()
}

func (tx *Tx) Rollback() error {
    return tx.tx.Rollback()
}

// 批量插入优化
func (tx *Tx) CreateBobbinsBatch(ctx context.Context, bobbins []*domain.Bobbin) ([]string, error) {
    // 使用预编译语句 + 批量执行
    stmt, err := tx.tx.PrepareContext(ctx, `
        INSERT INTO bobbins (id, lot_id, module_id, doffing_id, bobbin_code, position, lifecycle, produced_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()
    
    codes := make([]string, len(bobbins))
    for i, b := range bobbins {
        _, err := stmt.ExecContext(ctx,
            b.ID, b.LotID, b.ModuleID, b.DoffingID,
            b.BobbinCode, b.Position, b.Lifecycle, b.ProducedAt,
        )
        if err != nil {
            return nil, err
        }
        codes[i] = b.BobbinCode
    }
    
    return codes, nil
}
```

---

## 四、缓存策略

### 4.1 中心端 Redis 缓存

**缓存层（internal/pkg/cache/redis.go）：**

```go
package cache

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/go-redis/redis/v8"
)

type RedisCache struct {
    client *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
    return &RedisCache{
        client: redis.NewClient(&redis.Options{
            Addr: addr,
        }),
    }
}

// 缓存模式：Cache-Aside
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return false, nil  // 缓存未命中
    }
    if err != nil {
        return false, err
    }
    
    return true, json.Unmarshal([]byte(val), dest)
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return c.client.Set(ctx, key, data, ttl).Err()
}

// 批量获取（减少 RTT）
func (c *RedisCache) MGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
    results, err := c.client.MGet(ctx, keys...).Result()
    if err != nil {
        return nil, err
    }
    
    cache := make(map[string]interface{})
    for i, result := range results {
        if result != nil {
            var data interface{}
            json.Unmarshal([]byte(result.(string)), &data)
            cache[keys[i]] = data
        }
    }
    
    return cache, nil
}
```

**缓存使用示例：**

```go
func (s *Service) GetBobbin(ctx context.Context, bobbinID string) (*domain.Bobbin, error) {
    cacheKey := fmt.Sprintf("bobbin:%s", bobbinID)
    
    // 1. 尝试从缓存获取
    var bobbin domain.Bobbin
    hit, err := s.cache.Get(ctx, cacheKey, &bobbin)
    if err != nil {
        return nil, err
    }
    if hit {
        return &bobbin, nil  // 缓存命中
    }
    
    // 2. 从数据库查询
    bobbin, err = s.repo.GetBobbin(ctx, bobbinID)
    if err != nil {
        return nil, err
    }
    
    // 3. 写入缓存（5 分钟 TTL）
    s.cache.Set(ctx, cacheKey, bobbin, 5*time.Minute)
    
    return bobbin, nil
}
```

### 4.2 边端内存缓存

```go
package cache

import (
    "sync"
    "time"
)

type MemoryCache struct {
    data map[string]cacheItem
    mu   sync.RWMutex
}

type cacheItem struct {
    value  interface{}
    expiry time.Time
}

func NewMemoryCache() *MemoryCache {
    c := &MemoryCache{
        data: make(map[string]cacheItem),
    }
    
    // 启动过期清理协程
    go c.cleanupExpired()
    
    return c
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, ok := c.data[key]
    if !ok {
        return nil, false
    }
    
    if time.Now().After(item.expiry) {
        return nil, false  // 已过期
    }
    
    return item.value, true
}

func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.data[key] = cacheItem{
        value:  value,
        expiry: time.Now().Add(ttl),
    }
}
```

---

## 五、错误处理

### 5.1 错误类型定义

**错误码（internal/pkg/errors/codes.go）：**

```go
package errors

type ErrorCode int

const (
    // 通用错误 (10000-10999)
    CodeInvalidParameter ErrorCode = 10001
    CodeMissingParameter ErrorCode = 10002
    
    // 认证错误 (20000-20999)
    CodeUnauthorized ErrorCode = 20001
    CodeTokenExpired ErrorCode = 20003
    CodePermissionDenied ErrorCode = 20004
    
    // 业务错误 (30000-30999)
    CodeBusinessRuleViolation ErrorCode = 30001
    CodeInvalidStatus ErrorCode = 30002
    
    // 数据错误 (40000-40999)
    CodeResourceNotFound ErrorCode = 40001
    CodeResourceExists ErrorCode = 40002
    CodeDataConflict ErrorCode = 40003
    
    // 系统错误 (50000-50999)
    CodeInternalError ErrorCode = 50001
    CodeDatabaseError ErrorCode = 50002
    CodePLCError ErrorCode = 50003
)

var messages = map[ErrorCode]string{
    CodeInvalidParameter:      "参数错误",
    CodeUnauthorized:          "未登录",
    CodeBusinessRuleViolation: "业务规则违反",
    CodeResourceNotFound:      "资源不存在",
    CodeInternalError:         "服务器内部错误",
}
```

**业务错误封装（internal/pkg/errors/errors.go）：**

```go
package errors

import "fmt"

type Error struct {
    Code    ErrorCode
    Message string
    Details interface{}
}

func (e *Error) Error() string {
    return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func New(code ErrorCode, message string) *Error {
    if message == "" {
        message = messages[code]
    }
    return &Error{
        Code:    code,
        Message: message,
    }
}

func Wrap(code ErrorCode, err error) *Error {
    return &Error{
        Code:    code,
        Message: err.Error(),
    }
}

// 快捷方法
func NotFound(resource string) *Error {
    return New(CodeResourceNotFound, fmt.Sprintf("%s不存在", resource))
}

func InvalidParameter(field string, reason string) *Error {
    return &Error{
        Code:    CodeInvalidParameter,
        Message: fmt.Sprintf("参数错误: %s - %s", field, reason),
        Details: map[string]string{"field": field, "reason": reason},
    }
}
```

### 5.2 错误中间件

```go
package middleware

import (
    "context"
    "github.com/igh/internal/pkg/errors"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func ErrorInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        resp, err := handler(ctx, req)
        if err != nil {
            // 转换业务错误为 gRPC 错误
            if bizErr, ok := err.(*errors.Error); ok {
                grpcCode := mapErrorCode(bizErr.Code)
                return nil, status.Error(grpcCode, bizErr.Message)
            }
            
            // 未知错误
            return nil, status.Error(codes.Internal, err.Error())
        }
        
        return resp, nil
    }
}

func mapErrorCode(code errors.ErrorCode) codes.Code {
    switch {
    case code >= 10000 && code < 20000:
        return codes.InvalidArgument
    case code >= 20000 && code < 30000:
        return codes.Unauthenticated
    case code >= 40000 && code < 50000:
        return codes.NotFound
    case code >= 50000:
        return codes.Internal
    default:
        return codes.Unknown
    }
}
```

---

## 六、依赖注入（Wire）

### 6.1 Wire 配置

**中心端（cmd/igh-center/wire.go）：**

```go
//go:build wireinject
// +build wireinject

package main

import (
    "github.com/google/wire"
    "github.com/igh/internal/service/center/sync"
    "github.com/igh/internal/repository/postgres"
    "github.com/igh/internal/pkg/cache"
)

func InitializeApp(cfg *config.Config) (*App, error) {
    wire.Build(
        // 基础设施
        provideDB,
        provideRedis,
        
        // Repository
        postgres.NewRepository,
        
        // Cache
        cache.NewRedisCache,
        
        // Service
        sync.NewService,
        ota.NewService,
        
        // App
        NewApp,
    )
    return &App{}, nil
}

func provideDB(cfg *config.Config) (*ent.Client, error) {
    return ent.Open("postgres", cfg.Database.DSN)
}

func provideRedis(cfg *config.Config) (*redis.Client, error) {
    return redis.NewClient(&redis.Options{
        Addr: cfg.Redis.Addr,
    }), nil
}
```

**生成代码：**
```bash
cd cmd/igh-center
wire
```

**生成文件（wire_gen.go）：**
```go
// Code generated by Wire. DO NOT EDIT.

func InitializeApp(cfg *config.Config) (*App, error) {
    client, err := provideDB(cfg)
    if err != nil {
        return nil, err
    }
    
    repository := postgres.NewRepository(client)
    redisClient, err := provideRedis(cfg)
    if err != nil {
        return nil, err
    }
    
    redisCache := cache.NewRedisCache(redisClient)
    service := sync.NewService(repository, redisCache)
    
    app := NewApp(service)
    return app, nil
}
```

---

## 七、配置管理

### 7.1 配置文件

**中心端配置（configs/center.yaml）：**

```yaml
server:
  grpc:
    address: 0.0.0.0:9090
    timeout: 30s
  http:
    address: 0.0.0.0:8080
    timeout: 30s

database:
  driver: postgres
  dsn: "host=localhost port=5432 user=igh password=igh123 dbname=igh_center sslmode=disable"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 1h

redis:
  addr: localhost:6379
  password: ""
  db: 0
  pool_size: 10

log:
  level: info
  format: json
  output: stdout

auth:
  jwt_secret: "your-secret-key-here"
  token_expires: 2h
  refresh_expires: 168h

ota:
  package_dir: /var/igh/packages
  max_file_size: 100MB
```

**边端配置（configs/edge.yaml）：**

```yaml
server:
  grpc:
    address: 0.0.0.0:9091
  http:
    address: 0.0.0.0:8081

database:
  driver: sqlite3
  dsn: /var/igh/edge.db
  max_connections: 1  # SQLite 单连接

device:
  id: edge-prod-01
  zone_type: production

center:
  addresses:
    - host: 192.168.1.101
      port: 9090
      role: primary
    - host: 192.168.1.102
      port: 9090
      role: standby
  retry_interval: 5s
  max_retry_count: 3

plc:
  type: siemens_s7
  address: 192.168.1.10
  port: 102
  rack: 0
  slot: 1
  read_interval: 100ms

sync:
  batch_size: 100
  interval: 10s
  max_retry: 8

retention:
  bobbins_days: 30
  grades_days: 30
  doffings_days: 30
```

### 7.2 配置加载

```go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    Log      LogConfig
}

func Load(configFile string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(configFile)
    
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    
    return &cfg, nil
}
```

---

## 八、下一步工作

### 8.1 实现优先级

| 优先级 | 模块 | 说明 |
|--------|------|------|
| **P0** | 数据库 Schema（Ent） | 生成 ORM 代码 |
| **P0** | 边端落纱服务 | 核心业务流程 |
| **P0** | 边端质检服务 | 核心业务流程 |
| **P0** | 边端打包服务 | 核心业务流程 |
| **P0** | 中心端同步服务 | 数据上传 |
| **P1** | 边端仓储服务 | FIFO 算法 |
| **P1** | 中心端查询服务 | 批量查询、追溯 |
| **P1** | PLC 通信模块 | Siemens S7 协议 |
| **P1** | 打印机驱动 | Eidos/Macsa/ZPL |
| **P2** | OTA 更新服务 | 版本管理 |
| **P2** | 报表服务 | 统计聚合 |
| **P2** | 前端界面 | 管理后台 + 边端操作 |

### 8.2 代码生成脚本

**生成 Ent Schema（scripts/gen-ent.sh）：**
```bash
#!/bin/bash

go run -mod=mod entgo.io/ent/cmd/ent new Order Lot Module Doffing Bobbin Grade
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/ent/schema
```

**生成 Wire（scripts/gen-wire.sh）：**
```bash
#!/bin/bash

cd cmd/igh-center && wire
cd cmd/igh-edge && wire
```

---

> **文档状态：** ✅ 服务实现设计完成（第一部分）！  
> **下一步：** 继续完善 PLC 通信、打印机驱动、前端设计等模块
