# IGH API接口设计

> **文档编号:** API-DESIGN-001  
> **版本:** 1.0  
> **基于:** 系统需求规格说明书 v1.2 + 数据库详细设计 v1.0  
> **日期:** 2026-09-24  
> **设计者:** 浮浮酱

---

## 一、设计原则

### 1.1 架构约束（来自ADR）

| ADR编号 | 约束 | API影响 |
|---------|------|---------|
| **ADR-05** | Go全栈 + go-kratos框架 | HTTP/gRPC双协议支持 |
| **ADR-13** | WebSocket实时推送 | 边端前端实时数据通道 |
| **ADR-14** | 双层权限：边端简化 + 中心端RBAC | API需要区分边端/中心端认证 |
| **ADR-23** | OTA: Center通知 + Edge拉取 | WebSocket通知 + HTTP下载接口 |

### 1.2 设计目标

✅ **RESTful风格** - 资源导向，标准HTTP方法  
✅ **统一响应格式** - 成功/失败统一结构  
✅ **版本控制** - `/v1/` 路径前缀，支持演进  
✅ **认证授权** - JWT Token + casbin RBAC  
✅ **实时通信** - WebSocket推送关键事件  
✅ **OpenAPI文档** - 自动生成Swagger文档  

### 1.3 API分类

```
├── 中心端API (Center API)
│   ├── 管理后台API (HTTP REST)
│   ├── 边端数据上传API (HTTP/gRPC)
│   ├── WebSocket推送API
│   └── OTA管理API
│
└── 边端API (Edge API)
    ├── 本地操作API (HTTP REST)
    ├── WebSocket推送API (前端实时数据)
    └── 中心端连接API (心跳/同步/OTA)
```

---

## 二、通用规范

### 2.1 URL规范

**基础格式：**
```
[协议]://[域名]:[端口]/[版本]/[资源]/[操作]
```

**示例：**
```
# 中心端
https://center.example.com/v1/orders
https://center.example.com/v1/orders/123e4567-e89b-12d3-a456-426614174000

# 边端
http://192.168.1.100:8080/v1/doffings
http://192.168.1.100:8080/v1/bobbins/scan
```

**路径规则：**
- 资源名称复数形式：`/orders`, `/lots`, `/bobbins`
- 层级关系用嵌套：`/orders/{order_id}/lots`
- 操作用动词：`/bobbins/scan`, `/pallets/seal`

### 2.2 HTTP方法

| 方法 | 语义 | 幂等性 | 示例 |
|------|------|--------|------|
| **GET** | 查询资源 | ✅ | `GET /orders` |
| **POST** | 创建资源 | ❌ | `POST /orders` |
| **PUT** | 完整更新 | ✅ | `PUT /orders/{id}` |
| **PATCH** | 部分更新 | ❌ | `PATCH /orders/{id}` |
| **DELETE** | 删除资源 | ✅ | `DELETE /orders/{id}` |

### 2.3 统一响应格式

#### 2.3.1 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    // 响应数据
  },
  "meta": {
    "request_id": "req_123abc",
    "timestamp": "2026-09-24T10:30:00Z"
  }
}
```

#### 2.3.2 分页响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    // 数据数组
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 158,
    "total_pages": 8
  },
  "meta": {
    "request_id": "req_123abc",
    "timestamp": "2026-09-24T10:30:00Z"
  }
}
```

#### 2.3.3 错误响应

```json
{
  "code": 40001,
  "message": "订单号已存在",
  "error": "ORDER_NUMBER_DUPLICATE",
  "details": {
    "field": "order_number",
    "value": "FDY-2026-001"
  },
  "meta": {
    "request_id": "req_123abc",
    "timestamp": "2026-09-24T10:30:00Z"
  }
}
```

### 2.4 错误码规范

**错误码结构：** `XXYYZZ`
- `XX`: 错误类别（10通用、20认证、30业务、40数据、50系统）
- `YY`: 子类别
- `ZZ`: 具体错误

| 错误码 | 说明 | HTTP状态码 |
|--------|------|------------|
| **0** | 成功 | 200 |
| **10001** | 参数错误 | 400 |
| **10002** | 缺少必填参数 | 400 |
| **10003** | 参数格式错误 | 400 |
| **20001** | 未登录 | 401 |
| **20002** | Token无效 | 401 |
| **20003** | Token过期 | 401 |
| **20004** | 无权限 | 403 |
| **30001** | 业务规则违反 | 422 |
| **30002** | 状态不允许操作 | 422 |
| **40001** | 资源不存在 | 404 |
| **40002** | 资源已存在 | 409 |
| **40003** | 数据冲突 | 409 |
| **50001** | 服务器内部错误 | 500 |
| **50002** | 数据库错误 | 500 |
| **50003** | PLC通信失败 | 503 |

### 2.5 认证授权

#### 2.5.1 JWT Token结构

**Header:**
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

**Payload:**
```json
{
  "user_id": "uuid",
  "username": "operator01",
  "roles": ["operator"],
  "device_id": "edge-prod-01",  // 边端特有
  "exp": 1727171400,
  "iat": 1727167800
}
```

**使用方式：**
```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

#### 2.5.2 边端认证（ADR-14）

**固定角色 + IP绑定：**
```json
POST /v1/edge/auth/login
{
  "role": "operator",  // operator/supervisor/maintainer
  "password": "pin1234"
}

Response:
{
  "code": 0,
  "data": {
    "token": "eyJ...",
    "expires_in": 28800,  // 8小时
    "user": {
      "role": "operator",
      "ip_address": "192.168.1.100",
      "permissions": ["doffing.execute", "bobbin.query"]
    }
  }
}
```

#### 2.5.3 中心端认证（casbin RBAC）

```json
POST /v1/center/auth/login
{
  "username": "admin",
  "password": "password123"
}

