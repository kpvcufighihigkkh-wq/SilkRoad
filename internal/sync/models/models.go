package models

import (
	"time"

	"github.com/google/uuid"
)

// UploadEntry 表示一条待上传的数据记录
type UploadEntry struct {
	Table     string                 `json:"table"`      // 表名
	Operation string                 `json:"operation"`  // create/update/delete
	ID        uuid.UUID              `json:"id"`         // 记录ID
	Data      map[string]interface{} `json:"data"`       // 记录数据
	Version   int64                  `json:"version"`    // 版本号（用于冲突检测）
	CreatedAt time.Time              `json:"created_at"` // 创建时间
}

// UploadRequest 边端上传请求
type UploadRequest struct {
	// EdgeID 在 Center 侧承载**设备行的 UUID**（edges.id），不是 edge_code。
	//
	// 上传路径上该字段由 Center 的鉴权层用权威身份覆写，载荷里的值一律忽略；
	// HandleUpload 会 uuid.Parse 它并以此为数据归属，解析失败整批拒绝。
	// Edge 侧构造请求时填的是自己的 EDGE_ID，但那个值在 Center 侧会被丢弃 ——
	// 传 edge_code（如 "edge-001"）会解析失败。注意与下方
	// BaseDataPullRequest.EdgeID 区分：那个字段装的是 edge_code。
	EdgeID string `json:"edge_id"`
	// Entries 批量上传的记录
	Entries []UploadEntry `json:"entries"`
	// Cursor 当前游标位置
	Cursor int64 `json:"cursor"`
}

// UploadResponse 服务端上传响应
type UploadResponse struct {
	Applied  int      `json:"applied"`  // 成功应用的记录数
	Rejected int      `json:"rejected"` // 被拒绝的记录数
	Errors   []string `json:"errors"`   // 错误信息列表
	NewCursor int64   `json:"new_cursor"` // 新的游标位置
}

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	// ConflictServerWins 服务器优先（默认）
	ConflictServerWins ConflictResolution = "server_wins"
	// ConflictEdgeWins 边端优先
	ConflictEdgeWins ConflictResolution = "edge_wins"
	// ConflictMerge 尝试合并
	ConflictMerge ConflictResolution = "merge"
)

// SyncCursor 同步游标，记录各表的上传进度
type SyncCursor struct {
	EdgeID    string    `json:"edge_id"`    // 边端ID
	Table     string    `json:"table"`      // 表名
	Cursor    int64     `json:"cursor"`     // 游标位置（最后上传的ID）
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// BaseDataPullRequest 基础数据拉取请求
type BaseDataPullRequest struct {
	// EdgeID 在下发路径上装的是 **edge_code**（如 "edge-001"），不是设备行 UUID。
	//
	// Center 的 handleBaseDataPull 用 edgeRow.EdgeCode 填充它，取值来自鉴权结果；
	// BaseDataProvider 用它按 edge_code 过滤线体。注意与 UploadRequest.EdgeID
	// 区分：那个字段装的是设备行 UUID。
	EdgeID string `json:"edge_id"`
	// Tables 需要拉取的表名列表
	Tables []string `json:"tables"`
}

// BaseDataPullResponse 基础数据拉取响应
type BaseDataPullResponse struct {
	Data map[string][]map[string]interface{} `json:"data"` // 表名 -> 记录列表
}
