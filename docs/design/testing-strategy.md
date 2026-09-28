# IGH 测试策略

> **文档编号:** TEST-STRATEGY-001  
> **版本:** 1.0  
> **基于:** 系统需求规格说明书 v1.2 + 服务实现设计 v1.0  
> **日期:** 2026-09-28  
> **设计者:** 浮浮酱

---

## 一、测试策略概览

### 1.1 测试金字塔

```
           ┌─────────────┐
          /   E2E 测试    \     10%  (关键业务流程)
         /─────────────────\
        /   集成测试        \   30%  (服务间交互)
       /─────────────────────\
      /      单元测试          \  60%  (函数、方法)
     /───────────────────────────\
```

**比例分配：**
- **单元测试 (60%)** - 快速、稳定、覆盖核心业务逻辑
- **集成测试 (30%)** - 验证服务间交互、数据库操作
- **E2E 测试 (10%)** - 覆盖关键业务流程

### 1.2 测试目标

| 目标 | 指标 | 当前状态 |
|------|------|----------|
| **代码覆盖率** | ≥ 80% | 待建立 |
| **核心业务覆盖率** | 100% | 待建立 |
| **平均测试执行时间** | ≤ 5分钟 | 待测量 |
| **CI 通过率** | ≥ 95% | 待建立 |

### 1.3 测试工具链

| 类型 | 工具 | 用途 |
|------|------|------|
| **单元测试** | Go testing + testify | Go 标准测试框架 |
| **Mock 工具** | gomock + mockery | 自动生成 Mock |
| **断言库** | testify/assert | 流畅的断言 |
| **集成测试** | testcontainers-go | Docker 容器测试 |
| **E2E 测试** | Go testing + gRPC client | 端到端流程 |
| **性能测试** | Go benchmark + pprof | 性能分析 |
| **覆盖率** | go test -cover | 内置覆盖率 |
| **CI/CD** | GitHub Actions | 自动化测试 |

---

## 二、单元测试策略

### 2.1 测试组织

**目录结构：**

```
internal/
├── service/
│   ├── doffing.go
│   └── doffing_test.go        # 服务层单元测试
├── domain/
│   ├── bobbin.go
│   └── bobbin_test.go         # 领域模型单元测试
└── repository/
    ├── lot_repo.go
    └── lot_repo_test.go       # 仓储层单元测试
```

**命名规范：**
- 测试文件：`<source>_test.go`
- 测试函数：`Test<FunctionName>`
- 子测试：`t.Run("<场景描述>", func(t *testing.T) {...})`

### 2.2 服务层单元测试

**示例：DoffingService.ExecuteDoffing**

