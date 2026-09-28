# IGH 前端设计

> **文档编号:** FRONTEND-DESIGN-001  
> **版本:** 1.0  
> **基于:** 系统需求规格说明书 v1.2 + API设计 v1.0  
> **日期:** 2026-09-28  
> **设计者:** 浮浮酱

---

## 一、设计概述

### 1.1 系统划分

IGH 系统包含两类前端应用：

| 应用类型 | 用户 | 部署位置 | 技术栈 | 访问方式 |
|----------|------|----------|--------|----------|
| **管理后台** | 管理员、调度员、质检主管 | 中心端服务器 | React + Ant Design | 浏览器（PC） |
| **边端操作界面** | 操作工、质检员、仓管员 | 边端设备（Windows） | React + Ant Design | 浏览器（工业平板/触摸屏） |

### 1.2 设计原则

**通用原则：**
1. ✅ **响应式设计** - 适配不同屏幕尺寸
2. ✅ **一致性** - 统一的视觉语言和交互模式
3. ✅ **易用性** - 减少学习成本，提高操作效率
4. ✅ **可访问性** - 支持键盘导航、屏幕阅读器
5. ✅ **性能优化** - 快速响应，流畅体验

**边端界面特殊要求（工业场景）：**
1. ✅ **大按钮** - 适合触摸屏操作（最小 48px × 48px）
2. ✅ **高对比度** - 适应车间光照条件
3. ✅ **简化流程** - 减少点击次数，单任务聚焦
4. ✅ **实时反馈** - 操作即时响应，避免等待焦虑
5. ✅ **离线提示** - 网络状态明确显示

### 1.3 技术栈选型

| 技术 | 版本 | 用途 | 理由 |
|------|------|------|------|
| **React** | 18+ | UI框架 | 生态成熟、性能优秀 |
| **Ant Design** | 5.x | 组件库 | 企业级、国际化、可定制 |
| **TypeScript** | 5.x | 类型检查 | 提高代码质量 |
| **React Router** | 6.x | 路由管理 | 标准路由方案 |
| **Zustand** | 4.x | 状态管理 | 轻量、易用 |
| **React Query** | 5.x | 数据请求 | 缓存、重试、实时更新 |
| **Axios** | 1.x | HTTP客户端 | 拦截器、类型支持 |
| **Socket.io-client** | 4.x | WebSocket | 实时通信 |
| **Recharts** | 2.x | 图表库 | React友好、响应式 |
| **dayjs** | 1.x | 时间处理 | 轻量、国际化 |

---

## 二、管理后台设计

### 2.1 整体布局

**经典后台布局（侧边栏 + 顶栏 + 内容区）：**

```
┌─────────────────────────────────────────────────┐
│  Logo  │  订单管理 ▼ │ 质量管理 ▼ │ 仓储管理 ▼ │  🔔 👤 │
├────────┴─────────────────────────────────────────┤
│        │                                         │
│  侧边栏  │          主内容区                      │
│        │                                         │
│  📦 订单 │  ┌───────────────────────────────┐  │
│  📊 批次 │  │                               │  │
│  🏭 生产 │  │       页面内容                 │  │
│  ✅ 质检 │  │                               │  │
│  📦 仓储 │  │                               │  │
│  📈 报表 │  │                               │  │
│  ⚙️ 设置 │  └───────────────────────────────┘  │
│        │                                         │
└────────┴─────────────────────────────────────────┘
```

**Ant Design Layout 组件：**

```tsx
import { Layout, Menu } from 'antd';

const { Header, Sider, Content } = Layout;

function AdminLayout() {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      {/* 顶部导航栏 */}
      <Header>
        <div className="logo">IGH MES</div>
        <Menu mode="horizontal" items={topMenuItems} />
        <UserDropdown />
      </Header>
      
      <Layout>
        {/* 侧边栏 */}
        <Sider width={200} collapsible>
          <Menu
            mode="inline"
            defaultSelectedKeys={['orders']}
            items={sideMenuItems}
          />
        </Sider>
        
        {/* 内容区 */}
        <Content style={{ padding: 24 }}>
          <Outlet />  {/* React Router 插槽 */}
        </Content>
      </Layout>
    </Layout>
  );
}
```

### 2.2 核心功能页面

#### 2.2.1 订单管理

**订单列表页面：**

