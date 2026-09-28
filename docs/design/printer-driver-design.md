# IGH 打印机驱动设计

> **文档编号:** PRINTER-DRIVER-001  
> **版本:** 1.0  
> **基于:** 系统需求规格说明书 v1.2 + API设计 v1.0  
> **日期:** 2026-09-28  
> **设计者:** 浮浮酱

---

## 一、需求概述

### 1.1 业务背景

边端系统需要在打包完成后自动打印标签，用于：
- 📦 **托盘标签** - 标识托盘信息（批次号、产品规格、重量、数量）
- 📦 **箱标签** - DTY装箱后的箱子标签
- 🏭 **成品标签** - 完整的产品追溯信息
- 📊 **二维码/条形码** - 快速扫码识别

### 1.2 打印机类型

**目标打印机（来自需求）：**

| 品牌 | 型号 | 类型 | 通信方式 | 使用场景 |
|------|------|------|----------|----------|
| **Eidos** | EX系列 | 热敏/热转印 | TCP/IP | 生产区托盘标签 |
| **Macsa** | iD系列 | 激光打标 | TCP/IP | 高端追溯标签 |
| **Zebra** | ZT/ZD系列 | 热敏/热转印 | TCP/IP / USB | 通用标签打印 |

### 1.3 系统需求

| 需求编号 | 需求描述 | 优先级 |
|----------|----------|--------|
| **REQ-151** | 支持三种打印机（Eidos/Macsa/ZPL） | P0 |
| **REQ-152** | 托盘封装后自动打印标签（2份） | P0 |
| **REQ-153** | 打印队列管理，失败自动重试（3次） | P0 |
| **REQ-154** | 打印机状态监控（在线/离线/缺纸/故障） | P1 |
| **REQ-155** | 支持自定义标签模板 | P1 |
| **REQ-156** | 打印历史记录（7天） | P2 |

### 1.4 标签设计规范

**托盘标签尺寸：** 100mm × 150mm  
**内容要求：**
- 批次号（大号字体）
- 产品名称和规格
- 丝锭数量
- 毛重/净重
- 生产日期
- 二维码（40mm × 40mm）

**示例：**
```
┌─────────────────────────────────┐
│  批次号: FDY-2026-001-001       │
│  ═══════════════════════════    │
│  产品: FDY150D/48F             │
│  规格: 半消光 AA等级             │
│  数量: 24锭                     │
│  毛重: 198.0 KG                 │
│  净重: 180.0 KG                 │
│  日期: 2026-09-28               │
│                                 │
│  ┌─────────┐                   │
│  │ QR Code │  PLT-20260928-001 │
│  │  here   │                   │
│  └─────────┘                   │
└─────────────────────────────────┘
```

---

## 二、技术选型

### 2.1 打印协议对比

| 协议 | 打印机 | 特点 | 复杂度 |
|------|--------|------|--------|
| **ESC/POS** | 通用热敏打印机 | 简单、通用 | ⭐ |
| **ZPL** | Zebra系列 | 功能强大、行业标准 | ⭐⭐ |
| **TSPL** | TSC系列 | 类似ZPL | ⭐⭐ |
| **专有协议** | Eidos/Macsa | 厂商专有 | ⭐⭐⭐ |

### 2.2 Go 语言打印库

| 库名 | 功能 | Stars | 推荐度 |
|------|------|-------|--------|
| **github.com/skip2/go-qrcode** | 二维码生成 | 1.5K+ | ✅ 推荐 |
| **github.com/boombuler/barcode** | 条形码生成 | 1K+ | ✅ 推荐 |
| **标准库 image** | 图像处理 | - | ✅ 内置 |
| **自研驱动** | 打印机通信 | - | ✅ 必需 |

**决策：自研打印机驱动 + 第三方二维码库**

**理由：**
1. ✅ 打印机协议简单（TCP Socket + 文本命令）
2. ✅ 灵活控制打印逻辑
3. ✅ 无第三方库依赖风险
4. ✅ 易于扩展新打印机型号

---

## 三、架构设计

### 3.1 插件化架构

```
┌──────────────────────────────────────┐
│        Printer Service               │
│  (统一打印服务，业务层调用)           │
└──────────────┬───────────────────────┘
               │
               ↓
┌──────────────────────────────────────┐
│        Printer Manager               │
│  - 打印队列管理                       │
│  - 失败重试                           │
│  - 状态监控                           │
└──────────────┬───────────────────────┘
               │
               ↓
┌──────────────────────────────────────┐
│      Printer Driver Interface        │
│  (统一接口，所有驱动实现此接口)       │
└──────────────┬───────────────────────┘
               │
       ┌───────┴───────┬──────────┐
       ↓               ↓          ↓
┌──────────┐  ┌──────────┐  ┌──────────┐
│  Eidos   │  │  Macsa   │  │   ZPL    │
│  Driver  │  │  Driver  │  │  Driver  │
└──────────┘  └──────────┘  └──────────┘
```

### 3.2 核心接口定义

