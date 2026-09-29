package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Pallet holds the schema definition for the Pallet entity.
// 托盘表 - 托盘包装管理
type Pallet struct {
	ent.Schema
}

// Fields of the Pallet.
func (Pallet) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 托盘编号（唯一，条码）
		field.String("pallet_code").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("托盘编号（条码）"),

		// 来源边端设备（Edge端设备产生）
		field.UUID("edge_id", uuid.UUID{}).
			Optional().
			Comment("来源边端设备ID"),

		// 关联批次
		field.UUID("lot_id", uuid.UUID{}).
			Comment("关联批次ID"),

		// 层级（1-9）
		field.Int("level").
			Default(1).
			Min(1).
			Max(9).
			Comment("栈板层级（1-9）"),

		// 托盘内丝锭数量
		field.Int("bobbins_count").
			Default(0).
			NonNegative().
			Comment("托盘内丝锭数量"),

		// 重量信息
		field.Float("net_weight").
			Optional().
			Min(0).
			Comment("净重（kg）"),

		field.Float("gross_weight").
			Optional().
			Min(0).
			Comment("毛重（kg）"),

		field.Float("tare_weight").
			Optional().
			Min(0).
			Comment("皮重（kg）"),

		// 状态
		field.Enum("status").
			Values("building", "completed", "shipped").
			Default("building").
			Comment("托盘状态"),

		// 码垛机ID
		field.UUID("palletizer_id", uuid.UUID{}).
			Optional().
			Comment("码垛机ID"),

		// 标签打印
		field.Bool("printed").
			Default(false).
			Comment("标签是否已打印"),

		field.Time("printed_at").
			Optional().
			Comment("打印时间"),

		// 时间戳
		field.Time("completed_at").
			Optional().
			Comment("打包完成时间"),

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

// Edges of the Pallet.
func (Pallet) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个托盘属于一个批次
		edge.From("lot", Lot.Type).
			Ref("pallets").
			Field("lot_id").
			Required().
			Unique(),

		// 一个托盘有多个丝锭
		edge.To("bobbins", Bobbin.Type),
	}
}

// Indexes of the Pallet.
func (Pallet) Indexes() []ent.Index {
	return []ent.Index{
		// 批次ID索引
		index.Fields("lot_id"),

		// 层级索引
		index.Fields("level"),

		// 状态索引
		index.Fields("status"),

		// 码垛机ID索引
		index.Fields("palletizer_id"),

		// 打包完成时间索引
		index.Fields("completed_at"),

		// 同步状态索引（Edge端扫描待同步记录）
		index.Fields("sync_status"),

		// 创建时间倒序索引
		index.Fields("created_at"),

		// 复合索引：批次+状态
		index.Fields("lot_id", "status"),

		// 来源边端设备索引
		index.Fields("edge_id"),
	}
}
