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

// SysUserModulePermission holds the schema definition for the SysUserModulePermission entity.
type SysUserModulePermission struct {
	ent.Schema
}

func (SysUserModulePermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_user_module_permission"},
	}
}

func (SysUserModulePermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("user_module_permission_id"),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("module_id", uuid.UUID{}),
		field.UUID("permission_id", uuid.UUID{}),
		field.UUID("granted_by", uuid.UUID{}).Optional().Nillable(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SysUserModulePermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", SysUser.Type).Ref("user_module_permissions").Unique().Required().Field("user_id"),
		edge.From("module", SysModule.Type).Ref("user_module_permissions").Unique().Required().Field("module_id"),
		edge.From("permission", SysPermission.Type).Ref("user_module_permissions").Unique().Required().Field("permission_id"),
	}
}

func (SysUserModulePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "module_id", "permission_id").Unique(),
		index.Fields("user_id", "status"),
		index.Fields("module_id", "status"),
	}
}
