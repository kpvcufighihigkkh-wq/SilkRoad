package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Project holds the schema definition for the Project entity.
// 项目表 - 生产项目管理
type Project struct {
	ent.Schema
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 项目名称
		field.String("project_name").
			Unique().
			NotEmpty().
			MaxLen(100).
			Comment("项目名称"),

		// 描述
		field.Text("description").
			Optional().
			Comment("项目描述"),

		// 客户名称
		field.String("customer_name").
			Optional().
			MaxLen(100).
			Comment("客户名称"),

		// 项目状态
		field.Enum("status").
			Values("active", "completed", "archived").
			Default("active").
			Comment("项目状态"),

		// 时间范围
		field.Time("start_date").
			Optional().
			Comment("开始日期"),

		field.Time("end_date").
			Optional().
			Comment("结束日期"),

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

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个项目有多个订单
		edge.To("orders", Order.Type),
	}
}

// Indexes of the Project.
func (Project) Indexes() []ent.Index {
	return []ent.Index{
		// 状态索引
		index.Fields("status"),

		// 客户名称索引
		index.Fields("customer_name"),

		// 创建时间倒序索引
		index.Fields("created_at"),
	}
}
