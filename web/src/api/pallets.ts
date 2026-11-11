import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 托盘类型：bobbin = POY/FDY 直接码锭，carton = DTY 码箱。 */
export type PalletType = 'bobbin' | 'carton'

export const PALLET_TYPE_LABEL: Record<PalletType, string> = {
  bobbin: '码丝锭',
  carton: '码纸箱',
}

export function palletTypeLabel(t: string): string {
  return PALLET_TYPE_LABEL[t as PalletType] ?? t
}

/** 托盘状态枚举。与 schema pallet.go 的 Values 一致。 */
export type PalletStatus = 'building' | 'completed' | 'shipped'

export const PALLET_STATUS_LABEL: Record<PalletStatus, string> = {
  building: '码垛中',
  completed: '已封托',
  shipped: '已发货',
}

/** 状态配色，与 antd Tag 的 color 取值对应。未知值回退灰色。 */
export const PALLET_STATUS_COLOR: Record<PalletStatus, string> = {
  building: 'processing',
  completed: 'success',
  shipped: 'default',
}

/** 状态中文名。库值未知时原样返回，不吞掉。 */
export function palletStatusLabel(status: string): string {
  return PALLET_STATUS_LABEL[status as PalletStatus] ?? status
}

/** 状态配色。未知值回退灰色。 */
export function palletStatusColor(status: string): string {
  return PALLET_STATUS_COLOR[status as PalletStatus] ?? 'default'
}

/**
 * 托盘响应。与 service.PalletResponse 一一对应。
 *
 * 三层重量为 null 表示**未称重**，与「称得 0 kg」是两件事 ——
 * 后者在物理上不可能出现，但前端仍需按 null 判断而不是 `!weight`。
 */
export interface Pallet {
  id: string
  pallet_code: string
  lot_id: string
  edge_id: string | null
  level: number
  pallet_type: string
  bobbins_count: number
  cartons_count: number
  status: string
  net_weight: number | null
  gross_weight: number | null
  tare_weight: number | null
  printed: boolean
  printed_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export interface PalletListQuery extends PageQuery {
  lot_id?: string
  status?: PalletStatus
}

export async function listPallets(
  query: PalletListQuery,
): Promise<{ list: Pallet[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.lot_id) params.lot_id = query.lot_id
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Pallet>>('/v1/pallets', { params })
  return toPageResult(data)
}

export async function getPallet(id: string): Promise<Pallet> {
  const { data } = await http.get<ApiResponse<Pallet>>(`/v1/pallets/${id}`)
  return data.data
}

/** 重量展示：null 显示为「—」，避免把未称重读成 0 kg。 */
export function formatWeight(v: number | null): string {
  return v === null ? '—' : `${v} kg`
}
