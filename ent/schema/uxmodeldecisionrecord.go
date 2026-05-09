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

type UxModelDecisionRecord struct{ ent.Schema }

func (UxModelDecisionRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_model_decision_record"}}
}

func (UxModelDecisionRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("decision_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.UUID("step_id", uuid.UUID{}).Unique(),
		field.JSON("request_payload", map[string]interface{}{}).Optional(),
		field.JSON("response_payload", map[string]interface{}{}).Optional(),
		field.String("action_type").MaxLen(32).NotEmpty(),
		field.Enum("task_state").Values("IN_PROGRESS", "COMPLETED", "FAILED", "UNKNOWN").Default("IN_PROGRESS"),
		field.Text("decision_reason").Optional().Nillable(),
		field.Bool("need_continue").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxModelDecisionRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("decisions").Unique().Required().Field("session_id"),
		edge.From("step", UxExecutionStep.Type).Ref("decision").Unique().Required().Field("step_id"),
	}
}

func (UxModelDecisionRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("action_type"),
		index.Fields("task_state"),
	}
}
