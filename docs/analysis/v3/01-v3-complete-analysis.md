# IGH-Manager-Sys 第三版本程序 — 源码深度分析报告

> 分析日期：2026-09-17 | 项目版本：3.9.0 | Java 1.8 + Spring Boot 2.5.15 + PostgreSQL

---

## 目录

1. [系统概述](#1-系统概述)
2. [系统架构图](#2-系统架构图)
3. [Maven模块依赖分析](#3-maven模块依赖分析)
4. [数据模型分析（ER图）](#4-数据模型分析er图)
5. [核心基础模块分析](#5-核心基础模块分析)
6. [生产管理模块（igh-production）](#6-生产管理模块igh-production)
7. [PLC通信模块（igh-plc）](#7-plc通信模块igh-plc)
8. [打包管理模块（igh-packaging）](#8-打包管理模块igh-packaging)
9. [质量检测模块（igh-quality）](#9-质量检测模块igh-quality)
10. [仓库管理模块（igh-warehouse）](#10-仓库管理模块igh-warehouse)
11. [设备管理模块（igh-equipment）](#11-设备管理模块igh-equipment)
12. [标签打印模块（igh-label）](#12-标签打印模块igh-label)
13. [边端集成模块（igh-edge + bianduan）](#13-边端集成模块igh-edge--bianduan)
14. [配置中心模块（igh-config）](#14-配置中心模块igh-config)
15. [系统管理模块（igh-system）](#15-系统管理模块igh-system)
16. [框架层（igh-framework）](#16-框架层igh-framework)
17. [定时任务模块（igh-quartz）](#17-定时任务模块igh-quartz)
18. [启动入口（igh-admin）](#18-启动入口igh-admin)
19. [全局数据流图](#19-全局数据流图)
20. [端到端业务流程图](#20-端到端业务流程图)
21. [API端点清单](#21-api端点清单)
22. [技术栈总结](#22-技术栈总结)

---

## 1. 系统概述

IGH-Manager-Sys 是一套面向**化纤（丝绸）行业**的智能制造管理系统。系统围绕**卷绕机**产出的**丝饼**为核心物料，覆盖从生产→质检→打包→入库→出库的完整产品生命周期。

### 核心业务领域

| 领域 | 核心实体 | 业务含义 |
|------|---------|---------|
| 生产管理 | 批次(Lot)→落丝(Doffing)→丝饼(Bobbin) | 卷绕机生产丝饼的全过程追踪 |
| PLC通信 | PLC设备→信号点→数据快照 | 西门子S7 PLC实时数据采集 |
| 打包管理 | 包装规格→打包任务→托盘 | 丝饼组盘打包 |
| 质量检测 | 质检任务→检测明细→缺陷记录 | 丝饼质量抽检/全检 |
| 仓库管理 | 仓库→库位→入库单/出库单→库存 | 成品丝饼入库出库 |
| 设备管理 | 设备档案→设备部件→维护计划 | 卷绕机等设备全生命周期管理 |
| 标签打印 | 模板→打印机→打印任务 | 丝饼/托盘标签打印 |
| 边端集成 | 边端网关→班次→存置架→流转事件 | 车间边缘计算节点数据上报 |
| 配置中心 | 租户→模块注册→工作流模板 | 多租户、模块化配置 |

### 技术架构概要

- **后端**: Java 8 + Spring Boot 2.5.15 + MyBatis + PostgreSQL
- **数据库**: PostgreSQL 12+，使用Druid连接池
- **缓存**: Redis (Lettuce)
- **安全**: Spring Security + JWT
- **PLC通信**: Moka7 (S7协议)
- **API文档**: Swagger 3 (SpringFox)
- **前端**: 独立Vue.js项目（前后端分离，不在此仓库中）

---

## 2. 系统架构图

### 2.1 整体系统架构

```mermaid
graph TB
    subgraph "前端层 (独立仓库)"
        VUE[Vue.js 前端应用]
    end

    subgraph "网关/入口层"
        NGINX[Nginx 反向代理]
    end

    subgraph "应用层 - igh-admin (Spring Boot)"
        CTRL[REST Controllers]
        SEC[Spring Security + JWT]
        AOP[AOP 日志/权限]
    end

    subgraph "框架层 - igh-framework"
        AUTH[认证授权]
        CACHE[Redis 缓存]
        WS[WebSocket]
        INTC[拦截器链]
    end

    subgraph "核心业务层"
        PROD[igh-production<br/>生产管理]
        PLC[igh-plc<br/>PLC通信]
        PKG[igh-packaging<br/>打包管理]
        QC[igh-quality<br/>质量检测]
        WH[igh-warehouse<br/>仓库管理]
        EQ[igh-equipment<br/>设备管理]
        LBL[igh-label<br/>标签打印]
        EDGE[igh-edge<br/>边端集成]
        CFG[igh-config<br/>配置中心]
    end

    subgraph "公共基础层"
        COMMON[igh-common<br/>注解/常量/工具/实体]
        SYS[igh-system<br/>用户/角色/菜单]
    end

    subgraph "数据层"
        PG[(PostgreSQL<br/>igh-data)]
        REDIS[(Redis)]
    end

    subgraph "工业设备层"
        S7PLC[西门子 S7 PLC]
        EDGE_GW[边端网关<br/>bianduan]
        PRINTER[标签打印机]
    end

    VUE -->|HTTP/REST| NGINX
    NGINX -->|:8099| CTRL
    CTRL --> SEC
    SEC --> AOP
    AOP --> PROD & PLC & PKG & QC & WH & EQ & LBL & EDGE & CFG
    PROD & PLC & PKG & QC & WH & EQ & LBL & EDGE & CFG --> COMMON
    COMMON --> SYS
    SYS --> CACHE & PG
    PLC -->|Moka7 S7协议| S7PLC
    EDGE -->|REST API| EDGE_GW
    EDGE_GW -->|S7协议| S7PLC
    LBL -->|网络打印| PRINTER
    PROD & PKG & QC & WH --> PG
    PLC & EQ & LBL & EDGE --> PG
    AUTH & INTC --> REDIS
```

### 2.2 模块依赖分层图

```mermaid
graph BT
    subgraph "Layer 0 - 基础"
        COMMON[igh-common]
    end

    subgraph "Layer 1 - 仅依赖common"
        SYS[igh-system]
        QUARTZ[igh-quartz]
        GEN[igh-generator]
        CONFIG[igh-config]
    end

    subgraph "Layer 2 - 框架核心"
        FW[igh-framework]
    end

    subgraph "Layer 3 - 基础业务"
        PLC[igh-plc]
        LABEL[igh-label]
    end

    subgraph "Layer 4 - 核心业务"
        EQ[igh-equipment]
        PROD[igh-production]
    end

    subgraph "Layer 5 - 上层业务"
        PKG[igh-packaging]
        QC[igh-quality]
        EDGE[igh-edge]
    end

    subgraph "Layer 6 - 最上层"
        WH[igh-warehouse]
    end

    subgraph "Layer 7 - 聚合入口"
        ADMIN[igh-admin]
    end

    SYS --> COMMON
    QUARTZ --> COMMON
    GEN --> COMMON
    CONFIG --> COMMON
    FW --> SYS
    PLC --> COMMON & FW
    LABEL --> COMMON & FW
    EQ --> PLC
    PROD --> PLC & LABEL
    PKG --> PROD
    QC --> PROD
    EDGE --> PROD & LABEL & PLC
    WH --> PROD & PKG
    ADMIN --> FW & QUARTZ & GEN & PLC & EQ & PROD & LABEL & PKG & QC & WH & CONFIG & EDGE
```

---

## 3. Maven模块依赖分析

### 3.1 15个Maven模块总览

| 模块 | 层级 | 依赖的igh模块 | 核心第三方依赖 |
|------|------|-------------|--------------|
| **igh-common** | L0 基础 | 无 | Spring Security, Redis, PageHelper, POI, FastJSON2, JWT |
| **igh-system** | L1 | igh-common | 无额外 |
| **igh-quartz** | L1 | igh-common | Quartz |
| **igh-generator** | L1 | igh-common | Velocity, Druid |
| **igh-config** | L1 | igh-common | MyBatis, PostgreSQL, Lombok |
| **igh-framework** | L2 | igh-system | Druid, Kaptcha, OSHI, WebSocket, AOP |
| **igh-plc** | L3 | igh-common, igh-framework | **Moka7 1.0.3** (本地S7库), Quartz, WebSocket |
| **igh-label** | L3 | igh-common, igh-framework | MyBatis, PostgreSQL |
| **igh-equipment** | L4 | igh-common, igh-framework, igh-plc | Swagger, WebSocket |
| **igh-production** | L4 | igh-common, igh-framework, igh-plc, igh-label | Lombok, MyBatis |
| **igh-packaging** | L5 | igh-common, igh-framework, igh-production | Lombok, MyBatis |
| **igh-quality** | L5 | igh-common, igh-framework, igh-production | Lombok, MyBatis |
| **igh-edge** | L5 | igh-common, igh-framework, igh-production, igh-label, igh-plc | Lombok, MyBatis |
| **igh-warehouse** | L6 | igh-common, igh-framework, igh-production, igh-packaging | MyBatis |
| **igh-admin** | L7 | 全部12个业务模块 | SpringFox Swagger3, PostgreSQL |

### 3.2 关键配置

| 配置项 | 值 |
|-------|---|
| 服务端口 | 8099 |
| 数据库 | PostgreSQL localhost:5432/igh-data |
| 连接池 | Druid (初始5, 最小10, 最大20) |
| Redis | localhost:6379, Lettuce |
| JWT有效期 | 30分钟 |
| 分页方言 | postgresql (PageHelper) |
| MyBatis Mapper路径 | classpath*:mapper/**/*Mapper.xml |
| Swagger路径映射 | /dev-api |
| 文件上传限制 | 单文件10MB, 总20MB |

---

## 4. 数据模型分析（ER图）

### 4.1 核心业务ER图

```mermaid
erDiagram
    prod_lot ||--o{ prod_doffing : "1:N 批次含多次落丝"
    prod_lot ||--o{ prod_bobbin : "1:N 批次含多个丝饼"
    prod_doffing ||--o{ prod_bobbin : "1:N 落丝产出多个丝饼"

    prod_lot {
        bigserial lot_id PK
        varchar lot_code UK "批次编号"
        varchar product_code "产品编码"
        varchar product_name "产品名称"
        int planned_quantity "计划数量"
        int actual_quantity "实际数量"
        varchar lot_status "PENDING/IN_PROGRESS/COMPLETED/CANCELLED"
        bigint winder_device_id "卷绕机ID"
    }

    prod_doffing {
        bigserial doffing_id PK
        bigint lot_id FK "所属批次"
        varchar doffing_code UK "落丝编号"
        int spindle_number "锭位号1-144"
        decimal bobbin_weight "丝饼重量kg"
        varchar quality_grade "A/B/C/D"
        varchar doffing_reason "FULL/BREAK/MANUAL/ABNORMAL"
        jsonb plc_data_snapshot "PLC数据快照"
    }

    prod_bobbin {
        bigserial bobbin_id PK
        varchar bobbin_code UK "丝饼条码"
        bigint lot_id FK "批次ID"
        bigint doffing_id FK "落丝ID"
        decimal bobbin_weight "重量kg"
        varchar quality_grade "A/B/C/D"
        varchar bobbin_status "PRODUCED/IN_STOCK/PACKED/SHIPPED/REJECTED"
        varchar pallet_code "托盘编码"
    }

    plc_device ||--o{ plc_signal_point : "1:N 设备含多个信号点"
    plc_device ||--o{ plc_polling_config : "1:N 设备含多个轮询配置"
    plc_device ||--o{ plc_comm_log : "1:N"
    plc_signal_point ||--o{ plc_data_snapshot : "1:N"

    plc_device {
        serial device_id PK
        varchar device_code UK "PLC001"
        varchar device_type "SIEMENS_S7_1500"
        varchar ip_address "192.168.1.100"
        int port "102"
        varchar communication_mode "AUTO/LOCAL/EDGE"
        varchar connection_status "CONNECTED/DISCONNECTED/ERROR"
    }

    plc_signal_point {
        serial point_id PK
        int device_id FK
        varchar point_code UK "PLC001.DB1.DBW0"
        varchar data_block "DB1"
        varchar data_type "BOOL/WORD/DWORD/REAL"
        varchar address "DBW0"
        int byte_offset "字节偏移"
        varchar access_mode "READ/WRITE/READ_WRITE"
    }

    plc_data_snapshot {
        bigserial snapshot_id PK
        int device_id FK
        int point_id FK
        varchar snapshot_value "快照值"
        numeric numeric_value "数值"
        timestamp snapshot_time "快照时间"
    }

    qc_inspection_task ||--o{ qc_inspection_detail : "1:N"
    qc_inspection_detail ||--o{ qc_defect_record : "1:N"
    qc_defect_type ||--o{ qc_defect_record : "1:N"

    qc_inspection_task {
        bigserial task_id PK
        varchar task_code UK "QC+日期+序号"
        bigint lot_id "关联批次"
        varchar task_type "SAMPLING/FULL"
        varchar task_status "PENDING/IN_PROGRESS/COMPLETED"
        varchar inspection_result "PASS/REJECT/PARTIAL"
        decimal pass_rate "合格率%"
    }

    qc_inspection_detail {
        bigserial detail_id PK
        bigint task_id FK
        bigint bobbin_id "丝饼ID"
        varchar inspection_result "PASS/REJECT"
        varchar quality_grade "SPECIAL/FIRST/SECOND/DEFECT"
        decimal measured_weight "实测重量"
    }

    qc_defect_record {
        bigserial record_id PK
        varchar record_code UK "DR+日期+序号"
        bigint defect_type_id FK "缺陷类型"
        varchar defect_severity "CRITICAL/MAJOR/MINOR"
        varchar disposition "ACCEPT/REWORK/SCRAP/DOWNGRADE"
    }

    qc_defect_type {
        bigserial type_id PK
        varchar type_code UK "APP001"
        varchar type_name "毛丝/油污/色差"
        varchar category "APPEARANCE/PHYSICAL/PERFORMANCE"
        varchar severity "CRITICAL/MAJOR/MINOR"
    }
```

### 4.2 仓储包装ER图

```mermaid
erDiagram
    wh_warehouse ||--o{ wh_location : "1:N 仓库含多个库位"
    wh_warehouse ||--o{ wh_inbound_order : "1:N"
    wh_warehouse ||--o{ wh_outbound_order : "1:N"
    wh_warehouse ||--o{ wh_inventory : "1:N"
    wh_location ||--o{ wh_inventory : "1:N"
    wh_inbound_order ||--o{ wh_inbound_detail : "1:N"
    wh_outbound_order ||--o{ wh_outbound_detail : "1:N"

    wh_warehouse {
        bigserial warehouse_id PK
        varchar warehouse_code UK
        varchar warehouse_name
        varchar warehouse_type "0成品/1原料/2半成品"
        int total_capacity "总容量"
        int used_capacity "已用容量"
    }

    wh_location {
        bigserial location_id PK
        varchar location_code UK
        bigint warehouse_id FK
        varchar zone_code "区域"
        varchar rack_code "货架"
        int level_no "层号"
        varchar location_status "0空闲/1占用/2锁定"
    }

    wh_inventory {
        bigserial inventory_id PK
        bigint warehouse_id FK
        bigint location_id FK
        bigint pallet_id "托盘"
        bigint lot_id "批次"
        int quantity "数量"
        varchar quality_grade "质量等级"
        varchar inventory_status "0正常/1冻结/2待检"
    }

    wh_inbound_order {
        bigserial inbound_id PK
        varchar inbound_code UK
        varchar inbound_type "0生产/1采购/2退货/3调拨"
        varchar order_status "0待入库/1入库中/2已完成"
        int pallet_count "托盘数"
        int bobbin_count "丝饼数"
    }

    wh_outbound_order {
        bigserial outbound_id PK
        varchar outbound_code UK
        varchar outbound_type "0销售/1调拨/2报废"
        varchar order_status "0待拣货/1拣货中/2已完成"
        varchar customer_name "客户"
    }

    pkg_pallet ||--o{ pkg_pallet_bobbin : "1:N"
    prod_bobbin ||--o{ pkg_pallet_bobbin : "1:N"

    pkg_pallet {
        bigserial pallet_id PK
        varchar pallet_code UK
        bigint lot_id FK "批次"
        int pallet_sequence "托盘序号"
        int bobbin_count "丝饼数"
        decimal gross_weight "毛重"
        decimal net_weight "净重"
        varchar pallet_status "BUILDING/SEALED/IN_WAREHOUSE/SHIPPED"
    }

    package_spec {
        bigserial spec_id PK
        varchar spec_code UK
        varchar spec_name "规格名称"
        int bobbin_per_layer "每层丝饼数"
        int layer_count "层数"
        int total_bobbin_count "总丝饼数"
    }

    packing_task {
        bigserial task_id PK
        varchar task_code UK
        bigint spec_id FK "包装规格"
        bigint lot_id "批次"
        varchar task_status "PENDING/IN_PROGRESS/COMPLETED"
        int planned_pallet_count "计划托盘数"
    }
```

### 4.3 配置中心ER图

```mermaid
erDiagram
    sys_tenant ||--o{ sys_tenant_module : "1:N 租户启用模块"
    sys_tenant ||--o{ sys_device_config : "1:N 租户设备配置"
    sys_tenant ||--o{ sys_tenant_workflow : "1:N 租户工作流"
    sys_tenant ||--o| sys_ha_config : "1:1 高可用配置"
    sys_module_registry ||--o{ sys_tenant_module : "1:N"
    sys_device_type ||--o{ sys_device_config : "1:N"
    sys_workflow_template ||--o{ sys_tenant_workflow : "1:N"

    sys_tenant {
        varchar tenant_id PK
        varchar tenant_code UK "IGH_HQ"
        varchar tenant_name "IGH智能制造总部"
        varchar tenant_type "FACTORY/GROUP"
        jsonb config_profile "配置文件"
    }

    sys_module_registry {
        bigserial module_id PK
        varchar module_code UK "production/plc/label"
        varchar module_type "CORE/OPTIONAL"
        jsonb dependencies "依赖模块"
        jsonb config_schema "配置Schema"
    }

    sys_device_type {
        bigserial type_id PK
        varchar type_code UK "S7_PLC"
        varchar category "PLC/SCANNER/PDA/PRINTER"
        varchar protocol "S7/MODBUS/TCP"
        varchar driver_class "驱动类全路径"
    }

    sys_workflow_template {
        bigserial template_id PK
        varchar template_code UK
        varchar workflow_type "PACKAGING/PRODUCTION/QUALITY"
        jsonb process_steps "工作流步骤定义"
        jsonb required_modules "必需模块"
        jsonb required_devices "必需设备"
    }
```

### 4.4 边端集成ER图

```mermaid
erDiagram
    plc_edge_server ||--o{ prod_shift : "1:N 网关独立班次"
    plc_edge_server ||--o{ prod_storage_rack : "1:N 存置架状态"
    plc_edge_server ||--o{ plc_edge_data_log : "1:N 数据上报日志"
    plc_edge_server ||--o{ plc_edge_command_log : "1:N 指令执行日志"
    prod_bobbin ||--o{ label_print_record : "1:N 打印记录"
    prod_bobbin ||--o{ prod_transfer_event : "1:N 流转事件"

    plc_edge_server {
        varchar edge_id PK "网关唯一标识"
        varchar edge_name "网关名称"
        varchar api_base_url "API地址"
        varchar status "ONLINE/OFFLINE/ERROR"
        timestamp last_heartbeat_time "最后心跳"
    }

    prod_shift {
        bigserial shift_id PK
        varchar edge_id FK "网关ID"
        int shift_number "1早/2中/3晚"
        int bobbin_count "丝饼产量"
        int doffing_count "落丝次数"
    }

    prod_storage_rack {
        bigserial rack_id PK
        varchar edge_id FK
        varchar rack_type "TEMP_1/TEMP_2/FULL"
        varchar side "LEFT/RIGHT"
        int current_count "当前数量"
        int capacity "容量18"
    }

    prod_transfer_event {
        bigserial event_id PK
        bigint bobbin_id FK
        varchar from_location "来源"
        varchar to_location "目标"
        timestamp event_time "流转时间"
    }

    label_print_record {
        bigserial print_id PK
        bigint bobbin_id FK
        varchar edge_id "网关ID"
        varchar print_status "SUCCESS/FAILED/PENDING"
        timestamp print_time "打印时间"
    }
```

### 4.5 全系统数据库表清单

| 模块 | 表名 | 说明 | 记录数级别 |
|------|-----|------|----------|
| **生产** | prod_lot | 生产批次 | 千级 |
| **生产** | prod_doffing | 落丝记录 | 十万级 |
| **生产** | prod_bobbin | 丝饼记录 | 百万级 |
| **PLC** | plc_device | PLC设备 | 十级 |
| **PLC** | plc_signal_point | 信号点配置 | 百级 |
| **PLC** | plc_polling_config | 轮询配置 | 十级 |
| **PLC** | plc_comm_log | 通讯日志 | 百万级(需分区) |
| **PLC** | plc_data_snapshot | 数据快照 | 千万级(需分区) |
| **PLC** | plc_edge_server | 边端服务注册 | 十级 |
| **PLC** | plc_edge_data_log | 边端数据日志 | 百万级 |
| **PLC** | plc_edge_command_log | 边端指令日志 | 万级 |
| **质量** | qc_inspection_task | 质检任务 | 千级 |
| **质量** | qc_inspection_detail | 质检明细 | 万级 |
| **质量** | qc_defect_type | 缺陷类型(字典) | 十级 |
| **质量** | qc_defect_record | 缺陷记录 | 万级 |
| **仓库** | wh_warehouse | 仓库 | 个位 |
| **仓库** | wh_location | 库位 | 千级 |
| **仓库** | wh_inventory | 库存 | 万级 |
| **仓库** | wh_inbound_order | 入库单 | 千级 |
| **仓库** | wh_inbound_detail | 入库明细 | 万级 |
| **仓库** | wh_outbound_order | 出库单 | 千级 |
| **仓库** | wh_outbound_detail | 出库明细 | 万级 |
| **包装** | package_spec | 包装规格 | 十级 |
| **包装** | packing_task | 打包任务 | 千级 |
| **包装** | pallet (pkg_pallet) | 托盘 | 万级 |
| **标签** | label_template | 标签模板 | 十级 |
| **标签** | label_printer | 打印机 | 十级 |
| **标签** | label_print_task | 打印任务队列 | 万级 |
| **标签** | label_field_metadata | 字段元数据 | 百级 |
| **边端** | prod_shift | 班次记录 | 千级 |
| **边端** | prod_storage_rack | 存置架状态 | 百级 |
| **边端** | label_print_record | 边端打印记录 | 万级 |
| **边端** | prod_transfer_event | 丝饼流转事件 | 万级 |
| **配置** | sys_tenant | 租户 | 个位 |
| **配置** | sys_module_registry | 模块注册 | 十级 |
| **配置** | sys_tenant_module | 租户模块配置 | 十级 |
| **配置** | sys_device_type | 设备类型 | 十级 |
| **配置** | sys_device_config | 设备配置 | 十级 |
| **配置** | sys_workflow_template | 工作流模板 | 十级 |
| **配置** | sys_tenant_workflow | 租户工作流 | 十级 |
| **配置** | sys_ha_config | 高可用配置 | 个位 |
| **系统** | sys_user/role/menu/dept/... | 系统管理表 | 百级 |
| **定时** | sys_job/sys_job_log | 定时任务 | 十级/万级 |

---

## 5. 核心基础模块分析

### 5.1 igh-common (公共组件)

**包结构**:
- `com.igh.common.annotation` — 自定义注解: @Anonymous, @DataScope, @DataSource, @Excel, @Log, @RateLimiter, @RepeatSubmit, @Sensitive
- `com.igh.common.config` — IGHConfig(系统配置), SensitiveJsonSerializer
- `com.igh.common.constant` — CacheConstants, Constants, GenConstants, HttpStatus, ScheduleConstants, UserConstants
- `com.igh.common.core.controller` — BaseController (分页/响应封装基类)
- `com.igh.common.core.domain` — AjaxResult, BaseEntity, TreeEntity, R(泛型响应)
- `com.igh.common.core.domain.entity` — SysUser, SysRole, SysMenu, SysDept, SysDictData, SysDictType
- `com.igh.common.core.domain.model` — LoginBody, LoginUser, RegisterBody
- `com.igh.common.core.page` — PageDomain, TableDataInfo, TableSupport
- `com.igh.common.core.redis` — RedisCache (Redis工具封装)
- `com.igh.common.enums` — BusinessStatus, BusinessType, DataSourceType, HttpMethod, LimitType, OperatorType, UserStatus
- `com.igh.common.exception` — ServiceException, GlobalException, 文件/用户/任务异常
- `com.igh.common.filter` — XssFilter, RepeatableFilter, RefererFilter
- `com.igh.common.utils` — DateUtils, StringUtils, SecurityUtils, ip/文件/Bean/Excel等工具类

### 5.2 igh-system (系统管理)

标准RuoYi系统管理模块，提供RBAC权限体系:

| 实体 | 表 | 功能 |
|-----|---|------|
| SysUser | sys_user | 用户管理 |
| SysRole | sys_role | 角色管理 |
| SysMenu | sys_menu | 菜单权限(M目录/C菜单/F按钮) |
| SysDept | sys_dept | 部门管理(树形) |
| SysPost | sys_post | 岗位管理 |
| SysConfig | sys_config | 参数配置 |
| SysNotice | sys_notice | 通知公告 |
| SysOperLog | sys_oper_log | 操作日志 |
| SysLogininfor | sys_logininfor | 登录日志 |
| SysDictType/Data | sys_dict_type/data | 数据字典 |
| SysUserRole | sys_user_role | 用户-角色关联 |
| SysRoleMenu | sys_role_menu | 角色-菜单关联 |
| SysRoleDept | sys_role_dept | 角色-部门关联 |
| SysUserPost | sys_user_post | 用户-岗位关联 |

### 5.3 igh-framework (框架层)

**安全链**: SecurityConfig → JwtAuthenticationTokenFilter → TokenService → LoginUser
- JWT令牌生成、验证、刷新
- 密码加密(BCrypt)
- 登录限流(Redis计数)

**AOP切面**:
- DataScopeAspect — 数据权限过滤(按部门)
- LogAspect — 操作日志记录(@Log注解)
- RateLimiterAspect — 接口限流

**拦截器**:
- RepeatSubmitInterceptor — 防重复提交
- HeaderInterceptor — 请求头处理

**配置**:
- DruidConfig — 多数据源配置(主从)
- KaptchaConfig — 验证码
- WebSocketConfig — WebSocket支持

---

## 6. 生产管理模块（igh-production）

### 6.1 模块结构

```
com.igh.production/
├── controller/
│   ├── ProdLotController.java         — 批次管理API
│   ├── ProdDoffingController.java     — 落丝记录API
│   └── ProdBobbinController.java      — 丝饼管理API
├── domain/
│   ├── ProdLot.java                   — 生产批次实体
│   ├── ProdDoffing.java               — 落丝记录实体
│   ├── ProdBobbin.java                — 丝饼记录实体
│   └── enums/
│       ├── BobbinStatus.java          — PRODUCED/IN_STOCK/PACKED/SHIPPED/REJECTED
│       ├── DoffingReason.java         — FULL/BREAK/MANUAL/ABNORMAL
│       ├── LotStatus.java             — PENDING/IN_PROGRESS/COMPLETED/CANCELLED
│       └── QualityGrade.java          — A/B/C/D
├── mapper/
│   ├── ProdLotMapper.java
│   ├── ProdDoffingMapper.java
│   └── ProdBobbinMapper.java
└── service/
    ├── IProdLotService.java
    ├── IProdDoffingService.java
    ├── IProdBobbinService.java
    └── impl/
        ├── ProdLotServiceImpl.java
        ├── ProdDoffingServiceImpl.java
        └── ProdBobbinServiceImpl.java
```

### 6.2 核心业务实体

**ProdLot (生产批次)**:
- lot_id, lot_code(唯一编号), lot_name
- product_code, product_name — 产品信息
- planned_quantity, actual_quantity — 计划/实际产量
- lot_status — PENDING→IN_PROGRESS→COMPLETED/CANCELLED
- winder_device_id/code/name — 关联卷绕机
- start_time, end_time — 生产时段

**ProdDoffing (落丝记录)**:
- doffing_id, doffing_code(唯一编号)
- lot_id(FK批次), spindle_number(锭位号1-144)
- bobbin_weight/length/diameter — 丝饼物理参数
- quality_grade(A/B/C/D), is_qualified
- doffing_reason — FULL(满筒)/BREAK(断丝)/MANUAL(手动)/ABNORMAL(异常)
- plc_data_snapshot(JSONB) — 落丝时PLC数据快照
- source_edge_id, plc_timestamp, received_at — 边端溯源字段

**ProdBobbin (丝饼记录)**:
- bobbin_id, bobbin_code(丝饼条码,唯一)
- lot_id(FK批次), doffing_id(FK落丝)
- bobbin_weight/length/diameter, quality_grade
- bobbin_status — PRODUCED→IN_STOCK→PACKED→SHIPPED/REJECTED
- pallet_code, box_code — 包装信息
- warehouse_location — 仓库位置
- source_edge_id, plc_timestamp — 边端溯源

### 6.3 生产管理流程图

```mermaid
flowchart TD
    A[创建生产批次<br/>LOT20250107001] --> B{批次状态}
    B -->|PENDING| C[启动批次<br/>IN_PROGRESS]
    C --> D[卷绕机开始生产]
    D --> E[PLC监控<br/>主轴转速/温度/运行状态]
    E --> F{满筒/断丝/异常?}
    F -->|满筒FULL| G[自动落丝]
    F -->|断丝BREAK| G
    F -->|手动MANUAL| G
    F -->|异常ABNORMAL| G
    G --> H[创建落丝记录<br/>DOF20250107001]
    H --> I[记录PLC数据快照<br/>JSONB]
    I --> J[生成丝饼记录<br/>BOB20250107001]
    J --> K[质量等级评定<br/>A/B/C/D]
    K --> L{是否继续生产?}
    L -->|是| E
    L -->|计划完成| M[完成批次<br/>COMPLETED]
    L -->|取消| N[取消批次<br/>CANCELLED]
    J --> O[丝饼进入后续流程]
    O --> P[质检]
    O --> Q[打包]
    O --> R[入库]
```

### 6.4 API端点

| HTTP方法 | 路径 | 说明 | 权限标识 |
|---------|------|------|---------|
| GET | /production/lot/list | 查询批次列表 | production:lot:list |
| GET | /production/lot/{lotId} | 获取批次详情 | production:lot:query |
| POST | /production/lot | 新增批次 | production:lot:add |
| PUT | /production/lot | 修改批次 | production:lot:edit |
| DELETE | /production/lot/{lotIds} | 删除批次 | production:lot:remove |
| POST | /production/lot/export | 导出批次 | production:lot:export |
| GET | /production/doffing/list | 查询落丝列表 | production:doffing:list |
| POST | /production/doffing | 新增落丝记录 | production:doffing:add |
| GET | /production/bobbin/list | 查询丝饼列表 | production:bobbin:list |
| POST | /production/bobbin | 新增丝饼 | production:bobbin:add |
| PUT | /production/bobbin | 修改丝饼 | production:bobbin:edit |
| POST | /production/bobbin/export | 导出丝饼 | production:bobbin:export |

---

## 7. PLC通信模块（igh-plc）

### 7.1 模块结构

```
com.igh.plc/
├── controller/
│   ├── PlcDeviceController.java       — PLC设备管理
│   ├── PlcSignalPointController.java  — 信号点配置
│   ├── PlcPollingConfigController.java — 轮询配置
│   ├── PlcMonitorController.java      — 实时监控
│   ├── PlcConfigController.java       — PLC系统配置
│   ├── PlcEdgeServerController.java   — 边端服务管理
│   └── PlcEdgeDataController.java     — 边端数据接收
├── domain/
│   ├── PlcDevice.java                 — PLC设备
│   ├── PlcSignalPoint.java            — 信号点
│   ├── PlcPollingConfig.java          — 轮询配置
│   ├── PlcCommLog.java                — 通讯日志
│   ├── PlcDataSnapshot.java           — 数据快照
│   ├── PlcEdgeServer.java             — 边端服务
│   ├── PlcEdgeDataLog.java            — 边端数据日志
│   ├── PlcEdgeCommandLog.java         — 边端指令日志
│   └── dto/
│       ├── PlcEdgeDataDTO.java        — 边端数据上报DTO
│       ├── PlcEdgeCommandDTO.java     — 边端指令DTO
│       ├── PlcEdgeResponseDTO.java    — 边端响应DTO
│       └── PlcWriteResultDTO.java     — 写入结果DTO
├── communication/
│   └── S7PlcConnector.java            — Moka7 S7协议连接器
├── service/
│   ├── IPlcDeviceService.java
│   ├── IPlcSignalPointService.java
│   ├── IPlcMonitorService.java        — 实时监控服务
│   ├── IPlcRealtimeService.java       — 实时数据服务
│   ├── IPlcConfigService.java         — 配置服务
│   ├── IPlcEdgeServerService.java     — 边端管理
│   ├── IPlcEdgeDataService.java       — 边端数据处理
│   └── impl/ ...
└── websocket/
    └── PlcDataWebSocket.java          — PLC数据WebSocket推送
```

### 7.2 PLC通信架构图

```mermaid
flowchart LR
    subgraph "IGH-Manager-Sys 服务端"
        A[PlcMonitorController] --> B[PlcMonitorService]
        B --> C{通讯模式?}
        C -->|LOCAL本地直连| D[S7PlcConnector<br/>Moka7 S7协议]
        C -->|EDGE边端代理| E[PlcEdgeDataService<br/>REST API调用]
        D --> F[PlcDataSnapshot<br/>数据快照存储]
        E --> F
        F --> G[PlcDataWebSocket<br/>前端推送]
        H[PlcPollingConfig] --> I[Quartz定时轮询]
        I --> B
    end

    subgraph "工业现场"
        J[西门子 S7-1500 PLC<br/>192.168.1.100:102]
        K[边端网关<br/>bianduan/igh-edge]
    end

    D -->|TCP:102<br/>S7协议| J
    E -->|HTTP REST| K
    K -->|S7协议| J
```

### 7.3 S7通信流程图

```mermaid
flowchart TD
    A[开始轮询] --> B[加载轮询配置<br/>PlcPollingConfig]
    B --> C[获取设备信息<br/>PlcDevice: IP/Rack/Slot]
    C --> D{通讯模式?}
    D -->|LOCAL| E[S7PlcConnector.connect<br/>Moka7建立TCP连接]
    E --> F[加载信号点列表<br/>按point_group分组]
    F --> G[批量读取数据块<br/>ReadArea DB块]
    G --> H[解析数据类型<br/>BOOL/WORD/REAL等]
    H --> I[创建数据快照<br/>PlcDataSnapshot]
    I --> J[WebSocket推送<br/>前端实时显示]
    D -->|EDGE| K[HTTP POST到边端网关<br/>PlcEdgeDataDTO]
    K --> L[边端执行S7读取]
    L --> M[返回数据<br/>PlcEdgeResponseDTO]
    M --> I
    J --> N[更新通讯日志<br/>PlcCommLog]
    N --> O{轮询间隔等待<br/>默认2秒}
    O --> A
```

### 7.4 PLC API端点

| HTTP方法 | 路径 | 说明 |
|---------|------|------|
| GET | /plc/device/list | PLC设备列表 |
| POST | /plc/device | 新增PLC设备 |
| GET | /plc/signal/list | 信号点列表 |
| POST | /plc/signal | 新增信号点 |
| GET | /plc/polling/list | 轮询配置列表 |
| GET | /plc/monitor/realtime/{deviceId} | 实时数据 |
| GET | /plc/monitor/snapshot/list | 历史快照 |
| POST | /plc/monitor/write | 写入PLC信号 |
| GET | /plc/config/list | PLC系统配置 |
| GET | /plc/edge/server/list | 边端服务列表 |
| POST | /plc/edge/data/report | 边端数据上报 |
| POST | /plc/edge/command | 下发边端指令 |

---

## 8. 打包管理模块（igh-packaging）

### 8.1 模块结构

```
com.igh.packaging/
├── controller/
│   ├── PackageSpecController.java     — 包装规格管理
│   ├── PackingTaskController.java     — 打包任务管理
│   ├── PalletController.java          — 托盘管理
│   └── PalletPackingController.java   — 智能组盘API
├── domain/
│   ├── PackageSpec.java               — 包装规格
│   ├── PackingTask.java               — 打包任务
│   └── Pallet.java                    — 托盘
├── mapper/
│   ├── PackageSpecMapper.java
│   ├── PackingTaskMapper.java
│   └── PalletMapper.java
└── service/
    ├── IPackageSpecService.java
    ├── IPackingTaskService.java
    ├── IPalletService.java
    ├── IPalletPackingService.java     — 智能组盘服务
    └── impl/
        ├── PackageSpecServiceImpl.java
        ├── PackingTaskServiceImpl.java
        ├── PalletServiceImpl.java
        └── PalletPackingServiceImpl.java
```

### 8.2 打包业务流程

```mermaid
flowchart TD
    A[定义包装规格<br/>PackageSpec] --> B[创建打包任务<br/>PackingTask]
    B --> C[关联批次和规格]
    C --> D[选择待包装丝饼<br/>bobbin_status=PRODUCED]
    D --> E{组盘方式}
    E -->|自动| F[PalletPackingService<br/>智能组盘算法]
    E -->|手动| G[人工扫码组盘]
    F --> H[创建托盘<br/>Pallet: BUILDING]
    G --> H
    H --> I[绑定丝饼到托盘<br/>pkg_pallet_bobbin]
    I --> J[计算重量<br/>毛重/净重/皮重]
    J --> K[封箱<br/>pallet_status=SEALED]
    K --> L[打印标签<br/>igh-label模块]
    L --> M[更新丝饼状态<br/>bobbin_status=PACKED]
    M --> N[完成打包任务]
    N --> O[托盘可入库<br/>→仓库管理]
```

### 8.3 包装规格实体

**PackageSpec (包装规格)**:
- spec_code, spec_name — 规格编码/名称
- bobbin_per_layer — 每层丝饼数
- layer_count — 层数
- total_bobbin_count — 总丝饼数 = 每层 × 层数
- pallet_type, pallet_size — 托盘类型/尺寸

---

## 9. 质量检测模块（igh-quality）

### 9.1 模块结构

```
com.igh.quality/
├── controller/
│   ├── QcInspectionTaskController.java   — 质检任务
│   ├── QcInspectionDetailController.java — 质检明细
│   ├── QcDefectRecordController.java     — 缺陷记录
│   ├── QcDefectTypeController.java       — 缺陷类型
│   └── QcReportController.java           — 质量报表
├── domain/
│   ├── QcInspectionTask.java
│   ├── QcInspectionDetail.java
│   ├── QcDefectRecord.java
│   └── QcDefectType.java
├── mapper/ ...
└── service/
    ├── IQcInspectionTaskService.java
    ├── IQcInspectionDetailService.java
    ├── IQcDefectRecordService.java
    ├── IQcDefectTypeService.java
    ├── IQcReportService.java             — 质量分析报表
    └── impl/ ...
```

### 9.2 质检流程图

```mermaid
flowchart TD
    A[创建质检任务<br/>QcInspectionTask] --> B{任务类型}
    B -->|SAMPLING 抽检| C[确定抽样规则<br/>sample_size/sample_rule]
    B -->|FULL 全检| D[全部丝饼]
    C --> E[分配质检员<br/>assigned_to]
    D --> E
    E --> F[开始质检<br/>task_status=IN_PROGRESS]
    F --> G[逐个检测丝饼<br/>QcInspectionDetail]
    G --> H[测量数据<br/>重量/长度/断头次数]
    H --> I{是否有缺陷?}
    I -->|是| J[记录缺陷<br/>QcDefectRecord]
    J --> K[选择缺陷类型<br/>毛丝/油污/色差/强度不足...]
    K --> L[确定处置方式<br/>ACCEPT/REWORK/SCRAP/DOWNGRADE]
    I -->|否| M[标记合格<br/>PASS]
    L --> N[评定质量等级<br/>SPECIAL/FIRST/SECOND/DEFECT]
    M --> N
    N --> O{全部检完?}
    O -->|否| G
    O -->|是| P[计算合格率<br/>pass_rate]
    P --> Q[确定检验结论<br/>PASS/REJECT/PARTIAL]
    Q --> R[完成质检任务<br/>COMPLETED]
    R --> S[更新丝饼质量等级]
```

### 9.3 预置缺陷类型

| 代码 | 名称 | 分类 | 严重度 | 默认处置 |
|-----|------|------|-------|---------|
| APP001 | 毛丝 | 外观 | 次要 | 降级 |
| APP002 | 油污 | 外观 | 次要 | 返工 |
| APP003 | 色差 | 外观 | 主要 | 降级 |
| APP004 | 纸管破损 | 外观 | 主要 | 报废 |
| APP005 | 脱筒 | 外观 | 严重 | 报废 |
| APP006 | 塌边 | 外观 | 主要 | 降级 |
| PHY001 | 毛重不足 | 物理 | 次要 | 降级 |
| PHY003 | 长度不足 | 物理 | 主要 | 降级 |
| PHY004 | 断头超标 | 物理 | 主要 | 降级 |
| PER001 | 强度不足 | 性能 | 严重 | 报废 |
| PER002 | 伸长不合格 | 性能 | 主要 | 降级 |
| PER003 | 热收缩不合格 | 性能 | 主要 | 降级 |

---

## 10. 仓库管理模块（igh-warehouse）

### 10.1 模块结构

```
com.igh.warehouse/
├── controller/
│   ├── WhWarehouseController.java      — 仓库管理
│   ├── WhLocationController.java       — 库位管理
│   ├── WhInventoryController.java      — 库存管理
│   ├── WhInboundOrderController.java   — 入库管理
│   └── WhOutboundOrderController.java  — 出库管理
├── domain/
│   ├── WhWarehouse.java                — 仓库
│   ├── WhLocation.java                 — 库位
│   ├── WhInventory.java                — 库存
│   ├── WhInventoryAlert.java           — 库存预警
│   ├── WhInboundOrder.java             — 入库单
│   ├── WhInboundDetail.java            — 入库明细
│   ├── WhOutboundOrder.java            — 出库单
│   └── WhOutboundDetail.java           — 出库明细
├── mapper/ ...
└── service/
    ├── IWhWarehouseService.java
    ├── IWhLocationService.java
    ├── IWhInventoryService.java
    ├── IWhInboundOrderService.java
    ├── IWhOutboundOrderService.java
    ├── IWhInventoryAlertService.java   — 库存预警
    ├── IWhLocationAllocationService.java — 库位分配策略
    ├── IWhPickingStrategyService.java   — 拣货策略
    └── impl/ ...
```

### 10.2 入库出库流程

```mermaid
flowchart LR
    subgraph "入库流程"
        A1[创建入库单<br/>待入库] --> A2[选择目标仓库]
        A2 --> A3[自动分配库位<br/>LocationAllocation]
        A3 --> A4[扫描托盘入库]
        A4 --> A5[更新库存<br/>WhInventory]
        A5 --> A6[更新库位状态<br/>空闲→占用]
        A6 --> A7[更新丝饼状态<br/>PACKED→IN_STOCK]
        A7 --> A8[完成入库单]
    end

    subgraph "出库流程"
        B1[创建出库单<br/>待拣货] --> B2[拣货策略<br/>PickingStrategy]
        B2 --> B3[FIFO/按等级<br/>推荐拣货库位]
        B3 --> B4[扫描托盘出库]
        B4 --> B5[减少库存]
        B5 --> B6[释放库位<br/>占用→空闲]
        B6 --> B7[更新丝饼状态<br/>IN_STOCK→SHIPPED]
        B7 --> B8[完成出库单]
    end
```

### 10.3 仓库智能服务

- **WhLocationAllocationService** — 库位自动分配策略：根据仓库容量、区域、承重等条件智能推荐库位
- **WhPickingStrategyService** — 拣货策略服务：支持FIFO(先进先出)、按质量等级、按批次等多种拣货策略
- **WhInventoryAlertService** — 库存预警服务：监控库存水平，触发预警

---

## 11. 设备管理模块（igh-equipment）

### 11.1 模块结构

```
com.igh.equipment/
├── controller/
│   ├── EquipmentDeviceController.java         — 设备档案
│   ├── EquipmentDeviceTypeController.java     — 设备类型
│   ├── EquipmentDevicePartController.java     — 设备部件
│   ├── EquipmentPartInstanceController.java   — 部件实例
│   ├── EquipmentMaintenancePlanController.java — 维护计划
│   ├── EquipmentMaintenanceRecordController.java — 维护记录
│   ├── EquipmentWinderSpecController.java     — 卷绕机规格
│   ├── EquipmentDoffingSpecController.java    — 落丝机规格
│   ├── EquipmentCraneSpecController.java      — 天车规格
│   ├── EquipmentTrolleySpecController.java    — 丝车规格
│   ├── EquipmentWeighingSpecController.java   — 称重设备规格
│   ├── EquipmentPrinterSpecController.java    — 打印机规格
│   ├── EquipmentLabelingSpecController.java   — 贴标机规格
│   ├── EquipmentMaterialRackSpecController.java — 料架规格
│   └── WinderMonitorController.java           — 卷绕机实时监控
├── domain/
│   ├── EquipmentDevice.java         — 设备主表
│   ├── EquipmentDeviceType.java     — 设备类型
│   ├── EquipmentDevicePart.java     — 设备部件模板
│   ├── EquipmentPartInstance.java   — 部件实例
│   ├── EquipmentMaintenancePlan.java — 维保计划
│   ├── EquipmentMaintenanceRecord.java — 维保记录
│   ├── EquipmentWinderSpec.java     — 卷绕机规格参数
│   ├── EquipmentDoffingSpec.java    — 落丝机规格
│   ├── EquipmentCraneSpec.java      — 天车规格
│   ├── EquipmentTrolleySpec.java    — 丝车规格
│   ├── EquipmentWeighingSpec.java   — 称重设备
│   ├── EquipmentPrinterSpec.java    — 打印机规格
│   ├── EquipmentLabelingSpec.java   — 贴标机规格
│   └── EquipmentMaterialRackSpec.java — 料架规格
```

### 11.2 设备类型层次

```mermaid
graph TD
    DEV[EquipmentDevice<br/>设备主表] --> TYPE[EquipmentDeviceType<br/>设备类型]
    DEV --> PART[EquipmentDevicePart<br/>部件模板]
    PART --> INST[EquipmentPartInstance<br/>部件实例]
    DEV --> PLAN[MaintenancePlan<br/>维保计划]
    PLAN --> REC[MaintenanceRecord<br/>维保记录]

    TYPE --> W[WinderSpec<br/>卷绕机]
    TYPE --> D[DoffingSpec<br/>落丝机]
    TYPE --> CR[CraneSpec<br/>天车]
    TYPE --> TR[TrolleySpec<br/>丝车]
    TYPE --> WE[WeighingSpec<br/>称重设备]
    TYPE --> PR[PrinterSpec<br/>打印机]
    TYPE --> LA[LabelingSpec<br/>贴标机]
    TYPE --> MR[MaterialRackSpec<br/>料架]
```

### 11.3 WinderMonitorService

`WinderMonitorController` + `WinderMonitorServiceImpl` 提供卷绕机实时监控功能，通过WebSocket将PLC数据推送到前端，实现主轴转速、温度、运行状态的实时显示。

---

## 12. 标签打印模块（igh-label）

### 12.1 模块结构

```
com.igh.label/
├── controller/
│   ├── LabelTemplateController.java       — 标签模板管理
│   ├── LabelPrinterController.java        — 打印机管理
│   ├── LabelPrintTaskController.java      — 打印任务管理
│   └── LabelFieldMetadataController.java  — 字段元数据
├── domain/
│   ├── LabelTemplate.java    — 标签模板(含ZPL/TSPL模板代码)
│   ├── LabelPrinter.java     — 打印机(IP/端口/型号)
│   ├── LabelPrintTask.java   — 打印任务(队列)
│   └── LabelFieldMetadata.java — 字段元数据(模板字段定义)
└── service/
    ├── ILabelTemplateService.java
    ├── ILabelPrinterService.java
    ├── ILabelPrintTaskService.java
    └── ILabelFieldMetadataService.java
```

### 12.2 标签打印流程

```mermaid
flowchart TD
    A[设计标签模板<br/>LabelTemplate] --> B[配置模板字段<br/>LabelFieldMetadata]
    B --> C[注册打印机<br/>LabelPrinter: IP/Port]
    C --> D[创建打印任务<br/>LabelPrintTask]
    D --> E[绑定丝饼/托盘数据]
    E --> F[渲染模板<br/>填充字段值]
    F --> G[发送到打印机<br/>TCP网络打印]
    G --> H{打印结果}
    H -->|成功| I[更新状态SUCCESS]
    H -->|失败| J[记录错误<br/>重试或标记FAILED]
```

---

## 13. 边端集成模块（igh-edge + bianduan）

### 13.1 igh-edge 模块结构（服务端接收API）

```
com.igh.edge/
├── controller/
│   ├── EdgeHeartbeatController.java      — 心跳检测API
│   ├── EdgeConfigController.java         — 边端配置下发
│   ├── EdgeDoffingController.java        — 落丝数据上报
│   ├── EdgeBobbinController.java         — 丝饼数据上报
│   ├── EdgeShiftController.java          — 班次数据管理
│   ├── EdgeRackStatusController.java     — 存置架状态上报
│   ├── EdgePrintRecordController.java    — 打印记录上报
│   └── EdgeTransferEventController.java  — 丝饼流转事件
├── domain/
│   ├── ProdShift.java          — 班次记录(edge_id关联)
│   ├── ProdStorageRack.java    — 存置架状态(upsert模式)
│   ├── LabelPrintRecord.java   — 边端打印记录
│   └── ProdTransferEvent.java  — 丝饼流转事件
└── service/
    ├── IEdgeHeartbeatService.java
    ├── IEdgeConfigService.java
    ├── IEdgeDoffingService.java
    ├── IEdgeBobbinService.java
    ├── IEdgeShiftService.java
    ├── IEdgeRackService.java
    ├── IEdgePrintRecordService.java
    └── IEdgeTransferEventService.java
```

### 13.2 bianduan 目录

`bianduan/` 目录下仅包含设计文档(.md文件)，无实际可执行源码。边端应用为独立部署的Spring Boot应用，代码不在此仓库中。

### 13.3 边端通信架构

```mermaid
flowchart TB
    subgraph "车间现场 (Edge)"
        PLC1[西门子 PLC<br/>卷绕区]
        PLC2[西门子 PLC<br/>包装区]
        EDGE_APP[边端网关应用<br/>bianduan/igh-edge<br/>独立Spring Boot]
        PRINTER[标签打印机]
        SCANNER[扫码枪]

        PLC1 -->|S7协议| EDGE_APP
        PLC2 -->|S7协议| EDGE_APP
        EDGE_APP -->|网络打印| PRINTER
        SCANNER -->|USB| EDGE_APP
    end

    subgraph "IGH-Manager-Sys (Cloud/Server)"
        HEART[EdgeHeartbeatController<br/>心跳检测]
        DOFF[EdgeDoffingController<br/>落丝上报]
        BOB[EdgeBobbinController<br/>丝饼上报]
        SHIFT[EdgeShiftController<br/>班次管理]
        RACK[EdgeRackStatusController<br/>存置架状态]
        PRINT[EdgePrintRecordController<br/>打印记录]
        TRANS[EdgeTransferEventController<br/>流转事件]
        CFG[EdgeConfigController<br/>配置下发]
    end

    EDGE_APP -->|POST /edge/heartbeat| HEART
    EDGE_APP -->|POST /edge/doffing| DOFF
    EDGE_APP -->|POST /edge/bobbin| BOB
    EDGE_APP -->|POST /edge/shift| SHIFT
    EDGE_APP -->|POST /edge/rack| RACK
    EDGE_APP -->|POST /edge/print| PRINT
    EDGE_APP -->|POST /edge/transfer| TRANS
    CFG -->|GET /edge/config| EDGE_APP
```

### 13.4 存置架模型

系统支持三种存置架类型，每个架有左右两侧:
- **TEMP_1** — 临时架1
- **TEMP_2** — 临时架2
- **FULL** — 凑满架

每个架位容量为18个丝饼，采用upsert模式更新(UNIQUE约束: edge_id + rack_type + side + rack_number)。

---

## 14. 配置中心模块（igh-config）

### 14.1 模块结构

```
com.igh.config/
├── controller/
│   ├── SysTenantController.java         — 租户管理
│   └── SysTenantModuleController.java   — 租户模块配置
├── domain/
│   ├── SysTenant.java                   — 租户/工厂
│   ├── SysTenantModule.java             — 租户模块配置
│   ├── SysModuleRegistry.java           — 模块注册表
│   ├── SysDeviceType.java               — 设备类型注册
│   └── SysDeviceConfig.java             — 设备配置
├── mapper/
│   ├── SysTenantMapper.java
│   └── SysTenantModuleMapper.java
└── service/
    ├── ISysTenantService.java
    ├── ISysTenantModuleService.java
    └── impl/ ...
```

### 14.2 多租户模块化架构

```mermaid
flowchart TD
    TENANT[sys_tenant<br/>租户/工厂] --> MOD_CFG[sys_tenant_module<br/>启用的模块]
    TENANT --> DEV_CFG[sys_device_config<br/>设备配置]
    TENANT --> WF_CFG[sys_tenant_workflow<br/>工作流配置]
    TENANT --> HA[sys_ha_config<br/>高可用配置]

    REG[sys_module_registry<br/>模块注册表] --> MOD_CFG
    DEV_TYPE[sys_device_type<br/>设备类型注册] --> DEV_CFG
    WF_TPL[sys_workflow_template<br/>工作流模板] --> WF_CFG
```

**核心设计理念**: 通过配置中心实现工厂级别的功能定制——不同工厂可以启用不同的模块组合(生产+PLC为核心，质检/包装/仓储为可选)，配置不同的设备类型和工作流程。

**预置工作流模板**:
1. **FULL_AUTO_PACKAGING** — 全自动包装: PLC自动组盘→自动称重→自动打印
2. **SEMI_AUTO_PACKAGING** — 半自动包装: 扫码识别→人工组盘→PDA记录→打印
3. **QUALITY_WITH_SCAN** — 扫码质检: 扫描产品→质量检测→记录缺陷

---

## 15. 系统管理模块（igh-system）

见 [5.2 igh-system](#52-igh-system-系统管理)，提供标准RBAC权限管理体系。

---

## 16. 框架层（igh-framework）

见 [5.3 igh-framework](#53-igh-framework-框架层)，提供安全认证、AOP日志、缓存、WebSocket等基础框架能力。

---

## 17. 定时任务模块（igh-quartz）

基于Quartz框架的定时任务管理:

| 实体 | 表 | 功能 |
|-----|---|------|
| SysJob | sys_job | 定时任务定义(cron表达式/执行策略) |
| SysJobLog | sys_job_log | 任务执行日志 |

支持通过Web界面管理定时任务的创建、修改、暂停、恢复、立即执行。

---

## 18. 启动入口（igh-admin）

### 18.1 主类

- `IGHApplication` — Spring Boot启动类(@SpringBootApplication)
- `IGHServletInitializer` — WAR包部署支持

### 18.2 入口层Controller

| 控制器 | 路径前缀 | 功能 |
|-------|---------|------|
| SysLoginController | /login, /logout | 登录/登出/获取用户信息/路由 |
| SysRegisterController | /register | 用户注册 |
| CaptchaController | /captchaImage | 验证码生成 |
| CommonController | /common | 文件上传下载 |
| SysUserController | /system/user | 用户管理 |
| SysRoleController | /system/role | 角色管理 |
| SysMenuController | /system/menu | 菜单管理 |
| SysDeptController | /system/dept | 部门管理 |
| SysDictTypeController | /system/dict/type | 字典类型 |
| SysDictDataController | /system/dict/data | 字典数据 |
| SysConfigController | /system/config | 参数配置 |
| SysNoticeController | /system/notice | 通知公告 |
| SysProfileController | /system/user/profile | 个人信息 |
| CacheController | /monitor/cache | 缓存监控 |
| ServerController | /monitor/server | 服务器监控 |
| SysLogininforController | /monitor/logininfor | 登录日志 |
| SysOperlogController | /monitor/operlog | 操作日志 |
| SysUserOnlineController | /monitor/online | 在线用户 |
| SwaggerConfig | /swagger-ui/ | API文档 |

---

## 19. 全局数据流图

### 19.1 丝饼生命周期数据流

```mermaid
flowchart LR
    subgraph "生产阶段"
        A[卷绕机<br/>PLC信号] -->|S7协议<br/>实时采集| B[PLC模块<br/>数据快照]
        B -->|触发落丝| C[生产模块<br/>创建Doffing]
        C -->|生成丝饼| D[ProdBobbin<br/>status=PRODUCED]
    end

    subgraph "质检阶段"
        D -->|抽检/全检| E[质量模块<br/>QcInspectionTask]
        E -->|检测结果| F{合格?}
        F -->|合格| G[更新质量等级<br/>A/B/C/D]
        F -->|不合格| H[缺陷记录<br/>处置:返工/报废/降级]
        H -->|报废| I[REJECTED]
    end

    subgraph "打包阶段"
        G -->|组盘| J[打包模块<br/>PackingTask]
        J -->|装入托盘| K[Pallet<br/>BUILDING→SEALED]
        K -->|打印标签| L[标签模块<br/>LabelPrintTask]
        L --> M[ProdBobbin<br/>status=PACKED]
    end

    subgraph "仓储阶段"
        M -->|入库| N[仓库模块<br/>WhInboundOrder]
        N -->|分配库位| O[WhInventory<br/>status=IN_STOCK]
        O -->|出库| P[WhOutboundOrder]
        P --> Q[ProdBobbin<br/>status=SHIPPED]
    end

    subgraph "边端上报"
        R[边端网关] -->|HTTP POST| S[Edge模块]
        S -->|落丝数据| C
        S -->|丝饼数据| D
        S -->|班次/存置架| T[边端记录表]
    end
```

### 19.2 系统间数据流

```mermaid
flowchart TB
    subgraph "数据采集层"
        PLC_DEV[西门子PLC<br/>S7-1200/1500]
        EDGE_GW[边端网关<br/>Spring Boot]
        SCANNER[扫码枪]
        SCALE[电子秤]
    end

    subgraph "业务处理层"
        PROD[生产管理<br/>批次→落丝→丝饼]
        PLC_MOD[PLC通信<br/>信号采集/轮询]
        QC[质量检测<br/>抽检/全检]
        PKG[打包管理<br/>组盘/封箱]
        WH[仓库管理<br/>入库/出库]
        LBL[标签打印<br/>模板/打印]
    end

    subgraph "数据存储层"
        PG[(PostgreSQL<br/>业务数据)]
        REDIS[(Redis<br/>缓存/会话)]
    end

    subgraph "展示层"
        WEB[Vue.js 前端<br/>管理后台]
        WS[WebSocket<br/>实时监控]
    end

    PLC_DEV -->|S7协议| PLC_MOD
    PLC_DEV -->|S7协议| EDGE_GW
    EDGE_GW -->|REST API| PROD
    SCANNER -->|扫码| PKG & QC
    SCALE -->|称重| PKG

    PLC_MOD --> PROD
    PROD --> QC & PKG
    PKG --> WH
    PKG --> LBL
    QC --> PROD

    PROD & PLC_MOD & QC & PKG & WH & LBL --> PG
    PLC_MOD -->|实时数据| WS
    WEB -->|REST API| PROD & QC & PKG & WH
    PG --> WEB
    REDIS --> WEB
```

---

## 20. 端到端业务流程图

### 20.1 化纤生产全流程

```mermaid
flowchart TD
    START((开始)) --> LOT[1. 创建生产批次<br/>指定产品/卷绕机/计划数量]

    LOT --> WIND[2. 卷绕机开始生产<br/>PLC实时监控转速/温度]

    WIND --> DOFF{3. 落丝触发<br/>满筒/断丝/手动?}
    DOFF --> REC[4. 记录落丝事件<br/>PLC数据快照]
    REC --> BOB[5. 生成丝饼记录<br/>条码BOBxxxxxxx]

    BOB --> LABEL1[6. 打印丝饼标签<br/>模板渲染+网络打印]

    LABEL1 --> RACK[7. 放入存置架<br/>临时架→凑满架]

    RACK --> QC{8. 质量检测<br/>抽检或全检}
    QC -->|合格A/B| GRADE_OK[9a. 标记等级<br/>合格]
    QC -->|不合格C/D| GRADE_NG[9b. 缺陷处理<br/>返工/报废/降级]

    GRADE_OK --> PACK[10. 组盘打包<br/>按规格码放]
    GRADE_NG -->|降级| PACK
    GRADE_NG -->|报废| SCRAP[废品处理]

    PACK --> WEIGH[11. 称重<br/>毛重/净重]
    WEIGH --> SEAL[12. 封箱<br/>托盘状态SEALED]
    SEAL --> LABEL2[13. 打印托盘标签<br/>含批次/重量/数量]

    LABEL2 --> INBOUND[14. 入库<br/>分配库位]
    INBOUND --> STOCK[15. 在库管理<br/>库存盘点/预警]

    STOCK --> OUTBOUND[16. 出库<br/>拣货→发货]
    OUTBOUND --> SHIP[17. 发货<br/>更新状态SHIPPED]

    SHIP --> END((结束))

    %% 并行边端上报
    REC -.->|边端上报| EDGE[边端网关<br/>班次/存置架/流转]
    LABEL1 -.->|边端上报| EDGE
```

---

## 21. API端点清单

### 21.1 各模块API总览

| 模块 | Controller数 | 主要路径前缀 | 功能 |
|------|------------|------------|------|
| igh-admin (系统) | 17 | /system/*, /monitor/*, /login | 系统管理+监控 |
| igh-production | 3 | /production/lot, /doffing, /bobbin | 生产管理 |
| igh-plc | 7 | /plc/device, /signal, /monitor, /edge | PLC通信 |
| igh-packaging | 4 | /packaging/spec, /task, /pallet | 打包管理 |
| igh-quality | 5 | /quality/inspection, /defect, /report | 质量检测 |
| igh-warehouse | 5 | /warehouse/*, /location, /inventory | 仓库管理 |
| igh-equipment | 15 | /equipment/device, /winder, /crane, ... | 设备管理 |
| igh-label | 4 | /label/template, /printer, /task | 标签打印 |
| igh-edge | 8 | /edge/heartbeat, /doffing, /bobbin, ... | 边端集成 |
| igh-config | 2 | /config/tenant, /module | 配置中心 |
| igh-quartz | 2 | /monitor/job, /jobLog | 定时任务 |
| igh-generator | 1 | /tool/gen | 代码生成 |
| **合计** | **73** | | |

### 21.2 每个Controller标准操作

大多数业务Controller遵循统一的CRUD模式:
- `GET /xxx/list` — 分页查询列表
- `GET /xxx/{id}` — 根据ID查询详情
- `POST /xxx` — 新增
- `PUT /xxx` — 修改
- `DELETE /xxx/{ids}` — 删除(支持批量)
- `POST /xxx/export` — 导出Excel

---

## 22. 技术栈总结

### 22.1 后端技术栈

| 类别 | 技术 | 版本 | 用途 |
|------|-----|------|------|
| 运行时 | Java | 1.8 | 编译目标 |
| 框架 | Spring Boot | 2.5.15 | 应用框架 |
| 安全 | Spring Security | 5.7.14 | 认证授权 |
| ORM | MyBatis | (PageHelper 1.4.7) | 数据库访问 |
| 数据库 | PostgreSQL | 12+ | 主数据库 |
| 连接池 | Druid | 1.2.23 | 数据库连接管理 |
| 缓存 | Redis + Lettuce | - | 会话缓存 |
| 认证 | JWT (jjwt) | 0.9.1 | Token认证 |
| JSON | FastJSON2 | 2.0.58 | JSON序列化 |
| API文档 | SpringFox Swagger3 | 3.0.0 | API文档 |
| Excel | Apache POI | 4.1.2 | Excel导入导出 |
| 定时任务 | Quartz | - | 定时调度 |
| 模板引擎 | Velocity | 2.3 | 代码生成 |
| PLC通信 | **Moka7** | 1.0.3 | 西门子S7协议(本地安装) |
| 验证码 | Kaptcha | 2.3.3 | 登录验证码 |
| 系统信息 | OSHI | 6.8.3 | 服务器监控 |

### 22.2 数据库设计特点

1. **PostgreSQL特性充分利用**: JSONB(PLC数据快照/配置profile), BIGSERIAL自增, CHECK约束, 建议性分区
2. **编号序列**: seq_lot_code, seq_doffing_code, seq_bobbin_code — 独立序列生成业务编号
3. **审计字段**: 所有表统一包含 create_by, create_time, update_by, update_time
4. **软删除**: 部分表使用 del_flag/deleted/status 标记删除
5. **索引策略**: 按查询场景建立复合索引，高频时序表建议按月分区

### 22.3 架构设计亮点

1. **严格分层**: common→system→framework→业务模块，无循环依赖
2. **业务枢纽**: igh-production 是数据中心，被4个上层模块依赖
3. **双模通信**: PLC通信支持LOCAL(直连)和EDGE(边端代理)两种模式
4. **多租户就绪**: 配置中心支持租户级模块开关和工作流定制
5. **工业协议**: 通过Moka7本地库实现原生S7协议通信，非通用MODBUS
6. **边云协同**: 车间边端网关独立运行，通过REST API上报数据到云端

---

> 分析完成。本报告基于588个Java源码文件、67个MyBatis Mapper XML、60+个SQL脚本的纯源码级分析。前端Vue.js代码存放在独立仓库中，不在本次分析范围内。
