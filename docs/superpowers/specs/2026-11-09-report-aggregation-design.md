# Center 报表聚合端点设计

> **文档编号:** DESIGN-2026-11-09-001
> **日期:** 2026-11-09
> **状态:** 待评审
> **前置:** `2026-10-12-frontend-admin-design.md`（§5 报表页、§8 缺口 #8）、`2026-10-08-sync-update-design.md`（同步语义）

> **For agentic workers:** 本文是设计文档，不是实施计划。经确认后由 `superpowers:writing-plans` 生成逐任务实施计划。

**目标：** 为报表页提供**质量分布**与**机台与操作员**两个看板的聚合查询端点。

**技术栈：** Go 1.26.4 / Ent v0.14.6 / Gin，测试用 `modernc.org/sqlite` 内存库。

**范围说明：**
- 本文覆盖 spec §8 缺口 #8 的**子集**：看板二、看板三。看板一（生产概览）经 2026-11-09 裁定移出，理由见 §1.2。
- 缺口 #7（包装单元查询端点）**不在本文范围**，理由见 §1.1。

---

## 一、前置事实：Center 里到底有什么

2026-11-09 对 `e35eff1`（分支 `fix/#26-admin-backend-support` @ `70e4bc4`）核实的数据链现状：

| 实体 | Center 有数据？ | 依据 |
|---|---|---|
| `doffing` | ✅ | Center 同步 `handleCreate` 有 `doffings` 分支，含 `spinning_line_id` / `operator_id` |
| `bobbin_grade` | ⚠️ 部分 | 有 `bobbin_grades` 分支，但只写 `bobbin_id`/`grade`/`inspected_at`/`defect_reason`/`inspector_id`/`is_qualified` |
| `lot` | ⚠️ 有行，关键字段为空 | `lots` 分支写入六个字段，**不含 `start_time`**，见 §1.2 |
| `bobbin` | ⚠️ 依赖上游 | `bobbins` 分支存在，但 `upsertBobbinGrade` 要求父丝饼先解析到位 |
| `barrel`/`module`/`pallet`/`carton` | ❌ 恒为空 | 无任何代码创建；Edge 采集器不采集；Center 不接收 |

### 1.1 为什么缺口 #7 不做

四张包装表在 Center **恒为空**，数据链三环全断：

1. **无生产者**：全仓库（除 ent 生成代码与测试）没有任何 `Barrel/Module/Pallet/Carton.Create`；Edge 路由 `internal/server/edge.go` 也不含任何包装单元端点。
2. **Edge 不采集**：`internal/sync/edge/uploader.go` 的 `collectors()` 只含 lots/doffings/bobbins/bobbin_grades；`collectBarrels` 是保留的脚手架，注释明令「barrels 仍不得加回本列表」。
3. **Center 不接收**：`internal/sync/center/handler.go` 的 `handleCreate` 只有上述四个分支，其余返回 `unknown table`。

即便现在建出 8 个查询接口，它们只会返回空列表，筛选与序列化形态也无从验证。故 #7 推迟到「Edge 打包环节 + 对应同步」落地之后，那属 Edge 操作子系统（spec §1.4 非目标），需另立 spec。

另：spec §4.3 写「四表统一提供 `lot_id` 筛选」有误——`module` 没有 `lot_id`（只有 `barrel1_id`/`barrel2_id`/`edge_id`）；`barrel` 无 `edge_id`；`carton.lot_id` 为 Optional。若日后做 #7，须先修正该条。

### 1.2 为什么看板一移出本计划

看板一（生产概览）依赖 `lot.start_time` 作区间过滤字段。核实后发现该列**当前恒为 NULL**：

- Edge 侧 `lotToEntry`（`internal/sync/edge/uploader.go:490`）的上传载荷**不含** `start_time` 键，故 Center 的 `SetNillableStartTime(getTimePtr(data, "start_time"))`（`handler.go:190`、`:287`）是空操作。
- 全仓库（含 `cmd/seed`、`cmd/test-db` 与所有测试）**没有任何代码设置 `lot.StartTime`**。

替代方案 `lot.created_at` 同样不可用：它是**行插入时刻**（schema `Default(time.Now).Immutable()`），同步链从不写它，补传或积压重放的批次会整体落到「同步到达那天」，与实际生产日错位。

