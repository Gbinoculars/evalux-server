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

type UxOrg struct{ ent.Schema }

func (UxOrg) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_org"}}
}

func (UxOrg) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("org_id"),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.String("org_name").MaxLen(128).NotEmpty(),
		field.String("org_type").MaxLen(32).NotEmpty().Comment("组织类型: LAB, GROUP, TEAM"),
		field.Text("org_desc").Optional().Nillable(),
		field.UUID("created_by", uuid.UUID{}),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxOrg) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", UxOrg.Type).From("parent").Unique().Field("parent_id"),
		edge.From("creator", SysUser.Type).Ref("created_orgs").Unique().Required().Field("created_by"),
		edge.To("members", UxOrgMember.Type),
		edge.To("projects", UxProject.Type),
	}
}

func (UxOrg) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_id", "status"),
		index.Fields("created_by"),
		index.Fields("status"),
	}
}
