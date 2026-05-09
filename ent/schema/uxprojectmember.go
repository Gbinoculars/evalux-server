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

type UxProjectMember struct{ ent.Schema }

func (UxProjectMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project_member"}}
}

func (UxProjectMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("project_member_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("project_role_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxProjectMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("members").Unique().Required().Field("project_id"),
		edge.From("user", SysUser.Type).Ref("project_memberships").Unique().Required().Field("user_id"),
		edge.From("role", UxProjectRole.Type).Ref("members").Unique().Required().Field("project_role_id"),
	}
}

func (UxProjectMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "user_id").Unique(),
		index.Fields("user_id"),
		index.Fields("project_role_id"),
	}
}
