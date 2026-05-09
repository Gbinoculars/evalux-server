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

// SysUserRole holds the schema definition for the SysUserRole entity.
type SysUserRole struct {
	ent.Schema
}

func (SysUserRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_user_role"},
	}
}

func (SysUserRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("user_role_id"),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("role_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (SysUserRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", SysUser.Type).Ref("user_roles").Unique().Required().Field("user_id"),
		edge.From("role", SysRole.Type).Ref("user_roles").Unique().Required().Field("role_id"),
	}
}

func (SysUserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id").Unique(),
		index.Fields("user_id"),
		index.Fields("role_id"),
	}
}
