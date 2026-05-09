package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type UxSubjectiveEvaluation struct{ ent.Schema }

func (UxSubjectiveEvaluation) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_subjective_evaluation"}}
}

func (UxSubjectiveEvaluation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("evaluation_id"),
		field.UUID("session_id", uuid.UUID{}).Unique(),
		field.Float("overall_score").Optional().Nillable(),
		field.Text("summary_text").NotEmpty(),
		field.JSON("based_on_snapshot", map[string]interface{}{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxSubjectiveEvaluation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("evaluation").Unique().Required().Field("session_id"),
	}
}
