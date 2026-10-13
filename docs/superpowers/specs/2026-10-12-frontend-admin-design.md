# Center Web 管理后台设计

> **文档编号:** DESIGN-2026-10-12-001
> **日期:** 2026-10-12
> **状态:** 待评审
> **前置:** `2026-09-29-v3-data-model-design.md`（实体归属权威）、`2026-10-08-sync-update-design.md`（同步语义与 Ruling 37）

> **For agentic workers:** 本文是设计文档，不是实施计划。经确认后由 `superpowers:writing-plans` 生成逐任务实施计划。

**目标：** 定义 Center 侧 Web 管理后台的页面、交互与所需后端接口，使其与已落地的数据模型和同步语义一致。

**技术栈：** 前端 React 18 + TypeScript + Vite + Ant Design 5 + React Router 6；后端复用现有 Go 1.26.4 / Ent v0.14.6 / Gin / PostgreSQL。

---

## 一、背景

### 1.1 现状

`docs/design/frontend-design.md` §2 描述的管理后台，是在 `Order` 实体仍然存在时写的。此后发生了三件事，使该章节整体失效：

1. **`Order` 实体已从 Schema 移除**（commit `a368678` 系列），§2.1 的「订单管理」菜单与 §2.2.1 整页失去对应实体。
2. **前端代码从未存在**——仓库内无 `package.json`、无 `.tsx`。§2 与 §3 均为设计稿，§2.1 的 `AdminLayout` 代码示例是示意。
3. **Center 的写接口被有意收敛**——`internal/server/center.go:187` 注释明确：「写接口不在 Center：批次由 Edge 创建并上传（数据归属单向）」。文档设想的常规 CRUD 后台与已确立的数据归属原则相悖。

### 1.2 文档与实现的逐项偏离

| 文档 §2 假设 | 实现现状 | 结论 |
|---|---|---|
| 「订单」为一层实体，批次归属订单 | `Lot.order_code` 是普通字符串字段，注释注明「ERP 集成用，非外键」 | 订单层不存在 |
| §2.2.1 订单管理页（订单号/产品/数量/进度/状态） | 上述五列与 `Lot` 的 `lot_number`/`product_type`+`product_spec`/`planned_quantity`/`actual_quantity`/`status` 一一对应 | 与 §2.2.2 是同一份数据 |
| §2.2.2 追溯链首层为「订单」 | 无 Order 实体 | 追溯链须改为单层起点 |
| 追溯链为 `丝锭→托盘→仓储` 单线 | `bobbin` 同时挂 `barrel_id`/`pallet_id`/`carton_id` 三个可选外键 | 包装形态是并列而非包含 |
| 库位体系 `1号立体库 01-02-03` | 无 warehouse 表、无 location 表；仅有 `Module.current_location` 字符串 | 库位模型不存在 |
| §2.1 顶级菜单三项：订单/质量/仓储 | §2.2 实写四页：订单/批次/仓储/报表 | 菜单与页面本就对不上 |
| 质检为独立页 | `BobbinGrade` 是挂在丝锭上的一次检验记录，无「质检单」实体 | 质检属批次详情与报表 |
| §2.2.4 FIFO 出库推荐 | 无 FIFO 端点；且依赖不存在的库位与库龄字段 | 不在本次范围 |

### 1.3 目标

1. 定义四页管理后台的完整页面结构、列定义、筛选器与交互，全部有后端字段依据。
2. 明确本设计所需的**写入面边界**——后台能改什么、不能改什么，以及为什么。
3. 输出后端缺口清单，作为后续开发的依据。
4. 补齐角色权限模型，使 `admin/operator/inspector/viewer` 四角色从装饰变为强约束。

### 1.4 非目标

- **不覆盖边端操作界面**（`frontend-design.md` §3 的落纱/质检/打包/入库扫描五页）。那是独立子系统：用户是操作工，部署在 Edge 工业平板，触摸交互。它与本设计仅通过同步协议耦合，另立 spec。
- 不新建库位/仓库模型（见 §5）。
- 不实现报表实时推送。
- 不改动现有同步协议。批次管理页完全只读，不产生 Center 侧的批次写入（见 §2.5）。

---

## 二、核心架构决策

### 2.1 数据归属单向，后台以只读为主