**驱动接口（internal/pkg/printer/driver.go）：**

```go
package printer

import "context"

// Driver 打印机驱动接口
type Driver interface {
    // 连接打印机
    Connect(ctx context.Context) error
    
    // 断开连接
    Disconnect() error
    
    // 打印标签
    Print(ctx context.Context, label *Label) error
    
    // 查询状态
    GetStatus(ctx context.Context) (*Status, error)
    
    // 获取打印机信息
    GetInfo() *PrinterInfo
}

// Label 标签数据
type Label struct {
    Type       LabelType           // 标签类型
    Data       map[string]string   // 文本数据
    QRCode     string              // 二维码内容
    Barcode    string              // 条形码内容
    Template   string              // 模板名称（可选）
}

type LabelType int

const (
    LabelTypePallet LabelType = 1  // 托盘标签
    LabelTypeCarton LabelType = 2  // 箱标签
    LabelTypeProduct LabelType = 3 // 成品标签
)

// Status 打印机状态
type Status struct {
    Online       bool      // 在线
    Ready        bool      // 就绪
    PaperOut     bool      // 缺纸
    Error        bool      // 故障
    ErrorMessage string    // 错误消息
    QueueLength  int       // 队列长度
}

// PrinterInfo 打印机信息
type PrinterInfo struct {
    Type         string    // 类型（eidos/macsa/zpl）
    Model        string    // 型号
    Address      string    // IP地址
    Port         int       // 端口
}
```

---

## 四、驱动实现

### 4.1 ZPL 驱动（Zebra打印机）

**ZPL（Zebra Programming Language）命令示例：**

```zpl
^XA              // 开始标签
^FO50,50         // 字段原点（坐标）
^A0N,50,50       // 字体（0=默认，N=正常，50×50点）
^FD批次号: FDY-2026-001-001^FS   // 字段数据
^FO50,120
^A0N,30,30
^FD产品: FDY150D/48F^FS
^FO50,500
^BQN,2,8         // 二维码（N=正常，2=模型2，8=放大倍数）
^FDQA,PLT-20260928-001^FS  // 二维码内容
^XZ              // 结束标签
```

**Go 实现（internal/pkg/printer/zpl/driver.go）：**

```go
package zpl

import (
    "bytes"
    "context"
    "fmt"
    "net"
    "time"
    
    "github.com/igh/internal/pkg/printer"
)

type Driver struct {
    config *Config
    conn   net.Conn
}

type Config struct {
    Address string
    Port    int
    Timeout time.Duration
}

func NewDriver(cfg *Config) *Driver {
    return &Driver{
        config: cfg,
    }
}

func (d *Driver) Connect(ctx context.Context) error {
    addr := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)
    
    conn, err := net.DialTimeout("tcp", addr, d.config.Timeout)
    if err != nil {
        return fmt.Errorf("连接打印机失败: %w", err)
    }
    
    d.conn = conn
    return nil
}

func (d *Driver) Disconnect() error {
    if d.conn != nil {
        return d.conn.Close()
    }
    return nil
}

func (d *Driver) Print(ctx context.Context, label *printer.Label) error {
    // 生成 ZPL 命令
    zpl := d.generateZPL(label)
    
    // 发送到打印机
    _, err := d.conn.Write([]byte(zpl))
    if err != nil {
        return fmt.Errorf("打印失败: %w", err)
    }
    
    return nil
}

func (d *Driver) generateZPL(label *printer.Label) string {
    var buf bytes.Buffer
    
    buf.WriteString("^XA\n")  // 开始标签
    
    // 标题区域
    buf.WriteString("^FO50,50\n")
    buf.WriteString("^A0N,50,50\n")
    buf.WriteString(fmt.Sprintf("^FD批次号: %s^FS\n", label.Data["lot_number"]))
    
    // 分隔线
    buf.WriteString("^FO50,110^GB700,3,3^FS\n")
    
    // 产品信息
    y := 130
    fields := []struct {
        key   string
        label string
    }{
        {"product_name", "产品"},
        {"specification", "规格"},
        {"bobbin_count", "数量"},
        {"gross_weight", "毛重"},
        {"net_weight", "净重"},
        {"date", "日期"},
    }
    
    for _, field := range fields {
        buf.WriteString(fmt.Sprintf("^FO50,%d\n", y))
        buf.WriteString("^A0N,30,30\n")
        buf.WriteString(fmt.Sprintf("^FD%s: %s^FS\n", field.label, label.Data[field.key]))
        y += 50
    }
    
    // 二维码
    buf.WriteString("^FO600,130\n")
    buf.WriteString("^BQN,2,8\n")
    buf.WriteString(fmt.Sprintf("^FDQA,%s^FS\n", label.QRCode))
    
    // 托盘编码（二维码下方）
    buf.WriteString("^FO570,470\n")
    buf.WriteString("^A0N,25,25\n")
    buf.WriteString(fmt.Sprintf("^FD%s^FS\n", label.Data["pallet_code"]))
    
    buf.WriteString("^XZ\n")  // 结束标签
    
    return buf.String()
}

func (d *Driver) GetStatus(ctx context.Context) (*printer.Status, error) {
    // ZPL 状态查询命令
    _, err := d.conn.Write([]byte("~HS\n"))
    if err != nil {
        return nil, err
    }
    
    // 读取响应（简化示例）
    buffer := make([]byte, 256)
    n, err := d.conn.Read(buffer)
    if err != nil {
        return nil, err
    }
    
    response := string(buffer[:n])
    
    // 解析状态（根据 Zebra 协议）
    status := &printer.Status{
        Online: true,
        Ready:  true,
    }
    
    if bytes.Contains(buffer[:n], []byte("PAPER OUT")) {
        status.PaperOut = true
        status.Ready = false
    }
    
    return status, nil
}

func (d *Driver) GetInfo() *printer.PrinterInfo {
    return &printer.PrinterInfo{
        Type:    "zpl",
        Model:   "Zebra ZT410",
        Address: d.config.Address,
        Port:    d.config.Port,
    }
}
```