Response:
{
  "code": 0,
  "data": {
    "token": "eyJ...",
    "refresh_token": "refresh_...",
    "expires_in": 7200,  // 2小时
    "user": {
      "id": "uuid",
      "username": "admin",
      "full_name": "系统管理员",
      "roles": ["admin"],
      "permissions": ["*"]  // 管理员全权限
    }
  }
}
```

### 2.6 分页查询参数

**统一参数：**
```
GET /v1/orders?page=1&page_size=20&sort=-created_at&filter=status:in_progress
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码（从1开始） |
| `page_size` | int | 20 | 每页数量（最大100） |
| `sort` | string | `-created_at` | 排序字段（`-`表示降序） |
| `filter` | string | - | 过滤条件（格式：`field:value`） |
| `search` | string | - | 全文搜索关键词 |

**filter语法：**
```
单条件：status:in_progress
多条件：status:in_progress,product_type:FDY
范围：created_at:gte:2026-09-01
包含：product_type:in:FDY,POY,DTY
```

### 2.7 时间格式

**统一使用ISO 8601格式：**
```
日期时间：2026-09-24T10:30:00Z (UTC)
仅日期：2026-09-24
```

---

## 三、中心端API详细设计

### 3.1 订单管理 (Orders)

#### 3.1.1 创建订单

```http
POST /v1/orders
Content-Type: application/json
Authorization: Bearer {token}

{
  "order_number": "FDY-2026-001",
  "product_type": "FDY",
  "product_name": "FDY150D/48F",
  "specification": "半消光，AA等级",
  "planned_quantity": 50000,
  "target_weight_kg": 12500.0,
  "planned_start_at": "2026-09-25T08:00:00Z",
  "planned_end_at": "2026-09-30T18:00:00Z",
  "customer_name": "浙江某纺织公司",
  "customer_code": "CUS-001"
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "order_number": "FDY-2026-001",
    "product_type": "FDY",
    "status": "pending",
    "created_at": "2026-09-24T10:30:00Z",
    "created_by": "admin"
  }
}
```

#### 3.1.2 查询订单列表

```http
GET /v1/orders?page=1&page_size=20&filter=status:in_progress&sort=-created_at
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "order_number": "FDY-2026-001",
      "product_type": "FDY",
      "product_name": "FDY150D/48F",
      "status": "in_progress",
      "planned_quantity": 50000,
      "actual_quantity": 12500,
      "progress": 25.0,
      "planned_start_at": "2026-09-25T08:00:00Z",
      "planned_end_at": "2026-09-30T18:00:00Z",
      "created_at": "2026-09-24T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 158,
    "total_pages": 8
  }
}
```

#### 3.1.3 查询订单详情

```http
GET /v1/orders/{order_id}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "order_number": "FDY-2026-001",
    "product_type": "FDY",
    "product_name": "FDY150D/48F",
    "specification": "半消光，AA等级",
    "status": "in_progress",
    "planned_quantity": 50000,
    "actual_quantity": 12500,
    "target_weight_kg": 12500.0,
    "customer": {
      "name": "浙江某纺织公司",
      "code": "CUS-001"
    },
    "planned_start_at": "2026-09-25T08:00:00Z",
    "planned_end_at": "2026-09-30T18:00:00Z",
    "started_at": "2026-09-25T08:15:00Z",
    "lots": [
      {
        "id": "uuid",
        "lot_number": "FDY-2026-001-001",
        "status": "in_progress",
        "actual_weight_kg": 625.0
      }
    ],
    "created_at": "2026-09-24T10:30:00Z",
    "updated_at": "2026-09-24T10:30:00Z"
  }
}
```

#### 3.1.4 更新订单状态

```http
PATCH /v1/orders/{order_id}/status
Content-Type: application/json

{
  "status": "paused",
  "reason": "设备维护"
}
```

#### 3.1.5 删除订单

```http
DELETE /v1/orders/{order_id}
```

---

### 3.2 批次管理 (Lots)

#### 3.2.1 创建批次

```http
POST /v1/lots
Content-Type: application/json

{
  "order_id": "uuid",
  "lot_number": "FDY-2026-001-001",
  "prefix": "A",
  "target_weight_kg": 625.0
}
```

#### 3.2.2 查询订单的批次列表

```http
GET /v1/orders/{order_id}/lots
```

#### 3.2.3 批次锁定/解锁

```http
PATCH /v1/lots/{lot_id}/lock
Content-Type: application/json

{
  "is_locked": true,
  "reason": "准备打包"
}
```

---

### 3.3 丝锭管理 (Bobbins)

#### 3.3.1 查询丝锭列表

```http
GET /v1/bobbins?lot_id={lot_id}&lifecycle=produced&page=1&page_size=50
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "bobbin_code": "FDY2026001A001-001",
      "lot_number": "FDY-2026-001-001",
      "module_code": "M001",
      "position": 1,
      "lifecycle": "produced",
      "gross_weight_g": 8250.5,
      "net_weight_g": 8000.0,
      "final_grade": "AA",
      "is_qualified": true,
      "produced_at": "2026-09-25T10:15:30Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total": 240
  }
}
```

#### 3.3.2 丝锭详情（含追溯）

```http
GET /v1/bobbins/{bobbin_id}?include=tracing
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "bobbin_code": "FDY2026001A001-001",
    "lifecycle": "in_warehouse",
    "physical": {
      "gross_weight_g": 8250.5,
      "net_weight_g": 8000.0,
      "tube_weight_g": 250.5,
      "length_m": 15800.0
    },
    "quality": {
      "final_grade": "AA",
      "is_qualified": true,
      "grades": [
        {
          "dimension": "vision",
          "grade_value": "AA",
          "inspected_at": "2026-09-25T10:20:00Z"
        },
        {
          "dimension": "weight",
          "grade_value": "AA",
          "inspected_at": "2026-09-25T10:20:10Z"
        }
      ]
    },
    "tracing": {
      "order": {
        "order_number": "FDY-2026-001",
        "product_name": "FDY150D/48F"
      },
      "lot": {
        "lot_number": "FDY-2026-001-001",
        "prefix": "A"
      },
      "module": {
        "module_code": "M001",
        "spinning_line": "L01",
        "spinning_side": "A"
      },
      "doffing": {
        "doffing_number": "D001",
        "doffed_at": "2026-09-25T10:15:00Z",
        "operator": "张三"
      },
      "pallet": {
        "pallet_code": "PLT-001",
        "packed_at": "2026-09-25T14:30:00Z"
      },
      "warehouse": {
        "warehouse_name": "1号立体库",
        "location_code": "01-02-03",
        "inbound_at": "2026-09-25T15:00:00Z"
      }
    },
    "produced_at": "2026-09-25T10:15:30Z",
    "updated_at": "2026-09-25T15:00:00Z"
  }
}
```

