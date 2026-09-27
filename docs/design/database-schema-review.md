# 数据库表设计 - 自查与评审清单

> **评审对象:** database-schema-design.md v1.0 核心实体表设计
> **评审日期:** 2026-09-18
> **评审人:** 浮浮酱（初审），待主人终审

---

## 一、设计自查清单

### 1.1 与需求文档的一致性检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 是否覆盖需求文档9.1节ER图所有实体 | ✅ | orders/lots/modules/doffings/bobbins/grades/pallets/warehouse_locations 全部覆盖 |
| 是否支持四大功能区划分（生产/质检/立库/包装） | ✅ | modules表有zone_type字段，bobbins表有lifecycle_stage追踪 |
| 是否支持FDY/DTY两种生产模式差异 | ✅ | doffings表为FDY专用，DTY通过module直接关联bobbin |
| 是否支持5维等级体系 | ✅ | bobbin_grades表支持多种grade_type |
| 是否支持托盘码垛管理 | ✅ | pallets表 + bobbins.pallet_id外键 |
| 是否支持立库FIFO策略 | ⚠️ | **缺失入库时间索引优化，待补充** |
| 是否支持外部系统集成（ERP） | ✅ | 所有主表都有external_source + external_id |
| 是否支持软删除 | ✅ | 关键表都有deleted_at字段 |

### 1.2 数据完整性检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 主键定义完整 | ✅ | 所有表都使用UUID主键 |
| 外键关系正确 | ✅ | 已定义FK约束，PostgreSQL强制，SQLite应用层 |
| 唯一约束合理 | ✅ | 业务唯一标识（order_number/lot_number/bobbin_code等）都有UNIQUE |
| 非空约束合理 | ✅ | 关键业务字段都设置NOT NULL |
| 检查约束有效 | ✅ | 数量/重量/优先级等都有CHECK约束 |
| 级联删除策略 | ⚠️ | **未明确定义ON DELETE行为，待补充** |

### 1.3 索引设计检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 主键索引 | ✅ | 自动创建 |
| 唯一索引 | ✅ | 业务唯一标识都有UNIQUE INDEX |
| 外键索引 | ✅ | 所有外键字段都有索引 |
| 查询热点索引 | ✅ | status/时间字段/zone_type等都有索引 |
| 组合索引 | ⚠️ | **缺少常见查询场景的组合索引，待补充** |
| WHERE条件索引 | ✅ | 软删除使用WHERE deleted_at IS NULL |

### 1.4 性能优化检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 大表分区策略 | ❌ | **未考虑历史数据分区（如按月分区），待评估** |
| 冷热数据分离 | ❌ | **未考虑归档策略，待评估** |
| 索引数量合理 | ✅ | 每表3-8个索引，较合理 |
| JSONB字段使用 | ⚠️ | **未使用JSONB存储灵活字段，待评估是否需要** |

---

## 二、发现的问题与改进建议

### 2.1 严重问题（Must Fix）

#### 问题1：缺少级联删除策略定义
**当前状态：** 外键约束已定义，但未明确ON DELETE行为

**风险：**
- 删除订单时，批次、纱锭等子数据如何处理？
- 删除模组时，关联的落纱记录如何处理？

**改进建议：**
```sql
-- 订单 → 批次：级联软删除
ALTER TABLE lots ADD CONSTRAINT fk_lots_orders 
  FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE RESTRICT;

-- 批次 → 纱锭：禁止删除（必须先清空纱锭）
ALTER TABLE bobbins ADD CONSTRAINT fk_bobbins_lots 
  FOREIGN KEY (lot_id) REFERENCES lots(id) ON DELETE RESTRICT;

-- 托盘 → 纱锭：解除关联（纱锭归档，托盘可删除）
ALTER TABLE bobbins ADD CONSTRAINT fk_bobbins_pallets 
  FOREIGN KEY (pallet_id) REFERENCES pallets(id) ON DELETE SET NULL;
```

**建议策略：**
- 核心业务数据（订单/批次/纱锭）：`ON DELETE RESTRICT` 禁止物理删除，必须软删除
- 关联关系（托盘/仓位）：`ON DELETE SET NULL` 允许解除关联
- 配置数据（设备/用户）：`ON DELETE RESTRICT` 禁止删除被引用的记录