### 4.2 Eidos 驱动

**Eidos 专有协议（示例）：**
```
START
TEXT,50,50,50,批次号: FDY-2026-001-001
TEXT,50,120,30,产品: FDY150D/48F
QRCODE,600,130,8,PLT-20260928-001
END
PRINT,2
```

**Go 实现（internal/pkg/printer/eidos/driver.go）：**

```go
package eidos

import (
    "bytes"
    "context"
    "fmt"
    "net"
    "time"
    
    "github.com/igh/internal/pkg/printer"
)

type Driver struct {
    config *Config
    conn   net.Conn
}

type Config struct {
    Address string
    Port    int
    Timeout time.Duration
}

func NewDriver(cfg *Config) *Driver {
    return &Driver{
        config: cfg,
    }
}

func (d *Driver) Connect(ctx context.Context) error {
    addr := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)
    
    conn, err := net.DialTimeout("tcp", addr, d.config.Timeout)
    if err != nil {
        return fmt.Errorf("连接Eidos打印机失败: %w", err)
    }
    
    d.conn = conn
    return nil
}

func (d *Driver) Disconnect() error {
    if d.conn != nil {
        return d.conn.Close()
    }
    return nil
}

func (d *Driver) Print(ctx context.Context, label *printer.Label) error {
    // 生成 Eidos 命令
    commands := d.generateEidosCommands(label)
    
    // 发送到打印机
    _, err := d.conn.Write([]byte(commands))
    if err != nil {
        return fmt.Errorf("打印失败: %w", err)
    }
    
    return nil
}

func (d *Driver) generateEidosCommands(label *printer.Label) string {
    var buf bytes.Buffer
    
    buf.WriteString("START\n")
    
    // 标题
    buf.WriteString(fmt.Sprintf("TEXT,50,50,50,%s: %s\n",
        "批次号", label.Data["lot_number"]))
    
    // 其他字段
    y := 130
    fields := []struct {
        key   string
        label string
    }{
        {"product_name", "产品"},
        {"specification", "规格"},
        {"bobbin_count", "数量"},
        {"gross_weight", "毛重"},
        {"net_weight", "净重"},
        {"date", "日期"},
    }
    
    for _, field := range fields {
        buf.WriteString(fmt.Sprintf("TEXT,50,%d,30,%s: %s\n",
            y, field.label, label.Data[field.key]))
        y += 50
    }
    
    // 二维码
    buf.WriteString(fmt.Sprintf("QRCODE,600,130,8,%s\n", label.QRCode))
    
    // 托盘编码
    buf.WriteString(fmt.Sprintf("TEXT,570,470,25,%s\n", label.Data["pallet_code"]))
    
    buf.WriteString("END\n")
    buf.WriteString("PRINT,2\n")  // 打印2份
    
    return buf.String()
}

func (d *Driver) GetStatus(ctx context.Context) (*printer.Status, error) {
    // Eidos 状态查询（假设命令为 STATUS）
    _, err := d.conn.Write([]byte("STATUS\n"))
    if err != nil {
        return nil, err
    }
    
    // 读取响应
    buffer := make([]byte, 256)
    n, err := d.conn.Read(buffer)
    if err != nil {
        return nil, err
    }
    
    response := string(buffer[:n])
    
    // 解析状态（根据 Eidos 协议）
    status := &printer.Status{
        Online: true,
        Ready:  bytes.Contains(buffer[:n], []byte("READY")),
    }
    
    return status, nil
}

func (d *Driver) GetInfo() *printer.PrinterInfo {
    return &printer.PrinterInfo{
        Type:    "eidos",
        Model:   "Eidos EX2",
        Address: d.config.Address,
        Port:    d.config.Port,
    }
}
```

### 4.3 Macsa 驱动（激光打标）

**特点：**
- 激光打标机，用于永久性标识
- 通常用于高端产品追溯
- 协议相对复杂，支持图形、矢量

