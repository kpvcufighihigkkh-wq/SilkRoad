import http from './client'
import type { ApiResponse } from '@/types/api'

/**
 * 报表端点的时间范围。
 *
 * 后端接受 RFC3339 字符串，缺省为最近 7 天，跨度上限 366 天。
 * 超限或格式非法会返回 400，前端应在提交前先做基本校验，
 * 避免用户点了查询才看到报错。
 */
export interface ReportRange {
  start: string
  end: string
}

/** 合格率统计。is_qualified 是三态，unspecified 单独返回。 */
export interface QualifiedStats {
  qualified_count: number
  unqualified_count: number
  unspecified_count: number
  total_count: number
  qualified_rate: number
}

export interface GradeCount {
  grade: string
  count: number
}

export interface DefectCount {
  defect_reason: string
  count: number
}

/**
 * 质检项维度的一行。
 *
 * qualified_rate 的分母是 pass / (pass + fail)，**不含 na**：
 * na 表示该项未检，算进分母会让「没检」拉低合格率。
 */
export interface DimensionStat {
  dimension: string
  pass_count: number
  fail_count: number
  na_count: number
  qualified_rate: number
}

/**
 * 看板二「质量分布」。
 *
 * 注意 qualified_rate 的分母排除了「未判定」，即
 * qualified / (qualified + unqualified)，不是 qualified / total。
 * 前端直接展示该字段，不得用三个计数重算 —— 重算会把 NULL 算进分母。
 *
 * average_score 为 null 表示区间内没有任何已录入评分的记录；
 * 不能用 0 兜底，0 分是真实可能的评分。
 */
export interface QualityDistribution {
  range: ReportRange
  qualified: QualifiedStats
  by_grade: GradeCount[]
  top_defects: DefectCount[]
  average_score: number | null
  scored_count: number
  by_dimension: DimensionStat[]
}

export interface SpinningLineStat {
  spinning_line_id: string
  /**
   * 机台名称，可空。
   *
   * 报表在 Center 本地按 spinning_line_id 关联线体表得出；Edge 未从
   * Center 拉取过线体配置时查不到对应行，此处为 null。
   */
  spinning_line_name: string | null
  doffing_count: number
  actual_weight: number
}

/** 分组键可空：未记录操作员/检测员的记录归入 null 组。 */
export interface OperatorStat {
  operator_id: string | null
  doffing_count: number
}

export interface InspectorStat {
  inspector_id: string | null
  inspection_count: number
}

/**
 * 看板三「机台与操作员」。
 *
 * 落纱聚合已排除 cancelled 状态（后端 §3.2.2 的裁定），
 * pending 仍计入 —— 前端不需要也不应该再做过滤。
 */
export interface MachineOperator {
  range: ReportRange
  by_spinning_line: SpinningLineStat[]
  by_operator: OperatorStat[]
  by_inspector: InspectorStat[]
}

export interface ReportQuery {
  start?: string
  end?: string
}

// 看板一「生产概览」。
//
// 区间过滤在后端用 COALESCE(start_time, created_at)：
// start_time 缺失时回退到行创建时刻，避免历史数据整批消失。
export interface LotStatusStat {
  status: string
  lot_count: number
  planned_quantity: number
  actual_quantity: number
}

export interface ProductTypeStat {
  product_type: string
  lot_count: number
  planned_quantity: number
  actual_quantity: number
}

/** 分组键可空：未记录来源设备的批次归入 null 组，前端不得丢弃。 */
export interface EdgeLotStat {
  edge_id: string | null
  lot_count: number
  planned_quantity: number
  actual_quantity: number
}

export interface ProductionTotals {
  lot_count: number
  planned_quantity: number
  actual_quantity: number
  /** 完成率 = actual / planned，保留三位小数；planned 为 0 时为 0。 */
  completion_rate: number
}

export interface ProductionOverview {
  range: ReportRange
  totals: ProductionTotals
  by_status: LotStatusStat[]
  by_product: ProductTypeStat[]
  by_edge: EdgeLotStat[]
}

function toParams(query: ReportQuery & { top?: number }): Record<string, string> {
  const out: Record<string, string> = {}
  if (query.start) out.start = query.start
  if (query.end) out.end = query.end
  if (query.top !== undefined) out.top = String(query.top)
  return out
}

export async function fetchQualityDistribution(
  query: ReportQuery & { top?: number },
): Promise<QualityDistribution> {
  const { data } = await http.get<ApiResponse<QualityDistribution>>(
    '/v1/reports/quality-distribution',
    { params: toParams(query) },
  )
  return data.data
}

export async function fetchMachineOperator(query: ReportQuery): Promise<MachineOperator> {
  const { data } = await http.get<ApiResponse<MachineOperator>>(
    '/v1/reports/machine-operator',
    { params: toParams(query) },
  )
  return data.data
}

export async function fetchProductionOverview(query: ReportQuery): Promise<ProductionOverview> {
  const { data } = await http.get<ApiResponse<ProductionOverview>>(
    '/v1/reports/production-overview',
    { params: toParams(query) },
  )
  return data.data
}