`2026-09-29-v3-data-model-design.md` 确立的原则是「Edge 产生的实体，Center 只读汇总」。因此本后台**不是常规 CRUD 后台**。每一个写入面都必须单独论证其不与 Edge 权威源冲突。

**判定标准：** 该写入是否会产生一份 Edge 不认得的 Center 侧数据？会 → 不做。

### 2.2 两个受控写入面

| 写入面 | 端点 | 允许角色 | 不冲突的理由 |
|---|---|---|---|
| 用户管理 | `/v1/users` 全套（已有） | `admin` | Center 自有实体，Edge 不参与 |
| 设备管理 | `/v1/edges` 注册 / 签发 Token（已有） | `admin` | 同上 |

**两个写入面都是 Center 自有实体。** 这就是它们不冲突的充分理由——它们不参与同步，Edge 没有对应记录，不存在覆盖问题。

### 2.3 明确不做

| 不做 | 理由 |
|---|---|
| 新建批次 | Edge 是批次权威源。Center 建批次后会与 Edge 同步的 `lot_number` 唯一约束相撞——`2026-10-08-sync-update-design.md` §1.5 已记录该缺陷路径 |
| 编辑批次业务字段（产品、数量、时间） | 同步以 Edge 为准（Ruling 37），Center 改动会被覆盖 |
| **修正批次状态** | `status` **是同步载荷中的字段**（`sync/center/handler.go` 的 `handleCreate` 与 `handleUpdate` 均写入它）。Center 手工改动后，Edge 下一次上传会带着 Edge 侧的值覆盖回来——该按钮点了不生效。见 §2.5 |
| 删除批次 | `lot_id` 在 `doffing`/`bobbin`/`barrel`/`pallet`/`carton` 上均为外键，级联影响不可逆 |
| 编辑丝锭任何字段 | `center.go:198` 注释：「丝饼的全部信息来自 Edge」 |

### 2.4 关联的既有技术债

`api/center/v1/lot.go` 中的 `CreateLot`、`UpdateLotStatus`、`DeleteLot` 三个方法**均未在 `center.go` 挂载**，属死代码。本设计要求删除这三个方法——它们的保留会让人误以为 Center 具备这些写入能力。

### 2.5 批次状态修正的裁定记录

本节记录一次被否决的设计，避免后续重复提出。

**曾被考虑：** 在批次详情页提供「状态修正」按钮，供 `admin`/`inspector` 处理 Edge 掉线导致的批次滞留（如长期停在 `in_progress`）。

**否决理由（两条，各自独立充分）：**

1. **功能上不生效。** `status` 在同步载荷内，Center 的写入会被 Edge 的下一次上传覆盖，除非引入 Center 优先的字段级覆盖规则。
2. **与既有裁定冲突。** `8ae036f`「refactor: 删除Center的批次/丝饼写路由，Edge补齐现场写入口」已删除 Center 的批次写路由，并追加 `TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 作为回归防线，明确断言 `PUT /v1/lots/:id/status` **不得存在**。本设计的早期草案未核对该测试的反向断言，与之正面冲突。

**结论：** 批次管理页完全只读。该测试与 `8ae036f` 的决定均保持不动。

**若现场确有运维介入需求**，正确方向不是在 Center 覆盖 `status`（那会推翻 Ruling 37 的「以 Edge 为准」），而是设计一个 Center 自有的独立标注字段或审计表，不参与同步。该方向**不在本设计范围内**，需另立设计。

### 2.6 只读边界的可执行约束

`TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 已把「Center 不得有批次/丝饼写路由」钉为断言。本设计的实现**必须保持该测试为绿**，且不得为通过而放宽其断言强度。

同理 `TestRealRouter_CenterKeepsReadRoutes` 保证只读端点不被误删。两个测试共同构成只读边界的回归防线。

---

## 三、批次管理

对应 `frontend-design.md` §2.2.1 + §2.2.2 的合并。原「订单管理」并入本页，`order_code` 降为可筛项与详情字段。

### 3.1 列表页

**列定义：**

| 列 | 字段来源 | 渲染 |
|---|---|---|
| 批次号 | `lot_number` | 链接至详情；等宽字体 |
| 产品 | `product_type` + `product_spec` | `FDY150D/48F` 形式拼接；`product_type` 为枚举 `FDY`/`POY`/`DTY` |
| 订单号 | `order_code` | 空值显示 `—`；`order_code` 可空 |
| 计划数量 | `planned_quantity` | 千分位 |
| 实际数量 | `actual_quantity` | 千分位 |
| 进度 | **`progress`（后端已计算）** | Ant Design `Progress`；`percent` 直接取该字段 |
| 状态 | `status` | Tag，见 §3.4 |
| 边端设备 | `edge_id` | 需前端按设备列表映射为名称 |
| 创建时间 | `created_at` | 排序基准 |