```
┌─────────────────────────────────────────────────┐
│  订单管理                        [+ 新建订单]    │
├─────────────────────────────────────────────────┤
│  🔍 订单号 [________]  产品 [FDY ▼]  状态 [全部 ▼] │
│                                         [搜索]  │
├─────────────────────────────────────────────────┤
│  订单号    │ 产品      │ 数量    │ 进度  │ 状态  │ 操作      │
├───────────┼──────────┼────────┼──────┼──────┼──────────┤
│ FDY-2026-001 │ FDY150D/48F │ 50,000 │ 25% │ 进行中 │ 查看 编辑 │
│ POY-2026-002 │ POY100D/36F │ 30,000 │ 80% │ 进行中 │ 查看 编辑 │
│ DTY-2026-003 │ DTY150D/48F │ 40,000 │ 100% │ 已完成 │ 查看 归档 │
├───────────┴──────────┴────────┴──────┴──────┴──────────┤
│                   ◀ 1 2 3 4 5 ▶                         │
└─────────────────────────────────────────────────────────┘
```

**关键特性：**
- ✅ 表格支持排序、筛选、分页
- ✅ 进度条可视化（Ant Design Progress）
- ✅ 状态用 Tag 组件标识（颜色区分）
- ✅ 操作列使用 Space + Button 组件

**代码示例：**

```tsx
import { Table, Tag, Progress, Button, Space } from 'antd';

function OrderListPage() {
  const columns = [
    {
      title: '订单号',
      dataIndex: 'orderNumber',
      key: 'orderNumber',
      sorter: true,
      render: (text: string, record: Order) => (
        <Link to={`/orders/${record.id}`}>{text}</Link>
      ),
    },
    {
      title: '产品',
      dataIndex: 'productName',
      key: 'productName',
    },
    {
      title: '数量',
      dataIndex: 'plannedQuantity',
      key: 'plannedQuantity',
      render: (value: number) => value.toLocaleString(),
    },
    {
      title: '进度',
      key: 'progress',
      render: (_, record: Order) => {
        const percent = (record.actualQuantity / record.plannedQuantity) * 100;
        return <Progress percent={percent} size="small" />;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const color = {
          pending: 'default',
          in_progress: 'processing',
          completed: 'success',
          cancelled: 'error',
        }[status];
        
        const text = {
          pending: '待开始',
          in_progress: '进行中',
          completed: '已完成',
          cancelled: '已取消',
        }[status];
        
        return <Tag color={color}>{text}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record: Order) => (
        <Space>
          <Button size="small" onClick={() => handleView(record.id)}>
            查看
          </Button>
          <Button size="small" onClick={() => handleEdit(record.id)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];
  
  return (
    <div>
      <div className="page-header">
        <h2>订单管理</h2>
        <Button type="primary" onClick={handleCreate}>
          + 新建订单
        </Button>
      </div>
      
      <FilterForm onSearch={handleSearch} />
      
      <Table
        columns={columns}
        dataSource={orders}
        loading={loading}
        pagination={{
          current: page,
          pageSize: 20,
          total: total,
          onChange: handlePageChange,
        }}
      />
    </div>
  );
}
```

**订单详情页面：**

```
┌─────────────────────────────────────────────────┐
│  订单详情 - FDY-2026-001          [编辑] [暂停]  │
├─────────────────────────────────────────────────┤
│  基本信息                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ 订单号: FDY-2026-001                     │   │
│  │ 产品: FDY150D/48F                       │   │
│  │ 规格: 半消光，AA等级                     │   │
│  │ 计划数量: 50,000锭                       │   │
│  │ 实际数量: 12,500锭 (25%)                │   │
│  │ 状态: 进行中                             │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  批次列表                          [+ 创建批次]  │
│  ┌─────────────────────────────────────────┐   │
│  │ FDY-2026-001-001  A  25%  进行中  [查看]│   │
│  │ FDY-2026-001-002  B  0%   待开始  [查看]│   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  生产统计                                        │
│  ┌─────────────────────────────────────────┐   │
│  │  ████████████████░░░░░░░░░░  25%      │   │
│  │  已生产: 12,500  目标: 50,000         │   │
│  │  合格率: 95.2%  不合格: 600锭          │   │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

#### 2.2.2 批次管理

**批次追溯页面：**

```
┌─────────────────────────────────────────────────┐
│  批次追溯 - FDY-2026-001-001                     │
├─────────────────────────────────────────────────┤
│  🔍 丝锭编码 [______________]         [搜索]    │
├─────────────────────────────────────────────────┤
│  批次信息                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ 批次号: FDY-2026-001-001                │   │
│  │ 前缀: A                                 │   │
│  │ 订单: FDY-2026-001                      │   │
│  │ 产品: FDY150D/48F                       │   │
│  │ 状态: 进行中                             │   │
│  │ 已生产: 2,400锭 / 2,500锭 (96%)        │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  丝锭列表                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ 编码          │ 位置 │ 等级 │ 状态 │ 操作  │
│  ├──────────────┼─────┼─────┼─────┼──────┤
│  │ ...A001-001  │ 1   │ AA  │ 在库 │ 查看 │
│  │ ...A001-002  │ 2   │ AA  │ 在库 │ 查看 │
│  │ ...A001-003  │ 3   │ B   │ 在库 │ 查看 │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  质量分布                                        │
│  ┌─────────────────────────────────────────┐   │
│  │  AA ████████████████ 75% (1,800)      │   │
│  │  B  ████░░░░░░░░░░░ 20% (480)         │   │
│  │  C  █░░░░░░░░░░░░░░  4% (96)          │   │
│  │  D  ░░░░░░░░░░░░░░░  1% (24)          │   │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

