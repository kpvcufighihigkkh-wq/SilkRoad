# Edge→Center 更新同步设计

> **For agentic workers:** 本文是设计文档，不是实施计划。经确认后由 `superpowers:writing-plans` 生成逐任务实施计划。

**目标：** 让 Edge 侧对已存在记录的修改（称重、质检、落纱状态流转）真正同步到 Center，消除当前"报告成功却什么都没写"的假成功链路。

**技术栈：** Go 1.26.4、Ent v0.14.6、Gin、PostgreSQL（Center）/ SQLite（Edge）

**前置：** `docs/superpowers/specs/2026-09-29-backend-service-api-design.md`（实体归属权威）、Phase 6 已完成的鉴权与上传/下发链路。

---

## 一、问题

### 1.1 现状

Edge 侧对已存在记录的任何修改都不会到达 Center。净效果：**称重与质检结果永久停留在 Edge 本地**，运维在 Center 查询净重与等级会得到系统性错误数据，且没有任何报错、任何积压计数。

### 1.2 四个断点

经代码核实，四个断点各自独立，**修任何一个都无效**——必须四环一起通。

| # | 断点 | 位置 |
|---|------|------|
| 1 | 本地改完不重置 `sync_status` | `internal/service/bobbin.go` 的 `WeighBobbin`/`InspectBobbin` 等只改业务字段 |
| 2 | 采集器硬编码 `Operation: "create"` | `internal/sync/edge/uploader.go` |
| 3 | 上传载荷不含 `grade` 与 `tare_weight`（Center 的 `createBobbin` 已能接收这两个字段，是 Edge 侧没发） | `internal/sync/edge/uploader.go` |
| 4 | Center 的 `handleUpdate`/`handleDelete` 是 `// TODO` + `return nil`，而 `applyEntry` 把无 error 计为 `applied` | `internal/sync/center/handler.go` |

### 1.3 这不是"重试"问题

记录**没有失败**——它作为 create 成功上传了，`applied` 计数正常，没有任何 error。重试机制只处理失败的上传；这里缺的是"这条记录有新信息需要再传一次"这件事**根本没有触发条件**。这是本设计要解决的核心。

### 1.4 时序澄清（决定字段覆盖范围）

- **等级与皮重无条件丢失**：上传载荷里就没有这两个字段。
- **毛重/净重仅当称重发生在首次上传之前**才会随 create 送出。

机制上后者是时序相关而非结构必然，但产线实际时序（建丝饼 → 称重 → 质检）几乎总是晚于首次上传，故实践中全丢。

### 1.5 附带的永久失败缺陷

批次存在**结构性**失败路径，与瞬时网络故障共用同一套重试预算：

`lot_number` 是 `Unique + NotEmpty`，而 Center 自己也有创建批次的写接口（`lots.POST`）。当 Center 先建了 `LOT-X`（ID=A）、Edge 本地也有 `LOT-X`（ID=B）时：

1. 同步路径的幂等检查查的是 **ID**（`lot.IDEQ(id)`）→ 查 ID=B 不存在 → 放行
2. `builder.Save` 撞 `lot_number` 唯一约束 → 报错
3. 同步路径**完全不处理约束冲突**（`grep IsConstraintError` 在 `handler.go` 中命中 0 次）

重试一百次结果相同，却消耗为瞬时故障设计的预算；5 次后进 `failed`，而仓库**无任何回收路径**。`bobbin_number`、`barrel_number` 同为唯一，故三者同型。

更糟的是级联：`lot_id` 在 doffing/bobbin/barrel 上均为 **Required FK**，一个批次的失败会拖死其下所有下游记录。

### 1.6 归属关系的既有偏离

设计文档的归属原则是「**Edge 产生的实体，Center 只读汇总**」，但接口清单与实现与之相抵：

| 来源 | 内容 | 冲突 |
|------|------|------|
| spec §5.3 归属表 | `Lot \| 只读汇总 \| Edge 创建` | Center 只读 |
| spec §5.1「生产管理」接口清单 | `POST /v1/lots`、`DELETE /v1/lots/:id` | Center 可写 |
| spec §5.3 归属表（Bobbin 行） | `Bobbin \| 只读 + 改等级` | Center 可改等级 |
| spec §5.3 归属表 + 注意段 | Module/Pallet/Carton 的 Center 端「应设计为只读接口」 | 与实现一致 |

