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
		//
		// 三态：NULL 表示质检员未陈述结论，true/false 表示已陈述的结论。
		// 「未陈述」与「陈述为不合格」是两件事实，不能合并 —— 该表只增不改，
		// 一旦把 NULL 落成 false/true 就再也无法区分。
		//
		// Optional() 让列可空并去掉 DEFAULT；Nillable() 让 Go 字段生成 *bool。
		// 两者缺一不可：只加 Optional() 时列虽可空，Go 字段仍是 bool，SQL NULL
		// 读回成 false —— 正好复现要消除的混淆。证据：本实体 inspector_id 只写
		// Optional()，生成的是非指针 uuid.UUID；ent 源码 field.go:545-546 明写
		// 「Unlike "Optional" only fields, "Nillable" fields are pointers」。
		field.Bool("is_qualified").
			Optional().
			Nillable().
			Comment("是否合格（NULL 表示未陈述）"),

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
