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

type UxProjectInvitation struct{ ent.Schema }

func (UxProjectInvitation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project_invitation"}}
}

func (UxProjectInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("invitation_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("inviter_id", uuid.UUID{}),
		field.UUID("invitee_id", uuid.UUID{}),
		field.UUID("project_role_id", uuid.UUID{}),
		field.Enum("status").Values("PENDING", "ACCEPTED", "REJECTED", "EXPIRED").Default("PENDING"),
		field.Text("message").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("expired_at").Optional().Nillable(),
	}
}

func (UxProjectInvitation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("invitations").Unique().Required().Field("project_id"),
		edge.From("role", UxProjectRole.Type).Ref("invitations").Unique().Required().Field("project_role_id"),
	}
}

func (UxProjectInvitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id"),
		index.Fields("invitee_id", "status"),
		index.Fields("inviter_id"),
	}
}
