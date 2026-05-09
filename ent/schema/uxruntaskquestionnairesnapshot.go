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

// UxRunTaskQuestionnaireSnapshot 任务后问卷绑定快照：决定每个任务结束时该填哪些问卷。
type UxRunTaskQuestionnaireSnapshot struct{ ent.Schema }

func (UxRunTaskQuestionnaireSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_task_questionnaire_snapshot"}}
}

func (UxRunTaskQuestionnaireSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("source_binding_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("task_id_ref", uuid.UUID{}),
		field.UUID("template_id_ref", uuid.UUID{}),
		field.String("template_name_snapshot").MaxLen(128),
		field.Int("question_order").Default(0),
	}
}

func (UxRunTaskQuestionnaireSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("task_questionnaire_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunTaskQuestionnaireSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "task_id_ref", "template_id_ref").Unique(),
		index.Fields("run_id", "task_id_ref"),
	}
}
