package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Order holds the schema definition for the Order entity.
// 订单表 - 客户订单管理
type Order struct {
	ent.Schema
}

// Fields of the Order.
func (Order) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 订单编号（唯一）
		field.String("order_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("订单编号"),

		// 关联项目
		field.UUID("project_id", uuid.UUID{}).
			Optional().
			Comment("关联项目ID"),

		// 产品信息
		field.Enum("product_type").
			Values("FDY", "POY", "DTY").
			Comment("产品类型"),

		field.String("product_spec").
			Optional().
			MaxLen(100).
			Comment("产品规格"),

		// 订单数量
		field.Int("target_quantity").
			Positive().
			Comment("目标数量（锭）"),

		field.Int("actual_quantity").
			Default(0).
			NonNegative().
			Comment("实际完成数量（锭）"),

		// 订单状态
		field.Enum("status").
			Values("pending", "in_progress", "completed", "cancelled").
			Default("pending").
			Comment("订单状态"),

		// 优先级
		field.Int("priority").
			Default(0).
			Comment("优先级"),

		// 交期
		field.Time("delivery_date").
			Optional().
			Comment("交货日期"),

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

// Edges of the Order.
func (Order) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个订单属于一个项目
		edge.From("project", Project.Type).
			Ref("orders").
			Field("project_id").
			Unique(),

		// 一个订单有多个批次
		edge.To("lots", Lot.Type),
	}
}

// Indexes of the Order.
func (Order) Indexes() []ent.Index {
	return []ent.Index{
		// 项目ID索引
		index.Fields("project_id"),

		// 产品类型索引
		index.Fields("product_type"),

		// 状态索引
		index.Fields("status"),

		// 优先级索引
		index.Fields("priority"),

		// 交货日期索引
		index.Fields("delivery_date"),
	}
}
