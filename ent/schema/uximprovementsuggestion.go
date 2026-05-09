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

type UxImprovementSuggestion struct{ ent.Schema }

func (UxImprovementSuggestion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_improvement_suggestion"}}
}

func (UxImprovementSuggestion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("suggestion_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.String("suggestion_type").MaxLen(32).NotEmpty(),
		field.Enum("priority_level").Values("HIGH", "MEDIUM", "LOW").Default("MEDIUM"),
		field.Text("suggestion_text").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxImprovementSuggestion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("suggestions").Unique().Required().Field("session_id"),
	}
}

func (UxImprovementSuggestion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("suggestion_type"),
		index.Fields("priority_level"),
	}
}
