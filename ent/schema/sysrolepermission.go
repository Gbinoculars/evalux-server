package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SysRolePermission holds the schema definition for the SysRolePermission entity.
type SysRolePermission struct {
	ent.Schema
}

func (SysRolePermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_role_permission"},
	}
}

func (SysRolePermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("role_permission_id"),
		field.UUID("role_id", uuid.UUID{}),
		field.UUID("module_id", uuid.UUID{}),
		field.UUID("permission_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SysRolePermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", SysRole.Type).Ref("role_permissions").Unique().Required().Field("role_id"),
		edge.From("module", SysModule.Type).Ref("role_permissions").Unique().Required().Field("module_id"),
		edge.From("permission", SysPermission.Type).Ref("role_permissions").Unique().Required().Field("permission_id"),
	}
}

func (SysRolePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "module_id", "permission_id").Unique(),
	}
}