更根本的问题是：**没有任何代码生产批次**。Center 的批次写路由已删除并被 `TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 钉住，Edge 侧也没有对应路由，只有 `cmd/seed` / `cmd/test-db` 会插入样本行。

**裁定（2026-11-09，主人选定）：** 看板一移出本计划。它需要「Edge 侧批次产生路径 + 同步载荷补 `start_time`」这一整条数据链，属同步协议与 Edge 操作子系统变更，另行立项。本文交付的看板二、看板三不受影响。

**补记（2026-11-11）：** 上述前置数据链已补齐，看板一随本次一并交付：

- Edge 侧新增批次写接口 `POST /v1/lots` 与 `PUT /v1/lots/:id/status`（`api/edge/v1/lot.go`、`internal/service/lot.go`）。批次创建即视为投产，`start_time` 在 `CreateLot` 写入；状态流转到 `completed` / `cancelled` 时写 `end_time`，且只在**首次**进入对应状态时写（重复提交幂等）。
- `lotToEntry`（`internal/sync/edge/uploader.go`）的上传载荷补上 `start_time` / `end_time`。
- 区间过滤按 §3.2.3 的 `COALESCE(start_time, created_at)` 口径，历史批次（`start_time` 为空）按行创建时刻参与统计，不会整批消失。

### 1.3 `bobbin_grade` 的字段缺口

Schema 里有、但**同步链两端都不携带**的字段：

| 字段 | 影响 |
|---|---|
| `inspection_details`（JSON） | 「五维度合格率」无数据来源 |
| `scores`（JSON） | 同上 |
| `final_score`（float） | 「平均分」无数据来源 |

补这两项需要扩展同步载荷（Edge 采集器 + Center `upsertBobbinGrade` 两处），属**同步协议变更**。spec §1.4 把「不改动现有同步协议」列为非目标，因此本文**不包含**这两项指标，将在 §6 登记为缺口。

（有先例：PLN-17 收尾时打通过 `doffing.operator_id`，同样是两端各加一处。故技术上可行，只是需单独决策。）

**补记（2026-11-12）：录入与同步已打通，指标仍未实现。**

更深一层的发现：这三列不只是「没同步」，而是**全仓库没有任何代码写它们** —— `InspectBobbin` 此前只写 `grade`/`is_qualified`/`defect_reason`。因此补同步还不够，必须先有录入。

已落地（PR 待合并）：

- **录入（Edge）**：`InspectBobbinRequest` 新增三项，均为可选，一旦提供即校验：
  - `scores`：五项等级类型分，键与 `grade.grade_type` 枚举一致（sorting/weight/final/vision/knitting），**五项必须齐全**，各自 [0,100]。用显式结构体而非 map —— 少填一项要能报错，map 里少一个键无从分辨「没这项」与「漏填」。
  - `final_score`：[0,100]。
  - `inspection_details`：质检项判定，取值限定 `pass`/`fail`/`na`。自由文本会让「合格 / OK / 通过」在报表里变成三个分组。
  - 校验在**任何写库之前**：`InspectBobbin` 会先改 `bobbin.grade` 再插历史行，校验若放在后面，现场会看到「等级变了但查不到这条质检」。
- **同步**：`collectBobbinGrades` 三键按「有值才带」处理（空 map / 零值整键缺席），Center `upsertBobbinGrade` 对应写入；`final_score` 的本地字段是非指针 `float64`，按 `> 0` 判定有值。
- **仍未做**：「五维度合格率」「平均分」两项报表指标本身（见 §6 缺口 A 的更新）。

---

## 二、范围

### 2.1 要做的六个指标

**看板二：质量分布**（数据源 `bobbin_grade`，时间过滤 `bobbin_grade.inspected_at`）

| # | 指标 | 计算 |
|---|---|---|
| 1 | 合格率 | `is_qualified=true` 数 / 非 NULL 总数，**同时返回未设置数** |
| 2 | 等级分布 | `GROUP BY grade` + count |
| 3 | 缺陷原因 Top N | `GROUP BY defect_reason` + count，降序取前 N |

**看板三：机台与操作员**

| # | 指标 | 计算 | 时间过滤字段 |
|---|---|---|---|
| 4 | 按机台产量 | `doffing GROUP BY spinning_line_id`，count + `SUM(actual_weight)` | `doffing.doffing_time` |
| 5 | 按操作员落纱数 | `doffing GROUP BY operator_id`，count | `doffing.doffing_time` |
| 6 | 按检测员工检数 | `bobbin_grade GROUP BY inspector_id`，count | `bobbin_grade.inspected_at` |

**看板一：生产概览**（数据源 `lot`，时间过滤 `COALESCE(lot.start_time, lot.created_at)`）

| # | 指标 | 计算 |
|---|---|---|
| 7 | 状态分布 | `GROUP BY status`，count + `SUM(planned_quantity)` + `SUM(actual_quantity)` |
| 8 | 按产品类型 | `GROUP BY product_type`，同上 |
| 9 | 按来源设备 | `GROUP BY edge_id`，同上；`edge_id` 为空归入 null 组 |
| 10 | 总计与完成率 | 各状态行累加；完成率 = `actual / planned`，保留三位小数 |

### 2.2 明确不做

| 不做 | 理由 |
|---|---|
| 五维度合格率、平均分 | 字段未同步，见 §1.3 |
| 包装单元维度统计 | 四表恒为空，见 §1.1 |
| 实时推送 / 缓存 | spec §5.3 已裁定不做；页面进入时拉取一次 |
| 分页 | 聚合结果行数受枚举与实体数量约束；Top N 用参数控制 |
| 导出（CSV/Excel） | spec 未提出，不在范围 |

---

## 三、接口设计

### 3.1 路由与权限

两个端点，全部挂在 `authorized` 组（`requireUserPrincipal`）下，**不挂 `requireRole`**：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/v1/reports/quality-distribution` | 看板二 |
| GET | `/v1/reports/machine-operator` | 看板三 |
| GET | `/v1/reports/production-overview` | 看板一 |

