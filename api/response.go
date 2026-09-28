package api

import (
	"time"
)

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Code       int         `json:"code"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
	Pagination *Pagination `json:"pagination"`
	Meta       *Meta       `json:"meta,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Details interface{} `json:"details,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta 元数据
type Meta struct {
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// 错误码常量
const (
	// 成功
	CodeSuccess = 0

	// 通用错误 (100xx)
	CodeParamError        = 10001
	CodeInvalidParams     = 10001 // 别名，保持兼容
	CodeMissingParam      = 10002
	CodeParamFormatError  = 10003

	// 认证错误 (200xx)
	CodeUnauthorized      = 20001
	CodeTokenInvalid      = 20002
	CodeTokenExpired      = 20003
	CodeForbidden         = 20004

	// 业务错误 (300xx)
	CodeBusinessRuleError = 30001
	CodeStatusNotAllowed  = 30002

	// 数据错误 (400xx)
	CodeResourceNotFound  = 40001
	CodeNotFound          = 40001 // 别名，保持兼容
	CodeResourceExists    = 40002
	CodeDataConflict      = 40003

	// 系统错误 (500xx)
	CodeInternalError     = 50001
	CodeServerError       = 50001 // 别名，保持兼容
	CodeDatabaseError     = 50002
	CodePLCError          = 50003
)

// Success 成功响应
func Success(data interface{}) *Response {
	return &Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	}
}

// SuccessWithMeta 带元数据的成功响应
func SuccessWithMeta(data interface{}, requestID string) *Response {
	return &Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Meta: &Meta{
			RequestID: requestID,
			Timestamp: time.Now(),
		},
	}
}

// PageSuccess 分页成功响应
func PageSuccess(data interface{}, page, pageSize, total int) *PaginatedResponse {
	totalPages := (total + pageSize - 1) / pageSize
	return &PaginatedResponse{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
		Pagination: &Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// Error 错误响应
func Error(code int, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
	}
}

// ErrorWithDetails 带详情的错误响应
func ErrorWithDetails(code int, message string, errorType string, details interface{}) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
		Error:   errorType,
		Details: details,
	}
}

// ErrorWithMeta 带元数据的错误响应
func ErrorWithMeta(code int, message string, requestID string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
		Meta: &Meta{
			RequestID: requestID,
			Timestamp: time.Now(),
		},
	}
}
