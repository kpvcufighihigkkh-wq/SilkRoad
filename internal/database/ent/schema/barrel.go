package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Barrel holds the schema definition for the Barrel entity.
// 落纱桶表 - 载具管理（V2的work_bobbins概念）
type Barrel struct {
	ent.Schema
}

// Fields of the Barrel.
func (Barrel) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 桶编号（唯一）
		field.String("barrel_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("落纱桶编号"),

		// 关联落纱记录
		field.UUID("doffing_id", uuid.UUID{}).
			Comment("关联落纱记录ID"),

		// 关联批号
		field.UUID("lot_id", uuid.UUID{}).
			Comment("关联批号ID"),

		// 容量（通常9个锭位）
		field.Int("capacity").
			Default(9).
			Positive().
			Comment("桶容量（锭位数）"),

		// 当前装载数量
		field.Int("current_count").
			Default(0).
			NonNegative().
			Comment("当前装载丝饼数量"),

		// 状态
		field.Enum("status").
			Values("active", "full", "sorted", "packed").
			Default("active").
			Comment("落纱桶状态"),

		// 时间戳
		field.Time("filled_at").
			Optional().
			Comment("装满时间"),

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

// Edges of the Barrel.
func (Barrel) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个落纱桶属于一个落纱记录
		edge.From("doffing", Doffing.Type).
			Ref("barrels").
			Field("doffing_id").
			Required().
			Unique(),

		// 一个落纱桶属于一个批号
		edge.From("lot", Lot.Type).
			Ref("barrels").
			Field("lot_id").
			Required().
			Unique(),

		// 一个落纱桶有多个丝饼
		edge.To("bobbins", Bobbin.Type),
	}
}

// Indexes of the Barrel.
func (Barrel) Indexes() []ent.Index {
	return []ent.Index{
		// 落纱ID索引
		index.Fields("doffing_id"),

		// 批号ID索引
		index.Fields("lot_id"),

		// 状态索引
		index.Fields("status"),

		// 创建时间倒序索引
		index.Fields("created_at"),

		// 复合索引：批号+状态
		index.Fields("lot_id", "status"),
	}
}
