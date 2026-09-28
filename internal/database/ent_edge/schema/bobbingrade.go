package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// BobbinGrade holds the schema definition for the BobbinGrade entity (Edge version).
// 丝锭质检表 - 边端简化版
type BobbinGrade struct {
	ent.Schema
}

// Fields of the BobbinGrade.
func (BobbinGrade) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 关联丝锭
		field.UUID("bobbin_id", uuid.UUID{}).
			Comment("关联丝锭ID"),

		// 质检员ID（边端可能记录工号）
		field.String("inspector_id").
			Optional().
			MaxLen(50).
			Comment("质检员ID/工号"),

		// 质检结果
		field.String("grade").
			NotEmpty().
			MaxLen(20).
			Comment("质检等级（A/B/C/D）"),

		// 质检项明细（JSON）
		field.JSON("inspection_details", map[string]interface{}{}).
			Optional().
			Comment("质检项明细（外观、重量、断头等）"),

		// 分项得分（JSON）
		field.JSON("scores", map[string]interface{}{}).
			Optional().
			Comment("各项得分"),

		// 综合得分
		field.Float("final_score").
			Optional().
			Min(0).
			Comment("综合得分"),

		// 是否合格
		field.Bool("is_qualified").
			Default(true).
			Comment("是否合格"),

		// 不合格原因
		field.Text("defect_reason").
			Optional().
			Comment("不合格原因"),

		// 质检时间
		field.Time("inspected_at").
			Default(time.Now).
			Comment("质检时间"),

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

// Edges of the BobbinGrade.
func (BobbinGrade) Edges() []ent.Edge {
	return nil
}

// Indexes of the BobbinGrade.
func (BobbinGrade) Indexes() []ent.Index {
	return []ent.Index{
		// 丝锭ID索引（唯一）
		index.Fields("bobbin_id").
			Unique(),

		// 质检等级索引
		index.Fields("grade"),

		// 是否合格索引
		index.Fields("is_qualified"),

		// 同步状态索引
		index.Fields("synced"),
	}
}

// Annotations of the BobbinGrade.
func (BobbinGrade) Annotations() []entsql.Annotation {
	return nil
}
