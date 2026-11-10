import http from './client'
import { toPageResult, type PageQuery } from './auth'
import type { ApiResponse, PaginatedResponse } from '@/types/api'

/** 角色枚举。与 schema user.go 的 Values 一致。 */
export type Role = 'admin' | 'operator' | 'inspector' | 'viewer'

export const ROLE_LABEL: Record<Role, string> = {
  admin: '管理员',
  operator: '操作员',
  inspector: '质检员',
  viewer: '只读用户',
}

/** 角色权重，用于前端限制「不能给别人设比自己高的角色」。 */
export const ROLE_RANK: Record<Role, number> = {
  admin: 4,
  inspector: 3,
  operator: 2,
  viewer: 1,
}

/**
 * 用户响应。与 service.UserResponse 一一对应。
 *
 * 注意：spec §6.1 还要求 employee_id 与 last_login_at，但后端响应里没有
 * 这两个字段（且登录流程从不写 last_login_at），因此前端不展示。
 */
export interface User {
  id: string
  username: string
  real_name: string
  role: Role
  email?: string
  phone?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface UserListQuery extends PageQuery {
  role?: Role
}

export interface CreateUserInput {
  username: string
  password: string
  real_name: string
  role: Role
  email?: string
  phone?: string
}

/**
 * 更新用户。三个可选字段都是三态：
 *   undefined → 不传，后端保持不变
 *   ""        → 显式清空（仅 email / phone）
 *   其他值     → 设为该值
 */
export interface UpdateUserInput {
  real_name?: string
  email?: string
  phone?: string
  role?: Role
  is_active?: boolean
}

export interface ChangePasswordInput {
  old_password: string
  new_password: string
}

export async function listUsers(query: UserListQuery): Promise<{ list: User[]; total: number }> {
  const params: Record<string, string | number> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 20,
  }
  if (query.role) params.role = query.role

  const { data } = await http.get<PaginatedResponse<User>>('/v1/users', { params })
  return toPageResult(data)
}

export async function createUser(input: CreateUserInput): Promise<User> {
  const { data } = await http.post<ApiResponse<User>>('/v1/users', input)
  return data.data
}

export async function updateUser(id: string, input: UpdateUserInput): Promise<void> {
  await http.put(`/v1/users/${id}`, input)
}

export async function deleteUser(id: string): Promise<void> {
  await http.delete(`/v1/users/${id}`)
}

/** 改密：非管理员只能改自己的，且必须提供旧密码。 */
export async function changePassword(id: string, input: ChangePasswordInput): Promise<void> {
  await http.put(`/v1/users/${id}/password`, input)
}
