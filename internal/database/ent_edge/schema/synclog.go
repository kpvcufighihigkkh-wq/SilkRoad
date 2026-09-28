package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SyncLog holds the schema definition for the SyncLog entity (Edge version).
// 同步日志表 - 记录边端到中心端的数据同步状态
type SyncLog struct {
	ent.Schema
}

// Fields of the SyncLog.
func (SyncLog) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 同步实体类型
		field.Enum("entity_type").
			Values("lot", "bobbin", "bobbin_grade").
			Comment("同步实体类型"),

		// 实体ID
		field.UUID("entity_id", uuid.UUID{}).
			Comment("实体ID"),

		// 同步操作
		field.Enum("operation").
			Values("create", "update", "delete").
			Comment("同步操作类型"),

		// 同步状态
		field.Enum("status").
			Values("pending", "success", "failed", "conflict").
			Default("pending").
			Comment("同步状态"),

		// 重试次数
		field.Int("retry_count").
			Default(0).
			Min(0).
			Comment("重试次数"),

		// 错误信息
		field.Text("error_message").
			Optional().
			Comment("错误信息"),

		// 数据快照（JSON）
		field.JSON("data_snapshot", map[string]interface{}{}).
			Optional().
			Comment("数据快照（用于冲突解决）"),

		// 同步时间
		field.Time("synced_at").
			Optional().
			Comment("成功同步时间"),

		// 元数据
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

// Edges of the SyncLog.
func (SyncLog) Edges() []ent.Edge {
	return nil
}

// Indexes of the SyncLog.
func (SyncLog) Indexes() []ent.Index {
	return []ent.Index{
		// 实体类型索引
		index.Fields("entity_type"),

		// 实体ID索引
		index.Fields("entity_id"),

		// 状态索引（查询待同步的记录）
		index.Fields("status"),

		// 创建时间倒序索引
		index.Fields("created_at"),

		// 复合索引：实体类型+实体ID（快速查询某实体的同步记录）
		index.Fields("entity_type", "entity_id"),

		// 复合索引：状态+创建时间（查询待同步的记录）
		index.Fields("status", "created_at"),
	}
}
