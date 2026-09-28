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

		// 项目编号（唯一）
		field.String("project_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("项目编号"),

		// 项目名称
		field.String("project_name").
			NotEmpty().
			MaxLen(200).
			Comment("项目名称"),

		// 产品类型
		field.Enum("product_type").
			Values("FDY", "POY", "DTY").
			Comment("产品类型"),

		// 产品规格
		field.String("product_spec").
			MaxLen(100).
			Comment("产品规格"),

		// 项目状态
		field.Enum("status").
			Values("planning", "in_progress", "completed", "cancelled").
			Default("planning").
			Comment("项目状态"),

		// 计划数量
		field.Int("planned_quantity").
			Optional().
			NonNegative().
			Comment("计划生产数量（锭）"),

		// 实际数量
		field.Int("actual_quantity").
			Default(0).
			NonNegative().
			Comment("实际生产数量（锭）"),

		// 时间范围
		field.Time("start_date").
			Optional().
			Comment("计划开始日期"),

		field.Time("end_date").
			Optional().
			Comment("计划结束日期"),

		// 备注
		field.Text("notes").
			Optional().
			Comment("备注信息"),

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
		// 产品类型索引
		index.Fields("product_type"),

		// 状态索引
		index.Fields("status"),

		// 创建时间倒序索引
		index.Fields("created_at"),

		// 开始日期索引
		index.Fields("start_date"),
	}
}
