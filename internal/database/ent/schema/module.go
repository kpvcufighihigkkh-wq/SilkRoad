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

		// 第一个桶ID索引
		index.Fields("barrel1_id"),

		// 第二个桶ID索引
		index.Fields("barrel2_id"),
	}
}
