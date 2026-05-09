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

// SysModule holds the schema definition for the SysModule entity.
type SysModule struct {
	ent.Schema
}

func (SysModule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_module"},
	}
}

func (SysModule) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("module_id"),
		field.UUID("parent_module_id", uuid.UUID{}).Optional().Nillable(),
		field.String("module_code").MaxLen(128).Unique().NotEmpty(),
		field.String("module_name").MaxLen(128).NotEmpty(),
		field.Enum("module_type").Values("ROOT", "CATEGORY", "RESOURCE"),
		field.String("resource_type").MaxLen(32).Optional().Nillable(),
		field.UUID("resource_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SysModule) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", SysModule.Type).From("parent").Unique().Field("parent_module_id"),
		edge.To("role_permissions", SysRolePermission.Type),
		edge.To("user_module_permissions", SysUserModulePermission.Type),
	}
}

func (SysModule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_module_id", "status"),
		index.Fields("resource_type", "resource_id"),
	}
}
