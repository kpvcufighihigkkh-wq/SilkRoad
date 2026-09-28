# 边缘端后端深度分析

> **文档编号：** SILKROAD-Analysis-02  
> **系列：** Silkroad V5 源码深度分析（第2篇，共5篇）  
> **分析对象：** Edge Backend（Go 语言边缘端服务）  
> **分析日期：** 2026-09-17  
> **分析方法：** 逐文件源码级解读 + 架构拆解  
> **代码规模：** ~3,500 行核心 Go 代码（不含测试和生成代码）

---

## 目录

1. [系统概述与定位](#1-系统概述与定位)
2. [入口与启动流程](#2-入口与启动流程)
3. [配置体系](#3-配置体系)
4. [S7 传输层](#4-s7-传输层)
5. [数据规格定义层](#5-数据规格定义层)
6. [采集模块](#6-采集模块)
7. [Pattern 引擎](#7-pattern-引擎)
   - 7.1 [核心框架 patterns.go](#71-核心框架-patternsgo)
   - 7.2 [触发模式 trigger.go](#72-触发模式-triggergo)
   - 7.3 [请求确认模式 reqconfirm.go](#73-请求确认模式-reqconfirmgo)
   - 7.4 [序列执行模式 sequence.go](#74-序列执行模式-sequencego)
   - 7.5 [派发模式 dispatch.go](#75-派发模式-dispatchgo)
   - 7.6 [打印任务模式 printjob.go](#76-打印任务模式-printjobgo)
8. [上传模块](#8-上传模块)
9. [业务模块](#9-业务模块)
   - 9.1 [落筒服务 doffing](#91-落筒服务-doffing)
   - 9.2 [包装服务 packing](#92-包装服务-packing)
10. [本地存储层](#10-本地存储层)
11. [打印模块](#11-打印模块)
12. [架构图集](#12-架构图集)
13. [设计决策与 V4 经验教训](#13-设计决策与-v4-经验教训)
14. [总结与评价](#14-总结与评价)

---

## 1. 系统概述与定位

### 1.1 边缘端在整体架构中的角色

Silkroad V5 采用**边缘-云端**分离架构。边缘端（Edge）部署在工厂现场的工控机上，直接与 PLC（西门子 S7-300/1200/1500）通信，承担以下核心职责：

| 职责 | 说明 |
|------|------|
| **PLC 通信** | 通过 S7 协议直连 PLC，读写 DB 数据块 |
| **数据采集** | 周期性轮询 PLC 数据块，维护实时状态快照 |
| **事件检测** | 基于 Pattern 引擎检测 PLC 信号变化，触发业务逻辑 |
| **业务处理** | 落筒、包装、打印等业务逻辑在边缘端本地完成 |
| **数据暂存** | SQLite 本地存储，断网不丢数据 |
| **数据上传** | 按 FK 顺序批量上传至云端服务器 |
| **心跳上报** | 定期上报自身状态、PLC 连接状态、采集数据快照 |

### 1.2 技术栈

| 组件 | 技术选型 | 版本 | 说明 |
|------|----------|------|------|
| 语言 | Go | 1.21+ | 高并发、低资源占用 |
| PLC 通信 | gos7 | — | S7 协议 Go 实现 |
| 本地数据库 | SQLite | WAL 模式 | 嵌入式，零运维 |
| Web UI | 嵌入式静态文件 | Vue.js 3 | 本地监控界面 |
| 配置格式 | JSON | — | 结构化配置文件 |
| 日志 | 标准库 slog | — | 结构化日志 |

### 1.3 代码组织结构

```
cmd/edge/
  main.go               # 入口，~922行，8阶段启动

internal/edge/
  config.go              # 配置定义与校验，~370行
  transport/
    s7.go                # S7传输层，~136行
  collect/
    collect.go           # 周期采集，~152行
  patterns/
    patterns.go          # Pattern核心框架，~155行
    trigger.go           # 触发模式，~190行
    reqconfirm.go        # 请求确认模式，~238行
    sequence.go          # 序列执行，~114行
    dispatch.go          # 派发模式，~297行
    printjob.go          # 打印任务模式，~310行
  upload/
    upload.go            # 数据上传，~338行
  store/
    store.go             # SQLite存储层
  doffing/
    service.go           # 落筒业务核心
    winder.go            # 卷绕机监控
  packing/
    service.go           # 包装业务核心
  printer/
    driver.go            # 打印驱动接口
    zpl.go               # Zebra ZPL驱动
    brother.go           # Brother-TD驱动
    stub.go              # 测试桩驱动
    template.go          # 模板引擎

internal/shared/
  dbspec/
    spec.go              # DB数据规格定义，~235行
```

---

## 2. 入口与启动流程

### 2.1 main.go 总览（~922行）

`cmd/edge/main.go` 是边缘端的入口文件，实现了一个精心编排的 8 阶段启动序列。每个阶段都有明确的依赖关系——后续阶段依赖前序阶段的初始化结果。

### 2.2 启动序列详解

#### 阶段 1：配置加载

```
加载 JSON 配置文件 → 解析为 FileConfig 结构 → 校验所有 Binding 配置
```

配置文件路径通过命令行参数或环境变量指定。加载后立即执行 `validateBinding` 对每个 Binding 进行严格校验（详见第3章）。**任何校验失败都会阻止启动**——这是 V4 版本血泪教训的产物：运行时才发现配置错误导致的生产中断代价极高。

#### 阶段 2：PLC 连接建立

```
遍历 PLCs 配置 → 为每个 PLC 创建 S7 Transport → 初始连接（允许失败）
```

为配置中的每个 PLC 创建独立的 `transport.S7` 实例。注意：**初始连接允许失败**。边缘端设计为在 PLC 不可达时也能启动——连接会在后续操作中自动重建。这个设计决策源于工厂现场 PLC 可能因维护而暂时离线的实际场景。

每个 S7 实例包含：
- TCP 连接处理器（`gos7.TCPClientHandler`）
- S7 客户端（`gos7.Client`）
- 互斥锁（串行化访问）
- 断开时间戳（重连冷却计算）

#### 阶段 3：采集轮询器创建

```
遍历 Collect 配置 → 为每个采集点创建 Poller → 关联对应 PLC 的 S7 Transport
```

每个 `Collect` 配置项对应一个独立的 `Poller` goroutine。Poller 被创建但**尚未启动**——实际轮询在后续阶段统一启动。

#### 阶段 4：Binding 启动

```
遍历 Bindings → 按 Pattern 类型实例化 → 注入 Handler → 启动 goroutine
```

这是启动序列中最复杂的阶段。每个 Binding 根据其 `Pattern` 字段（trigger / reqconfirm / dispatch / printjob）选择对应的 Pattern 引擎实例化。业务逻辑通过 `Handler` 函数注入——这实现了 Pattern 引擎与业务逻辑的**关注点分离**。

每个 Binding 作为独立 goroutine 运行，拥有自己的：
- PLC 读写通道（通过 S7 Transport）
- 轮询间隔
- 状态跟踪（阶段、计数器、最近事件）
- 错误处理策略

#### 阶段 5：WebUI 启动

```
加载嵌入式前端资源 → 启动 HTTP 服务 → 暴露 API + 静态文件
```

在配置的 `Listen` 地址启动 HTTP 服务，提供：
- 嵌入式 Vue.js 3 监控界面
- 状态 API（采集数据、Pattern 状态、PLC 连接状态）
- 健康检查端点

#### 阶段 6：上传循环启动

```
启动上传 goroutine → 按 FK 顺序轮询 SQLite → 批量 POST 至服务端
```

上传模块作为后台 goroutine 持续运行，负责将 SQLite 中的业务数据同步到云端服务器。详见第8章。

#### 阶段 7：心跳循环启动

```
启动心跳 goroutine → 按 HeartbeatMS 间隔 → 上报边缘端状态
```

心跳包含完整的边缘端运行状态：
- 边缘端 ID 和名称
- 所有 PLC 的连接状态
- 所有采集器的当前数据快照
- 所有 Pattern 的运行状态和计数器
- 上传队列深度

#### 阶段 8：基础数据同步

```
从服务端拉取基础数据（品种、配方等）→ 写入 SQLite → 供业务模块使用
```

最后一个阶段从云端拉取业务基础数据。这些数据是业务逻辑的依赖——例如落筒模块需要知道当前订单的品种信息才能正确生成标签。

### 2.3 信号处理与优雅关闭

main.go 注册了 `SIGINT` 和 `SIGTERM` 信号处理器。收到信号后执行有序关闭：

1. 停止所有 Binding goroutine（通过 context cancellation）
2. 停止所有 Poller goroutine
3. 等待上传循环完成当前批次
4. 关闭所有 PLC 连接
5. 关闭 SQLite 数据库
6. 退出

**设计要点：** 关闭顺序与启动顺序严格相反。上传循环被允许完成当前正在进行的批次上传，避免数据丢失。

```mermaid
flowchart TD
    START([边缘端启动]) --> P1

    subgraph P1["阶段 1: 配置加载"]
        P1A[读取 JSON 配置文件] --> P1B[解析为 FileConfig]
        P1B --> P1C[validateBinding 校验每个 Binding]
        P1C --> P1D{校验通过?}
        P1D -->|否| P1E[输出错误详情并退出]
        P1D -->|是| P1F[配置就绪]
    end

    P1F --> P2

    subgraph P2["阶段 2: PLC 连接"]
        P2A[遍历 PLCs 配置列表] --> P2B[创建 S7 Transport 实例]
        P2B --> P2C[尝试初始连接]
        P2C --> P2D{连接成功?}
        P2D -->|是| P2E[记录连接成功]
        P2D -->|否| P2F[记录警告,后续自动重连]
        P2E --> P2G[PLC 连接池就绪]
        P2F --> P2G
    end

    P2G --> P3

    subgraph P3["阶段 3: 采集器创建"]
        P3A[遍历 Collect 配置] --> P3B[创建 Poller 实例]
        P3B --> P3C[关联 S7 Transport]
        P3C --> P3D[Poller 池就绪,待启动]
    end

    P3D --> P4

    subgraph P4["阶段 4: Binding 启动"]
        P4A[遍历 Bindings 配置] --> P4B{Pattern 类型}
        P4B -->|trigger| P4C[实例化 TriggerBinding]
        P4B -->|reqconfirm| P4D[实例化 ReqConfirmBinding]
        P4B -->|dispatch| P4E[实例化 DispatchBinding]
        P4B -->|printjob| P4F[实例化 PrintJobBinding]
        P4C --> P4G[注入业务 Handler]
        P4D --> P4G
        P4E --> P4G
        P4F --> P4G
        P4G --> P4H[启动独立 goroutine]
    end

    P4H --> P5

    subgraph P5["阶段 5: WebUI"]
        P5A[加载嵌入式前端资源] --> P5B[启动 HTTP 服务器]
        P5B --> P5C[暴露 API 和监控界面]
    end

    P5C --> P6

    subgraph P6["阶段 6: 上传循环"]
        P6A[启动上传 goroutine] --> P6B[按 FK 顺序遍历表]
        P6B --> P6C[游标增量查询 + 批量 POST]
    end

    P6C --> P7

    subgraph P7["阶段 7: 心跳循环"]
        P7A[启动心跳 goroutine] --> P7B[按 HeartbeatMS 间隔]
        P7B --> P7C[聚合全部状态并上报]
    end

    P7C --> P8

    subgraph P8["阶段 8: 基础数据同步"]
        P8A[从服务端拉取基础数据] --> P8B[写入 SQLite]
        P8B --> P8C[初始化完成]
    end

    P8C --> RUNNING([系统运行中])

    RUNNING --> SIG{收到 SIGINT/SIGTERM}
    SIG --> SD1[停止所有 Binding goroutine]
    SD1 --> SD2[停止所有 Poller goroutine]
    SD2 --> SD3[等待上传完成当前批次]
    SD3 --> SD4[关闭所有 PLC 连接]
    SD4 --> SD5[关闭 SQLite]
    SD5 --> EXIT([进程退出])

    style P1 fill:#e8f5e9,stroke:#2e7d32
    style P2 fill:#e3f2fd,stroke:#1565c0
    style P3 fill:#fff3e0,stroke:#e65100
    style P4 fill:#fce4ec,stroke:#c62828
    style P5 fill:#f3e5f5,stroke:#7b1fa2
    style P6 fill:#e0f7fa,stroke:#00695c
    style P7 fill:#fff8e1,stroke:#f57f17
    style P8 fill:#efebe9,stroke:#4e342e
    style RUNNING fill:#c8e6c9,stroke:#1b5e20,stroke-width:3px
    style EXIT fill:#ffcdd2,stroke:#b71c1c,stroke-width:3px
```

---

## 3. 配置体系

### 3.1 FileConfig 结构（internal/edge/config.go ~370行）

`FileConfig` 是边缘端的顶层配置结构，定义了整个边缘端的行为。以下是完整的字段分析：

#### 3.1.1 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | string | 边缘端唯一标识，心跳和上传时用于身份识别 |
| `Name` | string | 人类可读名称，显示在 WebUI 和日志中 |
| `Servers` | []string | 服务端地址列表，支持多服务器故障转移 |
| `Listen` | string | WebUI HTTP 监听地址，如 `:8090` |
| `HeartbeatMS` | int | 心跳间隔（毫秒），0 表示禁用 |
| `PLCs` | []PLCConf | PLC 连接配置列表 |
| `Collect` | []CollectConf | 周期采集配置列表 |
| `Bindings` | []BindingConf | Pattern 绑定配置列表 |
| `Printers` | []PrinterConf | 打印机配置列表 |
| `Shifts` | []ShiftConf | 班次定义列表 |
| `Doffing` | *DoffingConf | 落筒子配置（可选） |
| `Packing` | *PackingConf | 包装子配置（可选） |
| `Warehouse` | *WarehouseConf | 仓库子配置（可选） |
| `QC` | *QCConf | 质检子配置（可选） |

#### 3.1.2 PLCConf — PLC 连接定义

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | PLC 逻辑名称，Binding 通过此名称引用 |
| `Addr` | string | PLC IP 地址，如 `192.168.1.10` |
| `Rack` | int | S7 机架号（通常为 0） |
| `Slot` | int | S7 插槽号（S7-300: 2, S7-1200/1500: 1） |

#### 3.1.3 CollectConf — 采集点定义

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 采集点名称 |
| `PLC` | string | 关联 PLC 名称 |
| `DB` | int | DB 数据块编号 |
| `Spec` | string | 指向 DBSpec 定义文件 |
| `IntervalMS` | int | 轮询间隔（毫秒），默认 500 |

### 3.2 BindingConf — Pattern 绑定（最复杂的配置项）

BindingConf 是系统中**字段最多的配置结构**，因为它需要覆盖四种 Pattern 的所有场景。通过字段组合实现多态配置：

#### 3.2.1 通用字段（所有 Pattern 共享）

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 绑定名称，日志和状态报告使用 |
| `Pattern` | string | 模式类型：trigger / reqconfirm / dispatch / printjob |
| `PLC` | string | 关联 PLC 名称 |
| `DB` | int | DB 数据块编号 |
| `Spec` | string | DB 数据规格定义 |
| `Handler` | string | 业务处理器名称，如 `end_loading`、`pallet_assign` |
| `PollMS` | int | 轮询间隔（毫秒） |
| `TriggerField` | string | 触发/请求字段名 |
| `ActiveValue` | string | 激活值，默认 `"1"` |
| `AckField` | string | 确认应答字段名 |
| `AckValue` | string | 确认应答值 |
| `WriteBack` | map[string]string | 回写字段映射 |
| `RetryIntervalMS` | int | 重试间隔 |
| `WriteRetries` | int | 写入重试次数 |

#### 3.2.2 reqconfirm 专用字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `ActiveValues` | []string | 多值激活条件 |
| `ConfirmField` | string | 确认字段 |
| `ConfirmValue` | string | 确认值 |
| `ConfirmMirrorField` | string | 确认镜像字段（回显请求值） |
| `ClearTimeoutMS` | int | 清除超时（毫秒） |

#### 3.2.3 dispatch 专用字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `GateField` | string | PLC 门控字段（PLC 侧启用标志） |
| `LatchField` | string | 锁存字段（边缘端占用标志） |
| `ConfirmField` | string | 确认字段（PLC 侧完成确认） |
| `FieldOrder` | []string | 字段写入顺序（决定论保证） |

#### 3.2.4 printjob 专用字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `StartField` | string | 启动信号字段 |
| `RefIDField` | string | 引用 ID 字段 |
| `EndField` | string | 结束信号字段 |
| `StatusField` | string | 状态反馈字段 |
| `ErrorValue` | string | 错误状态值 |
| `OKValue` | string | 成功状态值 |
| `Printer` | string | 关联打印机名称 |

### 3.3 validateBinding — 启动时配置校验

`validateBinding` 是配置系统的**安全门**。它在启动阶段 1 对每个 Binding 执行两层校验：

**第一层：字段存在性校验**

根据 Pattern 类型检查必填字段是否已配置。例如：
- `trigger` 模式必须配置 `TriggerField`、`AckField`
- `dispatch` 模式必须配置 `GateField`、`LatchField`、`ConfirmField`
- `printjob` 模式必须配置 `StartField`、`RefIDField`、`EndField`、`StatusField`

**第二层：字段方向校验（关键创新）**

在 DBSpec 中，每个字段都有方向注解（`DirRead` 或 `DirWrite`），由 SCL 导入工具从 PLC 程序中提取。`validateBinding` 校验：

- 读取类字段（TriggerField、GateField、StartField 等）必须为 `DirRead`
- 写入类字段（AckField、LatchField、EndField 等）必须为 `DirWrite`

**设计意义：** 这种校验能在启动时捕获配置错误——例如把一个 PLC 只读字段配置为 AckField（需要写入）。在 V4 中，这类错误只在运行时才暴露，导致确认信号无法写入 PLC，生产线停滞。

### 3.4 子配置解构

DoffingConf 和 PackingConf 中包含的 Printers 和 Shifts 信息会在配置加载时被**提升**到顶层。这是一个去重策略：

```
加载 FileConfig
  → 提取 Doffing.Printers → 合并到顶层 Printers（去重）
  → 提取 Doffing.Shifts → 合并到顶层 Shifts（去重）
  → 提取 Packing.Printers → 合并到顶层 Printers（去重）
```

**动机：** 打印机和班次是全局资源，不应在多个子配置中重复定义。去重合并确保全局唯一性，避免同一打印机被初始化两次。

### 3.5 四种已知 Pattern 汇总

| Pattern | 通信模型 | 典型场景 | 核心机制 |
|---------|----------|----------|----------|
| `trigger` | 边沿触发 | 落筒完成、小车到达 | 上升沿检测 + 确认回写 |
| `reqconfirm` | 电平保持 | 数据就绪请求 | 签名去重 + 握手状态机 |
| `dispatch` | 主动派发 | 仓库指令下发 | 双门控 + 有序写入 |
| `printjob` | 四条件守卫 | 标签打印触发 | 多条件联合守卫 + 错误锁存 |

---

## 4. S7 传输层

### 4.1 概述（internal/edge/transport/s7.go ~136行）

S7 传输层封装了 gos7 库，提供安全的、串行化的 PLC 通信能力。**这是整个边缘端与物理世界的唯一接口**。

### 4.2 S7 结构体

```go
type S7 struct {
    name       string                    // PLC 逻辑名称
    addr       string                    // PLC IP 地址
    rack, slot int                       // S7 路由参数
    handler    *gos7.TCPClientHandler    // TCP 连接处理器
    client     *gos7.Client              // S7 协议客户端
    mu         sync.Mutex                // 串行化锁
    disconnAt  time.Time                 // 断开时间戳（冷却计算）
    timeout    time.Duration             // 默认 2s
    cooldown   time.Duration             // 默认 3s
}
```

### 4.3 连接管理策略

#### 连接建立（ensureConnected）

```
调用 ensureConnected
  → 检查 client 是否为 nil
    → 非 nil：已连接，直接返回
    → 是 nil：检查冷却期
      → 距离上次断开 < cooldown(3s)：返回 errCooldown
      → 冷却期已过：创建新 handler + client → 尝试连接
        → 成功：返回 nil
        → 失败：记录断开时间，返回错误
```

**冷却机制（V4 实战经验）：** 当 PLC 异常断开时，立即重连往往会失败（PLC 的 TCP 栈可能尚未释放旧连接）。V4 现场经验表明，3 秒冷却能有效避免重连风暴——在高频轮询（200ms）场景下，无冷却的重连会在 1 秒内产生 5 次无效连接尝试，加重 PLC 的网络负担。

#### 连接断开（dropLocked）

```
调用 dropLocked
  → 关闭 client 连接
  → 置 handler = nil, client = nil
  → 记录 disconnAt = time.Now()
```

**关键设计：** `dropLocked` 在持锁状态下调用，因此名称中包含 `Locked` 后缀。这是 Go 社区的命名约定——提醒调用者此函数假设锁已被持有。

### 4.4 ReadDB / WriteDB 操作序列

两个核心操作遵循相同的**锁-连接-操作-解锁**范式：

```
ReadDB(db, start, size):
  mu.Lock()
  defer mu.Unlock()
  err = ensureConnected()
  if err → return nil, err
  buf, err = client.AGReadDB(db, start, size)
  if err → dropLocked(); return nil, err
  return buf, nil

WriteDB(db, start, data):
  mu.Lock()
  defer mu.Unlock()
  err = ensureConnected()
  if err → return err
  err = client.AGWriteDB(db, start, size, data)
  if err → dropLocked(); return err
  return nil
```

**"任何错误即断开"策略：** 这是一个保守但安全的设计。S7 协议是有状态协议——一个通信错误可能意味着连接已处于不一致状态。与其尝试恢复（可能导致数据错乱），不如直接断开并在下次操作时重建连接。

**Mutex 串行化必要性：** gos7 库的 session 不是并发安全的。如果两个 goroutine 同时读写同一个 PLC 连接，会导致 S7 PDU 交错，产生不可预测的错误。Mutex 确保同一时刻只有一个操作在执行。

### 4.5 超时与冷却参数

| 参数 | 默认值 | 来源 | 说明 |
|------|--------|------|------|
| timeout | 2s | 硬编码 | S7 连接和操作超时 |
| cooldown | 3s | 硬编码 | 断开后重连等待时间 |

**2 秒超时的选择依据：** S7 协议在局域网中的典型响应时间 < 50ms。2 秒已经是极宽裕的超时值，足以应对网络抖动但不会让轮询线程长时间阻塞。

---

## 5. 数据规格定义层

### 5.1 概述（internal/shared/dbspec/spec.go ~235行）

DBSpec 是边缘端的**数据建模核心**——它定义了 PLC DB 数据块的字节布局，是 PLC 原始字节与 Go 结构化数据之间的桥梁。

### 5.2 字段类型体系

DBSpec 定义了 11 种字段类型，覆盖 S7 协议的所有基本数据类型：

| 类型 | S7 对应 | 字节数 | Go 映射 | 说明 |
|------|---------|--------|---------|------|
| `bool` | BOOL | 1 bit | bool | 位字段，需要 bit offset |
| `byte` | BYTE | 1 | uint8 | 无符号字节 |
| `int` | INT | 2 | int16 | 有符号 16 位整数 |
| `dint` | DINT | 4 | int32 | 有符号 32 位整数 |
| `word` | WORD | 2 | uint16 | 无符号 16 位字 |
| `dword` | DWORD | 4 | uint32 | 无符号 32 位双字 |
| `real` | REAL | 4 | float32 | IEEE 754 单精度浮点 |
| `lint` | LINT | 8 | int64 | 有符号 64 位整数 |
| `lreal` | LREAL | 8 | float64 | IEEE 754 双精度浮点 |
| `char` | CHAR/STRING | N | string | 定长字符串 |
| `raw` | — | N | []byte | 原始字节，不解码 |

### 5.3 RepeatGroup — 阵列结构建模

工业场景中，PLC 数据块常包含重复结构——例如一个 DB 块中包含 72 个卷绕机的状态数据，每个卷绕机有相同的字段集。

```
RepeatGroup 定义:
  Prefix: "winder"      // 名称前缀
  Count:  72             // 重复次数
  Fields: [              // 每组的字段定义
    { Name: "status", Type: "byte", Offset: 0 },
    { Name: "speed",  Type: "real", Offset: 1 },
    { Name: "doff_no", Type: "dint", Offset: 5 },
    ...
  ]
  Stride: 20             // 每组的字节步长
```

### 5.4 ExpandRepeats() — 展开算法

`ExpandRepeats()` 将 RepeatGroup 展开为扁平字段列表：

```
输入:  RepeatGroup{Prefix: "winder", Count: 3, Stride: 20, Fields: [{Name: "status"}, {Name: "speed"}]}
输出:
  winder_0_status  (offset = base + 0*20 + field_offset)
  winder_0_speed   (offset = base + 0*20 + field_offset)
  winder_1_status  (offset = base + 1*20 + field_offset)
  winder_1_speed   (offset = base + 1*20 + field_offset)
  winder_2_status  (offset = base + 2*20 + field_offset)
  winder_2_speed   (offset = base + 2*20 + field_offset)
```

**命名规则：** `{Prefix}_{Index}_{FieldName}`，索引从 0 开始。展开后的字段与普通字段在处理上完全一致——后续的 Decode、Encode、ChangedFields 操作不需要知道字段是否来自 RepeatGroup。

### 5.5 Validate() — 规格校验

对 DBSpec 执行四项校验：

1. **类型合法性：** 每个字段的 Type 必须在 11 种已知类型中
2. **名称唯一性：** 展开后的所有字段名不得重复
3. **边界检查：** 每个字段的 `Offset + Size` 不得超出 DB 块总大小
4. **Bit 校验：** bool 类型字段的 BitOffset 必须在 0-7 范围内

### 5.6 ChangedFields() — 差异检测

`ChangedFields()` 接收两份原始字节快照（old 和 new），逐字段比较并返回发生变化的字段列表。

```
old: [0x00, 0x01, 0x00, 0x0A, ...]
new: [0x00, 0x01, 0x00, 0x14, ...]

ChangedFields 比较:
  field "status" (offset 0, 1 byte): 0x00 == 0x00 → 无变化
  field "count"  (offset 1, 1 byte): 0x01 == 0x01 → 无变化
  field "speed"  (offset 2, 2 bytes): 0x000A != 0x0014 → 已变化！

返回: ["speed"]
```

**应用场景：** 采集模块（Collect）使用此函数实现增量变更检测——只有发生变化的字段才会进入环形缓冲区。

### 5.7 DecodeField / EncodeField — 字节序处理

S7 协议使用**大端字节序**（Big-Endian），而大多数现代 PC 使用小端字节序。Decode/Encode 函数负责转换：

| 操作 | 输入 | 输出 | 特殊处理 |
|------|------|------|----------|
| DecodeField(bool) | 字节 + bitOffset | "true"/"false" | 位掩码提取 |
| DecodeField(int) | 2 字节大端 | 有符号整数字符串 | binary.BigEndian.Uint16 + 符号扩展 |
| DecodeField(real) | 4 字节大端 | 浮点数字符串 | math.Float32frombits |
| DecodeField(char) | N 字节 | 字符串 | NUL 字符截断 |
| EncodeField(bool) | "true"/"false" | 位操作指令 | 需要 read-modify-write |
| EncodeField(dint) | 整数字符串 | 4 字节大端 | binary.BigEndian.PutUint32 |

### 5.8 DirRead / DirWrite 方向注解

每个字段携带方向元数据，来源于 SCL 导入工具对 PLC 程序的静态分析：

- **DirRead：** PLC 写入、边缘端只读的字段（如传感器数据、PLC 状态标志）
- **DirWrite：** 边缘端写入、PLC 只读的字段（如确认信号、控制指令）

方向注解在 `validateBinding` 中被校验——确保配置不会让边缘端尝试写入一个 PLC 只读字段。

---

## 6. 采集模块

### 6.1 概述（internal/edge/collect/collect.go ~152行）

采集模块实现了**模式 1：周期性轮询**。它是边缘端中最简单但最基础的模块——为心跳上报和 WebUI 实时展示提供数据源。

### 6.2 Poller 结构

```go
type Poller struct {
    name     string
    plc      *transport.S7
    db       int
    spec     *dbspec.Spec
    interval time.Duration       // 默认 500ms
    mu       sync.RWMutex
    values   map[string]string   // 当前值快照
    ring     []Snapshot          // 环形缓冲区（50条）
    lastRaw  []byte              // 上一次原始字节
    errStr   string              // 最近错误描述
}
```

### 6.3 tick() 执行流程

```
tick() 每 interval 调用一次:
  1. ReadDB(db, 0, spec.TotalSize)
     → 失败: 保留 lastValues, 设置 errStr, 返回
  2. Decode: 遍历 spec.Fields, 对每个字段调用 DecodeField
     → 得到 map[string]string（字段名 → 解码值）
  3. ChangedFields(lastRaw, newRaw)
     → 得到变化字段列表
  4. 更新 values map（全量覆盖）
  5. 如有变化字段: 将 Snapshot{Time, ChangedFields, Values} 加入 ring buffer
  6. 更新 lastRaw = newRaw
  7. 清空 errStr
```

### 6.4 环形缓冲区设计

Poller 维护一个固定大小为 50 的环形缓冲区，记录最近 50 次变更快照。这为 WebUI 提供了**近期历史**视图——操作人员可以看到过去的状态变化轨迹，而不仅仅是当前值。

50 条的大小选择基于：
- 默认 500ms 轮询间隔，50 条覆盖约 25 秒历史
- 足够回溯最近几次事件
- 内存占用可控

### 6.5 读取失败处理

当 ReadDB 失败时，Poller 选择**保留最后已知值**而不是清空。这是工业场景的标准做法——短暂的通信中断不应导致监控界面显示全部为零。errStr 字段被设置为错误描述，WebUI 可以据此展示告警。

### 6.6 State() — 状态导出

```go
func (p *Poller) State() CollectState {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return CollectState{
        Name:    p.name,
        Values:  copyMap(p.values),
        Error:   p.errStr,
        Ring:    copyRing(p.ring),
    }
}
```

`State()` 返回一份深拷贝的状态快照，供心跳上报使用。使用读锁（`RLock`）允许多个消费者并发读取而不阻塞轮询。

---

## 7. Pattern 引擎

Pattern 引擎是边缘端的**灵魂组件**——它将 PLC 通信协议的复杂性抽象为四种可组合的交互模式。每种模式封装了特定的握手协议，业务逻辑通过 Handler 接口注入。

### 7.1 核心框架 patterns.go

#### 7.1.1 ReadWriter 接口

```go
type ReadWriter interface {
    ReadDB(db, start, size int) ([]byte, error)
    WriteDB(db, start int, data []byte) error
}
```

这是 Pattern 引擎与传输层的唯一接口。所有 Pattern 通过此接口与 PLC 通信，而不直接依赖 S7 Transport——这使得 Pattern 可以用 mock ReadWriter 进行单元测试。

#### 7.1.2 Event / Result / Handler 类型

```go
type Event struct {
    Binding string            // 触发的 Binding 名称
    Fields  map[string]string // 触发时刻的字段值快照
    Trigger string            // 触发字段的值
}

type Result struct {
    AckWrites map[string]string // 额外需要写入的确认字段
    WriteBack map[string]string // 回写字段（来自业务逻辑）
    Error     error             // 业务逻辑错误
}

type Handler func(ctx context.Context, evt Event) Result
```

Handler 是业务逻辑的注入点。它接收事件（包含触发时刻的所有字段值），返回处理结果（包含需要回写给 PLC 的字段值）。Pattern 引擎负责**何时**调用 Handler，Handler 负责**做什么**。

#### 7.1.3 writeFieldsVerified — 可验证写入

```
writeFieldsVerified(rw, db, spec, fields, retries):
  for attempt = 0; attempt < retries; attempt++:
    writeFields(rw, db, spec, fields)           // 写入
    raw = ReadDB(db, 0, spec.TotalSize)          // 回读
    for each field in fields:
      actual = DecodeField(raw, field)
      if actual != expected:
        → 继续重试
    return nil (所有字段验证通过)
  return errWriteVerifyFailed
```

**写后回读验证**是工业通信的标准实践。PLC 可能因为各种原因未能接受写入值（例如 PLC 程序逻辑覆盖了值、通信瞬断等）。验证确保数据确实写入成功。

#### 7.1.4 writeFields — 精确字段写入

```
writeFields(rw, db, spec, fields):
  for each field:
    if field.Type == "bool":
      // Bool 字段需要 read-modify-write:
      raw = ReadDB(db, field.Offset, 1)   // 读取当前字节
      修改目标 bit（保留其他 bit）         // 位操作
      WriteDB(db, field.Offset, modified)  // 写回整个字节
    else:
      encoded = EncodeField(field, value)
      WriteDB(db, field.Offset, encoded)   // 直接写入
```

**Bool 字段的 read-modify-write：** S7 协议的最小写入单位是 1 字节，但 Bool 字段只占 1 bit。直接写入整个字节会覆盖同一字节中其他 Bool 字段的值。因此必须先读取当前字节值，只修改目标 bit，再写回。

#### 7.1.5 State 结构

```go
type State struct {
    Name      string    // Binding 名称
    Pattern   string    // 模式类型
    PLC       string    // PLC 名称
    DB        int       // DB 编号
    Phase     string    // 当前阶段（idle/handling/waitingClear等）
    Fired     int64     // 触发次数
    Succeeded int64     // 成功次数
    Failed    int64     // 失败次数
    LastEvent time.Time // 最近事件时间
    LastErr   string    // 最近错误
}
```

State 为心跳上报和 WebUI 提供了每个 Binding 的运行统计。

### 7.2 触发模式 trigger.go

#### 7.2.1 概述（~190行）

触发模式实现了经典的**上升沿检测 + 确认回写**协议。当 PLC 将触发字段从非活跃值变为活跃值时（上升沿），边缘端检测到变化并执行业务逻辑。

#### 7.2.2 核心状态

| 状态 | 含义 |
|------|------|
| `idle` | 空闲，等待触发信号 |
| `handling` | 正在处理业务逻辑 |

#### 7.2.3 tick() 详细流程

```
tick() 每 PollMS 调用:
  1. ReadDB(db, 0, spec.TotalSize)
     → 失败: 返回（保持当前状态）
  2. decode TriggerField 的值
  3. 检查是否为活跃值 (activeValue, 默认 "1")
     → 不活跃:
       重建 baseline（上次已知的非活跃快照）
       return
  4. 活跃状态下:
     → 检查是否为上升沿（baseline 存在 + 当前为首次检测到活跃）
       → 是上升沿: → fire()
     → 检查是否有待重试的失败事件
       → 距离上次失败 > RetryInterval: → fire()
```

#### 7.2.4 上升沿检测与 baseline 机制

**baseline** 是触发模式的核心概念——它记录了触发字段**上一次处于非活跃状态时的原始字节快照**。上升沿的判断依据是：

```
上升沿 = (baseline 存在) AND (当前值 == 活跃值) AND (baseline 中的值 != 活跃值)
```

**断线重连后的 baseline 重建（V4 教训）：** 当 PLC 连接断开又恢复时，如果不重建 baseline，可能产生假触发——因为边缘端不知道断线期间触发字段是否发生了变化。V4 中曾因此导致重复落筒事件。

解决方案：每次读取到非活跃值时重建 baseline，每次连接断开时清除 baseline。这样，重连后的第一次读取如果是活跃值，会因为 baseline 不存在而**不**触发——必须等到一个完整的"非活跃→活跃"周期才会触发。

#### 7.2.5 fire() 详细流程

```
fire():
  1. decodeFields: 从原始字节解码所有字段值
  2. 构建 Event{Binding, Fields, Trigger}
  3. 调用 handler(ctx, event) → Result
  4. 检查 Result.Error:
     → 有错误:
       不写入 AckField（PLC 保持信号）
       记录失败时间用于重试计算
       state.Failed++
       return
  5. 合并写入字段:
     AckWrites (来自 Result) + WriteBack (来自配置和 Result)
  6. writeFieldsVerified(合并后的字段, WriteRetries)
     → 失败: 同上错误处理
  7. state.Succeeded++
  8. 清除重试标记
```

**失败不确认（V4 合约）：** 当 Handler 返回错误时，边缘端**不写入** AckField。这保证了 PLC 侧的触发信号保持活跃，PLC 程序可以据此判断请求未被处理。在 V4 中，这是与 PLC 程序员的明确约定。

**重试机制：** 失败后，触发模式会在 RetryInterval（默认 3 秒）后自动重试，而不需要 PLC 重新发送触发信号。这减少了 PLC 程序的复杂度。

#### 7.2.6 默认参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| PollMS | 200ms | 轮询间隔 |
| ActiveValue | "1" | 活跃判定值 |
| RetryIntervalMS | 3000ms | 重试间隔 |
| WriteRetries | 3 | 写入验证重试次数 |

### 7.3 请求确认模式 reqconfirm.go

#### 7.3.1 概述（~238行）

请求确认模式处理**电平保持型**请求——PLC 持续保持一个字段为活跃值直到得到确认。与触发模式的上升沿检测不同，reqconfirm 需要处理一个完整的握手周期。

#### 7.3.2 关键特性

**多值激活（ActiveValues）：** 不同于 trigger 模式的单一 ActiveValue，reqconfirm 支持多个激活值。例如 `ActiveValues: ["1", "2", "3"]` 表示请求字段为 1、2 或 3 时都视为活跃请求，但请求语义不同（如 1=请求A，2=请求B）。

**签名去重（跨断线持久化！）：** 这是 reqconfirm 最精巧的设计。每个请求的"签名"是触发时刻所有字段值的组合哈希。如果断线重连后 PLC 仍在发送相同的请求（相同签名），边缘端会识别为已处理过的请求而跳过。

```
签名计算:
  signature = hash(field1_value + "|" + field2_value + "|" + ... + fieldN_value)

去重逻辑:
  if currentSignature == lastProcessedSignature:
    → 跳过（已处理）
  else:
    → 新请求，执行 Handler
```

**ConfirmMirrorField（确认镜像）：** 将请求字段的值回显到确认字段。例如：PLC 发送 DataReady=2，边缘端处理后写入 ConfirmData=2。这让 PLC 能够验证边缘端确认的是**哪个**请求。

#### 7.3.3 握手状态机

```
┌──────┐  请求检测   ┌──────────┐  Handler完成  ┌───────────────┐
│ idle │ ──────────→ │ handling │ ────────────→ │ waitingClear  │
│      │             │          │               │               │
└──┬───┘             └────┬─────┘               └───────┬───────┘
   │                      │                             │
   │                      │ Handler失败                 │ 检测到清除
   │                      │ (不确认,                    │ (active→!active)
   │                      │  待重试)                    │
   │                      ↓                             ↓
   │                 ┌──────────┐               ┌───────────────┐
   │                 │ idle     │               │ idle          │
   │                 │ (retry   │               │ (success:     │
   │                 │  pending)│               │  CONFIRMED    │
   │                 └──────────┘               │  _BY_CLEAR)   │
   │                                            └───────────────┘
   │  ClearTimeout                                     
   │  (仅告警,不撤回确认)                              
   └──────────────────────────────────────────────────→ 保持等待
```

#### 7.3.4 tick() 详细流程

```
tick():
  1. ReadDB → decode TriggerField
  2. 判断当前阶段:

  [idle 阶段]:
    → TriggerField 不在 ActiveValues 中: 返回
    → TriggerField 在 ActiveValues 中:
      计算签名
      → 签名 == lastSignature: 跳过（去重）
      → 新签名:
        调用 Handler
        → 失败: 记录失败, 保持 idle
        → 成功:
          写入 ConfirmField/ConfirmMirrorField
          进入 waitingClear 阶段

  [waitingClear 阶段]:
    → TriggerField 仍在 ActiveValues 中:
      检查 ClearTimeout
      → 超时: 记录告警（不撤回确认！）
    → TriggerField 不在 ActiveValues 中:
      = PLC 已确认收到确认并清除了请求
      → 记录 CONFIRMED_BY_CLEAR
      → 更新 lastSignature
      → 回到 idle 阶段, Succeeded++
```

#### 7.3.5 ClearTimeout 设计哲学

ClearTimeout 到期时**仅发出告警，不撤回确认**。这是一个深思熟虑的设计：

- 撤回确认意味着 PLC 侧可能已经基于确认执行了后续动作（如启动电机）
- 撤回会导致 PLC 和边缘端状态不一致
- 真正的超时原因应由操作人员排查（可能是 PLC 程序逻辑问题）

### 7.4 序列执行模式 sequence.go

#### 7.4.1 概述（~114行）

sequence.go 并不是一个独立的 Pattern，而是一个**辅助模块**——为 dispatch 模式提供阻塞式多步骤执行能力。

#### 7.4.2 RunSequence 函数

```go
func RunSequence(ctx context.Context, rw ReadWriter, db int, spec *dbspec.Spec, steps []Step) error
```

`RunSequence` 按顺序执行一系列 Step，每个 Step 可以包含：

- **Write（可选）：** 写入一组字段，使用 writeFieldsVerified 验证
- **WaitField（可选）：** 等待某个字段变为指定值

#### 7.4.3 waitFor — 等待字段变化

```
waitFor(rw, db, spec, fieldName, expectedValue, timeout):
  consecutiveErrors = 0
  loop:
    ReadDB → decode fieldName
    → 读取失败:
      consecutiveErrors++
      if consecutiveErrors >= 3:
        return errLinkFault  // 连接故障，区别于超时
      continue
    → 读取成功:
      consecutiveErrors = 0
      if fieldValue == expectedValue:
        return nil           // 等待成功
      if timeout exceeded:
        return errTimeout    // PLC 未响应
```

**连续错误计数器（3次阈值）：** 这是 sequence.go 中最精细的设计。它区分了两种不同的失败场景：

1. **链路故障（Link Fault）：** 连续 3 次读取都失败，说明 PLC 连接已断开
2. **等待超时（Timeout）：** 读取成功但字段值一直不等于期望值，说明 PLC 程序未按预期响应

两种故障的处理策略不同：链路故障需要等待重连，超时可能需要人工干预。

### 7.5 派发模式 dispatch.go

#### 7.5.1 概述（~297行）

派发模式实现了**主动推送型**交互——边缘端主动向 PLC 派发数据（如仓库入库指令、分拣目标位置）。这是唯一一个**边缘端主导发起**的模式。

#### 7.5.2 Source 接口

```go
type Source interface {
    Next(ctx context.Context) (*DispatchPayload, error)
    Done(ctx context.Context, ref string, err error)
}
```

Source 是派发数据的来源抽象：
- `Next()`：获取下一条待派发数据。阻塞直到有数据可用或 ctx 取消
- `Done()`：通知数据源派发结果。ref 是数据标识，err 为 nil 表示成功

#### 7.5.3 双门控机制

派发前必须同时满足两个条件：

```
门控1: LatchField == idle    → 边缘端侧锁存未占用
门控2: GateField == enabled  → PLC 侧已准备好接收数据
```

**LatchField（锁存字段）：** 由边缘端控制，标记"我正在往 PLC 写数据"。防止上传循环或其他 goroutine 同时操作同一组字段。

**GateField（门控字段）：** 由 PLC 控制，标记"我已准备好接收数据"。PLC 可能因为正在处理上一条指令而关闭门控。

双门控确保了边缘端和 PLC 双方都准备就绪才开始数据传输。

#### 7.5.4 effectiveOrder — 确定性写入顺序

```
effectiveOrder(fields, configOrder):
  if configOrder is defined:
    return configOrder     // 使用配置指定的顺序
  else:
    keys = sort(fields.keys())  // 字母序排列
    return keys            // 确定性但不一定是最优顺序
```

**为什么写入顺序重要：** PLC 程序可能会监控特定字段的变化作为"数据就绪"信号。如果数据字段尚未写入就先写入了就绪标志，PLC 会读到不完整的数据。`FieldOrder` 让配置者可以明确指定"先写数据字段，最后写就绪标志"。

#### 7.5.5 dispatchOne — 单次派发详细流程

```
dispatchOne(payload):
  1. defer: 无论成功失败，清除 LatchField（释放锁存）
  2. order = effectiveOrder(payload.Fields, config.FieldOrder)
  3. for each field in order:
       单字段写入 WriteDB
       → 失败: 清除 latch, Done(ref, err), return
  4. 设置 LatchField = active（"数据已写完"）
  5. RunSequence: 等待 ConfirmField == expectedValue
     → 成功: Done(ref, nil), Succeeded++
     → 失败/超时: Done(ref, err), Failed++
  6. 清除 LatchField
```

**单字段写入策略（V2 经验）：** V2 版本曾尝试一次性写入所有字段（批量写入），但在某些 PLC 型号上遇到了 ECONNRESET 错误。原因是 S7 协议的 PDU 大小限制——当写入数据超过 PDU 上限时，gos7 库需要拆分为多次传输，而某些 PLC 对连续快速写入不友好。

改为逐字段写入后，每次写入的数据量很小（通常 1-8 字节），完全在单个 PDU 内完成，彻底消除了此问题。虽然增加了通信往返次数，但在局域网环境下每次写入仅耗时几毫秒，总体延迟完全可接受。

### 7.6 打印任务模式 printjob.go

#### 7.6.1 概述（~310行）

打印任务模式是最复杂的 Pattern，实现了**四条件守卫 + 错误锁存**的打印控制协议。它管理的是 PLC 与工业打印机之间的协调——PLC 发出打印请求，边缘端负责获取标签数据并驱动打印机。

#### 7.6.2 四条件守卫

只有当四个条件**同时满足**时才触发打印：

```
条件1: StartField  == ActiveValue     → PLC 发出了打印启动信号
条件2: RefIDField  != 0               → PLC 已填充了引用ID（标签数据来源）
条件3: EndField    == idle            → 上一次打印已完成（无未完成的打印）
条件4: StatusField != ErrorValue      → 没有未处理的打印机错误
```

**四条件联合守卫的必要性：**

- 条件1 + 条件2 的分离：PLC 可能先设置 StartField，然后才填充 RefIDField（两步操作，见下文"两阶段读取"）
- 条件3 防止叠加：确保上一次打印的状态已被清除后才接受新的打印请求
- 条件4 错误锁存：打印机故障时必须人工清除错误才能继续打印

#### 7.6.3 错误锁存机制

```
打印失败时:
  写入 StatusField = ErrorValue       ← 设置错误状态
  不写入 EndField                     ← 不标记结束！

效果:
  → 条件4 (StatusField != ErrorValue) 不满足
  → 后续的打印请求被阻挡
  → 需要人工/PLC清除 StatusField 后才能继续

打印成功时:
  写入 StatusField = OKValue          ← 设置成功状态
  写入 EndField = OK                  ← 标记结束

效果:
  → PLC 检测到 EndField 变化，知道打印已完成
  → PLC 清除 StartField，为下一次打印做准备
```

**不写入 EndField 的精妙之处：** 如果在错误时也写入 EndField，PLC 可能会清除 StartField 并重新发起打印请求——但打印机仍处于故障状态，形成无限循环。不写 EndField 让 PLC 保持在"等待打印完成"状态，直到人工介入。

#### 7.6.4 doneRef 去重

```go
doneRef string  // 已完成的引用ID
```

`doneRef` 防止同一引用 ID 的重复打印。场景：打印成功后，PLC 可能需要几个轮询周期才能处理 EndField 并清除 StartField。在这个窗口期内，四个条件仍然满足（旧的 StartField 仍为活跃），如果不去重会导致重复打印。

#### 7.6.5 两阶段读取

```
PLC 填充打印数据的时序:
  T1: PLC 设置 StartField = active    (发起打印请求)
  T2: PLC 填充 RefIDField = 12345     (填入数据引用)

边缘端的处理:
  T1 读取: StartField=active, RefIDField=0
    → 条件2 不满足，等待
  T2 读取: StartField=active, RefIDField=12345
    → 四条件满足，触发打印
```

这种两阶段时序是 PLC 编程中的常见模式——先设置标志位表示"我要发数据"，然后填充实际数据。边缘端通过条件 2（RefIDField != 0）优雅地处理了这种时序差。

---

## 8. 上传模块

### 8.1 概述（internal/edge/upload/upload.go ~338行）

上传模块负责将边缘端 SQLite 中的业务数据同步到云端服务器。它是边缘端与服务端之间的**唯一数据通道**（除心跳外）。

### 8.2 上传策略

#### 8.2.1 FK 顺序上传

```
UploadOrder 示例:
  1. orders        (基础数据)
  2. batches       (依赖 orders)
  3. barrels       (依赖 batches)
  4. bobbins       (依赖 barrels)
  5. pallet_slots  (依赖 bobbins)
  6. labels        (依赖 bobbins)
```

**严格按照外键依赖顺序上传**——确保服务端收到子记录时，父记录一定已经存在。如果乱序上传，服务端会因为 FK 约束而拒绝插入。

#### 8.2.2 游标增量上传（Insert 路径）

```
上传循环:
  for each table in UploadOrder:
    cursor = 上次成功上传的最大 ID（持久化在 SQLite）
    rows = SELECT * FROM table WHERE id > cursor ORDER BY id LIMIT batch_size
    if len(rows) == 0: continue（表已同步）
    response = POST /edge/upload {table, rows}
    if response.applied == len(rows):
      cursor = rows[last].id     // 推进游标
      保存 cursor 到 SQLite
    else:
      不推进游标！               // 安全措施
      log 警告
```

**游标推进安全：** 只有当服务端确认 `applied == sent` 时才推进游标。这防止了**数据丢失**——如果服务端只部分处理了批次（例如因为某条记录的 FK 缺失），不推进游标意味着下次上传会重试整批数据。

#### 8.2.3 变更检测上传（Update 路径）

```
Update 检测:
  rows = SELECT * FROM table WHERE updated_at > uploaded_at
  // uploaded_at: 上次上传时间戳
  // updated_at: 最后修改时间戳
  → 找到已上传但后来被修改的记录
```

Insert 路径处理新增记录，Update 路径处理已上传但后来被修改的记录。两条路径独立运行，确保新增和修改都能被同步。

### 8.3 服务器故障转移

```
故障转移逻辑:
  servers = ["http://server1:8080", "http://server2:8080"]
  currentIdx = 0

  POST 请求:
    response = POST servers[currentIdx]/edge/upload
    switch response.status:
      200: 成功
      503: 服务端为备用状态
        currentIdx = (currentIdx + 1) % len(servers)
        重试下一台服务器
      其他: 网络错误/服务端错误
        currentIdx = (currentIdx + 1) % len(servers)
        等待下一个上传周期
```

**503 = Standby 的约定：** 服务端采用主备模式部署时，备用节点返回 503 表示"我是备用的，请找主节点"。边缘端收到 503 后立即切换到下一台服务器重试，而不是等待下一个周期——这最大限度地减少了故障转移延迟。

### 8.4 NUL 字符剥离

```go
func stripNUL(s string) string {
    return strings.ReplaceAll(s, "\x00", "")
}
```

PLC 的 CHAR/STRING 字段使用固定长度分配，短字符串后面用 0x00 填充。例如品种名 "FDY-150D" 在 PLC 中存储为 `FDY-150D\x00\x00\x00\x00...`（补齐到字段长度）。

PostgreSQL 的 TEXT 类型**拒绝包含 NUL 字符的字符串**，会返回错误。因此在上传前必须剥离 NUL 填充。这个看似简单的处理解决了 V4 中一个反复出现的数据同步错误。

---

## 9. 业务模块

### 9.1 落筒服务 doffing

#### 9.1.1 Service 结构（doffing/service.go）

```go
type Service struct {
    store      *store.Store        // SQLite 存储
    config     *config.DoffingConf // 落筒配置
    shiftCalc  *ShiftCalc          // 班次计算器
    idGen      *IDGenerator        // 全局ID生成器
    rackAssign *RackAssigner       // 储位分配器
    printers   map[string]printer.Driver  // PLC名→打印机映射
}
```

落筒服务是边缘端最核心的业务模块，管理从卷绕机监控到桶标签打印的完整流程。

#### 9.1.2 Handler 注册

```go
func (s *Service) Handler(name string) patterns.Handler {
    switch name {
    case "end_loading":
        return s.handleEndLoading
    // ... 其他 handler
    }
}
```

`Handler("end_loading")` 返回的处理函数被注入到 trigger 模式的 Binding 中。当 PLC 发送"装载完成"信号时，trigger 模式检测到上升沿并调用此 Handler。

#### 9.1.3 handleEndLoading — 装载完成处理

```
handleEndLoading(ctx, event):
  1. 从 event.Fields 提取:
     - barrel_id（桶编号）
     - winder_ids（卷绕机列表）
     - doff_no（落纱批次号）
  2. 查询 SQLite: 获取关联的 bobbin 记录
  3. 更新 barrel 记录: 状态 → loaded, 时间戳
  4. 更新 bobbins: 关联到 barrel
  5. 分配储位: rackAssign.Assign(barrel)
  6. 打印标签: printers[plcName].Print(labelData)
  7. 返回 Result{AckWrites: {"ack": "1"}}
```

#### 9.1.4 WinderMonitor（winder.go）

WinderMonitor 是一个独立的监控组件，不属于任何 Pattern——它使用采集模块的数据，而不是直接与 PLC 通信。

**监控目标：** 检测每个卷绕机的 `doff_no`（落纱批次号）变化。当 `doff_no` 递增时，说明该卷绕机完成了一次落纱操作。

```
WinderMonitor.tick():
  for i = 0; i < winder_count; i++:
    reading = readWinder(i)     // 从采集数据中提取
    if reading.doff_no != baseline[i].doff_no:
      // 检测到落纱事件
      创建 barrel 记录
      创建 bobbin 记录（根据卷绕机上的丝锭数）
      更新 baseline[i] = reading
```

#### 9.1.5 winderLayout — 预计算偏移量

```go
type winderLayout struct {
    statusOffset  int
    speedOffset   int
    doffNoOffset  int
    // ... 25+ 字段偏移量
}
```

winderLayout 在初始化时**预计算**每个字段在原始字节中的偏移量，避免在高频轮询中反复按名称查找字段定义。这是一个性能优化——72 个卷绕机 × 25+ 字段 = 1800+ 次查找，预计算将其降为 O(1) 的直接偏移访问。

#### 9.1.6 WinderReading — 卷绕机数据快照

WinderReading 包含 25+ 个从 PLC 字节/位布局中重组的字段：

| 字段分类 | 典型字段 | 说明 |
|----------|----------|------|
| 状态类 | status, running, alarm | 运行状态和告警 |
| 生产类 | doff_no, speed, tension | 生产参数 |
| 品种类 | recipe_id, denier | 品种信息 |
| 质量类 | breakage_count, cv_value | 质量指标 |
| 时间类 | start_time, run_duration | 时间跟踪 |

#### 9.1.7 Baseline 持久化

```
落纱 baseline 存储在 SQLite 中:
  CREATE TABLE winder_baseline (
    winder_index INTEGER PRIMARY KEY,
    doff_no      INTEGER,
    raw_snapshot BLOB,
    updated_at   TIMESTAMP
  )
```

**持久化的必要性：** 边缘端程序重启后，如果不恢复 baseline，会把所有卷绕机的当前 `doff_no` 当作"变化"，产生大量虚假落纱事件。持久化到 SQLite 确保重启后能准确识别真正的落纱操作。

### 9.2 包装服务 packing

#### 9.2.1 概述（packing/service.go）

包装服务处理丝锭从落筒到最终包装的全过程，支持两种包装范式：

| 范式 | 结构 | 典型场景 |
|------|------|----------|
| **2层** | bobbin → pallet | 丝锭直接放入托盘（81个槽位） |
| **3层** | bobbin → box → pallet | 丝锭先装箱（每箱6个），箱再放入托盘（每盘25箱） |

#### 9.2.2 Handler 类型矩阵

| Handler | Pattern | 说明 |
|---------|---------|------|
| `pallet_assign` | reqconfirm | 托盘分配请求 |
| `pallet_create` | trigger | 2层托盘创建（完成码垛后触发） |
| `box_create` | trigger | 3层装箱创建 |
| `pallet_create_3layer` | trigger | 3层托盘创建 |
| `monorail_batch` | dispatch | 天轨批量派发指令 |

#### 9.2.3 2层 vs 3层的分离策略

2层和3层的 Handler 是**完全独立的实现**，而不是用 if/else 分支切换。原因：

1. **字段名不同：** 2层和3层的 PLC 程序使用不同的 DB 块和字段命名
2. **业务逻辑差异大：** 3层多了"装箱"环节，计数逻辑完全不同
3. **配置独立：** 2层和3层可能部署在不同的 PLC 上

**SOLID-S 原则（单一职责）：** 每个 Handler 只处理一种包装范式，代码更清晰，修改一种范式不会影响另一种。

```
2层打包流程:
  PLC 信号"码垛完成" → trigger 触发
    → 查询 barrel 上的 bobbin 列表
    → 创建 pallet 记录（81 slots）
    → 将 bobbins 分配到 pallet slots
    → 打印托盘标签
    → 确认 PLC

3层打包流程:
  PLC 信号"装箱完成" → trigger 触发
    → 查询当前 6 个 bobbins
    → 创建 box 记录
    → 将 bobbins 分配到 box
    → 打印箱标签
  
  当 box count == 25:
    → 创建 pallet 记录
    → 将 25 个 boxes 分配到 pallet
    → 打印托盘标签
```

---

## 10. 本地存储层

### 10.1 概述（internal/edge/store/store.go）

边缘端使用 SQLite 作为本地存储，采用 WAL（Write-Ahead Logging）模式。

### 10.2 SQLite 配置

| 参数 | 值 | 说明 |
|------|-----|------|
| 模式 | WAL | 支持并发读取，写入不阻塞读取 |
| 忙等超时 | 5s | 写入冲突时等待而非立即失败 |
| 同步级别 | NORMAL | WAL 模式下足够安全 |

**WAL 模式的选择理由：**

在边缘端场景中，存在多个并发的数据库访问者：
- WinderMonitor 高频写入卷绕机数据
- Pattern Handler 写入业务记录
- Upload 模块读取待上传数据
- WebUI API 读取状态数据

默认的 DELETE journal 模式下，写入会获取排他锁，阻塞所有读取。WAL 模式允许写入和读取并发执行，极大提升了系统响应性。

### 10.3 自动迁移

store 在打开时自动执行 schema 迁移，确保数据库结构与代码版本一致。这使得边缘端程序升级后无需手动执行 SQL 迁移脚本。

### 10.4 UploadOrder / UpdateOrder

```go
var UploadOrder = []string{
    "orders", "batches", "barrels", "bobbins",
    "pallet_slots", "labels", "print_jobs",
}

var UpdateOrder = []string{
    "orders", "batches", "barrels", "bobbins",
}
```

这两个有序列表分别用于上传模块的 Insert 路径和 Update 路径。顺序严格遵循 FK 依赖——子表排在父表后面。

---

## 11. 打印模块

### 11.1 Driver 接口（internal/edge/printer/）

```go
type Driver interface {
    Print(data map[string]string) error
    Check() error
    Close() error
}
```

三个方法定义了打印机驱动的完整生命周期：
- `Print()`：发送打印数据
- `Check()`：检查打印机状态（在线/离线/错误）
- `Close()`：释放连接资源

### 11.2 驱动实现

#### 11.2.1 ZPL 驱动（Zebra 打印机）

ZPL（Zebra Programming Language）是 Zebra 工业打印机的标准控制语言。

```
打印流程:
  1. 连接打印机 TCP 端口（通常 9100）
  2. 加载 ZPL 模板
  3. 变量替换: {{barcode}} → "1234567890"
  4. 发送 ZPL 指令流
  5. 等待打印机确认
```

#### 11.2.2 Brother-TD 驱动

Brother-TD 系列是桌面型工业标签打印机，使用 Brother 专有打印协议。

#### 11.2.3 Stub 驱动（测试用）

Stub 驱动不执行实际打印，仅记录打印调用。用于：
- 开发环境无打印机时的测试
- 自动化测试中验证打印逻辑
- 配置验证（不连接真实打印机）

### 11.3 模板系统

打印模板使用简单的变量替换：

```
模板示例（ZPL）:
  ^XA
  ^FO50,50^A0N,40,40^FD品种: {{variety_name}}^FS
  ^FO50,100^BCN,100,Y,N,N^FD{{barcode}}^FS
  ^FO50,220^A0N,30,30^FD批次: {{batch_no}} 日期: {{date}}^FS
  ^XZ

变量替换:
  {{variety_name}} → "FDY-150D/48F"
  {{barcode}}      → "2026091700001"
  {{batch_no}}     → "B20260917-03"
  {{date}}         → "2026-09-17"
```

---

## 12. 架构图集

### 12.1 Pattern 引擎状态机图

```mermaid
stateDiagram-v2
    direction LR

    state "Trigger 模式 (trigger.go)" as TG {
        direction LR
        [*] --> TG_Idle
        TG_Idle --> TG_Idle : 轮询: 信号非活跃\n(重建baseline)
        TG_Idle --> TG_Handling : 上升沿检测\n(baseline→active)
        TG_Handling --> TG_Idle : Handler成功\n+ writeFieldsVerified\n(Succeeded++)
        TG_Handling --> TG_Idle : Handler失败\n不写AckField\n(Failed++, 待重试)
        TG_Idle --> TG_Handling : 重试到期\n(RetryInterval过期)
        TG_Idle --> TG_Idle : 断线→清除baseline
    }

    state "ReqConfirm 模式 (reqconfirm.go)" as RC {
        direction LR
        [*] --> RC_Idle
        RC_Idle --> RC_Idle : 轮询: 信号非活跃\n或签名重复(去重)
        RC_Idle --> RC_Handling : 新请求检测\n(ActiveValues + 新签名)
        RC_Handling --> RC_WaitClear : Handler成功\n写入ConfirmField\n/ConfirmMirrorField
        RC_Handling --> RC_Idle : Handler失败\n(Failed++, 待重试)
        RC_WaitClear --> RC_Idle : 清除检测\nactive→!active\n(CONFIRMED_BY_CLEAR\nSucceeded++)
        RC_WaitClear --> RC_WaitClear : ClearTimeout\n仅告警,不撤回确认
    }

    state "Dispatch 模式 (dispatch.go)" as DP {
        direction LR
        [*] --> DP_Idle
        DP_Idle --> DP_Idle : 等待Source.Next()\n或门控未就绪
        DP_Idle --> DP_Writing : 双门控通过\n(Latch==idle AND\nGate==enabled)
        DP_Writing --> DP_WaitConfirm : 有序逐字段写入完成\n设置LatchField
        DP_Writing --> DP_Idle : 写入失败\n清除Latch\nDone(ref, err)
        DP_WaitConfirm --> DP_Idle : RunSequence成功\n(ConfirmField==expected)\nDone(ref, nil)\nSucceeded++
        DP_WaitConfirm --> DP_Idle : 超时/链路故障\nDone(ref, err)\nFailed++
    }

    state "PrintJob 模式 (printjob.go)" as PJ {
        direction LR
        [*] --> PJ_Idle
        PJ_Idle --> PJ_Idle : 轮询: 四条件不满足\n或doneRef去重
        PJ_Idle --> PJ_Idle : 两阶段等待\n(Start==active但\nRefID==0)
        PJ_Idle --> PJ_Printing : 四条件全满足\n(Start+RefID+End+Status)
        PJ_Printing --> PJ_Idle : 打印成功\n写StatusField=OK\n写EndField=OK\n设doneRef\nSucceeded++
        PJ_Printing --> PJ_ErrorLatch : 打印失败\n写StatusField=Error\n不写EndField!\nFailed++
        PJ_ErrorLatch --> PJ_Idle : 外部清除\nStatusField\n(人工干预)
    }
```

### 12.2 边缘端数据流图

```mermaid
flowchart TB
    subgraph PLC_Layer["PLC 层 (西门子 S7-300/1200/1500)"]
        PLC1["PLC #1\nDB100: 卷绕机状态\nDB200: 落筒控制"]
        PLC2["PLC #2\nDB100: 包装控制\nDB200: 仓库控制"]
        PLC3["PLC #3\nDB100: 打印控制"]
    end

    subgraph Transport["S7 传输层"]
        S7_1["S7 Transport #1\nMutex串行化\n2s超时 / 3s冷却"]
        S7_2["S7 Transport #2"]
        S7_3["S7 Transport #3"]
    end

    subgraph CollectLayer["采集层 (Pattern 1)"]
        Poller1["Poller: 卷绕机\n500ms间隔\n环形缓冲50条"]
        Poller2["Poller: 包装状态\n500ms间隔"]
    end

    subgraph PatternLayer["Pattern 引擎层"]
        Trigger["Trigger Binding\n落筒完成信号\n200ms轮询"]
        ReqConfirm["ReqConfirm Binding\n数据就绪请求\n签名去重"]
        Dispatch["Dispatch Binding\n仓库指令派发\n双门控机制"]
        PrintJob["PrintJob Binding\n标签打印控制\n四条件守卫"]
    end

    subgraph BusinessLayer["业务逻辑层"]
        DoffSvc["落筒服务\nWinderMonitor\nhandleEndLoading"]
        PackSvc["包装服务\npallet_assign\nbox_create"]
        PrintSvc["打印模块\nZPL/Brother/Stub"]
    end

    subgraph StoreLayer["本地存储层"]
        SQLite[("SQLite (WAL)\norders / batches\nbarrels / bobbins\npallet_slots\nwinder_baseline")]
    end

    subgraph UploadLayer["上传层"]
        Upload["Upload Loop\nFK顺序上传\n游标增量\n变更检测"]
    end

    subgraph ServerLayer["云端服务器"]
        Server1["Server #1 (主)\nHTTP API"]
        Server2["Server #2 (备)\n503=Standby"]
    end

    subgraph Monitoring["监控层"]
        Heartbeat["心跳循环\nHeartbeatMS间隔\n全状态聚合"]
        WebUI["WebUI\nVue.js 3\n实时监控"]
    end

    PLC1 <-->|"ReadDB/WriteDB"| S7_1
    PLC2 <-->|"ReadDB/WriteDB"| S7_2
    PLC3 <-->|"ReadDB/WriteDB"| S7_3

    S7_1 --> Poller1
    S7_2 --> Poller2
    S7_1 <--> Trigger
    S7_2 <--> ReqConfirm
    S7_2 <--> Dispatch
    S7_3 <--> PrintJob

    Poller1 -.->|"采集数据"| DoffSvc
    Trigger -->|"Event"| DoffSvc
    ReqConfirm -->|"Event"| PackSvc
    PrintJob -->|"Print请求"| PrintSvc
    Dispatch -.->|"Source.Next()"| PackSvc

    DoffSvc --> SQLite
    PackSvc --> SQLite
    DoffSvc -->|"标签数据"| PrintSvc

    SQLite -->|"FK顺序查询"| Upload
    Upload -->|"POST /edge/upload"| Server1
    Upload -->|"503故障转移"| Server2

    Poller1 -.->|"CollectState"| Heartbeat
    Poller2 -.->|"CollectState"| Heartbeat
    Trigger -.->|"PatternState"| Heartbeat
    ReqConfirm -.->|"PatternState"| Heartbeat
    Dispatch -.->|"PatternState"| Heartbeat
    PrintJob -.->|"PatternState"| Heartbeat
    Heartbeat -->|"POST /edge/heartbeat"| Server1

    Poller1 -.->|"实时数据"| WebUI
    Trigger -.->|"状态"| WebUI
    ReqConfirm -.->|"状态"| WebUI

    style PLC_Layer fill:#e3f2fd,stroke:#1565c0
    style Transport fill:#fff3e0,stroke:#e65100
    style CollectLayer fill:#e8f5e9,stroke:#2e7d32
    style PatternLayer fill:#fce4ec,stroke:#c62828
    style BusinessLayer fill:#f3e5f5,stroke:#7b1fa2
    style StoreLayer fill:#fff8e1,stroke:#f57f17
    style UploadLayer fill:#e0f7fa,stroke:#00695c
    style ServerLayer fill:#efebe9,stroke:#4e342e
    style Monitoring fill:#e8eaf6,stroke:#283593
```

### 12.3 落筒业务流程图

```mermaid
flowchart TD
    START([落筒业务开始])

    subgraph WinderMon["卷绕机监控 (WinderMonitor)"]
        WM1["Poller 采集卷绕机DB\n72个卷绕机 × 25+ 字段"]
        WM2["WinderReading 重组\n预计算偏移量快速读取"]
        WM3{doff_no\n变化检测}
        WM4["检测到落纱事件\nwinder_i.doff_no 递增"]
        WM5["读取 WinderReading\nstatus/speed/recipe/denier..."]
    end

    subgraph DoffCreate["落纱数据创建"]
        DC1["创建 barrel 记录\n(桶/吊篮)"]
        DC2["创建 bobbin 记录\n(每个丝锭一条)"]
        DC3["关联 bobbin → barrel"]
        DC4["记录落纱时间/班次\nShiftCalc 计算当前班次"]
        DC5["更新 winder_baseline\n持久化到 SQLite"]
    end

    subgraph RackAssign["储位分配"]
        RA1["RackAssigner 获取请求"]
        RA2{可用储位?}
        RA3["分配储位编号\n更新 barrel.rack_slot"]
        RA4["储位已满告警"]
    end

    subgraph EndLoading["装载完成 (Trigger Pattern)"]
        EL0["PLC 设置触发信号\n(装载机完成装桶)"]
        EL1["trigger.go 检测上升沿"]
        EL2["调用 handleEndLoading"]
        EL3["从 Event.Fields 提取:\n- barrel_id\n- winder_ids\n- doff_no"]
        EL4["更新 barrel 状态\n→ loaded"]
        EL5["关联 bobbins → barrel"]
        EL6["查询品种/订单信息\n组装标签数据"]
    end

    subgraph LabelPrint["标签打印"]
        LP1["选择打印机\nprinters[plcName]"]
        LP2["加载 ZPL/Brother 模板"]
        LP3["变量替换\nbarcode/品种/批次/日期"]
        LP4["发送打印指令"]
        LP5{打印成功?}
        LP6["标签打印完成"]
        LP7["打印失败\n记录错误"]
    end

    subgraph AckPLC["PLC 确认"]
        AP1["writeFieldsVerified\n写入 AckField=1"]
        AP2["PLC 检测到确认\n清除触发信号"]
        AP3["Succeeded++\n清除重试标记"]
    end

    subgraph UploadSync["数据上传"]
        US1["Upload Loop 检测新记录"]
        US2["FK 顺序:\norders→batches→barrels→bobbins"]
        US3["POST 至云端服务器"]
    end

    START --> WM1
    WM1 --> WM2
    WM2 --> WM3
    WM3 -->|无变化| WM1
    WM3 -->|有变化| WM4
    WM4 --> WM5
    WM5 --> DC1
    DC1 --> DC2
    DC2 --> DC3
    DC3 --> DC4
    DC4 --> DC5
    DC5 --> RA1
    RA1 --> RA2
    RA2 -->|是| RA3
    RA2 -->|否| RA4
    RA3 --> WM1

    EL0 --> EL1
    EL1 --> EL2
    EL2 --> EL3
    EL3 --> EL4
    EL4 --> EL5
    EL5 --> EL6
    EL6 --> LP1
    LP1 --> LP2
    LP2 --> LP3
    LP3 --> LP4
    LP4 --> LP5
    LP5 -->|是| LP6
    LP5 -->|否| LP7
    LP6 --> AP1
    LP7 -.->|"重试\n(RetryInterval)"| EL2
    AP1 --> AP2
    AP2 --> AP3

    DC5 -.->|"SQLite 数据"| US1
    AP3 -.->|"SQLite 数据"| US1
    US1 --> US2
    US2 --> US3

    style WinderMon fill:#e8f5e9,stroke:#2e7d32
    style DoffCreate fill:#e3f2fd,stroke:#1565c0
    style RackAssign fill:#fff3e0,stroke:#e65100
    style EndLoading fill:#fce4ec,stroke:#c62828
    style LabelPrint fill:#f3e5f5,stroke:#7b1fa2
    style AckPLC fill:#c8e6c9,stroke:#1b5e20
    style UploadSync fill:#e0f7fa,stroke:#00695c
```

---

## 13. 设计决策与 V4 经验教训

### 13.1 从 V2/V4 到 V5 的关键改进

Silkroad V5 的边缘端设计包含大量源自 V2（Node.js）和 V4（早期 Go 版本）现场经验的改进。以下逐一说明：

#### 13.1.1 重连冷却（V4 Lesson #1）

| 版本 | 行为 | 问题 |
|------|------|------|
| V2 | nodes7 自带重连，无冷却 | PLC 网络栈不释放旧连接，新连接立即失败，CPU 空转 |
| V4 早期 | 立即重连 | 200ms 轮询产生 5次/秒无效连接，加重 PLC 负担 |
| V5 | 3 秒冷却 | 等待 PLC TCP 栈完成旧连接清理后再重连 |

#### 13.1.2 Baseline 重建（V4 Lesson #2）

| 版本 | 行为 | 问题 |
|------|------|------|
| V2 | 无 baseline 概念，每次检测到活跃值就触发 | 重连后假触发 |
| V4 早期 | baseline 在内存中，重启丢失 | 程序重启后假触发 |
| V5 | 断线清除 baseline + 卷绕机 baseline 持久化到 SQLite | 消除两种假触发 |

#### 13.1.3 单字段写入（V2 Lesson #3）

| 版本 | 行为 | 问题 |
|------|------|------|
| V2 | 批量写入（一次性写入所有字段） | 超过 PDU 大小时 ECONNRESET |
| V5 | 逐字段写入 | 每次写入在单个 PDU 内完成 |

#### 13.1.4 失败不确认（V4 Contract #4）

| 版本 | 行为 | 问题 |
|------|------|------|
| V2 | 失败也写确认（认为"边缘端已处理"） | PLC 认为成功，但数据实际未保存 |
| V5 | 失败不写 AckField | PLC 保持信号，操作人员/重试可以恢复 |

#### 13.1.5 方向校验（V5 New）

| 版本 | 行为 | 问题 |
|------|------|------|
| V2 | 无方向概念 | 配置错误只在运行时暴露 |
| V5 | SCL 导入方向注解 + 启动校验 | 启动时就能发现读写方向配置错误 |

### 13.2 Pattern 抽象的价值

V2 中每个业务模块（落筒、包装、仓库）都自己实现 PLC 交互逻辑——读取、判断、写入、重试。这导致：

1. **重复代码：** 同样的轮询/确认/重试逻辑在多个模块中重复实现
2. **一致性差：** 不同模块的错误处理策略不一致（有的重试，有的不重试）
3. **难以测试：** 业务逻辑与通信协议混合，无法独立测试

V5 的 Pattern 抽象将 PLC 交互协议提取为可复用的模式：

```
V2 架构:
  业务模块 A = PLC通信 + 协议处理 + 业务逻辑
  业务模块 B = PLC通信 + 协议处理 + 业务逻辑（重复）
  业务模块 C = PLC通信 + 协议处理 + 业务逻辑（重复）

V5 架构:
  Pattern 引擎 = PLC通信 + 协议处理（统一实现）
  业务模块 A = Handler 函数（纯业务逻辑）
  业务模块 B = Handler 函数（纯业务逻辑）
  业务模块 C = Handler 函数（纯业务逻辑）
```

### 13.3 边缘端自治设计

边缘端被设计为**能够独立于云端服务器运行**。这意味着：

1. **断网不停产：** SQLite 暂存所有数据，网络恢复后自动上传
2. **本地完整业务：** 落筒、包装等业务逻辑完全在本地执行
3. **服务器故障透明：** 多服务器故障转移 + 503 备用切换
4. **重启恢复：** baseline 持久化 + 游标持久化确保重启后继续工作

---

## 14. 总结与评价

### 14.1 架构优势

| 方面 | 评价 |
|------|------|
| **Pattern 抽象** | 将4种PLC交互模式参数化，实现业务逻辑与通信协议的完美分离 |
| **容错设计** | 重连冷却、baseline重建、失败不确认、写后验证——每个机制都源自真实生产事故 |
| **自治能力** | SQLite + 游标上传使边缘端在完全断网时仍能正常生产 |
| **启动校验** | 方向注解 + 启动时校验将大量运行时错误前移到启动阶段 |
| **代码简洁** | ~3500行实现了一个功能完整的工业边缘端（相比V2的Node.js版本大幅精简） |

### 14.2 代码规模统计

| 模块 | 行数 | 占比 |
|------|------|------|
| main.go（入口+启动） | ~922 | 26.3% |
| config.go（配置） | ~370 | 10.6% |
| transport/s7.go（传输） | ~136 | 3.9% |
| dbspec/spec.go（数据规格） | ~235 | 6.7% |
| collect/collect.go（采集） | ~152 | 4.3% |
| patterns/*.go（Pattern引擎） | ~1,304 | 37.3% |
| upload/upload.go（上传） | ~338 | 9.7% |
| 其他（store/printer等） | ~43 | 1.2% |
| **总计** | **~3,500** | **100%** |

**Pattern 引擎占 37.3%** 的代码量，体现了它作为系统核心的地位。

### 14.3 与 V2 的对比

| 维度 | V2 (Node.js) | V5 (Go) |
|------|-------------|---------|
| 运行时 | Electron + Node.js | 单一 Go 二进制 |
| 内存占用 | ~200MB（Electron开销） | ~30MB |
| 启动时间 | ~10s（Electron初始化） | ~1s |
| PLC 通信 | nodes7（JS实现） | gos7（Go实现） |
| 并发模型 | 单线程事件循环 | goroutine 原生并发 |
| 部署复杂度 | 需安装 Node.js + Electron | 单文件部署 |
| 业务抽象 | 无（每个模块自行实现） | Pattern 引擎 |
| 断网容忍 | MySQL（需要网络） | SQLite（本地嵌入） |
| 配置校验 | 运行时才发现错误 | 启动时完整校验 |

### 14.4 核心设计原则总结

1. **启动即校验：** 所有配置错误在启动时暴露，不带问题上线
2. **失败即断开：** S7 通信错误 → 立即断开 → 冷却 → 重连
3. **失败不确认：** 业务处理失败 → 不写 PLC 确认 → PLC 保持信号
4. **写后回读：** 每次写入 PLC 后都回读验证
5. **断网不停产：** SQLite 本地暂存 → 恢复后自动上传
6. **关注点分离：** Pattern 管协议，Handler 管业务

---

> **下一篇预告：** [03-cloud-backend.md] 云端后端深度分析——API 层、数据库迁移、ERP 集成、WebSocket 实时推送
