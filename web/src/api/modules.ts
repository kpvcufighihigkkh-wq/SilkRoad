import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 吊车状态枚举。与 schema module.go 的 Values 一致。 */
export type ModuleStatus = 'loading' | 'transporting' | 'sorting' | 'warehouse' | 'idle'

export const MODULE_STATUS_LABEL: Record<ModuleStatus, string> = {
  loading: '装车中',
  transporting: '运输中',
  sorting: '分拣中',
  warehouse: '已入库',
  idle: '空闲',
}

const MODULE_STATUS_COLOR: Record<ModuleStatus, string> = {
  loading: 'processing',
  transporting: 'blue',
  sorting: 'warning',
  warehouse: 'success',
  idle: 'default',
}

/** 状态中文名。库值未知时原样返回。 */
export function moduleStatusLabel(status: string): string {
  return MODULE_STATUS_LABEL[status as ModuleStatus] ?? status
}

export function moduleStatusColor(status: string): string {
  return MODULE_STATUS_COLOR[status as ModuleStatus] ?? 'default'
}

/**
 * 吊车响应。与 service.ModuleResponse 一一对应。
 *
 * 两个挂位都可空：空挂位是 null，不是全零 UUID。
 */
export interface Module {
  id: string
  module_number: string
  barrel1_id: string | null
  barrel2_id: string | null
  status: string
  rfid: string
  current_location: string
  created_at: string
  updated_at: string
}

export interface ModuleListQuery extends PageQuery {
  status?: ModuleStatus
}

export async function listModules(
  query: ModuleListQuery,
): Promise<{ list: Module[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Module>>('/v1/modules', { params })
  return toPageResult(data)
}

export async function getModule(id: string): Promise<Module> {
  const { data } = await http.get<ApiResponse<Module>>(`/v1/modules/${id}`)
  return data.data
}