```go
package service

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
    
    pb "igh/api/proto/edge/v1"
    "igh/internal/domain"
    "igh/internal/repository/mocks"
)

func TestDoffingService_ExecuteDoffing(t *testing.T) {
    t.Run("成功创建落纱记录", func(t *testing.T) {
        // Arrange - 准备测试数据
        ctx := context.Background()
        
        mockLotRepo := new(mocks.LotRepository)
        mockModuleRepo := new(mocks.ModuleRepository)
        mockDoffingRepo := new(mocks.DoffingRepository)
        mockBobbinRepo := new(mocks.BobbinRepository)
        mockSyncRepo := new(mocks.SyncQueueRepository)
        
        svc := NewDoffingService(
            mockLotRepo,
            mockModuleRepo,
            mockDoffingRepo,
            mockBobbinRepo,
            mockSyncRepo,
        )
        
        // Mock 批次查询
        lot := &domain.Lot{
            ID:              1,
            LotNumber:       "FDY-2026-001-001",
            Prefix:          "A",
            ProductType:     domain.ProductTypeFDY,
            PlannedQuantity: 2500,
            ActualQuantity:  0,
            Status:          domain.LotStatusInProgress,
        }
        mockLotRepo.On("FindByID", ctx, uint64(1)).Return(lot, nil)
        
        // Mock 机台查询
        module := &domain.Module{
            ID:          1,
            ModuleCode:  "M001",
            LineCode:    "L01",
            Side:        "A",
            PositionNum: 24,
            ProductType: domain.ProductTypeFDY,
            Status:      domain.ModuleStatusRunning,
        }
        mockModuleRepo.On("FindByID", ctx, uint64(1)).Return(module, nil)
        
        // Mock 创建落纱记录
        mockDoffingRepo.On("Create", ctx, mock.AnythingOfType("*domain.Doffing")).
            Return(nil).
            Run(func(args mock.Arguments) {
                doffing := args.Get(1).(*domain.Doffing)
                doffing.ID = 100
            })
        
        // Mock 批量创建丝锭
        mockBobbinRepo.On("BatchCreate", ctx, mock.AnythingOfType("[]*domain.Bobbin")).
            Return(nil)
        
        // Mock 更新批次数量
        mockLotRepo.On("UpdateActualQuantity", ctx, uint64(1), 24).Return(nil)
        
        // Mock 同步队列
        mockSyncRepo.On("Enqueue", ctx, mock.AnythingOfType("*domain.SyncTask")).
            Return(nil)
        
        // Act - 执行测试
        req := &pb.DoffingRequest{
            LotId:           1,
            ModuleId:        1,
            DoffingSequence: 1,
            OperatorId:      10,
        }
        
        resp, err := svc.ExecuteDoffing(ctx, req)
        
        // Assert - 验证结果
        require.NoError(t, err)
        require.NotNil(t, resp)
        assert.True(t, resp.Success)
        assert.Equal(t, uint64(100), resp.DoffingId)
        assert.Len(t, resp.BobbinCodes, 24)
        
        // 验证丝锭编码格式
        assert.Equal(t, "FDY2026001A001-001", resp.BobbinCodes[0])
        assert.Equal(t, "FDY2026001A001-024", resp.BobbinCodes[23])
        
        // 验证所有 Mock 都被调用
        mockLotRepo.AssertExpectations(t)
        mockModuleRepo.AssertExpectations(t)
        mockDoffingRepo.AssertExpectations(t)
        mockBobbinRepo.AssertExpectations(t)
        mockSyncRepo.AssertExpectations(t)
    })
    
    t.Run("批次不存在 - 返回错误", func(t *testing.T) {
        ctx := context.Background()
        
        mockLotRepo := new(mocks.LotRepository)
        mockLotRepo.On("FindByID", ctx, uint64(999)).
            Return(nil, domain.ErrLotNotFound)
        
        svc := NewDoffingService(mockLotRepo, nil, nil, nil, nil)
        
        req := &pb.DoffingRequest{
            LotId:    999,
            ModuleId: 1,
        }
        
        resp, err := svc.ExecuteDoffing(ctx, req)
        
        assert.Error(t, err)
        assert.Nil(t, resp)
        assert.Contains(t, err.Error(), "批次不存在")
    })
    
    t.Run("机台状态不匹配 - 返回错误", func(t *testing.T) {
        ctx := context.Background()
        
        mockLotRepo := new(mocks.LotRepository)
        mockModuleRepo := new(mocks.ModuleRepository)
        
        lot := &domain.Lot{
            ID:          1,
            ProductType: domain.ProductTypeFDY,
        }
        mockLotRepo.On("FindByID", ctx, uint64(1)).Return(lot, nil)
        
        // 机台类型不匹配
        module := &domain.Module{
            ID:          1,
            ProductType: domain.ProductTypePOY, // 不匹配
        }
        mockModuleRepo.On("FindByID", ctx, uint64(1)).Return(module, nil)
        
        svc := NewDoffingService(mockLotRepo, mockModuleRepo, nil, nil, nil)
        
        req := &pb.DoffingRequest{
            LotId:    1,
            ModuleId: 1,
        }
        
        resp, err := svc.ExecuteDoffing(ctx, req)
        
        assert.Error(t, err)
        assert.Nil(t, resp)
        assert.Contains(t, err.Error(), "产品类型不匹配")
    })
    
    t.Run("批次已完成 - 返回错误", func(t *testing.T) {
        ctx := context.Background()
        
        mockLotRepo := new(mocks.LotRepository)
        
        lot := &domain.Lot{
            ID:     1,
            Status: domain.LotStatusCompleted, // 已完成
        }
        mockLotRepo.On("FindByID", ctx, uint64(1)).Return(lot, nil)
        
        svc := NewDoffingService(mockLotRepo, nil, nil, nil, nil)
        
        req := &pb.DoffingRequest{
            LotId:    1,
            ModuleId: 1,
        }
        
        resp, err := svc.ExecuteDoffing(ctx, req)
        
        assert.Error(t, err)
        assert.Nil(t, resp)
        assert.Contains(t, err.Error(), "批次已完成")
    })
}
```

### 2.3 领域模型单元测试

**示例：Bobbin 领域模型**