#### 3.3.3 批量查询丝锭（扫码/RFID）

```http
POST /v1/bobbins/batch-query
Content-Type: application/json

{
  "bobbin_codes": [
    "FDY2026001A001-001",
    "FDY2026001A001-002",
    "FDY2026001A001-003"
  ]
}
```

---

### 3.4 质检管理 (Quality)

#### 3.4.1 查询质检记录

```http
GET /v1/quality/grades?lot_id={lot_id}&dimension=final&grade_value=AA
```

#### 3.4.2 质量统计分析

```http
GET /v1/quality/stats?lot_id={lot_id}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "total_count": 240,
    "qualified_count": 228,
    "qualified_rate": 95.0,
    "grade_distribution": {
      "AA": 180,
      "B": 48,
      "C": 10,
      "D": 2
    },
    "defect_distribution": {
      "oil_stain": 15,
      "broken_filament": 8,
      "uneven_dyeing": 5
    }
  }
}
```

---

### 3.5 仓储管理 (Warehouse)

#### 3.5.1 查询库存

```http
GET /v1/warehouses/{warehouse_id}/stocks?status=in_stock&lot_number=FDY-2026-001
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "pallet_code": "PLT-001",
      "lot_number": "FDY-2026-001-001",
      "location": {
        "warehouse_name": "1号立体库",
        "location_code": "01-02-03",
        "aisle": 1,
        "row": 2,
        "tier": 3
      },
      "bobbin_count": 24,
      "gross_weight_kg": 198.0,
      "final_grade": "AA",
      "inbound_at": "2026-09-25T15:00:00Z",
      "storage_days": 2
    }
  ],
  "pagination": {
    "total": 158
  }
}
```

#### 3.5.2 FIFO出库推荐

```http
POST /v1/warehouses/outbound/recommend
Content-Type: application/json

{
  "lot_pattern": "FDY%",
  "final_grade": "AA",
  "required_quantity": 10,
  "min_storage_days": 3  // 养丝期
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "recommended_pallets": [
      {
        "pallet_id": "uuid",
        "pallet_code": "PLT-001",
        "location_code": "01-02-03",
        "inbound_at": "2026-09-22T15:00:00Z",
        "storage_days": 5,
        "priority": 1  // FIFO排序
      }
    ],
    "total_quantity": 10
  }
}
```

#### 3.5.3 执行入库

```http
POST /v1/warehouses/inbound
Content-Type: application/json

{
  "warehouse_id": "uuid",
  "pallet_id": "uuid",
  "location_id": "uuid",  // 可选，不传则自动分配
  "operator_id": "uuid"
}
```

#### 3.5.4 执行出库

```http
POST /v1/warehouses/outbound
Content-Type: application/json

{
  "stock_ids": ["uuid1", "uuid2"],
  "operator_id": "uuid",
  "reason": "订单发货"
}
```

---

### 3.6 设备管理 (Devices)

#### 3.6.1 查询边端设备列表

```http
GET /v1/devices?zone_type=production&status=active
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "device_id": "edge-prod-01",
      "device_name": "生产区边端-01",
      "ip_address": "192.168.1.100",
      "zone_type": "production",
      "status": "active",
      "current_version": "v1.0.5",
      "last_heartbeat_at": "2026-09-24T10:29:45Z",
      "health": {
        "cpu_usage": 35.2,
        "memory_usage": 48.5,
        "disk_usage": 62.1,
        "sqlite_size_mb": 125.3,
        "pending_upload_count": 0
      }
    }
  ]
}
```

#### 3.6.2 注册边端设备（管理员）

```http
POST /v1/devices
Content-Type: application/json

{
  "device_id": "edge-prod-02",
  "device_name": "生产区边端-02",
  "ip_address": "192.168.1.101",
  "mac_address": "AA:BB:CC:DD:EE:FF",
  "zone_type": "production"
}
```

#### 3.6.3 设备停用

```http
PATCH /v1/devices/{device_id}/disable
```

---

### 3.7 报表 (Reports)

#### 3.7.1 产量日报

```http
GET /v1/reports/production/daily?date=2026-09-24&product_type=FDY
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "date": "2026-09-24",
    "product_type": "FDY",
    "summary": {
      "total_bobbins": 2400,
      "total_weight_kg": 19200.0,
      "qualified_count": 2280,
      "qualified_rate": 95.0
    },
    "by_line": [
      {
        "spinning_line": "L01",
        "bobbin_count": 800,
        "weight_kg": 6400.0,
        "qualified_rate": 96.5
      }
    ],
    "by_shift": [
      {
        "shift_name": "早班",
        "bobbin_count": 800,
        "weight_kg": 6400.0
      }
    ]
  }
}
```

#### 3.7.2 质量分析报表

```http
GET /v1/reports/quality/analysis?start_date=2026-09-01&end_date=2026-09-24&lot_id={lot_id}
```

#### 3.7.3 设备OEE报表

```http
GET /v1/reports/equipment/oee?module_id={module_id}&start_date=2026-09-01&end_date=2026-09-24
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "module_code": "M001",
    "period": {
      "start_date": "2026-09-01",
      "end_date": "2026-09-24"
    },
    "oee": {
      "availability": 92.5,  // 可用率%
      "performance": 88.3,   // 性能率%
      "quality": 95.0,       // 合格率%
      "overall": 77.5        // OEE = 92.5 × 88.3 × 95.0
    },
    "details": {
      "planned_hours": 576,
      "actual_running_hours": 533,
      "downtime_hours": 43,
      "target_output": 14400,
      "actual_output": 12700,
      "qualified_output": 12065
    }
  }
}
```