**丝锭详情（追溯链）：**

```
┌─────────────────────────────────────────────────┐
│  丝锭追溯 - FDY2026001A001-001                   │
├─────────────────────────────────────────────────┤
│  基本信息                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ 丝锭编码: FDY2026001A001-001            │   │
│  │ 批次: FDY-2026-001-001 (A)              │   │
│  │ 位置: 1号位                              │   │
│  │ 最终等级: AA                             │   │
│  │ 状态: 在库                               │   │
│  │ 毛重: 8,250.5g  净重: 8,000.0g         │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  追溯链                                          │
│  ┌─────────────────────────────────────────┐   │
│  │  📦 订单: FDY-2026-001                  │   │
│  │      ↓                                   │   │
│  │  📊 批次: FDY-2026-001-001              │   │
│  │      ↓                                   │   │
│  │  🏭 机台: M001 (L01-A侧)                │   │
│  │      ↓                                   │   │
│  │  🎯 落纱: D001 (2026-09-25 10:15)       │   │
│  │      ↓                                   │   │
│  │  ✅ 质检: 5维度全部AA                    │   │
│  │      ↓                                   │   │
│  │  📦 托盘: PLT-20260925-001              │   │
│  │      ↓                                   │   │
│  │  🏢 仓储: 1号立体库 01-02-03            │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  质检记录                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ 维度    │ 等级 │ 检测时间       │ 检测员 │
│  ├────────┼─────┼───────────────┼───────┤
│  │ 视觉    │ AA  │ 09-25 10:20   │ 张三  │
│  │ 重量    │ AA  │ 09-25 10:20   │ 张三  │
│  │ 分选    │ AA  │ 09-25 11:30   │ 李四  │
│  │ 织造    │ AA  │ 09-25 14:00   │ 王五  │
│  │ 综合    │ AA  │ 09-25 14:01   │ 系统  │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

#### 2.2.3 仓储管理

**库存查询页面：**

```
┌─────────────────────────────────────────────────┐
│  库存查询                                        │
├─────────────────────────────────────────────────┤
│  🔍 批次 [________]  等级 [全部▼]  仓库 [全部▼]  │
│     养丝期 [≥3天]                       [搜索]  │
├─────────────────────────────────────────────────┤
│  托盘码  │ 批次       │ 等级│数量│重量  │库位    │ 库龄│
├─────────┼───────────┼────┼───┼─────┼───────┼────┤
│ PLT-001  │FDY-2026-001│ AA │24 │198kg│01-02-03│ 5天│
│ PLT-002  │FDY-2026-001│ AA │24 │196kg│01-02-04│ 5天│
│ PLT-003  │FDY-2026-002│ B  │24 │194kg│01-03-01│ 3天│
├─────────┴───────────┴────┴───┴─────┴───────┴────┤
│  合计: 72锭  588kg               FIFO推荐: PLT-001│
└─────────────────────────────────────────────────┘
```

**FIFO出库推荐：**

```tsx
function OutboundRecommendPage() {
  const [form] = Form.useForm();
  const [recommendations, setRecommendations] = useState([]);
  
  const handleRecommend = async (values) => {
    const res = await api.post('/warehouses/outbound/recommend', {
      lot_pattern: values.lotPattern,
      final_grade: values.grade,
      required_quantity: values.quantity,
      min_storage_days: 3,  // 养丝期
    });
    
    setRecommendations(res.data.recommended_pallets);
  };
  
  return (
    <div>
      <Form form={form} onFinish={handleRecommend}>
        <Form.Item label="批次模糊匹配" name="lotPattern">
          <Input placeholder="FDY-2026-%" />
        </Form.Item>
        <Form.Item label="等级" name="grade">
          <Select options={gradeOptions} />
        </Form.Item>
        <Form.Item label="所需数量" name="quantity">
          <InputNumber min={1} />
        </Form.Item>
        <Button type="primary" htmlType="submit">
          FIFO推荐
        </Button>
      </Form>
      
      <Table
        columns={recommendColumns}
        dataSource={recommendations}
        rowSelection={{
          type: 'checkbox',
          onChange: handleSelectionChange,
        }}
      />
      
      <Button
        type="primary"
        disabled={!selectedPallets.length}
        onClick={handleCreateOutbound}
      >
        创建出库任务
      </Button>
    </div>
  );
}
```

#### 2.2.4 报表页面

**生产日报：**

```
┌─────────────────────────────────────────────────┐
│  生产日报           📅 2026-09-28              │
├─────────────────────────────────────────────────┤
│  产品类型: [FDY ▼]                [导出Excel]   │
├─────────────────────────────────────────────────┤
│  总览                                            │
│  ┌─────────────────────────────────────────┐   │
│  │  📦 2,400锭    📊 19,200kg  ✅ 95.2%   │   │
│  │  总产量        总重量       合格率       │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  产线统计                                        │
│  ┌─────────────────────────────────────────┐   │
│  │                产量柱状图                 │   │
│  │  800│    █                              │   │
│  │  600│    █     █                        │   │
│  │  400│    █     █     █                  │   │
│  │  200│    █     █     █                  │   │
│  │    0└────┴─────┴─────┴───              │   │
│  │        L01   L02   L03                  │   │
│  └─────────────────────────────────────────┘   │
├─────────────────────────────────────────────────┤
│  班次统计                                        │
│  ┌─────────────────────────────────────────┐   │
│  │  班次  │ 产量   │ 重量    │ 合格率       │
│  ├───────┼───────┼────────┼─────────────┤
│  │  早班  │ 800锭 │ 6,400kg │ 96.5%       │
│  │  中班  │ 800锭 │ 6,400kg │ 94.8%       │
│  │  晚班  │ 800锭 │ 6,400kg │ 94.3%       │
│  └─────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