**进度不由前端计算。** `service/lot.go:175` 已输出 `progress = actual_quantity / planned_quantity * 100`。前端重算违反 DRY，且后端已处理 `planned_quantity` 为 0 的边界。

**筛选器：**

| 筛选 | 参数 | 后端现状 |
|---|---|---|
| 状态 | `status` | ✅ 已有 |
| 边端设备 | `edge_id` | ✅ 已有 |
| 产品类型 | `product_type` | ❌ 待加 |
| 订单号 | `order_code` | ❌ 待加 |
| 关键词 | `keyword`（模糊匹配 `lot_number`） | ❌ 待加 |

**排序：** 固定 `created_at` 倒序，表头不做可点排序。

理由：`ListLots` 硬编码 `Order(ent.Desc(lot.FieldCreatedAt))`，无排序入参。若前端仅对当前页排序，跨页顺序错误，是欺骗性交互。服务端排序延后至真实数据量证明其必要性时再补。

**分页：** 复用 `api.ParsePagination`，页大小上限 200，默认 20。

### 3.2 详情页

```
┌──────────────────────────────────────────────┐
│ 批次 FDY-2026-001-001                          │
├──────────────────────────────────────────────┤
│ 批次信息                                      │
│  批次号 / PLC批次号 / 订单号 / 产品 / 规格     │
│  计划数量 / 实际数量 / 进度 / 状态             │
│  边端设备 / 开始时间 / 结束时间 / 创建时间      │
├──────────────────────────────────────────────┤
│ 丝锭列表                          [编码搜索]   │
│  丝锭编码 │ 位置 │ 等级 │ 净重 │ 状态 │ 操作   │
├──────────────────────────────────────────────┤
│ 质检记录                                      │
│  丝锭 │ 维度 │ 等级 │ 是否合格 │ 检测时间 │ 检测员 │
├──────────────────────────────────────────────┤
│ 追溯链（单层起点）                             │
└──────────────────────────────────────────────┘
```

**丝锭列表数据源：** `GET /v1/bobbins?lot_id={id}` —— 该端点已支持 `lot_id` 过滤（`service/bobbin.go`），**无需新建**。

**丝锭编码搜索：** 前端客户端过滤。理由：单批次丝锭数量有限（`planned_quantity` 量级），服务端搜索属过度设计。

**质检记录数据源：** 经 `bobbin_grade` 表按 `bobbin_id` 关联。`BobbinGrade` 现有字段：`grade`、`inspection_details`、`scores`、`final_score`、`is_qualified`、`defect_reason`、`notes`、`inspected_at`、`inspector_id`。

`is_qualified` 为三态布尔（`schema/bobbingrade.go:75` 注释说明「未表态不再被断言为合格」），前端须区分 `true`/`false`/未设置三种展示。

**追溯链（修正后）：**

```
📊 批次: FDY-2026-001-001
    ↓
🏭 机台: M001 (L01-A侧)     ← doffing → spinning_line
    ↓
🎯 落纱: D001 (2026-09-25 10:15)  ← doffing，含 operator_id
    ↓
🧵 丝锭: FDY2026001A001-001
    ↓
✅ 质检: 5 维度                   ← bobbin_grade
    ↓
📦 包装: 桶/托盘/纸箱              ← barrel_id / pallet_id / carton_id 三选其一或并列
```

与原文档的差异：**移除「订单」首层**（实体不存在），**「包装」为并列形态**而非单线串联（一个丝锭可同时挂三种外键），**移除「仓储」层**（无实体）。

### 3.3 只读约束

**本页无任何写操作。** 不提供新建、编辑、删除、状态修正入口，与前文所述后端边界一致（§2.3、§2.5）。

页面上唯一可变的操作是筛选、翻页、搜索——它们只影响查询，不改变数据。

**为什么不做「状态修正」按钮：** 见 §2.5 的完整裁定记录与 `8ae036f` 提交。

### 3.4 状态枚举与 Tag 映射

