package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Edge holds the schema definition for the Edge entity.
// 边端设备表 - 边端设备注册和管理
type Edge struct {
	ent.Schema
}

// Fields of the Edge.
func (Edge) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 边端设备编码（唯一）
		field.String("edge_code").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("边端设备编码（如：edge-001, line-01）"),

		// 边端设备名称
		field.String("edge_name").
			NotEmpty().
			MaxLen(100).
			Comment("边端设备名称"),

		// IP地址
		field.String("ip_address").
			Optional().
			MaxLen(50).
			Comment("边端设备IP地址"),

		// 设备状态
		field.Enum("status").
			Values("online", "offline", "error", "maintenance").
			Default("offline").
			Comment("设备状态"),

		// 最后心跳时间
		field.Time("last_seen").
			Optional().
			Comment("最后心跳时间"),

		// 软件版本
		field.String("version").
			Optional().
			MaxLen(50).
			Comment("软件版本号"),

		// 配置信息（JSON）
		field.JSON("config", map[string]interface{}{}).
			Optional().
			Comment("设备配置（JSON格式）"),

		// 备注
		field.Text("notes").
			Optional().
			Comment("备注信息"),

		// 时间戳
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

// Edges of the Edge.
func (Edge) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个边端设备有多个纺丝线
		edge.To("spinning_lines", SpinningLine.Type),

		// 一个边端设备产生多个批号
		edge.To("lots", Lot.Type),
	}
}

// Indexes of the Edge.
func (Edge) Indexes() []ent.Index {
	return []ent.Index{
		// 状态索引
		index.Fields("status"),

		// 最后心跳时间索引
		index.Fields("last_seen"),

		// 创建时间倒序索引
		index.Fields("created_at"),
	}
}
