import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 桶状态枚举。与 schema barrel.go 的 Values 一致。 */
export type BarrelStatus = 'active' | 'full' | 'sorted' | 'packed'

export const BARREL_STATUS_LABEL: Record<BarrelStatus, string> = {
  active: '装载中',
  full: '已装满',
  sorted: '已分拣',
  packed: '已打包',
}

const BARREL_STATUS_COLOR: Record<BarrelStatus, string> = {
  active: 'processing',
  full: 'warning',
  sorted: 'default',
  packed: 'success',
}

/** 状态中文名。库值未知时原样返回。 */
export function barrelStatusLabel(status: string): string {
  return BARREL_STATUS_LABEL[status as BarrelStatus] ?? status
}

export function barrelStatusColor(status: string): string {
  return BARREL_STATUS_COLOR[status as BarrelStatus] ?? 'default'
}

/** 桶响应。与 service.BarrelResponse 一一对应。 */
export interface Barrel {
  id: string
  barrel_number: string
  lot_id: string
  capacity: number
  current_count: number
  status: string
  filled_at: string | null
  created_at: string
  updated_at: string
}

export interface BarrelListQuery extends PageQuery {
  lot_id?: string
  status?: BarrelStatus
}

export async function listBarrels(
  query: BarrelListQuery,
): Promise<{ list: Barrel[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Barrel>>('/v1/barrels', { params })
  return toPageResult(data)
}

export async function getBarrel(id: string): Promise<Barrel> {
  const { data } = await http.get<ApiResponse<Barrel>>(`/v1/barrels/${id}`)
  return data.data
}
