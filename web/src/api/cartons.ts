import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 纸箱状态枚举。与 schema carton.go 的 Values 一致。 */
export type CartonStatus = 'packing' | 'packed' | 'shipped'

export const CARTON_STATUS_LABEL: Record<CartonStatus, string> = {
  packing: '装箱中',
  packed: '已封箱',
  shipped: '已发货',
}

const CARTON_STATUS_COLOR: Record<CartonStatus, string> = {
  packing: 'processing',
  packed: 'success',
  shipped: 'default',
}

export function cartonStatusLabel(status: string): string {
  return CARTON_STATUS_LABEL[status as CartonStatus] ?? status
}

export function cartonStatusColor(status: string): string {
  return CARTON_STATUS_COLOR[status as CartonStatus] ?? 'default'
}

/**
 * 纸箱响应。与 service.CartonResponse 一一对应。
 *
 * lot_id / pallet_id 都是可空的：箱子建出来时可能还没绑定批次，
 * 也可能尚未码到托盘上。
 */
export interface Carton {
  id: string
  carton_number: string
  lot_id: string | null
  pallet_id: string | null
  bobbin_count: number
  capacity: number
  total_weight: number | null
  status: string
  label_printed: boolean
  printed_at: string | null
  packed_at: string | null
  created_at: string
  updated_at: string
}

export interface CartonListQuery extends PageQuery {
  lot_id?: string
  pallet_id?: string
  status?: CartonStatus
}

export async function listCartons(
  query: CartonListQuery,
): Promise<{ list: Carton[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.pallet_id) params.pallet_id = query.pallet_id
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Carton>>('/v1/cartons', { params })
  return toPageResult(data)
}

export async function getCarton(id: string): Promise<Carton> {
  const { data } = await http.get<ApiResponse<Carton>>(`/v1/cartons/${id}`)
  return data.data
}

/** 重量展示：null 显示为「—」，与托盘同一约定。 */
export function formatCartonWeight(v: number | null): string {
  return v === null ? '—' : `${v} kg`
}
