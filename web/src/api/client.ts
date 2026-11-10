import axios, { AxiosError, type AxiosInstance } from 'axios'
import type { ApiError } from '@/types/api'
import { clearSession, getToken } from '@/auth/session'

/**
 * 请求层统一处理：
 *   1. 自动注入 Bearer token；
 *   2. 把后端的 { code, message } 错误信封转成 Error(message)，
 *      让调用方不必到处解包；
 *   3. 401 清空登录态并跳回登录页 —— token 过期或账号被停用都会走到这里。
 *
 * 注意不用 window.location.href 跳转：那会整页刷新并丢失当前路由。
 * 这里只清 session 并派发事件，由 AuthGate 统一响应。
 */

export const UNAUTHORIZED_EVENT = 'silkroad:unauthorized'

const http: AxiosInstance = axios.create({
  baseURL: '/',
  timeout: 15000,
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (resp) => resp,
  (error: AxiosError<ApiError>) => {
    // 无响应（网络故障、超时）
    if (!error.response) {
      return Promise.reject(new Error('网络异常，请稍后重试'))
    }

    const { status, data } = error.response

    if (status === 401) {
      clearSession()
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }

    // 后端失败响应统一带 message；没有时退回状态码描述。
    const message =
      data && typeof data === 'object' && 'message' in data && data.message
        ? data.message
        : `请求失败（HTTP ${status}）`

    return Promise.reject(new Error(message))
  },
)

export default http
