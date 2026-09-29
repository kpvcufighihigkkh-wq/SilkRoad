package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Grade holds the schema definition for the Grade entity.
// 等级表 - 统一质检等级定义（替代V2的多个grade表）
type Grade struct {
	ent.Schema
}

// Fields of the Grade.
func (Grade) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 等级类型
		field.Enum("grade_type").
			Values("sorting", "weight", "final", "vision", "knitting").
			Comment("等级类型：sorting-分拣等级, weight-重量等级, final-最终等级, vision-外观等级, knitting-编织等级"),

		// 等级编码
		field.String("grade_code").
			NotEmpty().
			MaxLen(20).
			Comment("等级编码（如：A, B, C, D, 废品）"),

		// 等级名称
		field.String("grade_name").
			NotEmpty().
			MaxLen(50).
			Comment("等级名称"),

		// 描述
		field.Text("description").
			Optional().
			Comment("等级描述"),

		// 排序顺序
		field.Int("sort_order").
			Default(0).
			Comment("排序顺序（数字越小优先级越高）"),

		// 是否启用
		field.Bool("is_active").
			Default(true).
			Comment("是否启用"),

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

// Edges of the Grade.
func (Grade) Edges() []ent.Edge {
	return nil // Grade是基础数据，通过字符串关联到Bobbin
}

// Indexes of the Grade.
func (Grade) Indexes() []ent.Index {
	return []ent.Index{
		// 等级类型索引
		index.Fields("grade_type"),

		// 等级编码索引
		index.Fields("grade_code"),

		// 是否启用索引
		index.Fields("is_active"),

		// 排序顺序索引
		index.Fields("sort_order"),

		// 复合唯一索引：类型+编码
		index.Fields("grade_type", "grade_code").
			Unique(),
	}
}