---

### 3.8 OTA更新管理

#### 3.8.1 上传版本包

```http
POST /v1/ota/packages
Content-Type: multipart/form-data

version=v1.0.6
component=igh-edge
changelog=修复PLC断线重连问题
file=@igh-edge-v1.0.6.exe
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "version": "v1.0.6",
    "component": "igh-edge",
    "file_name": "igh-edge-v1.0.6.exe",
    "file_size_bytes": 52428800,
    "sha256_hash": "a3d5f8...",
    "uploaded_at": "2026-09-24T10:30:00Z"
  }
}
```

#### 3.8.2 创建推送任务

```http
POST /v1/ota/push-tasks
Content-Type: application/json

{
  "package_id": "uuid",
  "target_type": "zone",  // all/zone/device
  "target_zone": "production",
  "push_strategy": "notify",  // notify/force
  "scheduled_at": "2026-09-24T22:00:00Z"  // 可选，不传则立即推送
}
```

#### 3.8.3 查询推送任务状态

```http
GET /v1/ota/push-tasks/{task_id}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "package": {
      "version": "v1.0.6",
      "component": "igh-edge"
    },
    "target_type": "zone",
    "target_zone": "production",
    "status": "in_progress",
    "progress": {
      "total_devices": 5,
      "notified_devices": 5,
      "downloading_devices": 2,
      "ready_devices": 1,
      "updated_devices": 2,
      "failed_devices": 0
    },
    "devices": [
      {
        "device_id": "edge-prod-01",
        "device_name": "生产区边端-01",
        "status": "completed",
        "current_version": "v1.0.6",
        "updated_at": "2026-09-24T22:15:30Z"
      },
      {
        "device_id": "edge-prod-02",
        "device_name": "生产区边端-02",
        "status": "downloading",
        "download_progress": 65,
        "current_version": "v1.0.5"
      }
    ],
    "started_at": "2026-09-24T22:00:00Z"
  }
}
```

#### 3.8.4 强制更新单个设备

```http
POST /v1/ota/devices/{device_id}/force-update
Content-Type: application/json

{
  "package_id": "uuid"
}
```

---

### 3.9 用户与权限

#### 3.9.1 用户管理

```http
# 创建用户
POST /v1/users
{
  "username": "operator01",
  "password": "password123",
  "full_name": "张三",
  "employee_number": "EMP001",
  "email": "zhangsan@example.com",
  "role_ids": ["uuid1", "uuid2"]
}

# 查询用户列表
GET /v1/users?status=active&role=operator

# 修改密码
PATCH /v1/users/{user_id}/password
{
  "old_password": "old123",
  "new_password": "new456"
}

# 禁用用户
PATCH /v1/users/{user_id}/disable
```

#### 3.9.2 角色管理

```http
# 创建角色
POST /v1/roles
{
  "role_code": "quality_inspector",
  "role_name": "质检员",
  "description": "负责质检操作",
  "permission_ids": ["uuid1", "uuid2"]
}

# 查询角色权限
GET /v1/roles/{role_id}/permissions
```

---

### 3.10 审计日志

```http
GET /v1/audit-logs?user_id={user_id}&resource_type=orders&operation=update&start_date=2026-09-01
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "operation": "update",
      "resource_type": "orders",
      "resource_id": "uuid",
      "user": {
        "username": "admin",
        "full_name": "系统管理员"
      },
      "changes": {
        "old_value": {"status": "pending"},
        "new_value": {"status": "in_progress"}
      },
      "ip_address": "192.168.1.50",
      "occurred_at": "2026-09-24T10:30:00Z"
    }
  ]
}
```

---

## 四、边端API详细设计

### 4.1 落纱操作 (Doffing)

#### 4.1.1 执行落纱（FDY/POY）

```http
POST /v1/doffings
Content-Type: application/json
Authorization: Bearer {edge_token}

{
  "lot_id": "uuid",
  "module_id": "uuid",
  "doffing_sequence": 1,
  "operator_id": "uuid",
  "shift_id": "uuid"
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "doffing_id": "uuid",
    "doffing_number": "D001",
    "bobbins_created": 24,
    "bobbin_codes": [
      "FDY2026001A001-001",
      "FDY2026001A001-002",
      // ... 共24个
    ],
    "doffed_at": "2026-09-25T10:15:00Z"
  }
}
```

#### 4.1.2 执行DTY装载

```http
POST /v1/dty/loadings
Content-Type: application/json

{
  "lot_id": "uuid",
  "module_id": "uuid",
  "positions": [
    {"position": 1, "gross_weight_g": 8250.5},
    {"position": 2, "gross_weight_g": 8100.3},
    // ... 共96个
  ]
}
```

#### 4.1.3 负落纱（回滚）

```http
POST /v1/doffings/{doffing_id}/rollback
Content-Type: application/json

{
  "reason": "操作失误"
}
```

---

### 4.2 质检操作 (Inspection)

#### 4.2.1 扫码获取丝锭信息

```http
POST /v1/bobbins/scan
Content-Type: application/json

{
  "bobbin_code": "FDY2026001A001-001"
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "bobbin_code": "FDY2026001A001-001",
    "lot_number": "FDY-2026-001-001",
    "product_type": "FDY",
    "position": 1,
    "lifecycle": "produced",
    "gross_weight_g": 8250.5,
    "existing_grades": [
      {
        "dimension": "vision",
        "grade_value": "AA"
      }
    ]
  }
}
```

#### 4.2.2 提交质检结果

```http
POST /v1/inspections
Content-Type: application/json

{
  "bobbin_id": "uuid",
  "dimension": "weight",
  "grade_value": "AA",
  "gross_weight_g": 8250.5,
  "net_weight_g": 8000.0,
  "tube_weight_g": 250.5,
  "defect_codes": [],
  "inspector_id": "uuid",
  "equipment_code": "SCALE-01"
}
```

