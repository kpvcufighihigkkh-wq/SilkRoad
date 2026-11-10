/**
 * 后端统一响应封装的类型定义。
 *
 * 与 api/response.go 一一对应，改动后端信封时这里必须同步。
 */

/** 成功响应：{ code: 0, message: "success", data, meta? } */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
  meta?: { request_id: string; timestamp: string }
}

/**
 * 分页响应：分页信息在**顶层 pagination**，不在 data 内。
 * 与 api.PaginatedResponse 对应。
 */
export interface PaginatedResponse<T> {
  code: number
  message: string
  data: T[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

/** 错误响应：{ code, message, error?, details? } */
export interface ApiError {
  code: number
  message: string
  error?: string
  details?: unknown
}

/** 错误码，与 api/response.go 的常量保持一致。 */
export const ErrorCode = {
  Success: 0,
  InvalidParams: 10001,
  Unauthorized: 20001,
  TokenInvalid: 20002,
  TokenExpired: 20003,
  Forbidden: 20004,
  BusinessRule: 30001,
  NotFound: 40001,
  ServerError: 50001,
} as const
