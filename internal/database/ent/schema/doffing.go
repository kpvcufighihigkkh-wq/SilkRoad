package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Doffing holds the schema definition for the Doffing entity.
// 落纱操作表 - 记录落纱操作
type Doffing struct {
	ent.Schema
}

// Fields of the Doffing.
func (Doffing) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 线体ID
		field.UUID("spinning_line_id", uuid.UUID{}).
			Comment("线体ID"),

		// 纺丝位号
		field.Int("spinning_position").
			Positive().
			Comment("纺丝位号"),

		// 批次ID
		field.UUID("lot_id", uuid.UUID{}).
			Comment("批次ID"),

		// 操作员ID
		field.UUID("operator_id", uuid.UUID{}).
			Optional().
			Comment("操作员ID"),

		// 落纱状态
		field.Enum("status").
			Values("pending", "confirmed", "cancelled").
			Default("pending").
			Comment("落纱状态"),

		// 丝锭编号（确认后填写）
		field.String("bobbin_number").
			Optional().
			MaxLen(50).
			Comment("丝锭编号"),

		// 实际重量（确认后填写）
		field.Float("actual_weight").
			Optional().
			Positive().
			Comment("实际重量（kg）"),

		// 等级（确认后填写）
		field.String("grade").
			Optional().
			MaxLen(10).
			Comment("丝锭等级"),

		// 取消原因
		field.Text("cancel_reason").
			Optional().
			Comment("取消原因"),

		// 落纱时间
		field.Time("doffing_time").
			Default(time.Now).
			Comment("落纱时间"),

		// 确认时间
		field.Time("confirmed_at").
			Optional().
			Comment("确认时间"),

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

// Edges of the Doffing.
func (Doffing) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个落纱记录属于一个批次
		edge.From("lot", Lot.Type).
			Ref("doffings").
			Field("lot_id").
			Required().
			Unique(),
	}
}

// Indexes of the Doffing.
func (Doffing) Indexes() []ent.Index {
	return []ent.Index{
		// 线体ID索引
		index.Fields("spinning_line_id"),

		// 批次ID索引
		index.Fields("lot_id"),

		// 状态索引
		index.Fields("status"),

		// 落纱时间索引
		index.Fields("doffing_time"),

		// 复合索引：线体+位号+状态
		index.Fields("spinning_line_id", "spinning_position", "status"),
	}
}
