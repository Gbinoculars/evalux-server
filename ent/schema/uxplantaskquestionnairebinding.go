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

// UxPlanTaskQuestionnaireBinding 每个"计划下的任务"完成后立刻填的问卷。
type UxPlanTaskQuestionnaireBinding struct{ ent.Schema }

func (UxPlanTaskQuestionnaireBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_plan_task_questionnaire_binding"}}
}

func (UxPlanTaskQuestionnaireBinding) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("binding_id"),
		field.UUID("plan_id", uuid.UUID{}),
		field.UUID("task_id", uuid.UUID{}),
		field.UUID("template_id", uuid.UUID{}),
		field.Int("question_order").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxPlanTaskQuestionnaireBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", UxExecutionPlan.Type).Ref("task_questionnaire_bindings").Unique().Required().Field("plan_id"),
	}
}

func (UxPlanTaskQuestionnaireBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "task_id", "template_id").Unique(),
		index.Fields("plan_id", "enabled"),
	}
}
