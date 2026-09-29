package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SpinningLine holds the schema definition for the SpinningLine entity.
// 纺丝线体表 - 设备管理
type SpinningLine struct {
	ent.Schema
}

// Fields of the SpinningLine.
func (SpinningLine) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 线体名称
		field.String("line_name").
			Unique().
			NotEmpty().
			MaxLen(100).
			Comment("线体名称"),

		// 线体编号
		field.String("line_number").
			Optional().
			MaxLen(50).
			Comment("线体编号"),

		// 关联边端设备
		field.UUID("edge_id", uuid.UUID{}).
			Optional().
			Comment("关联边端设备ID"),

		// 位置信息
		field.String("location").
			Optional().
			MaxLen(100).
			Comment("线体位置"),

		// 产能（位号数量）
		field.Int("capacity").
			Optional().
			Positive().
			Comment("线体产能（位号数量）"),

		// 状态
		field.Enum("status").
			Values("idle", "running", "maintenance", "offline").
			Default("idle").
			Comment("线体状态"),

		// 当前批次
		field.UUID("current_lot_id", uuid.UUID{}).
			Optional().
			Comment("当前生产批次ID"),

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

// Edges of the SpinningLine.
func (SpinningLine) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个纺丝线属于一个边端设备
		edge.From("edge", Edge.Type).
			Ref("spinning_lines").
			Field("edge_id").
			Unique(),
	}
}

// Indexes of the SpinningLine.
func (SpinningLine) Indexes() []ent.Index {
	return []ent.Index{
		// 边端设备ID索引
		index.Fields("edge_id"),

		// 状态索引
		index.Fields("status"),

		// 复合索引：边端设备+状态
		index.Fields("edge_id", "status"),
	}
}