| 值 | 中文 | Tag 色 |
|---|---|---|
| `in_progress` | 进行中 | `processing` |
| `paused` | 已暂停 | `warning` |
| `completed` | 已完成 | `success` |
| `cancelled` | 已取消 | `default` |

---

## 四、仓储管理

**本页的实质是包装单元查询。** 标题沿用「仓储管理」以保持菜单连续性，但页面内容如实反映模型——**不放库位相关字段**。

### 4.1 为什么不是原文档的库存查询页

原文档 §2.2.3 要「库位 `01-02-03`」「库龄 `5天`」「FIFO 推荐」，三者均无数据来源：

- **库位：** 无 warehouse/location 表。`Module.current_location` 是单个字符串字段，无结构、无编码规范。
- **库龄：** `Pallet`/`Carton` 有 `completed_at`，可算时间差，但原文档的「养丝期 ≥3天」是业务规则，无配置来源。
- **FIFO 推荐：** 需要库存台账与出库任务，二者均不存在。

### 4.2 四个 Tab

**Tab 1：落纱桶（`barrel`）**

| 列 | 字段 |
|---|---|
| 桶号 | `barrel_number` |
| 所属批次 | `lot_id` |
| 装载 | `current_count` / `capacity` |
| 状态 | `status` 枚举 `active`/`full`/`sorted`/`packed` |
| 装满时间 | `filled_at` |

**Tab 2：模组（`module`）**

| 列 | 字段 |
|---|---|
| 模组号 | `module_number` |
| 桶 1 / 桶 2 | `barrel1_id` / `barrel2_id` |
| 状态 | `status` 枚举 `loading`/`transporting`/`sorting`/`warehouse`/`idle` |
| RFID | `rfid` |
| 当前位置 | `current_location` ← **唯一真实存在的空间信息** |
| 边端设备 | `edge_id` |

**Tab 3：托盘（`pallet`）**

| 列 | 字段 |
|---|---|
| 托盘码 | `pallet_code` |
| 所属批次 | `lot_id` |
| 层级 | `level`（1–9） |
| 丝锭数 | `bobbins_count` |
| 重量 | `net_weight` / `gross_weight` / `tare_weight` |
| 状态 | `status` 枚举 `building`/`completed`/`shipped` |
| 标签 | `printed` / `printed_at` |
| 完成时间 | `completed_at` |

**Tab 4：纸箱（`carton`）**

| 列 | 字段 |
|---|---|
| 箱号 | `carton_number` |
| 所属批次 | `lot_id` |
| 丝锭数 | `bobbin_count` |
| 总重量 | `total_weight` |
| 状态 | `status` 枚举 `packing`/`packed`/`shipped` |
| 标签 | `label_printed` / `printed_at` |
| 装箱时间 | `packed_at` |

### 4.3 筛选

四表统一提供：`lot_id`、`status`。模组与托盘、纸箱另提供 `edge_id`。

**只读。** 包装单元全部由 Edge 产生，Center 不提供写入。

### 4.4 后端缺口

四张表**全部零端点**，需从零建立 `GET` 列表与详情接口，形态对齐 `ListBobbins`（分页 + 过滤 + `created_at` 倒序）。

---

## 五、报表

### 5.1 三个看板

**看板一：生产概览**

| 指标 | 数据源 |
|---|---|
| 批次数（按状态分组） | `lot.status` |
| 计划/实际产量 | `SUM(lot.planned_quantity)` / `SUM(lot.actual_quantity)` |
| 完成率 | `COUNT(status=completed) / COUNT(*)` |
| 按产品类型产量 | `lot.product_type` 分组 |
| 按边端设备产量 | `lot.edge_id` 分组 |

**看板二：质量分布**

| 指标 | 数据源 |
|---|---|
| 合格率 | `bobbin_grade.is_qualified`（**须排除未设置的三态**） |
| 等级分布 | `bobbin_grade.grade` 或 `bobbin.grade` |
| 五维度合格率 | `bobbin_grade.inspection_details` JSON 展开 |
| 缺陷原因 Top N | `bobbin_grade.defect_reason` |

**看板三：机台与操作员**

| 指标 | 数据源 |
|---|---|
| 按机台产量 | `doffing.spinning_line_id` 分组 |
| 按操作员落纱数 | `doffing.operator_id` 分组 |
| 按检测员工检数 | `bobbin_grade.inspector_id` 分组 |
| 平均分 | `AVG(bobbin_grade.final_score)` |