---

#### 问题2：立库FIFO查询性能优化不足
**当前状态：** warehouse_movements表有movement_time索引，但缺少FIFO专用索引

**风险：**
- 出库时查询"最早入库的托盘"效率低
- 可能需要全表扫描warehouse_movements

**改进建议：**
```sql
-- 针对FIFO出库查询的组合索引
CREATE INDEX idx_warehouse_movements_fifo 
  ON warehouse_movements(to_location_id, movement_time) 
  WHERE movement_type = 'IN' AND deleted_at IS NULL;

-- 或者在pallets表增加冗余字段
ALTER TABLE pallets ADD COLUMN warehoused_order INTEGER;  -- 入库顺序号
CREATE INDEX idx_pallets_fifo 
  ON pallets(warehouse_location_id, warehoused_order) 
  WHERE status = 'WAREHOUSED';
```

**建议策略：** 在pallets表增加warehoused_order字段（入库时自增），简化FIFO查询

---

### 2.2 中等问题（Should Fix）

#### 问题3：缺少常见查询场景的组合索引

**当前状态：** 只有单列索引，缺少多条件查询的组合索引

**常见查询场景分析：**
```sql
-- 场景1：查询某批次的所有纱锭，按生产时间排序
SELECT * FROM bobbins 
WHERE lot_id = ? AND deleted_at IS NULL 
ORDER BY produced_at;

-- 场景2：查询某区域某状态的模组
SELECT * FROM modules 
WHERE zone_type = ? AND status = ? AND deleted_at IS NULL;

-- 场景3：查询某托盘在某时间段的移动记录
SELECT * FROM warehouse_movements 
WHERE pallet_id = ? AND movement_time BETWEEN ? AND ?;
```

**改进建议：**
```sql
-- 针对纱锭查询优化
CREATE INDEX idx_bobbins_lot_produced 
  ON bobbins(lot_id, produced_at) 
  WHERE deleted_at IS NULL;

-- 针对模组查询优化
CREATE INDEX idx_modules_zone_status 
  ON modules(zone_type, status) 
  WHERE deleted_at IS NULL;

-- 移动记录时间范围查询（已有单列索引movement_time，暂可不加）
```

---

#### 问题4：缺少数据归档和分区策略

**当前状态：** 所有历史数据都在主表，未考虑长期增长

**风险：**
- warehouse_movements表增长速度快（每次出入库都记录）
- bobbins表会积累大量历史纱锭
- 查询性能随时间下降

**改进建议：**
```sql
-- 方案1：时间分区（PostgreSQL 10+支持）
CREATE TABLE warehouse_movements (
    ...
) PARTITION BY RANGE (movement_time);

CREATE TABLE warehouse_movements_2026_09 
  PARTITION OF warehouse_movements
  FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');

-- 方案2：定期归档到历史表
CREATE TABLE bobbins_archive (LIKE bobbins INCLUDING ALL);
-- 定期任务：将1年前的已出库纱锭归档
```

**建议策略：** 
- warehouse_movements：按月分区
- bobbins/pallets：暂不分区，观察1年数据量后决定
- 归档策略：通过配置定义保留期（如生产数据保留2年）

---

#### 问题5：未使用JSONB存储灵活扩展字段

**当前状态：** 所有字段都是固定列，扩展性有限

**场景：**
- 不同产品类型可能有不同的工艺参数
- 未来可能需要记录更多质检维度
- 设备可能有厂商特定的配置

**改进建议：**
```sql
-- 在关键表增加metadata字段
ALTER TABLE bobbins ADD COLUMN metadata jsonb;
CREATE INDEX idx_bobbins_metadata ON bobbins USING GIN (metadata);

-- 在lots表增加工艺参数
ALTER TABLE lots ADD COLUMN process_params jsonb;

-- 示例数据
-- bobbins.metadata: {"tension": 120, "speed": 3500, "temp": 285}
-- lots.process_params: {"winding_speed": 3500, "temp_zone1": 285}
```

**建议策略：** 在核心表适度增加1-2个jsonb字段用于扩展，但不要过度使用（保持结构化为主）

---