**Go 实现（internal/pkg/printer/macsa/driver.go）：**

```go
package macsa

import (
    "context"
    "fmt"
    "net"
    "time"
    
    "github.com/igh/internal/pkg/printer"
)

type Driver struct {
    config *Config
    conn   net.Conn
}

type Config struct {
    Address string
    Port    int
    Timeout time.Duration
}

func NewDriver(cfg *Config) *Driver {
    return &Driver{
        config: cfg,
    }
}

func (d *Driver) Connect(ctx context.Context) error {
    addr := fmt.Sprintf("%s:%d", d.config.Address, d.config.Port)
    
    conn, err := net.DialTimeout("tcp", addr, d.config.Timeout)
    if err != nil {
        return fmt.Errorf("连接Macsa打印机失败: %w", err)
    }
    
    d.conn = conn
    return nil
}

func (d *Driver) Disconnect() error {
    if d.conn != nil {
        return d.conn.Close()
    }
    return nil
}

func (d *Driver) Print(ctx context.Context, label *printer.Label) error {
    // Macsa 激光打标命令（简化示例）
    // 实际协议可能需要发送图形数据
    
    commands := fmt.Sprintf(
        "MARK TEXT:%s QR:%s\n",
        label.Data["lot_number"],
        label.QRCode,
    )
    
    _, err := d.conn.Write([]byte(commands))
    if err != nil {
        return fmt.Errorf("打标失败: %w", err)
    }
    
    return nil
}

func (d *Driver) GetStatus(ctx context.Context) (*printer.Status, error) {
    _, err := d.conn.Write([]byte("STATUS\n"))
    if err != nil {
        return nil, err
    }
    
    buffer := make([]byte, 256)
    n, err := d.conn.Read(buffer)
    if err != nil {
        return nil, err
    }
    
    // 简化状态解析
    status := &printer.Status{
        Online: true,
        Ready:  true,
    }
    
    return status, nil
}

func (d *Driver) GetInfo() *printer.PrinterInfo {
    return &printer.PrinterInfo{
        Type:    "macsa",
        Model:   "Macsa iD7",
        Address: d.config.Address,
        Port:    d.config.Port,
    }
}
```

---

## 五、打印管理器

### 5.1 打印队列

**实现（internal/pkg/printer/manager.go）：**

