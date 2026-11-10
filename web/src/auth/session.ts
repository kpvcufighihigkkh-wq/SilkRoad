/**
 * 登录态存储。
 *
 * 只保存 token 与登录时拿到的用户信息。token 内的角色是签发时的快照，
 * 但它仅用于「菜单显示什么」这类体验优化 —— 真正的授权判定在后端
 * （requireActiveUser 每请求回查数据库）。因此前端不因 token 过期外的
 * 原因自行降级，而是让 401/403 由请求层统一处理。
 */

export type Role = 'admin' | 'operator' | 'inspector' | 'viewer'

export interface CurrentUser {
  id: string
  username: string
  full_name: string
  role: Role
}

const TOKEN_KEY = 'silkroad.token'
const USER_KEY = 'silkroad.user'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getUser(): CurrentUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null

  try {
    return JSON.parse(raw) as CurrentUser
  } catch {
    // 存储被破坏时按未登录处理，而不是让应用在解析处崩溃。
    clearSession()
    return null
  }
}

export function saveSession(token: string, user: CurrentUser): void {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export function isLoggedIn(): boolean {
  return getToken() !== null
}