**使用 Recharts 图表库：**

```tsx
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend } from 'recharts';

function ProductionDailyReport() {
  const data = [
    { line: 'L01', output: 800, weight: 6400, rate: 96.5 },
    { line: 'L02', output: 800, weight: 6400, rate: 94.8 },
    { line: 'L03', output: 800, weight: 6400, rate: 94.3 },
  ];
  
  return (
    <div>
      <BarChart width={600} height={300} data={data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="line" />
        <YAxis />
        <Tooltip />
        <Legend />
        <Bar dataKey="output" fill="#1890ff" name="产量（锭）" />
      </BarChart>
    </div>
  );
}
```

---

## 三、边端操作界面设计

### 3.1 设计特点

**工业场景适配：**
- 🖐️ **大按钮** - 最小尺寸 80px × 60px，适合触摸
- 🎨 **高对比度** - 深色背景 + 白色/黄色文字
- 📱 **简化流程** - 每屏一个主任务，减少干扰
- ⚡ **实时反馈** - 操作立即响应，加载动画明确
- 🔌 **离线提示** - 网络状态、PLC状态、打印机状态

**布局结构：**

```
┌─────────────────────────────────────────────────┐
│  🏭 IGH边端系统   📶 在线   🔗 PLC连接   👤 张三  │
├─────────────────────────────────────────────────┤
│                                                 │
│                主操作区                          │
│            （单任务聚焦）                        │
│                                                 │
│                                                 │
│                                                 │
│                                                 │
│                                                 │
│          ┌──────┐  ┌──────┐                    │
│          │ 确认 │  │ 取消 │                    │
│          └──────┘  └──────┘                    │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 3.2 落纱操作界面

**流程：选择批次 → 选择机台 → 确认落纱 → 完成**

**步骤1：选择批次**

```
┌─────────────────────────────────────────────────┐
│  落纱操作 - 步骤1/3                              │
├─────────────────────────────────────────────────┤
│                                                 │
│  请扫描批次二维码或手动选择                      │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │                                         │   │
│  │          🔍 扫描批次二维码               │   │
│  │             [扫描框动画]                 │   │
│  │                                         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  或从列表选择：                                  │
│                                                 │
│  ┌───────────────────────────────────────┐     │
│  │  FDY-2026-001-001  (A)  进行中 25%  │     │
│  └───────────────────────────────────────┘     │
│  ┌───────────────────────────────────────┐     │
│  │  FDY-2026-001-002  (B)  待开始  0%   │     │
│  └───────────────────────────────────────┘     │
│                                                 │
│             ┌────────────┐                      │
│             │  下一步 →  │                      │
│             └────────────┘                      │
│                                                 │
└─────────────────────────────────────────────────┘
```

**步骤2：选择机台**

```
┌─────────────────────────────────────────────────┐
│  落纱操作 - 步骤2/3                              │
│  批次: FDY-2026-001-001 (A)                     │
├─────────────────────────────────────────────────┤
│                                                 │
│  请选择机台：                                    │
│                                                 │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │  M001   │  │  M002   │  │  M003   │        │
│  │  L01-A  │  │  L01-B  │  │  L02-A  │        │
│  │  ✅ 运行 │  │  ✅ 运行 │  │  ⚠️ 待机 │        │
│  │ FDY 24锭│  │ FDY 24锭│  │ FDY 24锭│        │
│  └─────────┘  └─────────┘  └─────────┘        │
│                                                 │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │  M004   │  │  M005   │  │  M006   │        │
│  │  L02-B  │  │  L03-A  │  │  L03-B  │        │
│  │  ✅ 运行 │  │  ❌ 故障 │  │  ✅ 运行 │        │
│  │ FDY 24锭│  │ FDY 24锭│  │ FDY 24锭│        │
│  └─────────┘  └─────────┘  └─────────┘        │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │ ← 上一步│  │  下一步 →  │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