实现跟从了接口清单（提交 `deaf696`，2026-09-28），而归属定稿于 `5034415`（2026-09-29 16:08）——**写路由比归属定稿早一天**，是过渡产物。实测这些路由在本仓库内**无任何调用方**（除 swagger 注释），无前端代码。

---

## 二、设计决策

### 2.1 重传触发：脏标记（同步字段重置）

**决策：** Edge 本地对已存在记录的业务修改，自动把该记录重置为待同步。

重置内容（三者缺一不可）：

- `sync_status` → `pending`
- `sync_retry_count` → `0`
- `synced_at` → 清空
- **`failed` 记录一并复活**

理由：本地修改意味着现场确实产生了新信息；大部分失败是网络/瞬时故障，而修改正是修好它的时机。若保持 `failed`，现场修好的数据永远上不去，运维必须改数据库。

**计数必须归零的原因（易漏）：** 采集器的过滤是 `sync_status = pending` **且** `sync_retry_count < 5`，而 `markExhausted` 把 `pending 且 count >= 5` 翻成 `failed`。若只置 `pending` 不重置计数，一条 `count >= 5` 的记录被编辑后会立刻被打回 `failed`——**编辑再次静默丢失**，正是要修的 bug 换个形态重现。

### 2.2 施加方式：ent Hook（机制保证，非约定）

**决策：** 在 Edge 侧的 ent client 上挂 mutation hook，凡对可采集实体做 Update/Delete 即自动置脏。

理由："每个业务写入点记得调用"是约定而非机制。当前已有 8 处会改已存在行的业务操作（`UpdateLotStatus`、`ConfirmDoffing`、`CancelDoffing`、`UpdateBobbinStatus`、`MarkBobbinPrinted`、`WeighBobbin`、`InspectBobbin` 等），将来只会更多，同一 bug 类必然复发。

**技术要点（已核实）：**

- 用 `ent.Hook`（`Client.Use`，生成于 `ent.go:512` 的 `withHooks`），**不是** `ent.Interceptor`——后者走查询路径（`withInterceptors` / `scanWithInterceptors`）。
- **自锁防护：** 生成的 `Mutation` 暴露 `Fields()`（`LotMutation.Fields()` 在 `mutation.go:8962`），故 hook 内检查改动字段集合，**当改动仅限 `sync_status`/`synced_at`/`sync_retry_count` 时跳过置脏**。这比 context 豁免标记可靠——不依赖调用方记得传标记，否则 `markSynced` 的写入会被拦回 `pending` 造成无限重传。
- **只挂 Edge 侧：** Center 不采集任何数据，在 Center 侧置脏没有消费者。

### 2.3 置脏范围：只覆盖采集得动的表

**决策：** hook 只对采集器实际会采集的表置脏（当前 `lots`、`bobbins`，本轮加 `doffings`）。

理由：对采集器不认的表置 `pending` 会造出**永远排不出去**的积压（`retry_count` 恒为 0，`markExhausted` 也碰不到），与 Phase 6 已确立的「延迟优于销毁」相悖。

实现上让这份表与 `collectors()` **同源**（单一列表），避免两处漂移。

### 2.4 Center 写语义：upsert

**决策：** Edge 永远发 `create`；Center 把 `createLot`/`createBobbin`（本轮加 `createDoffing`）改为 **存在则更新、不存在则插入**。Edge 侧发送逻辑不变。

理由：Edge 无需知道行的历史，重传/重试/网络重发天然安全；接口只有一条路径，改动集中在 Center。

### 2.5 冲突认领：按业务编号

**决策：** Center 的幂等检查从「只按 ID」改为「先按 ID，未命中再按业务编号」，编号命中即认领该行并 upsert，**保留 Center 行的 ID**。

适用字段：`lot_number`、`bobbin_number`、`barrel_number`（三者均 `Unique`）。
`doffing.bobbin_number` **不是**唯一，不适用。

理由：兼容「Center 上历史上已存在同编号行」的遗留数据；保留 Center 行 ID 可让 Center 侧既有 `lot_id` 引用继续有效。

**配套：** 同步路径需识别 `ent.IsConstraintError` 并转入认领逻辑，而非当作普通错误计入重试。

