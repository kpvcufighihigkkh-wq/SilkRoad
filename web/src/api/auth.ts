import http from './client'
import type { ApiResponse, PaginatedResponse } from '@/types/api'
import type { CurrentUser } from '@/auth/session'

/** 登录请求。与 center.handleLogin 的绑定字段一致。 */
export interface LoginRequest {
  username: string
  password: string
}

/** 登录响应。后端返回 gin.H{"token":..., "user":{...}}。 */
export interface LoginResponse {
  token: string
  user: CurrentUser
}

export async function login(req: LoginRequest): Promise<LoginResponse> {
  const { data } = await http.post<ApiResponse<LoginResponse>>('/v1/login', req)
  return data.data
}

/** 当前登录用户。中心端每请求会按库里最新状态刷新，故可用于校验会话有效性。 */
export async function fetchCurrentUser(): Promise<CurrentUser> {
  const { data } = await http.get<ApiResponse<CurrentUser>>('/v1/users/me')
  return data.data
}

export type { CurrentUser }

/** 列表页共用的分页参数。 */
export interface PageQuery {
  page?: number
  page_size?: number
}

/**
 * 把后端的 PaginatedResponse 摊平成 { list, total }。
 *
 * 分页信息在顶层 pagination，不在 data 内 —— 这是后端 PageSuccess 的形状，
 * 与普通 ApiResponse 不同，容易写错，集中在这里转换一次。
 */
export function toPageResult<T>(resp: PaginatedResponse<T>): { list: T[]; total: number } {
  return { list: resp.data ?? [], total: resp.pagination?.total ?? 0 }
}
