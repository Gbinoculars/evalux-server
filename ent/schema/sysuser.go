package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SysUser holds the schema definition for the SysUser entity.
type SysUser struct {
	ent.Schema
}

func (SysUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sys_user"},
	}
}

func (SysUser) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("user_id"),
		field.String("account").MaxLen(64).Unique().NotEmpty(),
		field.String("password_hash").MaxLen(255).NotEmpty(),
		field.String("nickname").MaxLen(64).NotEmpty(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Immutable(),
		field.Time("last_login_at").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SysUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_roles", SysUserRole.Type),
		edge.To("user_module_permissions", SysUserModulePermission.Type),
		edge.To("projects", UxProject.Type),
		edge.To("created_orgs", UxOrg.Type),
		edge.To("org_memberships", UxOrgMember.Type),
		edge.To("project_memberships", UxProjectMember.Type),
		edge.To("execution_plans", UxExecutionPlan.Type),
	}
}

func (SysUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}
