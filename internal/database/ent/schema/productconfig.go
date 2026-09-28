package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ProductConfig holds the schema definition for the ProductConfig entity.
// 产品配置表 - 工艺参数和质检标准
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

		// 质检等级系统（JSONB，灵活配置）
		field.JSON("grade_system", map[string]interface{}{}).
			Optional().
			SchemaType(map[string]string{
				"postgres": "jsonb",
			}).
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

// Edges of the ProductConfig.
func (ProductConfig) Edges() []ent.Edge {
	return nil // 配置表暂无边关系
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

		// 复合索引：产品类型+规格（确保唯一性）
		index.Fields("product_type", "product_spec").
			Unique(),
	}
}

// Annotations of the ProductConfig.
func (ProductConfig) Annotations() []entsql.Annotation {
	return nil
}
