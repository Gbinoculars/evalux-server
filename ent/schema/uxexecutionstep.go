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

type UxExecutionStep struct{ ent.Schema }

func (UxExecutionStep) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_execution_step"}}
}

func (UxExecutionStep) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("step_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.Int("step_no"),
		field.Text("screen_desc").Optional().Nillable(),
		field.String("action_type").MaxLen(32).Optional().Nillable(),
		field.JSON("action_param", map[string]interface{}{}).Optional(),
		field.Text("decision_summary").Optional().Nillable(),
		field.JSON("execution_result", map[string]interface{}{}).Optional(),
		field.Text("error_message").Optional().Nillable(),
		field.Int("retry_count").Default(0),
		field.Time("started_at").Default(time.Now),
		field.Time("ended_at").Optional().Nillable(),
	}
}

func (UxExecutionStep) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("steps").Unique().Required().Field("session_id"),
		edge.To("screenshots", UxScreenshotRecord.Type),
		edge.To("decision", UxModelDecisionRecord.Type).Unique(),
	}
}

func (UxExecutionStep) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "step_no").Unique(),
	}
}
