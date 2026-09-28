package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SpinningLine holds the schema definition for the SpinningLine entity.
// 纺丝线体表 - 设备管理
type SpinningLine struct {
	ent.Schema
}

// Fields of the SpinningLine.
func (SpinningLine) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 线体编号（唯一）
		field.String("line_number").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("线体编号"),

		// 线体名称
		field.String("line_name").
			NotEmpty().
			MaxLen(100).
			Comment("线体名称"),

		// 位号数量
		field.Int("position_count").
			Positive().
			Comment("纺丝位号数量（多少个位）"),

		// 车间区域
		field.String("workshop_area").
			Optional().
			MaxLen(50).
			Comment("车间区域"),

		// 产品类型
		field.Enum("product_type").
			Values("FDY", "POY", "DTY").
			Optional().
			Comment("适用产品类型（可选）"),

		// 状态
		field.Enum("status").
			Values("active", "maintenance", "inactive").
			Default("active").
			Comment("线体状态"),

		// PLC连接信息
		field.String("plc_ip").
			Optional().
			MaxLen(50).
			Comment("PLC IP地址"),

		field.Int("plc_port").
			Optional().
			Positive().
			Comment("PLC端口"),

		field.String("plc_protocol").
			Optional().
			MaxLen(20).
			Comment("PLC协议（S7/Modbus/etc）"),

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

// Edges of the SpinningLine.
func (SpinningLine) Edges() []ent.Edge {
	return nil // 线体表暂无边关系
}

// Indexes of the SpinningLine.
func (SpinningLine) Indexes() []ent.Index {
	return []ent.Index{
		// 状态索引
		index.Fields("status"),

		// 产品类型索引
		index.Fields("product_type"),

		// 车间区域索引
		index.Fields("workshop_area"),
	}
}
