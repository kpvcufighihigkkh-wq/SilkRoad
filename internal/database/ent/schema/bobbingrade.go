package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// BobbinGrade holds the schema definition for the BobbinGrade entity.
// 丝锭质检表 - 质检记录
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

		// 质检员
		field.UUID("inspector_id", uuid.UUID{}).
			Optional().
			Comment("质检员ID"),

		// 质检结果
		field.String("grade").
			NotEmpty().
			MaxLen(20).
			Comment("质检等级（A/B/C/D）"),

		// 质检项明细（JSONB）
		field.JSON("inspection_details", map[string]interface{}{}).
			Optional().
			SchemaType(map[string]string{
				"postgres": "jsonb",
			}).
			Comment("质检项明细（外观、重量、断头等）"),

		// 分项得分（JSONB）
		field.JSON("scores", map[string]interface{}{}).
			Optional().
			SchemaType(map[string]string{
				"postgres": "jsonb",
			}).
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

		// 备注
		field.Text("notes").
			Optional().
			Comment("备注信息"),

		// 质检时间
		field.Time("inspected_at").
			Default(time.Now).
			Comment("质检时间"),

		// 元数据
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),

		// 同步元数据（Edge → Center）
		field.Enum("sync_status").
			Values("pending", "synced", "failed").
			Default("pending").
			Comment("同步状态"),

		field.Time("synced_at").
			Optional().
			Comment("同步完成时间"),

		field.Int("sync_retry_count").
			Default(0).
			NonNegative().
			Comment("同步重试次数"),
	}
}

// Edges of the BobbinGrade.
func (BobbinGrade) Edges() []ent.Edge {
	return nil
}

// Indexes of the BobbinGrade.
func (BobbinGrade) Indexes() []ent.Index {
	return []ent.Index{
		// 丝锭ID索引（非唯一：同一丝饼可有多次质检，保留历史）
		index.Fields("bobbin_id"),

		// 质检等级索引
		index.Fields("grade"),

		// 是否合格索引
		index.Fields("is_qualified"),

		// 质检员索引
		index.Fields("inspector_id"),

		// 质检时间索引
		index.Fields("inspected_at"),

		// 同步状态索引（Edge 端扫描待同步记录）
		index.Fields("sync_status"),
	}
}

// Annotations of the BobbinGrade.
func (BobbinGrade) Annotations() []schema.Annotation {
	return nil
}
