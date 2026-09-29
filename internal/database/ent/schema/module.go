package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Module holds the schema definition for the Module entity.
// 吊车/模块表 - 载具管理（可装载多个落纱桶）
type Module struct {
	ent.Schema
}

// Fields of the Module.
func (Module) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 吊车编号（唯一）
		field.String("module_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("吊车编号"),

		// 来源边端设备（Edge端设备产生）
		field.UUID("edge_id", uuid.UUID{}).
			Optional().
			Comment("来源边端设备ID"),

		// 装载的第一个落纱桶
		field.UUID("barrel1_id", uuid.UUID{}).
			Optional().
			Comment("第一个落纱桶ID"),

		// 装载的第二个落纱桶
		field.UUID("barrel2_id", uuid.UUID{}).
			Optional().
			Comment("第二个落纱桶ID"),

		// 状态
		field.Enum("status").
			Values("loading", "transporting", "sorting", "warehouse", "idle").
			Default("idle").
			Comment("吊车状态"),

		// RFID标签
		field.String("rfid").
			Optional().
			MaxLen(100).
			Comment("RFID标签"),

		// 当前位置
		field.String("current_location").
			Optional().
			MaxLen(100).
			Comment("当前位置"),

		// 时间戳
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

// Edges of the Module.
func (Module) Edges() []ent.Edge {
	return nil // Module通过barrel_id字段关联，不使用edge
}

// Indexes of the Module.
func (Module) Indexes() []ent.Index {
	return []ent.Index{
		// 状态索引
		index.Fields("status"),

		// RFID索引
		index.Fields("rfid"),

		// 同步状态索引（Edge端扫描待同步记录）
		index.Fields("sync_status"),

		// 第一个桶ID索引
		index.Fields("barrel1_id"),

		// 第二个桶ID索引
		index.Fields("barrel2_id"),

		// 来源边端设备索引
		index.Fields("edge_id"),
	}
}