#### 4.2.3 批量质检（视觉检测系统推送）

```http
POST /v1/inspections/batch
Content-Type: application/json

{
  "dimension": "vision",
  "results": [
    {
      "bobbin_code": "FDY2026001A001-001",
      "grade_value": "AA",
      "defect_codes": []
    },
    {
      "bobbin_code": "FDY2026001A001-002",
      "grade_value": "B",
      "defect_codes": ["oil_stain"]
    }
  ],
  "equipment_code": "VISION-01"
}
```

---

### 4.3 打包操作 (Packing)

#### 4.3.1 创建托盘

```http
POST /v1/pallets
Content-Type: application/json

{
  "lot_id": "uuid",
  "pallet_type": "bobbin",  // FDY/POY直接码盘
  "capacity": 24
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "pallet_code": "PLT-20260925-001",
    "lot_number": "FDY-2026-001-001",
    "status": "packing",
    "capacity": 24,
    "current_count": 0
  }
}
```

#### 4.3.2 码盘（丝锭装入托盘）

```http
POST /v1/pallets/{pallet_id}/pack
Content-Type: application/json

{
  "bobbin_codes": [
    "FDY2026001A001-001",
    "FDY2026001A001-002"
  ],
  "layer": 1,
  "operator_id": "uuid"
}
```

#### 4.3.3 托盘封装

```http
POST /v1/pallets/{pallet_id}/seal
Content-Type: application/json

{
  "gross_weight_kg": 198.0,
  "tare_weight_kg": 18.0,
  "operator_id": "uuid"
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "pallet_id": "uuid",
    "pallet_code": "PLT-20260925-001",
    "status": "sealed",
    "bobbin_count": 24,
    "gross_weight_kg": 198.0,
    "net_weight_kg": 180.0,
    "sealed_at": "2026-09-25T14:30:00Z",
    "print_label": true  // 触发标签打印
  }
}
```

#### 4.3.4 DTY装箱

```http
# 创建箱子
POST /v1/cartons
{
  "lot_id": "uuid",
  "capacity": 12
}

# 装箱
POST /v1/cartons/{carton_id}/pack
{
  "bobbin_codes": ["DTY2026001B001-001", "DTY2026001B001-002"]
}

# 封箱
POST /v1/cartons/{carton_id}/seal
{
  "gross_weight_kg": 98.0
}
```

#### 4.3.5 DTY箱子码垛

```http
# 创建托盘
POST /v1/pallets
{
  "lot_id": "uuid",
  "pallet_type": "carton",
  "capacity": 8  // 8箱/托盘
}

# 码垛
POST /v1/pallets/{pallet_id}/pack-cartons
{
  "carton_codes": ["CTN-001", "CTN-002"],
  "layer": 1
}
```

---

### 4.4 丝车管理 (Silk Cars)

#### 4.4.1 扫描丝车RFID

```http
POST /v1/silk-cars/scan
Content-Type: application/json

{
  "rfid_tag": "RFID-SC-001"
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "silk_car_code": "SC-001",
    "status": "empty",
    "capacity": 100,
    "current_count": 0,
    "current_zone": "production"
  }
}
```

#### 4.4.2 丝锭装车

```http
POST /v1/silk-cars/{silk_car_id}/load
Content-Type: application/json

{
  "bobbin_codes": [
    "FDY2026001A001-001",
    "FDY2026001A001-002"
  ],
  "face": "A",  // A面/B面
  "operator_id": "uuid"
}
```

#### 4.4.3 丝车转运

```http
PATCH /v1/silk-cars/{silk_car_id}/transfer
Content-Type: application/json

{
  "from_zone": "production",
  "to_zone": "sorting",
  "operator_id": "uuid"
}
```

---

### 4.5 立库操作 (Warehouse)

#### 4.5.1 扫描托盘入库

```http
POST /v1/warehouse/inbound/scan
Content-Type: application/json

{
  "pallet_code": "PLT-20260925-001"
}
```

**响应（托盘信息+推荐库位）：**
```json
{
  "code": 0,
  "data": {
    "pallet": {
      "id": "uuid",
      "pallet_code": "PLT-20260925-001",
      "lot_number": "FDY-2026-001-001",
      "bobbin_count": 24,
      "gross_weight_kg": 198.0
    },
    "recommended_location": {
      "location_id": "uuid",
      "location_code": "01-02-03",
      "aisle": 1,
      "row": 2,
      "tier": 3,
      "reason": "就近原则+负载均衡"
    }
  }
}
```

#### 4.5.2 确认入库

```http
POST /v1/warehouse/inbound/confirm
Content-Type: application/json

{
  "pallet_id": "uuid",
  "location_id": "uuid",
  "operator_id": "uuid"
}
```

**PLC自动执行堆垛机任务后回调确认完成：**
```http
PATCH /v1/warehouse/inbound/{inbound_id}/complete
Content-Type: application/json

{
  "plc_result": "success"
}
```

#### 4.5.3 出库任务（FIFO）

```http
POST /v1/warehouse/outbound/create
Content-Type: application/json

{
  "lot_pattern": "FDY-2026-001%",
  "final_grade": "AA",
  "required_quantity": 5,
  "operator_id": "uuid"
}
```

**响应（自动FIFO选择）：**
```json
{
  "code": 0,
  "data": {
    "task_id": "uuid",
    "selected_pallets": [
      {
        "stock_id": "uuid",
        "pallet_code": "PLT-001",
        "location_code": "01-02-03",
        "inbound_at": "2026-09-22T15:00:00Z",
        "storage_days": 5
      }
    ],
    "plc_tasks": [
      {
        "task_id": "plc_task_001",
        "location_code": "01-02-03",
        "action": "retrieve"
      }
    ]
  }
}
```

---

### 4.6 打印操作 (Printing)

#### 4.6.1 打印托盘标签

