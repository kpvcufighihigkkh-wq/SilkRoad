import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 设备状态枚举。与 schema edge.go 的 Values 一致。 */
export type EdgeStatus = 'online' | 'offline' | 'error' | 'maintenance'

export const EDGE_STATUS_LABEL: Record<EdgeStatus, string> = {
  online: '在线',
  offline: '离线',
  error: '故障',
  maintenance: '维护中',
}

/** 设备响应。与 service.EdgeResponse 一一对应。 */
export interface Edge {
  id: string
  edge_code: string
  edge_name: string
  ip_address?: string
  status: EdgeStatus
  version?: string
  last_seen?: string
  created_at: string
  updated_at: string
}

export interface EdgeListQuery extends PageQuery {
  status?: EdgeStatus
}

export interface CreateEdgeInput {
  edge_code: string
  edge_name: string
  ip_address: string
  version?: string
  notes?: string
}

export interface IssueTokenResult {
  token: string
  edge_code: string
}

export async function listEdges(query: EdgeListQuery): Promise<{ list: Edge[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.status) params.status = query.status

  const { data } = await http.get<PaginatedResponse<Edge>>('/v1/edges', { params })
  return toPageResult(data)
}

export async function createEdge(input: CreateEdgeInput): Promise<Edge> {
  const { data } = await http.post<ApiResponse<Edge>>('/v1/edges', input)
  return data.data
}

/**
 * 签发设备凭证。
 *
 * 这是敏感操作：签出的 token 在有效期内可代表该设备上传数据，
 * 因此调用方必须先做二次确认，并明确告知用户有效期。
 */
export async function issueEdgeToken(code: string): Promise<IssueTokenResult> {
  const { data } = await http.post<ApiResponse<IssueTokenResult>>(`/v1/edges/${code}/token`)
  return data.data
}
