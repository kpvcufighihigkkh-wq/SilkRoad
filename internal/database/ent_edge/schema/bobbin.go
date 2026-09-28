package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Bobbin holds the schema definition for the Bobbin entity (Edge version).
// 丝锭表 - 边端简化版
type Bobbin struct {
	ent.Schema
}

// Fields of the Bobbin.
func (Bobbin) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 丝锭编号（唯一，条码）
		field.String("bobbin_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("丝锭编号（条码）"),

		// 关联批次
		field.UUID("lot_id", uuid.UUID{}).
			Comment("关联批次ID"),

		// 纺丝位号
		field.Int("spinning_position").
			Positive().
			Comment("纺丝位号（1-N）"),

		// 重量信息
		field.Float("gross_weight").
			Positive().
			Comment("毛重（kg）"),

		field.Float("net_weight").
			Positive().
			Comment("净重（kg）"),

		field.Float("tare_weight").
			Optional().
			Min(0).
			Comment("皮重（kg）"),

		// 质检等级
		field.String("grade").
			Optional().
			MaxLen(20).
			Comment("质检等级（A/B/C/D）"),

		// 状态
		field.Enum("status").
			Values("producing", "completed", "packed", "shipped").
			Default("producing").
			Comment("丝锭状态"),

		// 打印标识
		field.Bool("label_printed").
			Default(false).
			Comment("标签是否已打印"),

		field.Time("printed_at").
			Optional().
			Comment("打印时间"),

		// 时间戳
		field.Time("completed_at").
			Optional().
			Comment("落筒时间"),

		// 同步状态
		field.Bool("synced").
			Default(false).
			Comment("是否已同步到中心端"),

		field.Time("synced_at").
			Optional().
			Comment("同步时间"),

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

// Edges of the Bobbin.
func (Bobbin) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个丝锭属于一个批次
		edge.From("lot", Lot.Type).
			Ref("bobbins").
			Field("lot_id").
			Required().
			Unique(),
	}
}

// Indexes of the Bobbin.
func (Bobbin) Indexes() []ent.Index {
	return []ent.Index{
		// 批次ID索引
		index.Fields("lot_id"),

		// 纺丝位号索引
		index.Fields("spinning_position"),

		// 状态索引
		index.Fields("status"),

		// 同步状态索引
		index.Fields("synced"),

		// 复合索引：批次+位号
		index.Fields("lot_id", "spinning_position"),
	}
}
