package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// SysPermission holds the schema definition for the SysPermission entity.
type SysPermission struct {
	ent.Schema
}

func (SysPermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_permission"},
	}
}

func (SysPermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("permission_id"),
		field.String("permission_code").MaxLen(32).Unique().NotEmpty(),
		field.String("permission_name").MaxLen(32).NotEmpty(),
		field.Text("description").Optional(),
	}
}

func (SysPermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("role_permissions", SysRolePermission.Type),
		edge.To("user_module_permissions", SysUserModulePermission.Type),
	}
}