依据 spec §6.3 权限矩阵：「`GET /v1/lots`、`/v1/bobbins` 及**报表**」允许**全部已登录角色**。设备凭证被组级的 `requireUserPrincipal` 挡在外面，与批次/丝锭路由姿态一致。

组级 `requireUserPrincipal` 对只读路由的守护已由 `TestRealRouter_DeviceTokenStillBlockedFromUserRoutes`（`real_router_test.go:640`）钉住——它的 `GET /v1/lots` 用例正是「只有组级守卫、没有 `requireRole`」的判别用例。报表路由与它同构，§5.3 的新增用例沿用同一写法。注意**不要**引用 `TestRealRouter_DeviceTokenCannotReachUserRoutes`：它的用例表只含 `/v1/users` 四条与 `/v1/edges/:code/token`，不含只读路由。

### 3.2 时间范围参数

两个端点统一接受两个可选查询参数：

| 参数 | 类型 | 缺省 | 语义 |
|---|---|---|---|
| `start` | RFC3339 字符串 | `end - 7d` | 区间起点，**闭** |
| `end` | RFC3339 字符串 | 服务端当前时间 | 区间终点，**开** |

**缺省规则**（spec §5.2「缺省为最近 7 天」）：
- 两者都不传 → `[now-7d, now)`
- 只传 `start` → `[start, now)`
- 只传 `end` → `[end-7d, end)`

**校验规则：**

| 情形 | 响应 |
|---|---|
| 无法按 RFC3339 解析 | 400 + `CodeInvalidParams`（10001），message 指明是哪个参数 |
| `end` 早于或等于 `start` | 400 + `CodeInvalidParams` |
| 区间跨度 > 366 天 | 400 + `CodeInvalidParams`（防止全表扫描拖垮服务） |

第三项是本设计的自定约束，不是 spec 要求。理由：两个端点都是 `GROUP BY` 全表聚合，`bobbin_grade` 在长期运行后会达到百万级；无上限的区间等于提供一个 DoS 入口。366 天足够覆盖「日终」「月度」「年度」三种真实用法。

**时区：** 一律按 RFC3339 带偏移量解析，内部转 UTC 后与数据库比较。不做「本地日界」换算——spec 未提出，且会引入时区配置项。

**实现落点：** 参数解析写在 `api/center/v1/report.go` 的一个辅助函数里（`parseTimeRange`），不放 service 层——与 `api.ParsePagination` 的既有分工一致：请求形态归 API 层，业务逻辑归 service。

### 3.2.1 看板二按质检记录计数，不按丝饼去重

**裁定（2026-11-09，主人选定）：合格率、等级分布、缺陷 Top N、按检测员工检数，全部按 `bobbin_grade` 的**行**计数。**