```go
package printer

import (
    "context"
    "sync"
    "time"
    
    "go.uber.org/zap"
)

type Manager struct {
    drivers map[string]Driver  // 打印机代码 -> 驱动
    queue   *PrintQueue
    logger  *zap.Logger
    
    mu      sync.RWMutex
    running bool
    stopCh  chan struct{}
}

func NewManager(logger *zap.Logger) *Manager {
    return &Manager{
        drivers: make(map[string]Driver),
        queue:   NewPrintQueue(100),  // 队列容量100
        logger:  logger,
        stopCh:  make(chan struct{}),
    }
}

// RegisterDriver 注册打印机驱动
func (m *Manager) RegisterDriver(code string, driver Driver) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if _, exists := m.drivers[code]; exists {
        return fmt.Errorf("打印机已注册: %s", code)
    }
    
    // 连接打印机
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := driver.Connect(ctx); err != nil {
        m.logger.Warn("连接打印机失败，稍后重试",
            zap.String("code", code),
            zap.Error(err))
        // 不返回错误，允许注册，后台会尝试重连
    }
    
    m.drivers[code] = driver
    return nil
}

// Start 启动打印管理器
func (m *Manager) Start(ctx context.Context) error {
    m.mu.Lock()
    if m.running {
        m.mu.Unlock()
        return fmt.Errorf("管理器已在运行")
    }
    m.running = true
    m.mu.Unlock()
    
    // 启动打印队列处理
    go m.processQueue(ctx)
    
    // 启动状态监控
    go m.monitorStatus(ctx)
    
    return nil
}

// Stop 停止打印管理器
func (m *Manager) Stop() error {
    m.mu.Lock()
    if !m.running {
        m.mu.Unlock()
        return nil
    }
    m.running = false
    close(m.stopCh)
    m.mu.Unlock()
    
    // 断开所有打印机
    for _, driver := range m.drivers {
        driver.Disconnect()
    }
    
    return nil
}

// Print 提交打印任务
func (m *Manager) Print(ctx context.Context, printerCode string, label *Label, copies int) (string, error) {
    // 检查打印机是否存在
    m.mu.RLock()
    driver, exists := m.drivers[printerCode]
    m.mu.RUnlock()
    
    if !exists {
        return "", fmt.Errorf("打印机不存在: %s", printerCode)
    }
    
    // 创建打印任务
    job := &PrintJob{
        ID:           generateJobID(),
        PrinterCode:  printerCode,
        Label:        label,
        Copies:       copies,
        Status:       JobStatusQueued,
        CreatedAt:    time.Now(),
        MaxRetries:   3,
    }
    
    // 加入队列
    m.queue.Enqueue(job)
    
    m.logger.Info("打印任务已加入队列",
        zap.String("job_id", job.ID),
        zap.String("printer", printerCode))
    
    return job.ID, nil
}

// processQueue 处理打印队列
func (m *Manager) processQueue(ctx context.Context) {
    for {
        select {
        case <-m.stopCh:
            return
        case <-ctx.Done():
            return
        default:
            job := m.queue.Dequeue()
            if job == nil {
                time.Sleep(100 * time.Millisecond)
                continue
            }
            
            m.processPrintJob(ctx, job)
        }
    }
}

func (m *Manager) processPrintJob(ctx context.Context, job *PrintJob) {
    m.mu.RLock()
    driver, exists := m.drivers[job.PrinterCode]
    m.mu.RUnlock()
    
    if !exists {
        job.Status = JobStatusFailed
        job.ErrorMessage = "打印机不存在"
        m.logger.Error("打印机不存在", zap.String("code", job.PrinterCode))
        return
    }
    
    // 标记为打印中
    job.Status = JobStatusPrinting
    job.StartedAt = time.Now()
    
    // 执行打印（支持多份）
    var lastErr error
    for i := 0; i < job.Copies; i++ {
        err := driver.Print(ctx, job.Label)
        if err != nil {
            lastErr = err
            m.logger.Error("打印失败",
                zap.String("job_id", job.ID),
                zap.Int("copy", i+1),
                zap.Error(err))
            break
        }
    }
    
    if lastErr != nil {
        // 打印失败，检查是否需要重试
        job.RetryCount++
        if job.RetryCount < job.MaxRetries {
            job.Status = JobStatusQueued
            m.queue.Enqueue(job)  // 重新加入队列
            m.logger.Info("打印任务重新加入队列",
                zap.String("job_id", job.ID),
                zap.Int("retry", job.RetryCount))
        } else {
            job.Status = JobStatusFailed
            job.ErrorMessage = lastErr.Error()
            m.logger.Error("打印任务失败（达到最大重试次数）",
                zap.String("job_id", job.ID))
        }
    } else {
        // 打印成功
        job.Status = JobStatusCompleted
        job.CompletedAt = time.Now()
        m.logger.Info("打印任务完成",
            zap.String("job_id", job.ID),
            zap.Int("copies", job.Copies))
    }
}

// monitorStatus 监控打印机状态
func (m *Manager) monitorStatus(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-m.stopCh:
            return
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.checkAllPrinters(ctx)
        }
    }
}

func (m *Manager) checkAllPrinters(ctx context.Context) {
    m.mu.RLock()
    drivers := make(map[string]Driver)
    for code, driver := range m.drivers {
        drivers[code] = driver
    }
    m.mu.RUnlock()
    
    for code, driver := range drivers {
        status, err := driver.GetStatus(ctx)
        if err != nil {
            m.logger.Error("查询打印机状态失败",
                zap.String("code", code),
                zap.Error(err))
            continue
        }
        
        // 检查异常状态
        if !status.Online {
            m.logger.Warn("打印机离线", zap.String("code", code))
        }
        if status.PaperOut {
            m.logger.Warn("打印机缺纸", zap.String("code", code))
        }
        if status.Error {
            m.logger.Error("打印机故障",
                zap.String("code", code),
                zap.String("error", status.ErrorMessage))
        }
    }
}
```

### 5.2 打印队列

**实现（internal/pkg/printer/queue.go）：**

```go
package printer

import (
    "sync"
    "time"
)

type PrintQueue struct {
    jobs     []*PrintJob
    capacity int
    mu       sync.Mutex
}

type PrintJob struct {
    ID           string
    PrinterCode  string
    Label        *Label
    Copies       int
    Status       JobStatus
    ErrorMessage string
    RetryCount   int
    MaxRetries   int
    CreatedAt    time.Time
    StartedAt    time.Time
    CompletedAt  time.Time
}

type JobStatus int

const (
    JobStatusQueued    JobStatus = 1  // 队列中
    JobStatusPrinting  JobStatus = 2  // 打印中
    JobStatusCompleted JobStatus = 3  // 已完成
    JobStatusFailed    JobStatus = 4  // 失败
)

func NewPrintQueue(capacity int) *PrintQueue {
    return &PrintQueue{
        jobs:     make([]*PrintJob, 0, capacity),
        capacity: capacity,
    }
}

func (q *PrintQueue) Enqueue(job *PrintJob) bool {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    if len(q.jobs) >= q.capacity {
        return false  // 队列已满
    }
    
    q.jobs = append(q.jobs, job)
    return true
}

func (q *PrintQueue) Dequeue() *PrintJob {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    if len(q.jobs) == 0 {
        return nil
    }
    
    job := q.jobs[0]
    q.jobs = q.jobs[1:]
    return job
}

func (q *PrintQueue) Length() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return len(q.jobs)
}

func generateJobID() string {
    return fmt.Sprintf("PJ-%d", time.Now().UnixNano())
}
```

---