```go
package domain

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestBobbin_CalculateFinalGrade(t *testing.T) {
    t.Run("所有维度 AA - 最终等级 AA", func(t *testing.T) {
        bobbin := &Bobbin{
            InspectionGrade1: GradeAA,
            InspectionGrade2: GradeAA,
            InspectionGrade3: GradeAA,
            InspectionGrade4: GradeAA,
            InspectionGrade5: GradeAA,
        }
        
        finalGrade := bobbin.CalculateFinalGrade()
        
        assert.Equal(t, GradeAA, finalGrade)
    })
    
    t.Run("有一个维度 B - 最终等级 B", func(t *testing.T) {
        bobbin := &Bobbin{
            InspectionGrade1: GradeAA,
            InspectionGrade2: GradeB,
            InspectionGrade3: GradeAA,
            InspectionGrade4: GradeAA,
            InspectionGrade5: GradeAA,
        }
        
        finalGrade := bobbin.CalculateFinalGrade()
        
        assert.Equal(t, GradeB, finalGrade)
    })
    
    t.Run("有一个维度 D - 最终等级 D", func(t *testing.T) {
        bobbin := &Bobbin{
            InspectionGrade1: GradeAA,
            InspectionGrade2: GradeAA,
            InspectionGrade3: GradeD,
            InspectionGrade4: GradeAA,
            InspectionGrade5: GradeAA,
        }
        
        finalGrade := bobbin.CalculateFinalGrade()
        
        assert.Equal(t, GradeD, finalGrade)
    })
}

func TestBobbin_GenerateCode(t *testing.T) {
    t.Run("生成正确的丝锭编码", func(t *testing.T) {
        lot := &Lot{
            LotNumber: "FDY-2026-001-001",
            Prefix:    "A",
        }
        
        code := GenerateBobbinCode(lot, 1, 5)
        
        assert.Equal(t, "FDY2026001A001-005", code)
    })
    
    t.Run("编码补零正确", func(t *testing.T) {
        lot := &Lot{
            LotNumber: "POY-2026-002-010",
            Prefix:    "B",
        }
        
        code := GenerateBobbinCode(lot, 1, 1)
        
        assert.Equal(t, "POY2026002B001-001", code)
    })
}

func TestBobbin_IsQualified(t *testing.T) {
    tests := []struct {
        name     string
        grade    Grade
        expected bool
    }{
        {"AA等级合格", GradeAA, true},
        {"B等级合格", GradeB, true},
        {"C等级合格", GradeC, true},
        {"D等级不合格", GradeD, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            bobbin := &Bobbin{FinalGrade: tt.grade}
            assert.Equal(t, tt.expected, bobbin.IsQualified())
        })
    }
}
```

### 2.4 表驱动测试

**示例：质检等级计算**

```go
func TestInspectionService_CalculateGrade(t *testing.T) {
    tests := []struct {
        name        string
        defectCount int
        weight      float64
        expected    Grade
    }{
        {"无缺陷 - AA", 0, 8000.0, GradeAA},
        {"1个轻微缺陷 - AA", 1, 8000.0, GradeAA},
        {"3个轻微缺陷 - B", 3, 8000.0, GradeB},
        {"重量不足 - C", 0, 7500.0, GradeC},
        {"严重缺陷 - D", 10, 8000.0, GradeD},
    }
    
    svc := NewInspectionService(nil, nil)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := svc.calculateGrade(tt.defectCount, tt.weight)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

## 三、集成测试策略

### 3.1 数据库集成测试

**使用 testcontainers-go 启动真实数据库：**

```go
package repository_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    
    "igh/internal/repository"
    "igh/internal/domain"
)

// 集成测试套件
type IntegrationTestSuite struct {
    ctx         context.Context
    pgContainer testcontainers.Container
    db          *ent.Client
}

func (suite *IntegrationTestSuite) SetupSuite(t *testing.T) {
    ctx := context.Background()
    
    // 启动 PostgreSQL 容器
    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "igh_test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections").
            WithOccurrence(2).
            WithStartupTimeout(30 * time.Second),
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    
    suite.pgContainer = container
    suite.ctx = ctx
    
    // 获取连接信息
    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432")
    
    dsn := fmt.Sprintf("postgres://test:test@%s:%s/igh_test?sslmode=disable", host, port.Port())
    
    // 连接数据库
    client, err := ent.Open("postgres", dsn)
    require.NoError(t, err)
    
    // 运行迁移
    err = client.Schema.Create(ctx)
    require.NoError(t, err)
    
    suite.db = client
}

func (suite *IntegrationTestSuite) TearDownSuite(t *testing.T) {
    if suite.db != nil {
        suite.db.Close()
    }
    if suite.pgContainer != nil {
        suite.pgContainer.Terminate(suite.ctx)
    }
}