依据：`bobbin_grade` 是记录表而非状态表——`schema/bobbingrade.go:131` 的索引注释明写「非唯一：同一丝饼可有多次质检，保留历史」，Center 的 `upsertBobbinGrade`（`handler.go:511`）也只增不改。一个丝饼「不合格 → 返修 → 复检合格」会留下两行。

**该口径回答的问题是「这段时间内质检工作有多少通过」**，含返修改善趋势；它**不**回答「这些丝饼最终合格多少」。后者需要按 `bobbin_id` 取最新一条，Ent v0.14.6 的 `GroupBy` 不支持子查询/窗口函数，须写原生 SQL，且测试种子要重设计。

**由此产生的解读要求：** 前端在合格率旁标注「按质检次数统计」，避免用户把一次返修造成的 2 行（1 不合格 + 1 合格）误读为质量下滑。本设计只负责后端数值，文案归前端；若前端需要「丝饼口径」，那是另立的原生 SQL 端点，不在本文范围。

### 3.2.2 看板三只统计未取消的落纱

**裁定（本设计自定，可推翻）：`doffing` 的聚合排除 `status = cancelled` 的行，`pending` 与 `confirmed` 均计入。**

理由：`cancelled` 表示这次落纱被显式作废（`cancel_reason` 有值），计入产量会让「按机台产量」与「按操作员落纱数」这两个本应对齐的数字被作废记录污染。而 `pending` 只是「尚未确认」，落纱动作已经发生，实际产出存在，排除它会低估产量。

**代价：** 若现场认为「未确认即不可信」，则应改为只统计 `confirmed`。这是一行 `Where` 的差异，测试种子里放一条 `cancelled` 即可捕获口径变化。

### 3.2.3 看板一的区间字段用 COALESCE(start_time, created_at)

`lot.start_time` 是真正的开工时刻，由 Edge 侧 `CreateLot` 写定。但该列在引入批次写接口之前的存量行上为空，且早期补传的批次也可能缺这一列。

**裁定：区间过滤统一用 `COALESCE(lot.start_time, lot.created_at)`。**

- `start_time` 有值 → 以它为准。这就是「本期开工的批次」，也是报表语义上唯一正确的读法。
- `start_time` 为空 → 退回 `created_at`（行插入时刻）。牺牲精确性换取「历史数据不会整批消失」。
- 不用 `created_at` 兜底全表的理由：两者混用会让「补录批次」与「同步到达时刻」混淆。COALESCE 的语义是明确的——有开工时刻就用开工时刻。

实现上该表达式无法用 ent 的谓词构造器表达（`GTE("COALESCE(...)")` 会把整条表达式当成标识符加引号），改用 `Where(func(sel *entsql.Selector))` 闭包里的 `entsql.P(func(b *entsql.Builder) { b.Ident(col).WriteOp(entsql.OpGTE).Arg(start) })`。`Builder.Arg` 会按方言生成 `$n` 占位符，Postgres 与 SQLite 都正确。

### 3.3 响应形态

两个端点都返回**单个对象**（非分页），走 `api.Success(data)`。响应体外层与其他端点一致（`code` / `message` / `data`）。

**共用片段：**

```json
{
  "range": {
    "start": "2026-11-02T00:00:00Z",
    "end": "2026-11-09T00:00:00Z"
  }
}
```

`range` 回显**实际生效**的区间（即缺省值填充之后的值），让前端能把「我传了什么」与「服务端用了什么」区分开。

**看板二 `quality-distribution`：**

```json
{
  "code": 0,
  "data": {
    "range": {...},
    "qualified": {
      "qualified_count": 810,
      "unqualified_count": 90,
      "unspecified_count": 100,
      "total_count": 1000,
      "qualified_rate": 0.9
    },
    "by_grade": [
      {"grade": "A", "count": 600},
      {"grade": "B", "count": 210}
    ],
    "top_defects": [
      {"defect_reason": "毛丝", "count": 43},
      {"defect_reason": "断头", "count": 27}
    ]
  }
}
```