### 2.3 轻微问题（Nice to Have）

#### 问题6：审计字段created_by/updated_by可能为NULL

**当前状态：** 创建人/更新人字段允许NULL

**风险：** 系统级操作（如定时任务）会导致created_by为NULL，审计不完整

**改进建议：**
```sql
-- 增加系统用户（UUID固定值表示系统）
-- SYSTEM_USER_ID = '00000000-0000-0000-0000-000000000000'
ALTER TABLE orders ALTER COLUMN created_by SET NOT NULL;
ALTER TABLE orders ALTER COLUMN updated_by SET NOT NULL;
```

---

#### 问题7：时间字段精度统一性

**当前状态：** 混用timestamptz和date

**建议：** 统一使用timestamptz，需要日期时在应用层转换（便于时区处理）

---

## 三、需要与主人确认的设计决策

### 决策1：级联删除策略

**问题：** 当删除订单时，下游数据（批次/纱锭/托盘）如何处理？

**方案A（推荐）：** 全部使用软删除 + RESTRICT约束
- 应用层实现级联软删除（删除订单时，标记所有子记录deleted_at）
- 数据库层面禁止物理删除（ON DELETE RESTRICT）
- 优点：数据可追溯，可恢复
- 缺点：需要应用层代码保证一致性

**方案B：** 使用ON DELETE CASCADE
- 数据库自动级联物理删除
- 优点：实现简单
- 缺点：数据不可恢复，审计困难

**主人倾向哪个方案？**

---

### 决策2：立库FIFO实现方式

**问题：** 如何高效实现"先进先出"查询？

**方案A（推荐）：** pallets表增加warehoused_order自增字段
- 每次入库时分配递增序号
- 出库查询：`ORDER BY warehoused_order ASC LIMIT 1`
- 优点：查询快，逻辑清晰
- 缺点：需要增加字段和维护序号

**方案B：** 基于warehouse_movements.movement_time查询
- 出库时join查询最早的IN记录
- 优点：不增加字段
- 缺点：查询复杂，性能可能不佳

**主人倾向哪个方案？**

---

### 决策3：历史数据归档策略

**问题：** 什么时候开始考虑分区和归档？

**方案A：** 现在就规划分区表（warehouse_movements按月分区）
- 优点：提前规划，避免后期改造
- 缺点：初期数据量小，过度设计

**方案B（推荐）：** V1.0不分区，运行半年后根据数据量决定
- 优点：简单，避免过度设计
- 缺点：后期改造成本高

**方案C：** 只做定期归档（1年前数据归档到history表）
- 优点：平衡方案
- 缺点：需要定时任务和归档脚本

**主人倾向哪个方案？**

---

### 决策4：是否使用JSONB扩展字段

**问题：** 是否在核心表增加jsonb类型的metadata字段？

**方案A（推荐）：** 适度使用（每表最多1-2个jsonb字段）
- bobbins.metadata - 存储非标准质检数据
- lots.process_params - 存储工艺参数
- 优点：灵活扩展
- 缺点：查询和索引相对复杂

**方案B：** 不使用，完全结构化
- 优点：结构清晰，类型安全
- 缺点：扩展需要alter table

**主人倾向哪个方案？**

---

## 四、下一步行动

### 4.1 待主人评审确认

1. ✅ 核心实体表结构是否合理
2. ✅ 字段定义是否完整（类型/约束/默认值）
3. ✅ 索引设计是否满足查询需求
4. ❓ 级联删除策略选择（决策1）
5. ❓ 立库FIFO实现方式（决策2）
6. ❓ 归档策略选择（决策3）
7. ❓ 是否使用JSONB（决策4）

### 4.2 确认后的修正工作

1. 根据主人决策补充完善表定义
2. 添加缺失的组合索引
3. 明确级联删除约束
4. 补充FIFO相关字段和索引（如采用方案A）
5. 更新database-schema-design.md文档

### 4.3 完成后继续

- 设计边端设备管理表
- 设计OTA更新相关表
- 设计审计日志表
- 设计用户权限表

---

> **评审状态：** 等待主人审核和决策 🔍
> **发现问题：** 2个严重 + 4个中等 + 2个轻微
> **待决策项：** 4项关键设计决策