func TestLotRepository_Integration(t *testing.T) {
    suite := &IntegrationTestSuite{}
    suite.SetupSuite(t)
    defer suite.TearDownSuite(t)
    
    repo := repository.NewLotRepository(suite.db)
    
    t.Run("创建并查询批次", func(t *testing.T) {
        ctx := context.Background()
        
        // 创建批次
        lot := &domain.Lot{
            LotNumber:       "FDY-2026-001-001",
            OrderNumber:     "FDY-2026-001",
            Prefix:          "A",
            ProductType:     domain.ProductTypeFDY,
            PlannedQuantity: 2500,
            ActualQuantity:  0,
            Status:          domain.LotStatusInProgress,
        }
        
        err := repo.Create(ctx, lot)
        require.NoError(t, err)
        require.NotZero(t, lot.ID)
        
        // 查询批次
        found, err := repo.FindByID(ctx, lot.ID)
        require.NoError(t, err)
        assert.Equal(t, lot.LotNumber, found.LotNumber)
        assert.Equal(t, lot.Prefix, found.Prefix)
        assert.Equal(t, lot.PlannedQuantity, found.PlannedQuantity)
    })
    
    t.Run("更新批次实际数量", func(t *testing.T) {
        ctx := context.Background()
        
        // 创建批次
        lot := &domain.Lot{
            LotNumber:       "FDY-2026-001-002",
            OrderNumber:     "FDY-2026-001",
            Prefix:          "B",
            ProductType:     domain.ProductTypeFDY,
            PlannedQuantity: 2500,
            ActualQuantity:  0,
        }
        repo.Create(ctx, lot)
        
        // 更新数量
        err := repo.UpdateActualQuantity(ctx, lot.ID, 24)
        require.NoError(t, err)
        
        // 验证
        found, _ := repo.FindByID(ctx, lot.ID)
        assert.Equal(t, 24, found.ActualQuantity)
    })
    
    t.Run("查询批次列表 - 分页", func(t *testing.T) {
        ctx := context.Background()
        
        // 创建多个批次
        for i := 1; i <= 5; i++ {
            lot := &domain.Lot{
                LotNumber:   fmt.Sprintf("FDY-2026-001-%03d", i),
                OrderNumber: "FDY-2026-001",
                Prefix:      "A",
                ProductType: domain.ProductTypeFDY,
            }
            repo.Create(ctx, lot)
        }
        
        // 分页查询
        lots, total, err := repo.List(ctx, &repository.LotFilter{
            Page:     1,
            PageSize: 3,
        })
        
        require.NoError(t, err)
        assert.Len(t, lots, 3)
        assert.GreaterOrEqual(t, total, 5)
    })
}
```

### 3.2 gRPC 服务集成测试

**测试完整的 gRPC 调用链：**

```go
package integration_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    
    pb "igh/api/proto/edge/v1"
)

func TestDoffingService_gRPC_Integration(t *testing.T) {
    // 启动测试服务器
    server := startTestServer(t)
    defer server.Stop()
    
    // 连接 gRPC 服务
    conn, err := grpc.Dial(
        "localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewDoffingServiceClient(conn)
    
    t.Run("完整落纱流程", func(t *testing.T) {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        // 1. 准备数据（假设数据库已初始化）
        lotID := uint64(1)
        moduleID := uint64(1)
        
        // 2. 执行落纱
        req := &pb.DoffingRequest{
            LotId:           lotID,
            ModuleId:        moduleID,
            DoffingSequence: 1,
            OperatorId:      10,
        }
        
        resp, err := client.ExecuteDoffing(ctx, req)
        
        // 3. 验证响应
        require.NoError(t, err)
        require.NotNil(t, resp)
        assert.True(t, resp.Success)
        assert.NotZero(t, resp.DoffingId)
        assert.Len(t, resp.BobbinCodes, 24)
        
        // 4. 验证数据库状态
        // （查询数据库验证落纱记录、丝锭记录、同步队列）
        verifyDoffingInDatabase(t, resp.DoffingId)
        verifyBobbinsInDatabase(t, resp.BobbinCodes)
        verifySyncQueueInDatabase(t, resp.DoffingId)
    })
}
```

### 3.3 缓存集成测试

**使用 miniredis 模拟 Redis：**

```go
package cache_test

import (
    "context"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "igh/internal/cache"
)

func TestLotCache_Integration(t *testing.T) {
    // 启动 miniredis
    mr, err := miniredis.Run()
    require.NoError(t, err)
    defer mr.Close()
    
    // 创建 Redis 客户端
    rdb := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    
    lotCache := cache.NewLotCache(rdb)
    
    t.Run("缓存读写", func(t *testing.T) {
        ctx := context.Background()
        
        lot := &domain.Lot{
            ID:        1,
            LotNumber: "FDY-2026-001-001",
            Prefix:    "A",
        }
        
        // 写入缓存
        err := lotCache.Set(ctx, lot, 5*time.Minute)
        require.NoError(t, err)
        
        // 读取缓存
        cached, err := lotCache.Get(ctx, 1)
        require.NoError(t, err)
        assert.Equal(t, lot.LotNumber, cached.LotNumber)
    })
    
    t.Run("缓存过期", func(t *testing.T) {
        ctx := context.Background()
        
        lot := &domain.Lot{ID: 2, LotNumber: "FDY-2026-001-002"}
        
        // 写入缓存（1秒过期）
        lotCache.Set(ctx, lot, 1*time.Second)
        
        // 立即读取 - 应该存在
        _, err := lotCache.Get(ctx, 2)
        require.NoError(t, err)
        
        // 模拟时间流逝
        mr.FastForward(2 * time.Second)
        
        // 再次读取 - 应该不存在
        _, err = lotCache.Get(ctx, 2)
        assert.Error(t, err)
    })
}
```

---

## 四、E2E 测试策略

### 4.1 关键业务流程

**流程1：完整的落纱 → 质检 → 打包 → 入库流程**

```go
package e2e_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    edgePb "igh/api/proto/edge/v1"
    centerPb "igh/api/proto/center/v1"
)

