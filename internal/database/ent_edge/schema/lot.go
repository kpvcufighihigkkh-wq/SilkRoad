package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Lot holds the schema definition for the Lot entity (Edge version).
// 批次表 - 边端简化版，只存储当前生产批次
type Lot struct {
	ent.Schema
}

// Fields of the Lot.
func (Lot) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 批次编号（唯一）
		field.String("lot_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("批次编号"),

		// 关联订单ID（仅记录，不建立外键）
		field.UUID("order_id", uuid.UUID{}).
			Comment("关联订单ID"),

		// 产品信息
		field.Enum("product_type").
			Values("FDY", "POY", "DTY").
			Comment("产品类型"),

		field.String("product_spec").
			Optional().
			MaxLen(100).
			Comment("产品规格"),

		// 计划数量
		field.Int("planned_quantity").
			Positive().
			Comment("计划生产数量（锭）"),

		field.Int("actual_quantity").
			Default(0).
			Min(0).
			Comment("实际生产数量（锭）"),

		// 批次状态
		field.Enum("status").
			Values("in_progress", "paused", "completed", "cancelled").
			Default("in_progress").
			Comment("批次状态"),

		// 时间
		field.Time("start_time").
			Optional().
			Comment("开始时间"),

		field.Time("end_time").
			Optional().
			Comment("结束时间"),

		// 同步状态
		field.Bool("synced").
			Default(false).
			Comment("是否已同步到中心端"),

		field.Time("synced_at").
			Optional().
			Comment("同步时间"),

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

// Edges of the Lot.
func (Lot) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个批次有多个丝锭
		edge.To("bobbins", Bobbin.Type),
	}
}

// Indexes of the Lot.
func (Lot) Indexes() []ent.Index {
	return []ent.Index{
		// 状态索引
		index.Fields("status"),

		// 同步状态索引（查询未同步的批次）
		index.Fields("synced"),

		// 创建时间倒序索引
		index.Fields("created_at"),
	}
}