要点：
- **`is_qualified` 是三态**（`true` / `false` / `NULL` = 质检员未陈述结论）。`qualified_rate = qualified_count / (qualified_count + unqualified_count)`，**分母排除 NULL**；`unspecified_count` 单独返回，让前端能提示「有 100 条未判定」。
- 分母为 0（区间内全为 NULL 或无记录）时 `qualified_rate` 返回 `0`，并靠 `total_count` 让前端识别「无数据」。
- `by_grade` 取 `bobbin_grade.grade`（检验结论），非 `bobbin.grade`。
- `top_defects` 只统计 `defect_reason` **非 NULL 且非空串**的行（`DefectReasonNotNil()` 与 `DefectReasonNEQ("")` 同时施加）。Edge 侧只在非空时才放进载荷，故空串理论上只可能来自 Center 侧的手工写入；把它当成「有缺陷原因」统计会造出一个假的分组。「不合格但未填写原因」这种情况不进 Top N，也不单独返回——spec §5.1 只要 Top N，把未填原因单列属新增指标，需另行裁定。
- **查询参数 `top`**：控制 `top_defects` 行数，缺省 10。取值必须为 1–50；`0`、负数、非数字、超过 50 一律 400 + `CodeInvalidParams`（**不静默截断**——静默修正会让前端误以为拿到了想要的数量）。上限 50 属防御性约束，与 `api.ParsePagination` 的 pageSize 上限 200 同理。

**看板三 `machine-operator`：**

```json
{
  "code": 0,
  "data": {
    "range": {...},
    "by_spinning_line": [
      {"spinning_line_id": "a1b2...", "doffing_count": 320, "actual_weight": 1536.5}
    ],
    "by_operator": [
      {"operator_id": "c3d4...", "doffing_count": 210}
    ],
    "by_inspector": [
      {"inspector_id": "e5f6...", "inspection_count": 480}
    ]
  }
}
```

要点：
- `by_spinning_line` / `by_operator` 的时间过滤用 `doffing.doffing_time`，且排除 `cancelled`（§3.2.2）。
- `operator_id` 可空（schema `Optional()`），未记录操作员的落纱归入 `null` 组，**不丢弃**。
- `actual_weight` 只累加非 NULL 的 `doffing.actual_weight`；某机台全为空时返回 `0`。
- `by_inspector` 的时间过滤用 `bobbin_grade.inspected_at`；`inspector_id` 同样可空。
- 所有列表按指标降序、同值时按键升序，保证结果稳定。
- **只返回 UUID，不返回名称。** `user` 表虽有 `full_name`，但 `operator_id` / `inspector_id` 是 Edge 侧身份，不保证能在 Center `users` 里找到对应行。**注意：不能用 `GET /v1/users` 做前端映射**——该端点挂在 `requireRole("admin")` 下（`center.go` 的 `users` 组），而报表对全部已登录角色开放，viewer 看得到报表却调不了映射接口。当前结论：**所有角色都只看到 UUID**。

  机台名称的补法见 §6 缺口 C：正确方向**不是**由 Edge 上报，而是把 Center 已有的 `spinning_line` 配置下发到 Edge——Center 是这张表的权威源。

**看板一 `production-overview`：**

```json
{
  "code": 0,
  "data": {
    "range": {...},
    "totals": {
      "lot_count": 12,
      "planned_quantity": 48000,
      "actual_quantity": 45360,
      "completion_rate": 0.945
    },
    "by_status": [
      {"status": "completed", "lot_count": 9, "planned_quantity": 36000, "actual_quantity": 35900},
      {"status": "in_progress", "lot_count": 3, "planned_quantity": 12000, "actual_quantity": 9460}
    ],
    "by_product": [
      {"product_type": "FDY", "lot_count": 6, "planned_quantity": 24000, "actual_quantity": 23800},
      {"product_type": "DTY", "lot_count": 6, "planned_quantity": 24000, "actual_quantity": 21560}
    ],
    "by_edge": [
      {"edge_id": "3d11df20-...", "lot_count": 8, "planned_quantity": 32000, "actual_quantity": 31000},
      {"edge_id": null, "lot_count": 4, "planned_quantity": 16000, "actual_quantity": 14360}
    ]
  }
}
```

要点：
- `totals` 由 `by_status` 各行累加得出（状态是全集的划分），不额外发一次无分组聚合。
- `completion_rate = actual / planned`，保留三位小数；`planned` 为 0 时返回 `0`（`planned_quantity` 有 `Positive()` 约束，走到这里是数据异常，返回 0 比返回 `NaN`/`Infinity` 安全——JSON 无法编码后两者）。
- 三组分布都返回**空切片而非 null**：前端对 `null` 调 `.map` 会白屏。
- `by_edge` 的 `edge_id` 可空，未记录来源的批次归入 `null` 组，**不丢弃**；`edge_id` 指针不加 `omitempty`，让前端能区分「没有这一组」与「这一组是未记录的」。
- 所有分组用 `ent.As(ent.Sum(...), "sum_planned")` 起别名：ent 的扫描器把列名按 `"("` 截断后小写匹配结构体字段，两个未别名的 `SUM` 都会匹配到 `sum` 而互相覆盖。