```http
POST /v1/printing/pallet-label
Content-Type: application/json

{
  "pallet_id": "uuid",
  "printer_code": "PRINTER-01",
  "copies": 2
}
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "print_job_id": "uuid",
    "status": "queued",
    "printer_code": "PRINTER-01",
    "label_data": {
      "pallet_code": "PLT-20260925-001",
      "lot_number": "FDY-2026-001-001",
      "product_name": "FDY150D/48F",
      "bobbin_count": 24,
      "gross_weight_kg": 198.0,
      "qr_code": "data:image/png;base64,..."
    }
  }
}
```

#### 4.6.2 查询打印机状态

```http
GET /v1/printing/printers/{printer_code}/status
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "printer_code": "PRINTER-01",
    "printer_type": "eidos",  // eidos/macsa/zpl
    "status": "online",  // online/offline/error
    "ip_address": "192.168.1.200",
    "queue_length": 3,
    "last_job_at": "2026-09-25T14:35:00Z"
  }
}
```

---

### 4.7 本地配置与状态

#### 4.7.1 获取边端配置

```http
GET /v1/edge/config
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "device_id": "edge-prod-01",
    "zone_type": "production",
    "center_addresses": [
      {"host": "192.168.1.101", "port": 9090, "role": "primary"},
      {"host": "192.168.1.102", "port": 9090, "role": "standby"}
    ],
    "plc_config": {
      "address": "192.168.1.10",
      "port": 102,
      "rack": 0,
      "slot": 1
    },
    "product_config": {
      "product_type": "FDY",
      "position_count": 24,
      "grade_system": {
        "dimensions": ["vision", "weight", "sorting", "knitting", "final"],
        "grades": ["AA", "B", "C", "D"]
      }
    }
  }
}
```

#### 4.7.2 查询本地状态

```http
GET /v1/edge/status
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "device_id": "edge-prod-01",
    "version": "v1.0.5",
    "uptime_seconds": 86400,
    "center_connection": {
      "status": "connected",
      "active_address": "192.168.1.101:9090",
      "last_heartbeat_at": "2026-09-25T10:29:50Z"
    },
    "plc_connection": {
      "status": "connected",
      "address": "192.168.1.10",
      "last_read_at": "2026-09-25T10:29:55Z"
    },
    "local_storage": {
      "sqlite_size_mb": 125.3,
      "pending_sync_count": 0,
      "oldest_unsynced_at": null
    },
    "system_health": {
      "cpu_usage": 35.2,
      "memory_usage": 48.5,
      "disk_usage": 62.1
    }
  }
}
```

---

### 4.8 边端与中心端同步

#### 4.8.1 数据上传（边端主动）

```http
POST /v1/center/sync/upload
Content-Type: application/json
Authorization: Bearer {edge_token}

{
  "device_id": "edge-prod-01",
  "batch": [
    {
      "entity_type": "BOBBIN",
      "entity_id": "uuid",
      "operation": "INSERT",
      "data": {
        // 丝锭完整数据
      },
      "created_at": "2026-09-25T10:15:30Z"
    }
  ]
}
```

#### 4.8.2 心跳上报（边端→中心端）

```http
POST /v1/center/heartbeat
Content-Type: application/json
Authorization: Bearer {edge_token}

{
  "device_id": "edge-prod-01",
  "version": "v1.0.5",
  "health": {
    "cpu_usage": 35.2,
    "memory_usage": 48.5,
    "disk_usage": 62.1,
    "sqlite_size_mb": 125.3,
    "pending_upload_count": 0
  },
  "timestamp": "2026-09-25T10:30:00Z"
}
```

**响应（中心端可能下发指令）：**
```json
{
  "code": 0,
  "data": {
    "server_time": "2026-09-25T10:30:01Z",
    "commands": [
      {
        "command_type": "OTA_NOTIFY",
        "payload": {
          "package_id": "uuid",
          "version": "v1.0.6"
        }
      }
    ]
  }
}
```

---

### 4.9 边端OTA更新

#### 4.9.1 检查更新

```http
GET /v1/edge/ota/check-update
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "has_update": true,
    "package": {
      "id": "uuid",
      "version": "v1.0.6",
      "file_size_bytes": 52428800,
      "sha256_hash": "a3d5f8...",
      "changelog": "修复PLC断线重连问题"
    },
    "download_url": "https://center.example.com/v1/ota/packages/{package_id}/download"
  }
}
```

#### 4.9.2 下载更新包

```http
GET /v1/center/ota/packages/{package_id}/download
Authorization: Bearer {edge_token}

# 支持HTTP Range请求（断点续传）
Range: bytes=0-1048575
```

**边端验证SHA256后存储到临时目录，提示操作员确认更新**

#### 4.9.3 上报下载进度

```http
PATCH /v1/center/ota/devices/{device_id}/download-progress
Content-Type: application/json

{
  "package_id": "uuid",
  "progress": 65,
  "status": "downloading"  // downloading/ready/installing/failed
}
```

#### 4.9.4 操作员确认更新

**边端前端调用：**
```http
POST /v1/edge/ota/confirm-update
Content-Type: application/json

{
  "package_id": "uuid",
  "operator_id": "uuid"
}
```

**边端执行：**
1. 完成当前操作周期（如正在落纱则完成）
2. 优雅停机（保存SQLite WAL）
3. 替换exe文件
4. 执行迁移脚本（golang-migrate）
5. 看门狗拉起新版本

---

## 五、WebSocket实时推送API

### 5.1 连接建立

#### 5.1.1 边端前端连接

```javascript
// 边端WebSocket连接
const ws = new WebSocket('ws://192.168.1.100:8080/v1/ws?token=' + edgeToken);

ws.onopen = () => {
  console.log('Edge WebSocket connected');
  
  // 订阅感兴趣的事件
  ws.send(JSON.stringify({
    type: 'SUBSCRIBE',
    channels: ['doffing', 'inspection', 'pallet']
  }));
};
```

#### 5.1.2 中心端前端连接