**步骤3：确认落纱**

```
┌─────────────────────────────────────────────────┐
│  落纱操作 - 步骤3/3 确认                         │
├─────────────────────────────────────────────────┤
│                                                 │
│  批次: FDY-2026-001-001 (A)                     │
│  机台: M001 (L01-A侧)                           │
│  产品: FDY 24位                                  │
│  落纱序号: 1                                     │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │                                         │   │
│  │  将创建 24 个丝锭记录                    │   │
│  │  编码范围: FDY2026001A001-001 至 -024   │   │
│  │                                         │   │
│  │  确认执行落纱？                          │   │
│  │                                         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │  取消  │  │  ✅ 确认   │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

**完成界面：**

```
┌─────────────────────────────────────────────────┐
│  落纱操作 - 完成 ✅                              │
├─────────────────────────────────────────────────┤
│                                                 │
│              ┌───────────┐                      │
│              │     ✅     │                      │
│              │  落纱成功  │                      │
│              └───────────┘                      │
│                                                 │
│  批次: FDY-2026-001-001 (A)                     │
│  机台: M001 (L01-A侧)                           │
│  创建丝锭: 24个                                  │
│  落纱时间: 2026-09-28 10:15:30                  │
│                                                 │
│  编码范围:                                       │
│  FDY2026001A001-001 至 FDY2026001A001-024      │
│                                                 │
│             ┌────────────┐                      │
│             │  完成返回  │                      │
│             └────────────┘                      │
│                                                 │
│  3秒后自动返回...                                │
│                                                 │
└─────────────────────────────────────────────────┘
```

**代码示例：**

```tsx
function DoffingWizard() {
  const [step, setStep] = useState(1);
  const [selectedLot, setSelectedLot] = useState(null);
  const [selectedModule, setSelectedModule] = useState(null);
  
  const handleConfirm = async () => {
    try {
      const res = await api.post('/doffings', {
        lot_id: selectedLot.id,
        module_id: selectedModule.id,
        doffing_sequence: 1,
        operator_id: currentUser.id,
      });
      
      message.success('落纱成功！');
      setStep(4);  // 跳转到完成页
      
      // 3秒后自动返回
      setTimeout(() => {
        navigate('/edge/doffing');
      }, 3000);
      
    } catch (error) {
      message.error('落纱失败：' + error.message);
    }
  };
  
  return (
    <div className="doffing-wizard">
      <Steps current={step - 1}>
        <Step title="选择批次" />
        <Step title="选择机台" />
        <Step title="确认落纱" />
      </Steps>
      
      {step === 1 && <LotSelectionStep onNext={handleLotSelect} />}
      {step === 2 && <ModuleSelectionStep onNext={handleModuleSelect} onBack={() => setStep(1)} />}
      {step === 3 && <ConfirmationStep onConfirm={handleConfirm} onBack={() => setStep(2)} />}
      {step === 4 && <CompletionStep />}
    </div>
  );
}
```

### 3.3 质检操作界面

**扫码质检流程：**

```
┌─────────────────────────────────────────────────┐
│  质检操作                                        │
├─────────────────────────────────────────────────┤
│                                                 │
│  请扫描丝锭二维码                                │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │                                         │   │
│  │          🔍 扫描丝锭二维码               │   │
│  │             [扫描框动画]                 │   │
│  │                                         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  或手动输入编码：                                │
│  ┌─────────────────────────────────────────┐   │
│  │  [___________________________]  [确定]  │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
└─────────────────────────────────────────────────┘
```

**质检录入界面：**

```
┌─────────────────────────────────────────────────┐
│  质检录入                                        │
│  丝锭: FDY2026001A001-001  位置: 1号            │
├─────────────────────────────────────────────────┤
│  已完成质检:                                     │
│  ✅ 视觉: AA    ✅ 重量: AA                      │
│                                                 │
│  当前质检维度: 【分选】                          │
│                                                 │
│  请选择等级：                                    │
│                                                 │
│  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐  │
│  │  AA   │  │   B   │  │   C   │  │   D   │  │
│  │ 优等  │  │ 一等  │  │ 二等  │  │ 等外  │  │
│  └───────┘  └───────┘  └───────┘  └───────┘  │
│                                                 │
│  缺陷类型（可多选）：                            │
│  ☐ 油污    ☐ 断丝    ☐ 染色不均                │
│  ☐ 毛丝    ☐ 并丝    ☐ 其他                    │
│                                                 │
│  备注：                                          │
│  ┌─────────────────────────────────────────┐   │
│  │  [可选文字说明]                         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │  取消  │  │  ✅ 提交   │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

