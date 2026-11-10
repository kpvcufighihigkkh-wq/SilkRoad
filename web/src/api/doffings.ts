import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { PaginatedResponse } from '@/types/api'

/** 落纱状态枚举。与 schema doffing.go 的 Values 一致。 */
export type DoffingStatus = 'pending' | 'confirmed' | 'cancelled'

export const DOFFING_STATUS_LABEL: Record<DoffingStatus, string> = {
  pending: '待确认',
  confirmed: '已确认',
  cancelled: '已作废',
}

/**
 * 落纱记录。与 service.DoffingResponse 一一对应。
 *
 * 注意 operator_id 在响应里是 omitempty 的空串而非 null —— 后端用
 * uuid.Nil 判断后不赋值。前端统一按「空串 = 未记录」处理。
 */
export interface Doffing {
  id: string
  spinning_line_id: string
  spinning_position: number
  lot_id: string
  operator_id?: string
  status: DoffingStatus
  bobbin_number?: string
  actual_weight?: number
  grade?: string
  cancel_reason?: string
  doffing_time: string
  confirmed_at?: string
  created_at: string
  updated_at: string
}

export interface DoffingListQuery extends PageQuery {
  lot_id?: string
  spinning_line_id?: string
  operator_id?: string
  status?: DoffingStatus
}

export async function listDoffings(
  query: DoffingListQuery,
): Promise<{ list: Doffing[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.spinning_line_id) params.spinning_line_id = query.spinning_line_id
  if (query.operator_id) params.operator_id = query.operator_id
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Doffing>>('/v1/doffings', { params })
  return toPageResult(data)
}
