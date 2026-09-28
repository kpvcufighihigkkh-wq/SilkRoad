package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ProductConfig holds the schema definition for the ProductConfig entity (Edge version).
// 产品配置表 - 边端只读缓存，从中心端同步
type ProductConfig struct {
	ent.Schema
}

// Fields of the ProductConfig.
func (ProductConfig) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 产品类型
		field.Enum("product_type").
			Values("FDY", "POY", "DTY").
			Comment("产品类型"),

		// 产品规格
		field.String("product_spec").
			NotEmpty().
			MaxLen(100).
			Comment("产品规格"),

		// 工艺参数（JSON）
		field.JSON("process_params", map[string]interface{}{}).
			Optional().
			Comment("工艺参数（温度、速度等）"),

		// 质检标准（JSON）
		field.JSON("quality_standards", map[string]interface{}{}).
			Optional().
			Comment("质检标准（合格范围）"),

		// 质检等级系统（JSON）
		field.JSON("grade_system", map[string]interface{}{}).
			Optional().
			Comment("质检等级配置（计算规则、权重等）"),

		// 目标重量
		field.Float("target_weight").
			Optional().
			Positive().
			Comment("目标净重（kg）"),

		// 重量公差
		field.Float("weight_tolerance").
			Optional().
			Positive().
			Comment("重量公差（±kg）"),

		// 是否启用
		field.Bool("is_active").
			Default(true).
			Comment("是否启用"),

		// 同步时间（从中心端）
		field.Time("synced_at").
			Optional().
			Comment("从中心端同步的时间"),

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

// Edges of the ProductConfig.
func (ProductConfig) Edges() []ent.Edge {
	return nil
}

// Indexes of the ProductConfig.
func (ProductConfig) Indexes() []ent.Index {
	return []ent.Index{
		// 产品类型索引
		index.Fields("product_type"),

		// 产品规格索引
		index.Fields("product_spec"),

		// 启用状态索引
		index.Fields("is_active"),

		// 复合索引：产品类型+规格
		index.Fields("product_type", "product_spec").
			Unique(),
	}
}

// Annotations of the ProductConfig.
func (ProductConfig) Annotations() []entsql.Annotation {
	return nil
}