**质检完成（5维度全部完成）：**

```
┌─────────────────────────────────────────────────┐
│  质检完成 ✅                                     │
│  丝锭: FDY2026001A001-001                       │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │  ✅ 视觉: AA                            │   │
│  │  ✅ 重量: AA                            │   │
│  │  ✅ 分选: AA                            │   │
│  │  ✅ 织造: AA                            │   │
│  │  ✅ 综合: AA                            │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│           ┌───────────────┐                     │
│           │  最终等级: AA  │                     │
│           │    ✅ 合格     │                     │
│           └───────────────┘                     │
│                                                 │
│             ┌────────────┐                      │
│             │  继续质检  │                      │
│             └────────────┘                      │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 3.4 打包操作界面

**托盘码盘：**

```
┌─────────────────────────────────────────────────┐
│  托盘码盘                                        │
│  托盘: PLT-20260928-001  批次: FDY-2026-001-001 │
├─────────────────────────────────────────────────┤
│  容量: 24锭  已装: 18锭  剩余: 6锭              │
│  ████████████████████░░░░░░ 75%                │
│                                                 │
│  请扫描丝锭二维码装入托盘：                      │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │          🔍 扫描丝锭二维码               │   │
│  │             [扫描框动画]                 │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  已装丝锭：                                      │
│  ┌─────────────────────────────────────────┐   │
│  │  ...A001-001  AA  8.25kg  ✅            │   │
│  │  ...A001-002  AA  8.10kg  ✅            │   │
│  │  ...A001-003  B   8.05kg  ✅            │   │
│  │  ...           (显示最近5条)             │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │  撤销  │  │  封装托盘  │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

**托盘封装：**

```
┌─────────────────────────────────────────────────┐
│  托盘封装                                        │
│  托盘: PLT-20260928-001                         │
├─────────────────────────────────────────────────┤
│                                                 │
│  丝锭数量: 24锭  ✅                              │
│                                                 │
│  请输入托盘重量：                                │
│                                                 │
│  毛重 (kg):                                     │
│  ┌─────────────────────────────────────────┐   │
│  │  [198.0        ]                        │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  皮重 (kg):                                     │
│  ┌─────────────────────────────────────────┐   │
│  │  [18.0         ]                        │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  净重: 180.0 kg  (自动计算)                     │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │  封装后将自动打印标签 (2份)              │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │  取消  │  │  ✅ 封装   │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 3.5 入库扫描界面

**扫描托盘：**

```
┌─────────────────────────────────────────────────┐
│  立库入库                                        │
├─────────────────────────────────────────────────┤
│                                                 │
│  请扫描托盘二维码                                │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │                                         │   │
│  │          🔍 扫描托盘二维码               │   │
│  │             [扫描框动画]                 │   │
│  │                                         │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
└─────────────────────────────────────────────────┘
```

**推荐库位：**

```
┌─────────────────────────────────────────────────┐
│  立库入库 - 推荐库位                             │
│  托盘: PLT-20260928-001                         │
├─────────────────────────────────────────────────┤
│  托盘信息:                                       │
│  ┌─────────────────────────────────────────┐   │
│  │  批次: FDY-2026-001-001                 │   │
│  │  数量: 24锭                              │   │
│  │  毛重: 198.0 kg                         │   │
│  │  等级: AA                                │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  系统推荐库位:                                   │
│  ┌─────────────────────────────────────────┐   │
│  │                                         │   │
│  │        🏢 01巷-02排-03层                 │   │
│  │           (01-02-03)                    │   │
│  │                                         │   │
│  │  推荐理由: 就近原则 + 负载均衡          │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  或手动选择其他库位：                            │
│  ┌────┐ ┌────┐ ┌────┐                        │
│  │巷位▼│ │排位▼│ │层位▼│                        │
│  └────┘ └────┘ └────┘                        │
│                                                 │
│         ┌────────┐  ┌────────────┐             │
│         │  取消  │  │  ✅ 确认   │             │
│         └────────┘  └────────────┘             │
│                                                 │
└─────────────────────────────────────────────────┘
```

**入库中（PLC执行）：**

```
┌─────────────────────────────────────────────────┐
│  立库入库 - 执行中                               │
│  托盘: PLT-20260928-001                         │
├─────────────────────────────────────────────────┤
│                                                 │
│              ┌───────────┐                      │
│              │    ⏳      │                      │
│              │  入库中... │                      │
│              └───────────┘                      │
│                                                 │
│  目标库位: 01-02-03                             │
│  堆垛机运行中，请稍候...                         │
│                                                 │
│  ┌─────────────────────────────────────────┐   │
│  │         [动画：堆垛机移动]               │   │
│  └─────────────────────────────────────────┘   │
│                                                 │
│  ⚠️ 请勿操作设备                                │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## 四、实时通信

