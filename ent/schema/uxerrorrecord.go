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

type UxErrorRecord struct{ ent.Schema }

func (UxErrorRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_error_record"}}
}

func (UxErrorRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("error_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.UUID("step_id", uuid.UUID{}).Optional().Nillable(),
		field.String("error_type").MaxLen(32).NotEmpty(),
		field.Text("error_message").NotEmpty(),
		field.Bool("is_recovered").Default(false),
		field.Time("occurred_at").Default(time.Now),
	}
}

func (UxErrorRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("errors").Unique().Required().Field("session_id"),
	}
}

func (UxErrorRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("error_type"),
		index.Fields("is_recovered"),
		index.Fields("occurred_at"),
	}
}
