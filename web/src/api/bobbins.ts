import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { PaginatedResponse } from '@/types/api'

/** 丝锭状态枚举。与 schema bobbin.go 的 Values 一致。 */
export type BobbinStatus = 'producing' | 'completed' | 'packed' | 'shipped'

export const BOBBIN_STATUS_LABEL: Record<BobbinStatus, string> = {
  producing: '生产中',
  completed: '已完成',
  packed: '已打包',
  shipped: '已发货',
}

/**
 * 丝锭响应。与 service.BobbinResponse 一一对应。
 *
 * 注意 status 是 string 而非枚举：后端直接透传库值，前端做宽松处理，
 * 遇到未知值时原样展示而不是崩溃。
 */
export interface Bobbin {
  id: string
  bobbin_number: string
  lot_id: string
  spinning_position: number
  gross_weight: number
  net_weight: number
  tare_weight: number
  grade?: string
  /** 所属托盘。null 表示尚未码盘 —— 托盘详情页据此判断盘内丝锭。 */
  pallet_id: string | null
  status: string
  label_printed: boolean
  printed_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface BobbinListQuery extends PageQuery {
  lot_id?: string
  status?: BobbinStatus
}

export async function listBobbins(
  query: BobbinListQuery,
): Promise<{ list: Bobbin[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Bobbin>>('/v1/bobbins', { params })
  return toPageResult(data)
}