### 4.1 WebSocket 集成

**连接管理（utils/websocket.ts）：**

```typescript
import { io, Socket } from 'socket.io-client';

class WebSocketManager {
  private socket: Socket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  
  connect(url: string, token: string) {
    this.socket = io(url, {
      auth: { token },
      transports: ['websocket'],
    });
    
    this.socket.on('connect', () => {
      console.log('WebSocket connected');
      this.reconnectAttempts = 0;
    });
    
    this.socket.on('disconnect', () => {
      console.log('WebSocket disconnected');
      this.handleReconnect();
    });
    
    this.socket.on('error', (error) => {
      console.error('WebSocket error:', error);
    });
  }
  
  on(event: string, handler: (data: any) => void) {
    this.socket?.on(event, handler);
  }
  
  emit(event: string, data: any) {
    this.socket?.emit(event, data);
  }
  
  disconnect() {
    this.socket?.disconnect();
  }
  
  private handleReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      setTimeout(() => {
        console.log(`Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
        this.socket?.connect();
      }, 5000);
    }
  }
}

export const wsManager = new WebSocketManager();
```

**订阅实时事件：**

```typescript
import { wsManager } from '@/utils/websocket';
import { message, notification } from 'antd';

function useWebSocketEvents() {
  useEffect(() => {
    // 连接 WebSocket
    wsManager.connect(WS_URL, token);
    
    // 订阅落纱完成事件
    wsManager.on('DOFFING_COMPLETED', (data) => {
      notification.success({
        message: '落纱完成',
        description: `批次 ${data.lot_number} 在 ${data.module_code} 落纱完成`,
      });
      
      // 刷新数据
      queryClient.invalidateQueries(['lots', data.lot_id]);
    });
    
    // 订阅质检完成事件
    wsManager.on('INSPECTION_COMPLETED', (data) => {
      message.success(`丝锭 ${data.bobbin_code} 质检完成: ${data.final_grade}`);
    });
    
    // 订阅托盘封装事件
    wsManager.on('PALLET_SEALED', (data) => {
      notification.info({
        message: '托盘封装完成',
        description: `托盘 ${data.pallet_code} 已封装，正在打印标签...`,
      });
    });
    
    // 订阅设备状态变化
    wsManager.on('EDGE_OFFLINE', (data) => {
      notification.error({
        message: '设备离线',
        description: `边端设备 ${data.device_name} 已离线`,
        duration: 0,  // 不自动关闭
      });
    });
    
    return () => {
      wsManager.disconnect();
    };
  }, []);
}
```

### 4.2 实时数据展示

**生产监控大屏：**

```tsx
function ProductionMonitor() {
  const [realtimeData, setRealtimeData] = useState({
    totalOutput: 0,
    currentRate: 0,
    qualifiedRate: 0,
    lines: [],
  });
  
  useEffect(() => {
    // 订阅实时生产数据
    wsManager.on('PRODUCTION_UPDATE', (data) => {
      setRealtimeData(data);
    });
    
    // 定时刷新
    const interval = setInterval(() => {
      fetchProductionData();
    }, 10000);  // 每10秒刷新一次
    
    return () => {
      clearInterval(interval);
    };
  }, []);
  
  return (
    <div className="production-monitor">
      <Row gutter={16}>
        <Col span={8}>
          <Card>
            <Statistic
              title="今日产量"
              value={realtimeData.totalOutput}
              suffix="锭"
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="当前效率"
              value={realtimeData.currentRate}
              suffix="%"
              precision={1}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="合格率"
              value={realtimeData.qualifiedRate}
              suffix="%"
              precision={2}
              valueStyle={{ color: realtimeData.qualifiedRate >= 95 ? '#3f8600' : '#cf1322' }}
            />
          </Card>
        </Col>
      </Row>
      
      <LineStatusBoard lines={realtimeData.lines} />
    </div>
  );
}
```

---

## 五、状态管理

### 5.1 Zustand Store

**全局状态（stores/global.ts）：**

```typescript
import create from 'zustand';