func TestE2E_FullProductionFlow(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过 E2E 测试（使用 -short 标志）")
    }
    
    // 准备测试环境
    env := setupE2EEnvironment(t)
    defer env.Teardown()
    
    ctx := context.Background()
    
    t.Run("完整生产流程", func(t *testing.T) {
        // ========== 步骤1：落纱 ==========
        t.Log("步骤1：执行落纱操作")
        
        doffingReq := &edgePb.DoffingRequest{
            LotId:           env.TestLotID,
            ModuleId:        env.TestModuleID,
            DoffingSequence: 1,
            OperatorId:      100,
        }
        
        doffingResp, err := env.EdgeClient.Doffing.ExecuteDoffing(ctx, doffingReq)
        require.NoError(t, err)
        require.True(t, doffingResp.Success)
        require.Len(t, doffingResp.BobbinCodes, 24)
        
        bobbinCodes := doffingResp.BobbinCodes
        t.Logf("✓ 落纱完成，生成 %d 个丝锭", len(bobbinCodes))
        
        // ========== 步骤2：质检（5个维度）==========
        t.Log("步骤2：执行质检操作")
        
        for i, bobbinCode := range bobbinCodes {
            // 扫描丝锭
            scanReq := &edgePb.ScanBobbinRequest{
                BobbinCode: bobbinCode,
            }
            
            scanResp, err := env.EdgeClient.Inspection.ScanBobbin(ctx, scanReq)
            require.NoError(t, err)
            require.NotNil(t, scanResp.Bobbin)
            
            // 提交5维度质检
            inspectionReq := &edgePb.SubmitInspectionRequest{
                BobbinId:  scanResp.Bobbin.Id,
                Dimension: 1, // 视觉维度
                Grade:     edgePb.Grade_GRADE_AA,
                Defects:   []string{},
            }
            
            _, err = env.EdgeClient.Inspection.SubmitInspection(ctx, inspectionReq)
            require.NoError(t, err)
            
            // ... 提交其他4个维度
            
            if i < 3 {
                t.Logf("✓ 丝锭 %s 质检完成", bobbinCode)
            }
        }
        
        t.Logf("✓ 全部 %d 个丝锭质检完成", len(bobbinCodes))
        
        // ========== 步骤3：打包托盘 ==========
        t.Log("步骤3：打包托盘")
        
        palletReq := &edgePb.CreatePalletRequest{
            LotId:      env.TestLotID,
            Capacity:   24,
            OperatorId: 100,
        }
        
        palletResp, err := env.EdgeClient.Packing.CreatePallet(ctx, palletReq)
        require.NoError(t, err)
        
        palletCode := palletResp.PalletCode
        t.Logf("✓ 托盘创建成功: %s", palletCode)
        
        // 扫描丝锭装入托盘
        for _, bobbinCode := range bobbinCodes {
            addReq := &edgePb.AddBobbinToPalletRequest{
                PalletCode: palletCode,
                BobbinCode: bobbinCode,
            }
            
            _, err := env.EdgeClient.Packing.AddBobbinToPallet(ctx, addReq)
            require.NoError(t, err)
        }
        
        t.Logf("✓ 24 个丝锭已装入托盘")
        
        // 封装托盘
        sealReq := &edgePb.SealPalletRequest{
            PalletCode:  palletCode,
            GrossWeight: 198.0,
            TareWeight:  18.0,
        }
        
        sealResp, err := env.EdgeClient.Packing.SealPallet(ctx, sealReq)
        require.NoError(t, err)
        require.True(t, sealResp.Success)
        require.Equal(t, 2, sealResp.PrintJobCount) // 2份标签
        
        t.Logf("✓ 托盘封装完成，打印 %d 份标签", sealResp.PrintJobCount)
        
        // ========== 步骤4：入库 ==========
        t.Log("步骤4：入库操作")
        
        inboundReq := &edgePb.InboundPalletRequest{
            PalletCode: palletCode,
            Aisle:      1,
            Row:        2,
            Level:      3,
        }
        
        inboundResp, err := env.EdgeClient.Warehouse.InboundPallet(ctx, inboundReq)
        require.NoError(t, err)
        require.True(t, inboundResp.Success)
        require.Equal(t, "01-02-03", inboundResp.LocationCode)
        
        t.Logf("✓ 托盘入库成功: %s", inboundResp.LocationCode)
        
        // ========== 步骤5：同步到中心端 ==========
        t.Log("步骤5：同步到中心端")
        
        time.Sleep(2 * time.Second) // 等待异步同步
        
        // 验证中心端数据
        queryReq := &centerPb.QueryBobbinRequest{
            BobbinCode: bobbinCodes[0],
        }
        
        queryResp, err := env.CenterClient.Bobbin.QueryBobbin(ctx, queryReq)
        require.NoError(t, err)
        require.NotNil(t, queryResp.Bobbin)
        assert.Equal(t, bobbinCodes[0], queryResp.Bobbin.BobbinCode)
        assert.Equal(t, palletCode, queryResp.Bobbin.PalletCode)
        assert.Equal(t, "01-02-03", queryResp.Bobbin.LocationCode)
        
        t.Logf("✓ 中心端数据同步完成")
        
        // ========== 验证完整追溯链 ==========
        t.Log("步骤6：验证追溯链")
        
        traceReq := &centerPb.TraceBobbinRequest{
            BobbinCode: bobbinCodes[0],
        }
        
        traceResp, err := env.CenterClient.Bobbin.TraceBobbin(ctx, traceReq)
        require.NoError(t, err)
        
        trace := traceResp.Trace
        assert.Equal(t, env.TestLotNumber, trace.LotNumber)
        assert.Equal(t, env.TestModuleCode, trace.ModuleCode)
        assert.NotNil(t, trace.DoffingTime)
        assert.Len(t, trace.Inspections, 5) // 5个维度
        assert.Equal(t, palletCode, trace.PalletCode)
        assert.Equal(t, "01-02-03", trace.LocationCode)
        
        t.Logf("✓ 追溯链完整")
        
        t.Log("========== E2E 测试全部通过 ==========")
    })
}
```

### 4.2 边端离线场景

**测试边端离线时的本地自治能力：**

```go
func TestE2E_EdgeOfflineAutonomy(t *testing.T) {
    env := setupE2EEnvironment(t)
    defer env.Teardown()
    
    ctx := context.Background()
    
    t.Run("边端离线自治", func(t *testing.T) {
        // ========== 步骤1：正常在线操作 ==========
        t.Log("步骤1：在线状态 - 执行落纱")
        
        doffingReq := &edgePb.DoffingRequest{
            LotId:    env.TestLotID,
            ModuleId: env.TestModuleID,
        }
        
        doffingResp, err := env.EdgeClient.Doffing.ExecuteDoffing(ctx, doffingReq)
        require.NoError(t, err)
        
        // ========== 步骤2：模拟离线 ==========
        t.Log("步骤2：模拟网络断开")
        
        env.SimulateNetworkDisconnect()
        
        // ========== 步骤3：离线时继续操作 ==========
        t.Log("步骤3：离线状态 - 继续质检")
        
        // 质检操作应该能正常进行（本地自治）
        scanReq := &edgePb.ScanBobbinRequest{
            BobbinCode: doffingResp.BobbinCodes[0],
        }
        
        scanResp, err := env.EdgeClient.Inspection.ScanBobbin(ctx, scanReq)
        require.NoError(t, err)
        
        inspectionReq := &edgePb.SubmitInspectionRequest{
            BobbinId:  scanResp.Bobbin.Id,
            Dimension: 1,
            Grade:     edgePb.Grade_GRADE_AA,
        }
        
        _, err = env.EdgeClient.Inspection.SubmitInspection(ctx, inspectionReq)
        require.NoError(t, err)
        
        t.Log("✓ 离线状态下质检操作成功")
        
        // ========== 步骤4：恢复在线 ==========
        t.Log("步骤4：恢复网络连接")
        
        env.SimulateNetworkReconnect()
        
        // ========== 步骤5：验证自动同步 ==========
        t.Log("步骤5：等待自动同步")
        
        time.Sleep(5 * time.Second) // 等待同步完成
        
        // 查询中心端数据
        queryReq := &centerPb.QueryBobbinRequest{
            BobbinCode: doffingResp.BobbinCodes[0],
        }
        
        queryResp, err := env.CenterClient.Bobbin.QueryBobbin(ctx, queryReq)
        require.NoError(t, err)
        assert.NotNil(t, queryResp.Bobbin.Inspections)
        
        t.Log("✓ 离线数据已同步到中心端")
    })
}
```

---

## 五、性能测试

### 5.1 基准测试

**使用 Go benchmark：**

```go
package service