```javascript
// 中心端WebSocket连接
const ws = new WebSocket('wss://center.example.com/v1/ws?token=' + centerToken);

ws.onopen = () => {
  console.log('Center WebSocket connected');
  
  // 订阅感兴趣的事件
  ws.send(JSON.stringify({
    type: 'SUBSCRIBE',
    channels: ['edge_status', 'production', 'warehouse', 'ota']
  }));
};
```

### 5.2 消息格式

**通用消息结构：**
```json
{
  "type": "EVENT",
  "channel": "doffing",
  "event": "DOFFING_COMPLETED",
  "data": {
    // 事件数据
  },
  "timestamp": "2026-09-25T10:15:30Z",
  "device_id": "edge-prod-01"  // 边端来源（中心端推送时包含）
}
```

### 5.3 事件类型

#### 5.3.1 生产事件 (边端推送给前端)

**落纱完成：**
```json
{
  "type": "EVENT",
  "channel": "doffing",
  "event": "DOFFING_COMPLETED",
  "data": {
    "doffing_id": "uuid",
    "doffing_number": "D001",
    "lot_number": "FDY-2026-001-001",
    "module_code": "M001",
    "bobbin_count": 24,
    "doffed_at": "2026-09-25T10:15:00Z"
  }
}
```

**质检完成：**
```json
{
  "type": "EVENT",
  "channel": "inspection",
  "event": "INSPECTION_COMPLETED",
  "data": {
    "bobbin_code": "FDY2026001A001-001",
    "dimension": "vision",
    "grade_value": "AA",
    "inspected_at": "2026-09-25T10:20:00Z"
  }
}
```

**托盘封装：**
```json
{
  "type": "EVENT",
  "channel": "pallet",
  "event": "PALLET_SEALED",
  "data": {
    "pallet_code": "PLT-20260925-001",
    "bobbin_count": 24,
    "gross_weight_kg": 198.0,
    "sealed_at": "2026-09-25T14:30:00Z"
  }
}
```

#### 5.3.2 设备状态事件 (中心端推送)

**边端上线/离线：**
```json
{
  "type": "EVENT",
  "channel": "edge_status",
  "event": "EDGE_ONLINE",  // EDGE_ONLINE/EDGE_OFFLINE
  "data": {
    "device_id": "edge-prod-01",
    "device_name": "生产区边端-01",
    "ip_address": "192.168.1.100",
    "zone_type": "production"
  }
}
```

**PLC断线：**
```json
{
  "type": "EVENT",
  "channel": "plc_status",
  "event": "PLC_DISCONNECTED",
  "data": {
    "device_id": "edge-prod-01",
    "plc_address": "192.168.1.10",
    "last_connected_at": "2026-09-25T10:25:00Z"
  }
}
```

#### 5.3.3 OTA事件

**新版本通知（中心端→边端）：**
```json
{
  "type": "COMMAND",
  "channel": "ota",
  "event": "OTA_NEW_VERSION",
  "data": {
    "package_id": "uuid",
    "version": "v1.0.6",
    "component": "igh-edge",
    "changelog": "修复PLC断线重连问题",
    "file_size_bytes": 52428800,
    "push_strategy": "notify"  // notify/force
  }
}
```

**更新状态推送（边端→中心端前端）：**
```json
{
  "type": "EVENT",
  "channel": "ota",
  "event": "OTA_STATUS_UPDATE",
  "data": {
    "device_id": "edge-prod-01",
    "package_id": "uuid",
    "status": "downloading",  // downloading/ready/installing/completed/failed
    "progress": 65,
    "message": "下载中..."
  }
}
```

#### 5.3.4 告警事件

**阈值告警：**
```json
{
  "type": "EVENT",
  "channel": "alert",
  "event": "THRESHOLD_EXCEEDED",
  "data": {
    "alert_type": "DISK_USAGE_HIGH",
    "severity": "warning",  // info/warning/error/critical
    "device_id": "edge-prod-01",
    "metric": "disk_usage",
    "current_value": 85.3,
    "threshold": 80.0,
    "message": "边端磁盘使用率超过80%"
  }
}
```

### 5.4 心跳保活

**客户端→服务端：**
```json
{
  "type": "PING",
  "timestamp": "2026-09-25T10:30:00Z"
}
```

**服务端→客户端：**
```json
{
  "type": "PONG",
  "server_time": "2026-09-25T10:30:01Z"
}
```

**心跳间隔：** 10秒  
**超时阈值：** 30秒（3次心跳）

---

## 六、gRPC接口设计（可选）

### 6.1 使用场景

**边端数据上传使用gRPC（性能优化）：**
- 批量数据上传（数千条丝锭记录）
- 低延迟要求的PLC数据同步
- 双向流式传输（心跳+数据推送）

### 6.2 Proto定义示例

```protobuf
syntax = "proto3";

package igh.center.v1;

option go_package = "github.com/igh/api/center/v1;centerv1";

// 数据同步服务
service SyncService {
  // 批量上传数据
  rpc BatchUpload(BatchUploadRequest) returns (BatchUploadResponse);
  
  // 双向流式同步
  rpc StreamSync(stream SyncMessage) returns (stream SyncMessage);
}

message BatchUploadRequest {
  string device_id = 1;
  repeated SyncEntity entities = 2;
}

message SyncEntity {
  string entity_type = 1;  // BOBBIN/GRADE/PALLET
  string entity_id = 2;
  string operation = 3;    // INSERT/UPDATE/DELETE
  bytes data = 4;          // JSON序列化后的数据
  string created_at = 5;   // ISO 8601
}

message BatchUploadResponse {
  int32 success_count = 1;
  int32 failed_count = 2;
  repeated SyncError errors = 3;
}

message SyncError {
  string entity_id = 1;
  string error_code = 2;
  string error_message = 3;
}
```

---

## 七、性能优化

### 7.1 API性能要求

| 接口类别 | 响应时间要求 | QPS要求 |
|----------|--------------|---------|
| **边端本地API** | ≤ 200ms (P95) | 50+ |
| **中心端查询API** | ≤ 500ms (P95) | 500+ |
| **中心端写入API** | ≤ 1000ms (P95) | 100+ |
| **批量上传** | ≤ 5000ms (1000条) | 10+ |