## 六、二维码生成

### 6.1 二维码库

**使用 go-qrcode：**

```bash
go get github.com/skip2/go-qrcode
```

**实现（internal/pkg/printer/qrcode.go）：**

```go
package printer

import (
    "fmt"
    
    "github.com/skip2/go-qrcode"
)

// GenerateQRCode 生成二维码（PNG格式）
func GenerateQRCode(content string, size int) ([]byte, error) {
    png, err := qrcode.Encode(content, qrcode.Medium, size)
    if err != nil {
        return nil, fmt.Errorf("生成二维码失败: %w", err)
    }
    return png, nil
}

// GenerateQRCodeToFile 生成二维码并保存到文件
func GenerateQRCodeToFile(content string, filename string, size int) error {
    err := qrcode.WriteFile(content, qrcode.Medium, size, filename)
    if err != nil {
        return fmt.Errorf("保存二维码失败: %w", err)
    }
    return nil
}
```

### 6.2 标签数据准备

**实现（internal/pkg/printer/label_builder.go）：**

```go
package printer

import (
    "fmt"
    "time"
)

type LabelBuilder struct {
    labelType LabelType
    data      map[string]string
    qrCode    string
    barcode   string
}

func NewLabelBuilder(labelType LabelType) *LabelBuilder {
    return &LabelBuilder{
        labelType: labelType,
        data:      make(map[string]string),
    }
}

func (b *LabelBuilder) SetData(key, value string) *LabelBuilder {
    b.data[key] = value
    return b
}

func (b *LabelBuilder) SetQRCode(content string) *LabelBuilder {
    b.qrCode = content
    return b
}

func (b *LabelBuilder) SetBarcode(content string) *LabelBuilder {
    b.barcode = content
    return b
}

func (b *LabelBuilder) Build() *Label {
    return &Label{
        Type:    b.labelType,
        Data:    b.data,
        QRCode:  b.qrCode,
        Barcode: b.barcode,
    }
}

// BuildPalletLabel 构建托盘标签（快捷方法）
func BuildPalletLabel(
    lotNumber string,
    productName string,
    specification string,
    bobbinCount int,
    grossWeight float64,
    netWeight float64,
    palletCode string,
) *Label {
    return NewLabelBuilder(LabelTypePallet).
        SetData("lot_number", lotNumber).
        SetData("product_name", productName).
        SetData("specification", specification).
        SetData("bobbin_count", fmt.Sprintf("%d锭", bobbinCount)).
        SetData("gross_weight", fmt.Sprintf("%.1f KG", grossWeight)).
        SetData("net_weight", fmt.Sprintf("%.1f KG", netWeight)).
        SetData("date", time.Now().Format("2006-01-02")).
        SetData("pallet_code", palletCode).
        SetQRCode(palletCode).
        Build()
}
```

---

## 七、业务集成

### 7.1 打包服务集成

**边端打包服务（internal/service/edge/packing/service.go）：**

```go
package packing

import (
    "context"
    "fmt"
    
    "github.com/igh/internal/pkg/printer"
    pb "github.com/igh/internal/generated/edge/v1"
)

type Service struct {
    pb.UnimplementedPackingServiceServer
    
    repo          *sqlite.Repository
    printerMgr    *printer.Manager
    logger        *zap.Logger
}

func (s *Service) SealPallet(
    ctx context.Context,
    req *pb.SealPalletRequest,
) (*pb.SealPalletResponse, error) {
    // 1. 查询托盘信息
    pallet, err := s.repo.GetPallet(ctx, req.PalletId)
    if err != nil {
        return nil, err
    }
    
    // 2. 查询批次信息
    lot, err := s.repo.GetLot(ctx, pallet.LotID)
    if err != nil {
        return nil, err
    }
    
    // 3. 事务：更新托盘状态
    tx, err := s.repo.BeginTx(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    err = tx.SealPallet(ctx, req.PalletId, req.GrossWeightKg, req.TareWeightKg)
    if err != nil {
        return nil, err
    }
    
    err = tx.Commit()
    if err != nil {
        return nil, err
    }
    
    // 4. 触发打印（异步）
    go s.printPalletLabel(pallet, lot)
    
    // 5. 返回响应
    return &pb.SealPalletResponse{
        PalletId:      pallet.ID,
        PalletCode:    pallet.PalletCode,
        Status:        "sealed",
        BobbinCount:   int32(pallet.BobbinCount),
        GrossWeightKg: req.GrossWeightKg,
        NetWeightKg:   req.GrossWeightKg - req.TareWeightKg,
        SealedAt:      timestampPB(time.Now()),
        PrintLabel:    true,
    }, nil
}

func (s *Service) printPalletLabel(pallet *domain.Pallet, lot *domain.Lot) {
    // 构建标签数据
    label := printer.BuildPalletLabel(
        lot.LotNumber,
        lot.ProductName,
        lot.Specification,
        pallet.BobbinCount,
        pallet.GrossWeightKg,
        pallet.NetWeightKg,
        pallet.PalletCode,
    )
    
    // 提交打印任务（打印2份）
    jobID, err := s.printerMgr.Print(
        context.Background(),
        "PRINTER-01",  // 打印机代码（从配置读取）
        label,
        2,  // 打印2份
    )
    
    if err != nil {
        s.logger.Error("提交打印任务失败",
            zap.String("pallet_code", pallet.PalletCode),
            zap.Error(err))
        return
    }
    
    s.logger.Info("打印任务已提交",
        zap.String("job_id", jobID),
        zap.String("pallet_code", pallet.PalletCode))
}
```