### 5.2 时间范围

全部聚合端点接受 `start` / `end`（ISO 8601），按下列字段过滤：

- 生产类指标 → `lot.created_at` 或 `doffing.doffing_time`
- 质量类指标 → `bobbin_grade.inspected_at`

缺省为最近 7 天。

### 5.3 明确不做实时推送

原文档 §4 设计 WebSocket + Zustand 实时刷新。报表页**不采用**：

- 用户是质检主管与调度员，看的是班后/日终结论，非盯屏看数字跳动。
- 现有推送通道仅 Edge↔Center 同步，向浏览器推送需新建通道。
- 实时性需求属于边端操作界面（`frontend-design.md` §3），不属于后台报表。

页面进入时拉取一次，提供手动刷新按钮。

### 5.4 后端缺口

零聚合端点。需为三看板各建一个聚合接口。

---

## 六、系统管理

### 6.1 用户 Tab

端点齐备（`/v1/users` 七个接口）。页面功能：列表（分页+按角色/状态筛选）、新建、编辑、改密、停用/启用。

**用户响应字段：** `username`、`full_name`、`role`、`employee_id`、`email`、`phone`、`is_active`、`last_login_at`。

### 6.2 设备 Tab

端点齐备（`/v1/edges` 四个接口）。页面功能：列表（分页+按状态筛选）、注册、查看详情、签发 Token。

| 列 | 字段 |
|---|---|
| 设备编码 | `edge_code` |
| 设备名称 | `edge_name` |
| IP | `ip_address` |
| 状态 | `status` 枚举 `online`/`offline`/`error`/`maintenance` |
| 版本 | `version` |
| 最后在线 | `last_seen` |

设备状态 Tag 映射：`online`→`success`，`offline`→`default`，`error`→`error`，`maintenance`→`warning`。

**签发 Token** 是敏感操作，需二次确认并显示 Token 有效期（`2026-09-30-phase6-sync-auth` 已定义 TTL）。

### 6.3 RBAC

**现状：零角色校验。** 经代码核实：

- `internal/server/center.go:269-287` 的 `requireUserPrincipal` 仅检查 `claims.DeviceID != ""`（挡设备凭证），**不检查角色**。
- 全仓库无任何 `role == "admin"` 形式的判断。
- `jwt.go:16` 签发了 `Roles []string`，但无人读取。
- `UserService.CreateUser`（`service/user.go:63`）直接 `SetRole(user.Role(req.Role))`，不校验调用者权限。
- `UpdateUserRequest` **没有 `role` 字段**，故无法通过更新接口改角色。

`center.go:167-171` 的注释自己承认该问题：「`POST /v1/users` 可以创建 `role=admin` 的账号（`UserService.CreateUser` 不校验 `role`）」。

**净效果：任何已登录用户（含 `viewer`）可创建 `admin` 账号并提权。**

**设计方案：**

1. **新增 `RequireRole(roles ...string)` 中间件**，读取 `claims.Roles`，不在允许列表则返回 `CodeForbidden`（20004）。挂载：

| 路由 | 允许角色 |
|---|---|
| `POST /v1/users` | `admin` |
| `PUT /v1/users/:id` | `admin` |
| `DELETE /v1/users/:id` | `admin` |
| `PUT /v1/users/:id/password` | `admin` 或本人 |
| `GET /v1/users`、`GET /v1/users/:id` | `admin` |
| `POST /v1/edges` | `admin` |
| `POST /v1/edges/:code/token` | `admin` |
| `GET /v1/edges`、`GET /v1/edges/:code` | `admin` |
| `GET /v1/lots`、`/v1/bobbins` 及报表 | 全部已登录角色 |

**注意表中无任何批次/丝饼写路由。** Center 的批次写路由已被 `8ae036f` 删除并有回归测试守护（§2.6），`RequireRole` 不得成为重新引入它们的理由。

2. **禁止越权提权**：`CreateUser` 校验「目标角色不得高于调用者角色」。角色高低序：`admin` > `inspector` > `operator` > `viewer`。
3. **`UpdateUserRequest` 增加 `role` 字段**，同一校验适用。
4. **前端同步按角色隐藏**，但安全性由后端保证——前端隐藏仅为体验优化。

**前端登录态携带角色：** 登录响应需返回用户角色，前端据此控制菜单与按钮可见性。角色应内嵌于 JWT 或登录响应体中。