### 2.6 ID 错位解法：按业务编号解引用

**决策：** Center 处理落纱/丝饼时**不信任载荷里的 `lot_id`**，而是用载荷里的 `lot_number` 查父行，以 Center 自己的 ID 写外键。

理由：与整个 Phase 6 的原则一致（身份由 Center 推导、绝不采信载荷）；不动 schema；Edge 无需知道映射；依赖顺序（`lots → doffings/barrels/bobbins`）已保证父行先到。

**代价：** 载荷需多带 `lot_number`；Center 侧多一次按编号的查询。

### 2.7 字段归属：严格单向

**决策：** **所有数据只能由 Edge 端修改**；凡双向相抵的字段与实体，一律改为单向可控（Edge 写、Center 只读汇总）。

落地：

| 实体 | 归属 |
|------|------|
| Lot | 全部字段归 Edge（含 `plc_lot_number`） |
| Bobbin | 全部字段归 Edge（含 `grade`、`tare_weight`） |
| Doffing | 全部字段归 Edge |
| BobbinGrade | 全部字段归 Edge（只增历史） |

**推论：** upsert 时**不需要任何字段级例外规则**——整行以 Edge 为准。这消除了原本设想的"Center 拥有字段不被覆盖"的复杂度。

**连带修正：**

- 删除 Center 的 7 条写路由：`lots.POST`、`lots.PUT /:id/status`、`lots.DELETE`、`bobbins.POST`、`bobbins.PUT /:id/status`、`bobbins.POST /:id/print`、`bobbins.DELETE`（`users.DELETE` 不在本例：用户实体不属 Edge 产生）。
- spec 需同步修正：spec §5.1「生产管理」移除批次写端点、spec §5.1「载具与丝锭」的 `PUT /v1/bobbins/:id/grade` 与 §5.3 归属表 Bobbin 行的「Center 质检改等级」作废、spec §4.2 中 `PLCLotNumber` 放在 `CreateLotRequest` 的表述需重新说明。

### 2.8 BobbinGrade 启用：只增历史

**决策：** 质检明细上传并写入 `BobbinGrade` 实体；**每次质检新增一条**（保留谁、何时、什么结果）。

理由：与现有字段设计意图一致（`inspected_at`、`inspector_id`、`notes`，且无唯一索引挂在 `bobbin_id` 上）；上传语义天然是 create（只增不改），**无需 upsert，直接避开字段认领问题**。

**启用的真实成本（已核实）：**

- `BobbinGrade` 与 `ProductConfig` 此前**从未被 ent 生成过**：其 `Annotations()` 返回类型写成 `[]entsql.Annotation`，而 ent 要求 `[]schema.Annotation`，导致 `types.Implements` 为假并被加载器静默跳过（`entc/load/load.go:147`），`ent generate` 退出码 0 却零产出。启用前必须先修签名。
- `InspectBobbin` 当前**完全不写 `BobbinGrade`**，只 `SetGrade` 到 `bobbin` 表。
- `BobbinGrade.Edges()` 返回 `nil`——没有任何外键约束，`bobbin_id` 有值但不指向 `bobbin` 表。
- `BobbinGrade` 原本**没有** `sync_status`/`sync_retry_count`/`synced_at`，需加 schema 字段 + 迁移 + 重新生成 ent。
- 其 `bobbin_id` 原带**唯一索引**（注释「一个丝锭只有一条质检记录」），与「只增历史」冲突，必须改为非唯一。

### 2.9 failed 的回收路径

**决策：** 在 Edge 的 `/v1/sync` 组下加一个受 JWT 保护的端点，提供两种模式：

1. **重传指定记录**：把 `failed` 记录重置为 `pending`、计数归零。
2. **全量重传**：把指定表的所有 `synced` 记录重置为 `pending`。

理由：与 2.1 的"本地编辑即复活"互补——那条靠人改数据触发、这条靠人显式下令；不给自动化开无限重试的口子，符合"`failed` 是等人看一眼的标记，而非数据终点"的定位。

**全量重传的用途：** 存量已 `synced` 的记录不会被重传（本次给载荷补 `grade`/`tare_weight` 后，这些字段在 Center 永远为空）。Center 已是 upsert，重传不重复也无副作用。各实体均有 `synced_at`，但**没有任何 `updated_at` 与 `synced_at` 的比较机制**，故无法自动识别陈旧记录——这正是需要显式全量重传的原因。

