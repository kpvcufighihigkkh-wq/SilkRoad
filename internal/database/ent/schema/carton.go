package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Carton holds the schema definition for the Carton entity.
// 纸箱表 - 纸箱包装管理
type Carton struct {
	ent.Schema
}

// Fields of the Carton.
func (Carton) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 纸箱编号（唯一，条码）
		field.String("carton_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("纸箱编号（条码）"),

		// 关联批次
		field.UUID("lot_id", uuid.UUID{}).
			Optional().
			Comment("关联批次ID"),

		// 纸箱内丝锭数量
		field.Int("bobbin_count").
			Default(0).
			NonNegative().
			Comment("纸箱内丝锭数量"),

		// 总重量
		field.Float("total_weight").
			Optional().
			Min(0).
			Comment("纸箱总重量（kg）"),

		// 状态
		field.Enum("status").
			Values("packing", "packed", "shipped").
			Default("packing").
			Comment("纸箱状态"),

		// 标签打印
		field.Bool("label_printed").
			Default(false).
			Comment("标签是否已打印"),

		field.Time("printed_at").
			Optional().
			Comment("打印时间"),

		// 时间戳
		field.Time("packed_at").
			Optional().
			Comment("打包完成时间"),

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

// Edges of the Carton.
func (Carton) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个纸箱有多个丝锭
		edge.To("bobbins", Bobbin.Type),
	}
}

// Indexes of the Carton.
func (Carton) Indexes() []ent.Index {
	return []ent.Index{
		// 批次ID索引
		index.Fields("lot_id"),

		// 状态索引
		index.Fields("status"),

		// 打包时间索引
		index.Fields("packed_at"),

		// 创建时间倒序索引
		index.Fields("created_at"),
	}
}