interface GlobalState {
  user: User | null;
  setUser: (user: User | null) => void;
  
  online: boolean;
  setOnline: (online: boolean) => void;
  
  plcConnected: boolean;
  setPLCConnected: (connected: boolean) => void;
  
  printerStatus: { [code: string]: PrinterStatus };
  updatePrinterStatus: (code: string, status: PrinterStatus) => void;
}

export const useGlobalStore = create<GlobalState>((set) => ({
  user: null,
  setUser: (user) => set({ user }),
  
  online: true,
  setOnline: (online) => set({ online }),
  
  plcConnected: false,
  setPLCConnected: (connected) => set({ plcConnected: connected }),
  
  printerStatus: {},
  updatePrinterStatus: (code, status) =>
    set((state) => ({
      printerStatus: {
        ...state.printerStatus,
        [code]: status,
      },
    })),
}));
```

**使用示例：**

```tsx
function StatusBar() {
  const { online, plcConnected, printerStatus } = useGlobalStore();
  
  return (
    <Space>
      <Badge status={online ? 'success' : 'error'} text={online ? '在线' : '离线'} />
      <Badge status={plcConnected ? 'success' : 'error'} text="PLC" />
      {Object.entries(printerStatus).map(([code, status]) => (
        <Badge
          key={code}
          status={status.online ? 'success' : 'error'}
          text={`打印机-${code}`}
        />
      ))}
    </Space>
  );
}
```

---

## 六、测试方案

### 6.1 单元测试

**使用 Vitest + React Testing Library：**

```typescript
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import DoffingWizard from './DoffingWizard';

describe('DoffingWizard', () => {
  it('renders step 1 correctly', () => {
    render(<DoffingWizard />);
    expect(screen.getByText('选择批次')).toBeInTheDocument();
  });
  
  it('navigates to step 2 after lot selection', async () => {
    render(<DoffingWizard />);
    
    const lot = screen.getByText('FDY-2026-001-001');
    fireEvent.click(lot);
    
    const nextButton = screen.getByText('下一步');
    fireEvent.click(nextButton);
    
    expect(screen.getByText('选择机台')).toBeInTheDocument();
  });
});
```

### 6.2 E2E 测试

**使用 Playwright：**

```typescript
import { test, expect } from '@playwright/test';

test('complete doffing flow', async ({ page }) => {
  // 登录
  await page.goto('http://localhost:8081');
  await page.fill('[name="username"]', 'operator');
  await page.fill('[name="password"]', 'password');
  await page.click('button[type="submit"]');
  
  // 进入落纱页面
  await page.click('text=落纱操作');
  
  // 选择批次
  await page.click('text=FDY-2026-001-001');
  await page.click('text=下一步');
  
  // 选择机台
  await page.click('text=M001');
  await page.click('text=下一步');
  
  // 确认落纱
  await page.click('text=✅ 确认');
  
  // 验证成功
  await expect(page.locator('text=落纱成功')).toBeVisible();
});
```

---

## 七、总结

### 7.1 技术栈

| 层次 | 技术 | 理由 |
|------|------|------|
| **UI框架** | React 18 | 生态成熟、性能优秀 |
| **组件库** | Ant Design 5 | 企业级、国际化、可定制 |
| **路由** | React Router 6 | 标准路由方案 |
| **状态** | Zustand | 轻量、易用 |
| **数据** | React Query | 缓存、重试、实时 |
| **实时** | Socket.io | WebSocket封装 |
| **图表** | Recharts | React友好 |

### 7.2 核心特性

**管理后台：**
- ✅ 完整的订单、批次、仓储管理
- ✅ 丝锭追溯链可视化
- ✅ 实时生产监控
- ✅ 多维度报表分析

**边端界面：**
- ✅ 大按钮触摸友好（80px × 60px）
- ✅ 简化操作流程（单任务聚焦）
- ✅ 扫码快速录入
- ✅ 实时状态反馈
- ✅ 离线提示明确

### 7.3 下一步工作

1. ✅ **前端设计** - 本文档
2. ⏳ **测试方案** - 单元测试 + 集成测试 + E2E测试
3. ⏳ **部署方案** - Docker + 双机热备 + 监控

---

> **文档状态：** ✅ 前端设计完成！  
> **下一步：** 测试方案 + 部署方案