---

## 四、实现结构

### 4.1 文件布局

| 文件 | 内容 |
|---|---|
| `internal/service/report.go` | `ReportService`：两个方法，各自返回响应结构体 |
| `internal/service/report_test.go` | 服务层测试（内存 SQLite） |
| `api/center/v1/report.go` | `ReportHandler` + `parseTimeRange` / `parseTopN` 辅助函数 |
| `api/center/v1/report_test.go` | handler 层测试（参数解析与错误映射） |
| `internal/server/center.go` | 注册 `/v1/reports` 路由组 |
| `internal/server/real_router_test.go` | 路由存在性 + 设备凭证拒绝 + 角色放行断言 |

### 4.2 Service 层

```go
type ReportService struct{ client *ent.Client }

func NewReportService(client *ent.Client) *ReportService

func (s *ReportService) QualityDistribution(ctx context.Context, start, end time.Time, top int) (*QualityDistributionResponse, error)
func (s *ReportService) MachineOperator(ctx context.Context, start, end time.Time) (*MachineOperatorResponse, error)
```

**聚合实现用 Ent 的 `GroupBy` + `Aggregate`**（v0.14.6 已支持，生成代码见 `lot_query.go:441` 的注释示例：`GroupBy(field).Aggregate(ent.Count()).Scan(ctx, &v)`）。求和用 `ent.Sum(doffing.FieldActualWeight)` 配合匿名 struct 扫描。

**注意：** 生成的实体包里没有 `*_groupby.go` 这类独立文件，`GroupBy` 返回的 builder 定义在 `*_query.go` 内（如 `DoffingGroupBy`）。实现时以实际生成代码为准，不要照文档猜文件位置。

**`SUM` 的空值语义：** SQL 的 `SUM` 在整列为 NULL 时返回 NULL。扫描目标用 `sql.NullFloat64` 或 `*float64`，再归一化为 `0`，避免把 `null` 透传成 JSON `null`（前端会要特判）。

**分组键的空值：** `doffing.operator_id` / `bobbin_grade.inspector_id` 在 schema 中是 `Optional()` 但**没有** `Nillable()`，生成的 Go 字段是非指针 `uuid.UUID`。**它不会报错，只会静默变成 `uuid.Nil`**：ent 的扫描器跳过 nil 列（`ent@v0.14.6/dialect/sql/scan.go:245-251`），`google/uuid` 的 `UUID.Scan(nil)` 返回 nil 错误且不改值（`uuid@v1.6.0/sql.go:15-19`）。真正的 NULL 组与 UUID 全零的行因此**无法区分**，且失败方式是静默的零值——必须有测试才能发现。因此：

- GroupBy 的扫描目标用 `*uuid.UUID`（保留 NULL 与零值的区分）。
- 响应结构体对应字段用 `*string` 且**不加 `omitempty`**——否则 `null` 组会被 JSON 序列化吞掉，而 §7 验收标准第 3 条要求断言它存在。
- 测试必须包含「分组键为 NULL」的种子行，并断言它出现在 `null` 组而非零值 UUID 组（§5.1 已列用例）。

仓库里此前没有 `GroupBy` 的用法，也没有 `sql.Null*` 扫描的先例，这两条是本设计新立的约束，实现时不要照其他 service 的扫描写法套。

### 4.3 Handler 层

```go
func (h *ReportHandler) QualityDistribution(c *gin.Context)
func (h *ReportHandler) MachineOperator(c *gin.Context)
```

- `parseTimeRange(c) (time.Time, time.Time, error)`：读 `start`/`end`，套用 §3.2 的缺省与校验规则。签名内部接受 `now` 参数以便测试注入。
- `parseTopN(c) (int, error)`：读 `top`，缺省 10，合法区间 1–50，越界即错误。
- 参数解析失败 → `400` + `api.Error(api.CodeInvalidParams, ...)`。
- service 返回错误 → `500` + `api.Error(api.CodeServerError, ...)`，与其他 handler 一致。聚合查询没有「业务拒绝」路径，不需要 `ServiceError`。

### 4.4 路由注册

