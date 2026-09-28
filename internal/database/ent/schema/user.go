package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
// 用户表 - 系统用户和权限
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// 主键
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			StorageKey("id"),

		// 用户名（唯一）
		field.String("username").
			Unique().
			NotEmpty().
			MaxLen(50).
			Comment("用户名"),

		// 密码哈希
		field.String("password_hash").
			Sensitive().
			NotEmpty().
			Comment("密码哈希（bcrypt）"),

		// 真实姓名
		field.String("full_name").
			NotEmpty().
			MaxLen(100).
			Comment("真实姓名"),

		// 角色
		field.Enum("role").
			Values("admin", "operator", "inspector", "viewer").
			Default("viewer").
			Comment("用户角色"),

		// 工号
		field.String("employee_id").
			Optional().
			MaxLen(50).
			Comment("工号"),

		// 邮箱
		field.String("email").
			Optional().
			MaxLen(100).
			Comment("邮箱"),

		// 手机号
		field.String("phone").
			Optional().
			MaxLen(20).
			Comment("手机号"),

		// 状态
		field.Bool("is_active").
			Default(true).
			Comment("是否激活"),

		// 最后登录
		field.Time("last_login_at").
			Optional().
			Comment("最后登录时间"),

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

// Edges of the User.
func (User) Edges() []ent.Edge {
	return nil // 用户表暂无边关系
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// 角色索引
		index.Fields("role"),

		// 激活状态索引
		index.Fields("is_active"),

		// 工号索引
		index.Fields("employee_id"),

		// 最后登录时间索引
		index.Fields("last_login_at"),
	}
}