import (
    "context"
    "testing"
)

func BenchmarkDoffingService_ExecuteDoffing(b *testing.B) {
    svc := setupMockedService()
    ctx := context.Background()
    
    req := &pb.DoffingRequest{
        LotId:    1,
        ModuleId: 1,
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        svc.ExecuteDoffing(ctx, req)
    }
}

func BenchmarkBobbinRepository_BatchCreate(b *testing.B) {
    repo := setupTestRepository(b)
    ctx := context.Background()
    
    bobbins := make([]*domain.Bobbin, 24)
    for i := range bobbins {
        bobbins[i] = &domain.Bobbin{
            BobbinCode: fmt.Sprintf("TEST-%03d", i),
        }
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        repo.BatchCreate(ctx, bobbins)
    }
}
```

### 5.2 压力测试

**使用 k6 进行 gRPC 压力测试：**

```javascript
// test/load/doffing_load_test.js
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const client = new grpc.Client();
client.load(['../../api/proto'], 'edge/v1/operations.proto');

export let options = {
    vus: 10,        // 10 个虚拟用户
    duration: '30s', // 持续 30 秒
    thresholds: {
        'grpc_req_duration{method="ExecuteDoffing"}': ['p(95)<500'], // 95% 响应 < 500ms
        'checks': ['rate>0.95'], // 95% 请求成功
    },
};

