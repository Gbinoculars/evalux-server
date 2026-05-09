package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// UxRunTaskSnapshot 任务集合快照：本次 run 要跑哪些任务及其语义。
type UxRunTaskSnapshot struct{ ent.Schema }

func (UxRunTaskSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_task_snapshot"}}
}

func (UxRunTaskSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("source_binding_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("task_id_ref", uuid.UUID{}),
		field.String("task_name_snapshot").MaxLen(128),
		field.Text("task_goal_snapshot"),
		field.Text("success_criteria_snapshot"),
		field.Text("precondition_snapshot").Optional().Nillable(),
		field.Text("execution_guide_snapshot").Optional().Nillable(),
		field.Int("timeout_seconds_snapshot").Default(300),
		field.Int("min_steps_snapshot").Optional().Nillable(),
		field.Int("max_steps_snapshot").Optional().Nillable(),
		field.Int("execution_order").Default(0),
	}
}

func (UxRunTaskSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("task_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunTaskSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "task_id_ref").Unique(),
		index.Fields("run_id", "execution_order"),
	}
}
