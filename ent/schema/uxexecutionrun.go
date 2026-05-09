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

// UxExecutionRun 评估运行档案：plan 主表标量的快照容器。
// plan_id_ref 为弱引用，不建外键，便于 plan 被改/删后仍可还原历史。
// 详细绑定快照与模型快照拆到子表，避免任何 jsonb 字段。
type UxExecutionRun struct{ ent.Schema }

func (UxExecutionRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_execution_run"}}
}

func (UxExecutionRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("run_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("plan_id_ref", uuid.UUID{}).Optional().Nillable().Comment("启动时的 plan_id 弱引用，无外键约束"),
		// plan 主表快照
		field.String("plan_name_snapshot").MaxLen(128).NotEmpty(),
		field.Enum("plan_type_snapshot").Values("NORMAL", "AB_TEST", "EXPERT").Default("NORMAL"),
		field.Int("max_concurrency_snapshot").Default(1),
		field.Int("step_timeout_sec_snapshot").Default(60),
		field.Int("session_timeout_sec_snapshot").Default(300),
		field.Int("retry_limit_snapshot").Default(3),
		field.UUID("prompt_override_id_snapshot", uuid.UUID{}).Optional().Nillable(),
		field.Text("prompt_override_content_snapshot").Optional().Nillable(),
		field.Text("hypothesis_snapshot").Optional().Nillable().Comment("A/B 测试假设说明"),
		// 运行状态
		field.Enum("status").Values("RUNNING", "FINISHED", "ABORTED").Default("RUNNING"),
		field.UUID("started_by", uuid.UUID{}),
		field.Time("started_at").Default(time.Now).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}

func (UxExecutionRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("batches", UxExecutionBatch.Type),
		edge.To("model_configs", UxRunModelConfig.Type),
		edge.To("task_snapshots", UxRunTaskSnapshot.Type),
		edge.To("profile_snapshots", UxRunProfileSnapshot.Type),
		edge.To("task_questionnaire_snapshots", UxRunTaskQuestionnaireSnapshot.Type),
		edge.To("profile_questionnaire_snapshots", UxRunProfileQuestionnaireSnapshot.Type),
		edge.To("template_snapshots", UxRunQuestionnaireTemplateSnapshot.Type),
		edge.To("question_snapshots", UxRunQuestionnaireQuestionSnapshot.Type),
		edge.To("option_snapshots", UxRunQuestionnaireOptionSnapshot.Type),
		edge.To("result_snapshots", UxResultSnapshot.Type),
	}
}

func (UxExecutionRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id"),
		index.Fields("plan_id_ref"),
		index.Fields("status"),
		index.Fields("started_at"),
	}
}
