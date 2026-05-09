package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// SysRole holds the schema definition for the SysRole entity.
type SysRole struct {
	ent.Schema
}

func (SysRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_role"},
	}
}

func (SysRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("role_id"),
		field.String("role_code").MaxLen(32).Unique().NotEmpty(),
		field.String("role_name").MaxLen(64).NotEmpty(),
		field.JSON("permission_codes", []string{}).
			Comment("系统级权限码列表，ADMIN为[*]，USER_ADMIN为用户管理码，PROJECT_ADMIN为项目管理码，MEMBER为[PROJECT:CREATE]"),
		field.Text("description").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SysRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_roles", SysUserRole.Type),
		edge.To("role_permissions", SysRolePermission.Type),
	}
}
