import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { PaginatedResponse } from '@/types/api'

/** 批次状态枚举。与 schema lot.go 的 Values 一致。 */
export type LotStatus = 'in_progress' | 'paused' | 'completed' | 'cancelled'

/** 产品类型枚举。 */
export type ProductType = 'FDY' | 'POY' | 'DTY'

/**
 * 批次响应。与 service.LotResponse 一一对应。
 *
 * 注意 progress 由后端计算并下发（service/lot.go），前端**不得**用
 * actual/planned 自己算 —— 后端已处理 planned_quantity 为 0 的边界，
 * 前端重算会引入除零和口径不一致。
 */
export interface Lot {
  id: string
  lot_number: string
  edge_id?: string
  plc_lot_number?: string
  order_code?: string
  product_type: ProductType
  product_spec?: string
  status: LotStatus
  planned_quantity: number
  actual_quantity: number
  progress: number
  start_time?: string
  end_time?: string
  created_at: string
  updated_at: string
}

/** 批次列表筛选条件。全部可选，空值不参与查询。 */
export interface LotListQuery extends PageQuery {
  status?: LotStatus
  edge_id?: string
  order_code?: string
  product_type?: ProductType
  keyword?: string
}

export async function listLots(query: LotListQuery): Promise<{ list: Lot[]; total: number }> {
  const params = Object.fromEntries(
    Object.entries(query).filter(([, v]) => v !== undefined && v !== null && v !== ''),
  )

  const { data } = await http.get<PaginatedResponse<Lot>>('/v1/lots', { params })
  return toPageResult(data)
}

export async function getLot(id: string): Promise<Lot> {
  const { data } = await http.get<{ code: number; message: string; data: Lot }>(`/v1/lots/${id}`)
  return data.data
}
