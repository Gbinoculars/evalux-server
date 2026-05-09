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

type UxOrgMember struct{ ent.Schema }

func (UxOrgMember) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_org_member"}}
}

func (UxOrgMember) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("org_member_id"),
		field.UUID("org_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("org_role_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxOrgMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("org", UxOrg.Type).Ref("members").Unique().Required().Field("org_id"),
		edge.From("user", SysUser.Type).Ref("org_memberships").Unique().Required().Field("user_id"),
		edge.From("role", UxOrgRole.Type).Ref("members").Unique().Required().Field("org_role_id"),
	}
}

func (UxOrgMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("org_id", "user_id").Unique(),
		index.Fields("user_id"),
		index.Fields("org_role_id"),
	}
}
