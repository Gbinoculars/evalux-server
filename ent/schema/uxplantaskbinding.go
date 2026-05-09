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

// UxPlanTaskBinding 计划要跑哪些任务，以及顺序。
type UxPlanTaskBinding struct{ ent.Schema }

func (UxPlanTaskBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_plan_task_binding"}}
}

func (UxPlanTaskBinding) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("binding_id"),
		field.UUID("plan_id", uuid.UUID{}),
		field.UUID("task_id", uuid.UUID{}),
		field.Int("execution_order").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxPlanTaskBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", UxExecutionPlan.Type).Ref("task_bindings").Unique().Required().Field("plan_id"),
	}
}

func (UxPlanTaskBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "task_id").Unique(),
		index.Fields("plan_id", "enabled"),
	}
}
