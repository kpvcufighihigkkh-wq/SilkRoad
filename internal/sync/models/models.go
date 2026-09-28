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
	EdgeID  string        `json:"edge_id"`  // 边端ID
	Entries []UploadEntry `json:"entries"`  // 批量上传的记录
	Cursor  int64         `json:"cursor"`   // 当前游标位置
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
	EdgeID string   `json:"edge_id"` // 边端ID
	Tables []string `json:"tables"`  // 需要拉取的表名列表
}

// BaseDataPullResponse 基础数据拉取响应
type BaseDataPullResponse struct {
	Data map[string][]map[string]interface{} `json:"data"` // 表名 -> 记录列表
}