```go
reportService := service.NewReportService(s.client)
reportHandler := centerv1.NewReportHandler(reportService)

reports := authorized.Group("/reports")
{
    reports.GET("/quality-distribution", reportHandler.QualityDistribution)
    reports.GET("/machine-operator", reportHandler.MachineOperator)
}
```

**不加 `requireRole`**，理由见 §3.1。在 `center.go` 该处加一行注释说明「报表对全部已登录角色开放（spec §6.3）」，与现有 `/users/me` 的注释风格一致。

---

## 五、测试策略

### 5.1 服务层（`internal/service/report_test.go`）

用 `newTestClient(t)`（`setup_test.go` 的内存 SQLite），每个用例自造种子。**每个指标的断言必须能区分「算对了」与「算错了但恰好相等」**：

| 用例 | 种子设计 | 关键断言 |
|---|---|---|
| 合格率 | 6 条：3 true / 1 false / 2 NULL | `rate=0.75`，`unspecified_count=2`（**分母只含 4**） |
| 合格率边界 | 全 NULL | `rate=0`，`total_count` 非 0 |
| 复检各计一次 | 同一 `bobbin_id` 两条：不合格 + 合格 | 两条都计入，`total_count=2`（钉住 §3.2.1 口径） |
| 等级分布 | 两个等级 | 分组键来自 `bobbin_grade.grade` |
| 缺陷 Top N | 3 种原因、计数 5/3/1，`top=2` | 只返回 2 行且按降序 |
| 缺陷空值 | 一条 `defect_reason` 为 NULL、一条空串 | 两者都不出现在 `top_defects` |
| 机台产量 | 两条落纱线、`actual_weight` 一有一无 | 空值不带崩整组求和 |
| 操作员 | 一个有 ID 一个 NULL | `null` 组存在且非零值 UUID 组 |
| 检测员 | 两个检测员 | 计数来自 `bobbin_grade` |
| 取消的落纱 | 同一机台一条 `confirmed` + 一条 `cancelled` | `doffing_count=1`（钉住 §3.2.2 口径） |
| 时间窗口（落纱） | `doffing_time` 落在窗口内外各一条 | 窗口外被排除 |
| 时间窗口（质检） | `inspected_at` 落在窗口内外各一条 | 窗口外被排除（**与上一行分别验证，两个端点字段不同**） |

**时间窗口用例是本设计的重点防线。** 两个端点用了两个不同的时间字段（`doffing.doffing_time`、`bobbin_grade.inspected_at`），错接一个不会有任何症状，只会让数字静默偏差。

### 5.2 Handler 层（`api/center/v1/report_test.go`）

`parseTimeRange` 与 `parseTopN` 是纯函数式解析逻辑（入参为字符串与 `now`），单独测试：

| 用例 | 期望 |
|---|---|
| 都不传 | `[now-7d, now)` |
| 只传 start | `[start, now)` |
| 只传 end | `[end-7d, end)` |
| start 晚于 end | 错误 |
| start == end | 错误 |
| 非法格式 | 错误，message 含参数名 |
| 跨度 367 天 | 错误 |
| `top` 不传 | `10` |
| `top=1` / `top=50` | 原值 |
| `top=0` / `top=-1` / `top=51` / `top=abc` | 错误 |

**关键点：** 函数签名接受 `now time.Time` 作为参数（而非内部调 `time.Now()`），否则缺省窗口的测试无法稳定断言。这与仓库既有做法一致（`internal/server/center.go:64` 的 `edgeTokenTTLFromEnv` 同样是「先解析、再校验取值」，并显式处理 `err` 与 `ttl <= 0` 两种失败）。

### 5.3 路由层（`internal/server/real_router_test.go`）

1. **存在性**：两条路径出现在 `router.Routes()` 中（沿用 `TestRealRouter_CenterKeepsReadRoutes` 的范式）。
2. **设备凭证拒绝**：设备 token 打 `GET /v1/reports/quality-distribution` 得 403。这条钉住的是组级 `requireUserPrincipal`——与 Ruling 13 补的那条同理，防止「报表新组忘了继承守卫」。
3. **角色放行**：viewer token 打两个端点均**不是 403**（报表对全部已登录角色开放，若日后有人误挂 `requireRole("admin")` 会变红）。

断言用状态码精确判断，不用 `!= 403` 这种宽断言——viewer 那条例外，因为它本就要排除 403 而允许 200/400/500（空库下的实现细节由服务层测试覆盖）。