---

## 八、配置管理

### 8.1 配置文件

**边端配置（configs/edge.yaml）：**

```yaml
printers:
  - code: PRINTER-01
    type: zpl
    model: Zebra ZT410
    address: 192.168.1.200
    port: 9100
    enabled: true
    
  - code: PRINTER-02
    type: eidos
    model: Eidos EX2
    address: 192.168.1.201
    port: 8080
    enabled: true
    
  - code: PRINTER-03
    type: macsa
    model: Macsa iD7
    address: 192.168.1.202
    port: 3000
    enabled: false  # 禁用

# 打印队列配置
print_queue:
  capacity: 100
  max_retries: 3
  retry_delay: 5s

# 标签配置
label:
  size:
    width: 100mm
    height: 150mm
  qr_code_size: 40mm
  copies: 2  # 默认打印份数
```

### 8.2 配置加载与初始化

```go
package printer

type PrinterConfig struct {
    Code    string `yaml:"code"`
    Type    string `yaml:"type"`
    Model   string `yaml:"model"`
    Address string `yaml:"address"`
    Port    int    `yaml:"port"`
    Enabled bool   `yaml:"enabled"`
}

type Config struct {
    Printers   []PrinterConfig `yaml:"printers"`
    QueueSize  int             `yaml:"queue_size"`
    MaxRetries int             `yaml:"max_retries"`
}

func InitializeFromConfig(cfg *Config, logger *zap.Logger) (*Manager, error) {
    mgr := NewManager(logger)
    
    for _, printerCfg := range cfg.Printers {
        if !printerCfg.Enabled {
            continue
        }
        
        var driver Driver
        
        switch printerCfg.Type {
        case "zpl":
            driver = zpl.NewDriver(&zpl.Config{
                Address: printerCfg.Address,
                Port:    printerCfg.Port,
                Timeout: 5 * time.Second,
            })
        case "eidos":
            driver = eidos.NewDriver(&eidos.Config{
                Address: printerCfg.Address,
                Port:    printerCfg.Port,
                Timeout: 5 * time.Second,
            })
        case "macsa":
            driver = macsa.NewDriver(&macsa.Config{
                Address: printerCfg.Address,
                Port:    printerCfg.Port,
                Timeout: 5 * time.Second,
            })
        default:
            return nil, fmt.Errorf("不支持的打印机类型: %s", printerCfg.Type)
        }
        
        err := mgr.RegisterDriver(printerCfg.Code, driver)
        if err != nil {
            logger.Error("注册打印机驱动失败",
                zap.String("code", printerCfg.Code),
                zap.Error(err))
            // 继续注册其他打印机
        }
    }
    
    return mgr, nil
}
```

---

## 九、错误处理

### 9.1 错误类型

```go
type PrinterError struct {
    Code    ErrorCode
    Message string
    Printer string
}

type ErrorCode int

const (
    ErrConnectionFailed  ErrorCode = 2001  // 连接失败
    ErrPrinterOffline    ErrorCode = 2002  // 打印机离线
    ErrPaperOut          ErrorCode = 2003  // 缺纸
    ErrPrintFailed       ErrorCode = 2004  // 打印失败
    ErrQueueFull         ErrorCode = 2005  // 队列已满
    ErrInvalidLabel      ErrorCode = 2006  // 标签数据无效
)
```

### 9.2 降级策略

**打印失败时：**
1. ✅ **自动重试** - 失败后自动重试（最多3次）
2. ✅ **队列保存** - 打印任务保存在队列中，稍后重试
3. ✅ **手动补打** - 操作员可以手动触发补打
4. ✅ **告警通知** - 显示打印机故障告警

```go
func (s *Service) ReprintLabel(ctx context.Context, palletID string) error {
    // 查询托盘信息
    pallet, err := s.repo.GetPallet(ctx, palletID)
    if err != nil {
        return err
    }
    
    lot, err := s.repo.GetLot(ctx, pallet.LotID)
    if err != nil {
        return err
    }
    
    // 重新打印
    return s.printPalletLabel(pallet, lot)
}
```

---

## 十、监控和告警

### 10.1 Prometheus 指标

