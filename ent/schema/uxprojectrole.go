package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type UxProjectRole struct{ ent.Schema }

func (UxProjectRole) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project_role"}}
}

func (UxProjectRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("project_role_id"),
		field.String("role_code").MaxLen(32).Unique().NotEmpty(),
		field.String("role_name").MaxLen(64).NotEmpty(),
		field.JSON("permission_codes", []string{}).Comment("权限码列表: VIEW, EDIT, EXECUTE, DELETE, MANAGE_MEMBER"),
		field.Text("description").Optional(),
	}
}

func (UxProjectRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", UxProjectMember.Type),
		edge.To("invitations", UxProjectInvitation.Type),
	}
}