### 2.10 doffing 打通

**决策：** 采集器扩展到 `doffings`，Center 补 `createDoffing`；**每次状态变更都重传**（`pending` 创建时传一次、`confirmed` 再传、`cancelled` 再传）。

理由：与脏标记机制天然契合（hook 不区分状态）；Center 始终能反映现场真实状态，包括掉线期间未确认的落纱也可见。

**落纱状态机：** `pending → confirmed / cancelled`。`ConfirmDoffing` 写 `status`/`actual_weight`/`bobbin_number`/`confirmed_at`/`grade`；`CancelDoffing` 写 `status`/`cancel_reason`。

**跨端引用：** `doffing.spinning_line_id` **不是外键**（`Doffing.Edges()` 只声明 `lot`），故跨端引用不会触发约束失败。`lot_id` 是真 FK，由 2.6 的解引用处理。

### 2.11 barrel：不开采集

**决策：** 本轮 **barrels 不纳入采集**；仅在 Center 补接收侧能力并保留采集器扩展位。

理由：Edge 侧**没有创建桶的路径**——`internal/service/` 下没有 `BarrelService`，全仓库唯一的 `Barrel.Create()` 调用点全在测试里，业务与 `cmd/seed` 均无，落纱确认也不建桶。故"打通 barrels 的同步"目前**无数据可同步**。

不写无法验证的代码：同步链路的每一环都要有真实数据可测。

### 2.12 Edge 侧补写路由

**决策：** 给 Edge 补上 `POST /v1/bobbins/:id/print` 与 `PUT /v1/bobbins/:id/status`。

理由：删除 Center 写路由后，`bobbin.status` 与 `label_printed` 两侧都没有合法写入方（Edge 的 `UpdateBobbinStatus`/`MarkBobbinPrinted` 方法存在但无路由），将成为"创建后永不可变"。打印标签与状态流转是现场操作，确实需要入口；脏标记机制会自动触发重传。

---

## 三、数据流（本轮完成后）

```
Edge 本地业务写入（称重/质检/落纱确认）
   ↓ ent Hook 拦截（改动字段非纯同步字段）
置 pending + 计数归零 + 清 synced_at
   ↓
采集器 CollectPending（按外键依赖顺序，limit 截断）
   ↓ 载荷补齐 grade / tare_weight / lot_number
POST {center}/v1/edges/{code}/upload
   ↓ 四重鉴权（Phase 6 已实现）
Center HandleUpload
   ↓ 按依赖排序
createLot / createBobbin / createDoffing / createBobbinGrade
   ↓ 幂等检查：先 ID、再业务编号认领
   ↓ 解引用父行（按编号）
upsert（整行以 Edge 为准）
   ↓
逐条结果 → 按 error 串定位被拒记录
   ↓
Edge 逐条标记：应用→synced；被拒→计数+1，保持 pending
   ↓ 达上限 → failed（等人查看，可用重传端点回收）
```

**响应契约（已有，无需改）：** `UploadResponse.Applied/Rejected/Errors`。错误串格式 `"<table>/<uuid>: <message>"`（`handler.go:90`）被 Edge 用于逐条定位被拒记录；任何对账失败（计数不符、解析失败、ID 越界）一律退化为"整批计入重试"，绝不猜。

---

## 四、错误处理与保守回退

**沿用 Phase 6 已确立的原则：** 无法**证明**某条记录已应用时，保持 `pending` 而非标记 `synced`。猜错 `synced` 的代价是记录被静默丢弃；多传一轮的代价只是一次往返。

**本轮新增的保守点：**

- 唯一约束冲突 → 转入认领逻辑，认领失败则计重试（而非当作永久错误）。
- 父行按编号查不到 → 该条计入重试（保持 `pending`），不写入 NULL 外键。
- `BobbinGrade` 的 `bobbin_id` 在 Center 侧无 FK，故不引入新的级联失败面。

**不复用 `ConflictResolution`：** spec 定义了 `server_wins`/`edge_wins`/`merge` 与错误码 `CodeSyncConflict=30007`，但本轮采纳**严格单向**（Edge 全胜且无竞争方），故不引入版本号与冲突协商机制。

