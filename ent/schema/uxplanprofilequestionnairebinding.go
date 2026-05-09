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

// UxPlanProfileQuestionnaireBinding 每个"计划下的画像"跑完该计划下所有任务后再填的收尾问卷。
type UxPlanProfileQuestionnaireBinding struct{ ent.Schema }

func (UxPlanProfileQuestionnaireBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_plan_profile_questionnaire_binding"}}
}

func (UxPlanProfileQuestionnaireBinding) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("binding_id"),
		field.UUID("plan_id", uuid.UUID{}),
		field.UUID("profile_id", uuid.UUID{}),
		field.UUID("template_id", uuid.UUID{}),
		field.Int("question_order").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxPlanProfileQuestionnaireBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", UxExecutionPlan.Type).Ref("profile_questionnaire_bindings").Unique().Required().Field("plan_id"),
	}
}

func (UxPlanProfileQuestionnaireBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "profile_id", "template_id").Unique(),
		index.Fields("plan_id", "enabled"),
	}
}
