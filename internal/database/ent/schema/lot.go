package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Lot holds the schema definition for the Lot entity.
// 批次表 - 生产批次管理
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
			Comment("批次编号，格式: {产品类型}-{年}-{计划号}-{批次号}"),

		// 关联边端设备
		field.UUID("edge_id", uuid.UUID{}).
			Optional().
			Comment("来源边端设备ID"),

		// PLC原始批号
		field.String("plc_lot_number").
			Optional().
			MaxLen(50).
			Comment("PLC原始批号"),

		// ERP订单编号（可选，用于ERP集成）
		field.String("order_code").
			Optional().
			MaxLen(50).
			Comment("ERP订单编号（可选）"),

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
			NonNegative().
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

		// 锁定状态（防止修改）
		field.Bool("is_locked").
			Default(false).
			Comment("是否锁定（锁定后不可修改）"),

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
		// 一个批次属于一个边端设备
		edge.From("edge", Edge.Type).
			Ref("lots").
			Field("edge_id").
			Unique(),

		// 一个批次有多个落纱桶
		edge.To("barrels", Barrel.Type),

		// 一个批次有多个丝锭
		edge.To("bobbins", Bobbin.Type),

		// 一个批次有多个落纱记录
		edge.To("doffings", Doffing.Type),
	}
}

// Indexes of the Lot.
func (Lot) Indexes() []ent.Index {
	return []ent.Index{
		// 边端设备ID索引
		index.Fields("edge_id"),

		// 产品类型索引
		index.Fields("product_type"),

		// 状态索引（部分索引）
		index.Fields("status").
			Annotations(
				// 只索引非completed状态（PostgreSQL部分索引）
				// WHERE status != 'completed'
			),

		// 创建时间倒序索引
		index.Fields("created_at"),

		// 复合索引：边端设备+状态
		index.Fields("edge_id", "status"),
	}
}
