package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type UxOrgRole struct{ ent.Schema }

func (UxOrgRole) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_org_role"}}
}

func (UxOrgRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("org_role_id"),
		field.String("role_code").MaxLen(32).Unique().NotEmpty(),
		field.String("role_name").MaxLen(64).NotEmpty(),
		field.JSON("permission_codes", []string{}).Comment("权限码列表: ORG_VIEW, ORG_EDIT, ORG_MANAGE_MEMBER, ORG_MANAGE_CHILD, ORG_MANAGE_PROJECT, ORG_CREATE_PROJECT"),
		field.Text("description").Optional(),
	}
}

func (UxOrgRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", UxOrgMember.Type),
	}
}