export default () => {
    client.connect('localhost:50051', { plaintext: true });
    
    const req = {
        lot_id: 1,
        module_id: 1,
        doffing_sequence: 1,
        operator_id: 100,
    };
    
    const response = client.invoke('igh.edge.v1.DoffingService/ExecuteDoffing', req);
    
    check(response, {
        'status is OK': (r) => r && r.status === grpc.StatusOK,
        'response has bobbin codes': (r) => r && r.message.bobbin_codes.length === 24,
    });
    
    client.close();
};
```

**运行压力测试：**

```bash
k6 run test/load/doffing_load_test.js
```

---

## 六、CI/CD 集成

### 6.1 GitHub Actions 配置

**.github/workflows/test.yml：**

```yaml
name: Test

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  unit-test:
    name: Unit Tests
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
      
      - name: Run unit tests
        run: |
          go test -v -race -coverprofile=coverage.txt -covermode=atomic ./internal/...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.txt
  
  integration-test:
    name: Integration Tests
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: igh_test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Run integration tests
        run: |
          go test -v -tags=integration ./test/integration/...
        env:
          DATABASE_URL: postgres://test:test@localhost:5432/igh_test?sslmode=disable
          REDIS_URL: redis://localhost:6379/0
  
  e2e-test:
    name: E2E Tests
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Start test environment
        run: |
          docker-compose -f test/e2e/docker-compose.yml up -d
          sleep 10  # 等待服务启动
      
      - name: Run E2E tests
        run: |
          go test -v -tags=e2e ./test/e2e/...
      
      - name: Cleanup
        if: always()
        run: |
          docker-compose -f test/e2e/docker-compose.yml down
```

### 6.2 测试覆盖率报告

**生成覆盖率报告：**

```bash
# 运行测试并生成覆盖率
go test -coverprofile=coverage.out ./...

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

---

## 七、测试最佳实践

### 7.1 测试命名规范

