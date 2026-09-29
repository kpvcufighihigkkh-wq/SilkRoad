package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Bobbin holds the schema definition for the Bobbin entity.
// 丝锭表 - 单锭追溯管理
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

		// 关联落纱桶
		field.UUID("barrel_id", uuid.UUID{}).
			Optional().
			Comment("关联落纱桶ID"),

		// 桶内位置
		field.Int("barrel_position").
			Optional().
			Min(1).
			Max(9).
			Comment("桶内位置（1-9）"),

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

		// 包装信息
		field.UUID("pallet_id", uuid.UUID{}).
			Optional().
			Comment("关联托盘ID"),

		field.UUID("carton_id", uuid.UUID{}).
			Optional().
			Comment("关联纸箱ID"),

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

		// 同步元数据（Edge → Center）
		field.Enum("sync_status").
			Values("pending", "synced", "failed").
			Default("pending").
			Comment("同步状态"),

		field.Time("synced_at").
			Optional().
			Comment("同步时间"),

		field.Int("sync_retry_count").
			Default(0).
			NonNegative().
			Comment("同步重试次数"),

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

		// 一个丝锭可以属于一个落纱桶
		edge.From("barrel", Barrel.Type).
			Ref("bobbins").
			Field("barrel_id").
			Unique(),

		// 一个丝锭可以属于一个托盘
		edge.From("pallet", Pallet.Type).
			Ref("bobbins").
			Field("pallet_id").
			Unique(),

		// 一个丝锭可以属于一个纸箱
		edge.From("carton", Carton.Type).
			Ref("bobbins").
			Field("carton_id").
			Unique(),
	}
}

// Indexes of the Bobbin.
func (Bobbin) Indexes() []ent.Index {
	return []ent.Index{
		// 批次ID索引
		index.Fields("lot_id"),

		// 落纱桶ID索引
		index.Fields("barrel_id"),

		// 纺丝位号索引（用于查询某位号的所有丝锭）
		index.Fields("spinning_position"),

		// 状态索引
		index.Fields("status"),

		// 质检等级索引
		index.Fields("grade"),

		// 托盘ID索引
		index.Fields("pallet_id"),

		// 纸箱ID索引
		index.Fields("carton_id"),

		// 完成时间倒序索引
		index.Fields("completed_at"),

		// 复合索引：批次+位号（快速查询某批次某位号的丝锭）
		index.Fields("lot_id", "spinning_position"),

		// 同步状态索引（Edge端扫描待同步记录）
		index.Fields("sync_status"),

		// 复合索引：批次+状态
		index.Fields("lot_id", "status"),

		// 复合索引：落纱桶+桶内位置
		index.Fields("barrel_id", "barrel_position"),
	}
}