---

## 七、整体布局

### 7.1 导航结构（修正后）

原文档顶级菜单「订单管理 / 质量管理 / 仓储管理」三项与页面清单不符。修正为四项，与页面一一对应：

```
Log / 批次管理 / 仓储管理 / 报表 / 系统管理
```

侧边栏展开二级：

```
📊 批次管理
📦 仓储管理
   ├─ 落纱桶
   ├─ 模组
   ├─ 托盘
   └─ 纸箱
📈 报表
   ├─ 生产概览
   ├─ 质量分布
   └─ 机台与操作员
⚙️ 系统管理
   ├─ 用户
   └─ 设备
```

「质量管理」不再作为顶级项（质检属批次详情与报表），「订单管理」已并入批次管理。

### 7.2 布局组件

沿用 `frontend-design.md` §2.1 的 `Layout` + `Sider` + `Header` + `Content` 结构，`Header` 置 Logo、当前页标题、用户下拉（含角色显示与登出）。

---

## 八、后端缺口清单

| # | 缺口 | 类型 | 关联页面 |
|---|---|---|---|
| 1 | `ListLots` 增加 `order_code`、`product_type`、`keyword` 入参 | 改现有 | 批次管理 |
| 2 | 删除 `LotHandler.CreateLot` / `UpdateLotStatus` / `DeleteLot` 死代码 | 清理 | — |
| 3 | 新增 `RequireRole` 中间件并挂载到管理路由 | 新建 | 系统管理 |
| 4 | `CreateUser` 增加越权提权校验 | 改现有 | 系统管理 |
| 5 | `UpdateUserRequest` 增加 `role` 字段及校验 | 改现有 | 系统管理 |
| 6 | 登录响应返回用户角色 | 改现有 | 全站 |
| 7 | 包装单元四表查询端点（列表 + 详情 × 4） | 新建 | 仓储管理 |
| 8 | 报表三个聚合端点（含 `start`/`end`） | 新建 | 报表 |

**本清单不含任何批次/丝饼写接口。** 早期草案曾含「挂载 `PUT /v1/lots/:id/status`」，经 §2.5 的裁定删除。

**缺口规模提示：** #7 与 #8 各为独立工作量，且 #7 需先确定四表的序列化形态（是否复用统一的 `PageResponse` 包装、嵌套关联是否展开）。建议 **#1–#6 作为一个批次交付**（它们服务批次管理与系统管理两页，改动集中且互相独立），#7/#8 各自另立计划。

---

## 九、待定问题

| # | 问题 | 影响 |
|---|---|---|
| 1 | 角色体系是否有现场依据？`operator`/`inspector` 是否真的对应岗位？若现场只有「管理员 / 其他人」两类，`RequireRole` 的粒度可简化 | 决定 §6.3 的复杂度 |
| 2 | 仓储四页是否需要按 `edge_id` 过滤？模组/托盘/纸箱有 `edge_id`，桶没有 | 决定缺口 #7 的接口形态 |
| 3 | 报表三个看板的指标是否需要与现场现有报表对齐？ | 决定缺口 #8 的指标定义 |
| 4 | 登录态如何携带角色——JWT 内嵌，还是登录响应单独返回？ | 决定缺口 #6 的实现方式 |

---

## 十、验收标准

1. **数据一致性：** 后台展示的批次进度与 `GET /v1/lots` 返回的 `progress` 字段完全一致，前端无独立计算逻辑。
2. **权限有效性：** 以 `viewer` 身份调用 `POST /v1/users` 返回 `20004`（`CodeForbidden`），而非创建成功。
3. **提权阻断：** 以 `inspector` 身份创建 `admin` 用户被拒绝。
4. **只读边界：** 后台界面不存在新建/编辑/删除批次或丝锭的入口；`TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 与 `TestRealRouter_CenterKeepsReadRoutes` 均为绿，且断言强度未被放宽。
5. **死代码清除：** `api/center/v1/lot.go` 中不再存在 `CreateLot`/`UpdateLotStatus`/`DeleteLot` 方法。
6. **枚举完整：** 所有状态 Tag 的取值均来自 §3.4、§4.2、§6.2 列出的枚举，无硬编码字符串。
7. **缺口可验证：** §8 清单中每一条在实现后可通过一次 HTTP 调用验证存在性。