---

## 五、范围

### 5.1 本轮包含

1. Edge 侧 ent Hook 脏标记（含自锁防护、只覆盖可采集表）
2. Center 侧 upsert + 按业务编号认领 + `IsConstraintError` 处理
3. 按业务编号解引用父行（`lot_number`）
4. 上传载荷补齐 `grade`/`tare_weight`/`lot_number`
5. `BobbinGrade` 启用（schema 加同步字段 + `InspectBobbin` 写入 + 采集 + Center 接收）
6. `doffings` 打通（采集 + `createDoffing` + 每次状态变更重传）
7. failed 重传端点（指定记录重传 + 全量重传）
8. 删除 Center 的 7 条写路由；Edge 补 2 条写路由
9. spec 归属表与接口清单的矛盾修正

### 5.2 明确排除（各自另开任务）

| 项 | 理由 |
|----|------|
| `delete` 同步 | 涉及软删/硬删语义决定（硬删丢历史，软删需加字段），比 update 多一层设计 |
| Center 侧 doffing/barrel/BobbinGrade 查询端点 | 与"只管同步机制"一致，不把读侧需求混入 |
| Edge 侧 lot/bobbin 创建 API | 属 Phase 2–5「各实体 Service/API 重建」的账 |
| barrel 采集 | Edge 侧无建桶路径，无数据可同步 |
| Module/Pallet/Carton 同步 | 既有范围外；其 `edge_id` 无外键 |
| `Edge.status` 超时离线判定 | 既有范围外 |
| `RefreshToken` 洗身份、`provider.go` 的未知表假成功、`getSpinningLines` 返回 nil | Phase 6 已记录的遗留项 |

### 5.3 已知风险

- **Edge 侧 lot/bobbin 无创建 API**（只能靠 `cmd/seed` 与测试造数），故 `lots`/`bobbins` 的同步链路本轮只能靠测试与 seed 验证，现场真实跑通要等 Phase 2–5。
- **`-race` 在本环境不可用**（无 gcc/cgo），故 Phase 6 的 `uploadMu` 互斥仍只有阅读级验证；本轮若新增并发面，同样受限。
- **全量重传的负载**取决于数据量，需运维自行把握时机。

---

## 六、测试策略

**核心原则：每个机制都要有能证伪的测试。** Phase 6 的教训是"断言文字"容易掩盖缺陷——排序接线曾可被删除而全绿。

| 机制 | 必须能证伪的测试 |
|------|-----------------|
| 脏标记 | 做一次业务修改 → 记录变 `pending`；**变异：去掉 hook → 测试必须失败** |
| 自锁防护 | `markSynced` 之后记录**不得**被重新置脏（否则无限重传） |
| 计数归零 | `count=5` 的记录被编辑后**能被采集**（而非立刻被打回 failed） |
| upsert | 重传同一批两次 → Center 无重复行、字段被更新 |
| 编号认领 | Center 预置同编号不同 ID 的行 → 上传后**认领该行**（不新建、不报错） |
| 解引用 | 载荷带 Edge 本地 `lot_id` 但 `lot_number` 指向 Center 已有行 → 落纱挂到 **Center 的行** |
| BobbinGrade | 同一丝饼质检两次 → **两条记录**（只增历史） |
| failed 回收 | 造 `failed` → 调重传端点 → 变 `pending` 并被采集 |
| 全量重传 | 造 `synced` → 调用 → 变 `pending` |
| 单向化 | 移除的 Center 写路由返回 404（路由表断言） |

**变异验证要求：** 每项安全/正确性机制都要做"删掉修复即失败"的实证，并把输出记入报告。Phase 6 中该方法抓到过一个静默失效的探针。

---

## 七、待确认

本文经确认后，由 `superpowers:writing-plans` 生成逐任务实施计划。

**需特别注意的两处文档修正：** spec 的归属表与接口清单自相矛盾（§5.3 归属表 vs §5.1「生产管理」），且 §5.3 归属表的 Bobbin 行与 §5.1「载具与丝锭」要求 Center 改等级——这三处需与本设计的单向原则统一，否则文档与实现会再次不一致。项目 `CLAUDE.md` 的文档同步规则为硬性要求。