---

## 六、遗留缺口（不在本文范围）

| # | 缺口 | 补法 | 阻塞什么 |
|---|---|---|---|
| A | ~~`bobbin_grade.inspection_details` / `scores` / `final_score` 未同步~~ **录入与同步已打通（2026-11-12，见 §1.3 补记）**；仍缺的是「五维度合格率」「平均分」两项报表指标本身 | 在报表服务里按 `scores` / `inspection_details` 展开聚合 | 两项指标（字段已有数据，缺聚合） |
| B | `barrel` / `module` / `pallet` / `carton` 无生产者、无同步、无端点 | 属 Edge 操作子系统，需另立 spec | spec §8 缺口 #7 整个 |
| C | ~~Edge 侧无 `spinning_line` 消费方~~ **已闭合（2026-11-11）**：`SyncClient.DownloadData` 拉取 `spinning_lines` / `grades` 并按 Center 的 UUID 落地（同名冲突时保留本地 ID 并告警）；报表 `by_spinning_line` 新增 `spinning_line_name`，查不到时为 `null` | — | — |
| D | ~~看板一所需的 `lot.start_time` 无生产者、不在同步载荷内~~ **已闭合（2026-11-11）**，见 §1.2 补记 | — | — |

A 与 D 都能用 PLN-17 打通 `doffing.operator_id` 的同一手法解决（两端各加一处），成本可控；但都改同步协议，需单独裁定，不应顺带塞进报表实现。C 的方向相反，成本取决于 Edge 下载链路本身，属另一个子系统。

### 6.1 已裁定暂缓的两项（2026-11-10）

| # | 项 | 裁定 | 何时再处理 |
|---|---|---|---|
| E | `defect_reason` 分组无基数上限：宽窗口 + 自由文本可能一次拉出大量分组，`top` 是在 Go 侧排序后截断的 | **保持现状**（主人裁定）。366 天的区间上限提供了粗略约束；当前数据量下不构成问题 | 真实数据量上来、单次响应明显变慢时，改为 SQL 侧 `ORDER BY COUNT(*) DESC LIMIT ?` |
| F | 管理员无法重置他人密码 | **暂缓**（主人裁定）。改密接口要求旧密码，故「admin 或本人」中的 admin 分支实际走不通，忘记密码的用户只能改库 | 现场提出「忘记密码」诉求时，另立 admin 专用重置端点（不复用本接口） |

这两项是**有意为之的取舍**，不是遗漏。E 的实现见 `internal/service/report.go` 的 `TopDefects` 段；F 的背景见 `api/center/v1/user.go` 中 `ChangePassword` 的注释。

---

## 七、验收标准

1. 两个端点对 viewer / operator / inspector / admin 均可访问（非 403），设备凭证得 403。
2. 缺省区间为最近 7 天，`range` 字段回显实际生效值。
3. 合格率的分母排除 `is_qualified IS NULL` 的行，且 `unspecified_count` 可观测；计数单位为质检记录行（复检各计一次），响应中不做丝饼去重。
4. 空 `operator_id` / `inspector_id` 的行不被丢弃，归入 `null` 组，且该组在 JSON 中确实存在（不因 `omitempty` 被吞）。
5. 两个端点的区间过滤分别使用 `doffing.doffing_time` 与 `bobbin_grade.inspected_at`；落纱聚合排除 `cancelled`。
6. `top` 参数缺省 10，越界返回 400 而非静默截断。
7. `go build ./...` 与 `go test ./...` 全绿，且 `TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 与 `TestRealRouter_CenterKeepsReadRoutes` 断言强度未被放宽。
8. 报表端点不引入任何写路径。

---

## 八、待主人确认的实现取舍

以下四项是本设计按既有惯例定的，若主人有不同意见可推翻：

1. **区间上限 366 天**（§3.2）——spec 未要求，为防止全表聚合拖垮服务而加。
2. **只返回 UUID 不返回名称**（§3.3）——`GET /v1/users` 仅 admin 可调，不能作为前端映射手段，故所有角色当前只看到 UUID。
3. **`top` 越界报错而非截断**（§3.3）——静默修正会让前端误以为拿到了想要的数量。
4. **落纱排除 `cancelled`，保留 `pending`**（§3.2.2）——若现场认为「未确认即不可信」，应改为只统计 `confirmed`。