```go
var (
    printJobsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "igh_print_jobs_total",
            Help: "打印任务总数",
        },
        []string{"printer", "status"},  // success/failed
    )
    
    printDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "igh_print_duration_seconds",
            Help:    "打印耗时",
            Buckets: []float64{0.5, 1.0, 2.0, 5.0, 10.0},
        },
        []string{"printer"},
    )
    
    printerStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "igh_printer_status",
            Help: "打印机状态（1=在线, 0=离线）",
        },
        []string{"printer", "model"},
    )
    
    printQueueLength = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "igh_print_queue_length",
            Help: "打印队列长度",
        },
    )
)
```

### 10.2 告警规则

**Prometheus Alert Rules:**

```yaml
groups:
  - name: printer_alerts
    rules:
      - alert: PrinterOffline
        expr: igh_printer_status == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "打印机离线"
          description: "打印机 {{ $labels.printer }} 已离线超过5分钟"
      
      - alert: PrintQueueTooLong
        expr: igh_print_queue_length > 50
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "打印队列过长"
          description: "打印队列长度 {{ $value }}，可能存在打印机故障"
      
      - alert: PrintFailureRateHigh
        expr: rate(igh_print_jobs_total{status="failed"}[5m]) > 0.1
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "打印失败率过高"
          description: "打印机 {{ $labels.printer }} 失败率超过10%"
```

---

## 十一、测试方案

### 11.1 单元测试

**Mock 打印机驱动：**

```go
package mock

type MockDriver struct {
    connected    bool
    printError   error
    statusError  error
    status       *printer.Status
}

func NewMockDriver() *MockDriver {
    return &MockDriver{
        status: &printer.Status{
            Online: true,
            Ready:  true,
        },
    }
}

func (m *MockDriver) Connect(ctx context.Context) error {
    m.connected = true
    return nil
}

func (m *MockDriver) Print(ctx context.Context, label *printer.Label) error {
    if m.printError != nil {
        return m.printError
    }
    return nil
}

func (m *MockDriver) SetPrintError(err error) {
    m.printError = err
}
```

**测试用例：**

```go
func TestPrintManager_Print(t *testing.T) {
    logger := zap.NewNop()
    mgr := printer.NewManager(logger)
    
    // 注册 Mock 驱动
    mockDriver := mock.NewMockDriver()
    mgr.RegisterDriver("TEST-01", mockDriver)
    
    // 启动管理器
    ctx := context.Background()
    mgr.Start(ctx)
    defer mgr.Stop()
    
    // 构建标签
    label := printer.BuildPalletLabel(
        "FDY-2026-001-001",
        "FDY150D/48F",
        "半消光 AA等级",
        24,
        198.0,
        180.0,
        "PLT-20260928-001",
    )
    
    // 提交打印
    jobID, err := mgr.Print(ctx, "TEST-01", label, 2)
    
    assert.NoError(t, err)
    assert.NotEmpty(t, jobID)
    
    // 等待打印完成
    time.Sleep(200 * time.Millisecond)
}
```

### 11.2 集成测试（需要真实打印机）

```go
func TestRealPrinter_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }
    
    // 创建真实驱动
    driver := zpl.NewDriver(&zpl.Config{
        Address: "192.168.1.200",
        Port:    9100,
        Timeout: 5 * time.Second,
    })
    
    // 连接打印机
    err := driver.Connect(context.Background())
    assert.NoError(t, err)
    defer driver.Disconnect()
    
    // 构建标签
    label := printer.BuildPalletLabel(
        "TEST-001",
        "测试产品",
        "测试规格",
        1,
        10.0,
        9.5,
        "TEST-PLT-001",
    )
    
    // 打印
    err = driver.Print(context.Background(), label)
    assert.NoError(t, err)
    
    t.Log("打印测试标签成功")
}
```

---

## 十二、总结

### 12.1 技术选型

| 决策点 | 方案 | 理由 |
|--------|------|------|
| **架构** | 插件化驱动 | 易于扩展新打印机型号 |
| **协议** | TCP Socket | 打印机标准通信方式 |
| **队列** | 内存队列 | 简单可靠，支持重试 |
| **二维码** | go-qrcode | 成熟稳定，易于使用 |
| **监控** | Prometheus | 标准监控方案 |

### 12.2 核心特性

✅ **多打印机支持** - Eidos/Macsa/ZPL三种驱动  
✅ **打印队列** - 异步打印，失败重试（3次）  
✅ **插件化架构** - 易于扩展新打印机  
✅ **状态监控** - 实时监控打印机状态  
✅ **降级策略** - 打印失败不影响业务流程  
✅ **Mock测试** - 无需真实打印机即可测试  
✅ **Prometheus监控** - 实时指标和告警  

### 12.3 下一步工作

1. ✅ **PLC通信设计** - 已完成
2. ✅ **打印机驱动设计** - 本文档
3. ⏳ **前端设计** - 管理后台 + 边端界面
4. ⏳ **测试方案** - 单元测试 + 集成测试 + E2E测试
5. ⏳ **部署方案** - Docker + 双机热备 + 监控

---

> **文档状态：** ✅ 打印机驱动设计完成！  
> **下一步：** 前端设计（管理后台 + 边端操作界面）