| 层次 | 命名格式 | 示例 |
|------|----------|------|
| **测试文件** | `<source>_test.go` | `doffing_test.go` |
| **测试函数** | `Test<Type>_<Method>` | `TestDoffingService_ExecuteDoffing` |
| **子测试** | `<场景描述>` | `成功创建落纱记录` |
| **基准测试** | `Benchmark<Type>_<Method>` | `BenchmarkDoffingService_ExecuteDoffing` |

### 7.2 AAA 模式

**Arrange - Act - Assert（准备 - 执行 - 验证）：**

```go
func TestExample(t *testing.T) {
    // Arrange - 准备测试数据和依赖
    mockRepo := new(mocks.Repository)
    svc := NewService(mockRepo)
    
    // Act - 执行测试操作
    result, err := svc.DoSomething()
    
    // Assert - 验证结果
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 7.3 测试隔离

- ✅ 每个测试独立运行
- ✅ 不依赖测试顺序
- ✅ 不共享状态
- ✅ 使用 t.Cleanup() 清理资源

### 7.4 测试数据管理

**使用工厂函数创建测试数据：**

```go
package testdata

func NewTestLot() *domain.Lot {
    return &domain.Lot{
        LotNumber:       "FDY-2026-001-001",
        OrderNumber:     "FDY-2026-001",
        Prefix:          "A",
        ProductType:     domain.ProductTypeFDY,
        PlannedQuantity: 2500,
        ActualQuantity:  0,
        Status:          domain.LotStatusInProgress,
    }
}

func NewTestBobbins(count int) []*domain.Bobbin {
    bobbins := make([]*domain.Bobbin, count)
    for i := range bobbins {
        bobbins[i] = &domain.Bobbin{
            BobbinCode:  fmt.Sprintf("TEST-%03d", i+1),
            Position:    i + 1,
            GrossWeight: 8250.0,
            NetWeight:   8000.0,
            FinalGrade:  domain.GradeAA,
        }
    }
    return bobbins
}
```

---

## 八、测试执行

### 8.1 本地测试命令

```bash
# 运行所有测试
make test

# 运行单元测试
go test ./internal/...

# 运行集成测试
go test -tags=integration ./test/integration/...

# 运行 E2E 测试
go test -tags=e2e ./test/e2e/...

# 运行特定测试
go test -v -run TestDoffingService_ExecuteDoffing ./internal/service/

# 运行基准测试
go test -bench=. -benchmem ./internal/service/

# 查看覆盖率
go test -cover ./...
```

### 8.2 Makefile

```makefile
.PHONY: test test-unit test-integration test-e2e test-coverage

# 运行所有测试
test: test-unit test-integration

# 单元测试
test-unit:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.txt ./internal/...

# 集成测试
test-integration:
	@echo "Running integration tests..."
	docker-compose -f test/docker-compose.test.yml up -d
	sleep 5
	go test -v -tags=integration ./test/integration/... || (docker-compose -f test/docker-compose.test.yml down && exit 1)
	docker-compose -f test/docker-compose.test.yml down

# E2E 测试
test-e2e:
	@echo "Running E2E tests..."
	docker-compose -f test/e2e/docker-compose.yml up -d
	sleep 10
	go test -v -tags=e2e ./test/e2e/... || (docker-compose -f test/e2e/docker-compose.yml down && exit 1)
	docker-compose -f test/e2e/docker-compose.yml down

# 覆盖率报告
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
```

---

## 九、总结

### 9.1 测试策略总览

| 测试类型 | 比例 | 执行时间 | 目标 |
|----------|------|----------|------|
| **单元测试** | 60% | < 2分钟 | 代码覆盖率 ≥ 80% |
| **集成测试** | 30% | < 3分钟 | 服务间交互验证 |
| **E2E 测试** | 10% | < 5分钟 | 关键流程验证 |

### 9.2 测试清单

**单元测试：**
- [x] 服务层业务逻辑测试
- [x] 领域模型测试
- [x] 工具函数测试
- [x] 表驱动测试

**集成测试：**
- [x] 数据库集成测试（testcontainers）
- [x] gRPC 服务集成测试
- [x] 缓存集成测试（miniredis）

**E2E 测试：**
- [x] 完整生产流程测试
- [x] 边端离线自治测试
- [x] 数据同步测试

**性能测试：**
- [x] 基准测试（Go benchmark）
- [x] 压力测试（k6）

**CI/CD：**
- [x] GitHub Actions 配置
- [x] 覆盖率报告

### 9.3 下一步工作

1. ✅ **测试策略** - 本文档
2. ⏳ **部署方案** - Docker + 双机热备 + 监控

---

> **文档状态：** ✅ 测试策略完成！  
> **下一步：** 部署方案设计