### 7.2 优化策略

**1. 数据库查询优化**
- 避免N+1查询（使用JOIN或预加载）
- 使用索引覆盖查询
- 分页查询使用窗口函数

**2. 缓存策略**
```go
// 使用本地缓存（边端）或Redis（中心端）
func GetBobbin(ctx context.Context, bobbinID uuid.UUID) (*Bobbin, error) {
  // 1. 尝试从缓存获取
  if cached, ok := cache.Get(bobbinID); ok {
    return cached.(*Bobbin), nil
  }
  
  // 2. 从数据库查询
  bobbin, err := db.Bobbin.Get(ctx, bobbinID)
  if err != nil {
    return nil, err
  }
  
  // 3. 写入缓存（5分钟TTL）
  cache.Set(bobbinID, bobbin, 5*time.Minute)
  
  return bobbin, nil
}
```

**3. 批量操作**
```go
// ❌ 循环单条INSERT（V2教训）
for _, bobbin := range bobbins {
  db.Exec("INSERT INTO bobbins ...")
}

// ✅ 批量INSERT
bulk := make([]*ent.BobbinCreate, len(bobbins))
for i, b := range bobbins {
  bulk[i] = client.Bobbin.Create().SetLotID(b.LotID)...
}
client.Bobbin.CreateBulk(bulk...).Exec(ctx)
```

**4. 响应压缩**
```go
// go-kratos中间件启用gzip压缩
import "github.com/go-kratos/kratos/v2/middleware/compress"

httpSrv := http.NewServer(
  http.Middleware(
    compress.Server(),  // 自动gzip压缩响应
  ),
)
```

---

## 八、API文档生成

### 8.1 OpenAPI/Swagger配置

**使用swag自动生成：**
```go
// @title IGH MES API
// @version 1.0
// @description IGH纺织MES系统API接口文档
// @contact.name API Support
// @contact.email support@example.com
// @BasePath /v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @Summary 创建订单
// @Description 创建新的生产订单
// @Tags orders
// @Accept json
// @Produce json
// @Param order body CreateOrderRequest true "订单信息"
// @Success 200 {object} Response{data=Order}
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Router /orders [post]
func CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Response, error) {
  // ...
}
```

**生成命令：**
```bash
swag init -g cmd/igh-center/main.go -o docs/swagger
```

**访问Swagger UI：**
```
http://center.example.com/swagger/index.html
```

---

## 九、API版本演进策略

### 9.1 版本控制规则

**URL路径版本：**
```
/v1/orders  - 当前版本
/v2/orders  - 新版本（不兼容变更时）
```

**向后兼容窗口：**
- 新版本API必须兼容前一个版本至少6个月
- 废弃API在响应头标记：`Deprecated: true`
- 提供迁移指南文档

### 9.2 废弃流程

**1. 公告阶段（T+0）：**
```http
GET /v1/orders

Response Headers:
Deprecation: true
Sunset: Wed, 01 Jun 2027 00:00:00 GMT
Link: </v2/orders>; rel="successor-version"
```

**2. 警告阶段（T+3个月）：**
```json
{
  "code": 0,
  "data": [...],
  "warnings": [
    {
      "code": "API_DEPRECATED",
      "message": "此接口将在2027-06-01废弃，请迁移到/v2/orders"
    }
  ]
}
```

**3. 下线（T+6个月）：**
```http
GET /v1/orders

HTTP/1.1 410 Gone
{
  "code": 41001,
  "message": "此接口已废弃，请使用/v2/orders"
}
```

---

## 十、总结

### 10.1 API设计统计

| 维度 | 数量 |
|------|------|
| **中心端REST API** | 80+ 个端点 |
| **边端REST API** | 50+ 个端点 |
| **WebSocket事件** | 20+ 种事件类型 |
| **gRPC服务** | 2个服务（可选） |
| **统一响应格式** | ✅ |
| **认证授权** | JWT + casbin RBAC |

### 10.2 核心设计决策

| 决策点 | 方案 | 理由 |
|--------|------|------|
| **API风格** | RESTful | 资源导向，标准HTTP方法，易于理解 |
| **实时通信** | WebSocket | 双向通信，实时推送，适合工业场景 |
| **认证方式** | JWT Token | 无状态，易于扩展，支持分布式 |
| **权限模型** | casbin RBAC | 灵活策略引擎，支持复杂权限规则 |
| **版本控制** | URL路径版本 | 明确版本，易于并存 |
| **文档生成** | swag自动生成 | 代码即文档，保持同步 |

### 10.3 与系统需求的对应关系

| 需求章节 | API覆盖 |
|----------|---------|
| **4.2 落纱管理** | ✅ POST /v1/doffings |
| **4.3 质检分拣** | ✅ POST /v1/inspections |
| **4.4 打包管理** | ✅ POST /v1/pallets/* |
| **4.5 标签打印** | ✅ POST /v1/printing/* |
| **4.6 立库区** | ✅ POST /v1/warehouse/* |
| **4.10 OTA更新** | ✅ POST /v1/ota/* + WebSocket通知 |
| **5.1 生产管理** | ✅ CRUD /v1/orders, /v1/lots |
| **5.2 质量管理** | ✅ GET /v1/quality/* |
| **5.3 仓储管理** | ✅ GET /v1/warehouses/* |
| **5.6 报表** | ✅ GET /v1/reports/* |

### 10.4 下一步工作

1. **Protobuf定义** - 完善gRPC接口的proto文件
2. **API Mock服务** - 提供前端开发用的Mock数据
3. **集成测试** - 编写端到端API测试用例
4. **性能基准测试** - 验证API响应时间和吞吐量
5. **API网关配置** - 限流、熔断、日志聚合

---

> **文档状态：** ✅ API接口设计完成！
> **下一步：** Protobuf定义 + API Mock服务 + 集成测试
