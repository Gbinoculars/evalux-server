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

// UxExecutionSession 执行会话，强制归属到 ux_execution_batch。
// 不再持有 project_id、plan_id 等"快捷外键"，所有归属链路必须从 batch->run 反查。
// task_id / profile_id 仅作"这次会话跑的是哪个任务/画像"的指针，不参与归属聚合。
type UxExecutionSession struct{ ent.Schema }

func (UxExecutionSession) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_execution_session"}}
}

func (UxExecutionSession) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("session_id"),
		field.String("batch_id").MaxLen(128).NotEmpty(),
		field.UUID("task_id", uuid.UUID{}),
		field.UUID("profile_id", uuid.UUID{}),
		field.String("model_session_id").MaxLen(128).Unique().NotEmpty(),
		field.String("device_serial").MaxLen(128).Optional().Nillable(),
		field.Time("started_at").Default(time.Now),
		field.Time("ended_at").Optional().Nillable(),
		field.Enum("status").Values("PENDING", "RUNNING", "PAUSED", "COMPLETED", "FAILED", "TIMEOUT", "CANCELLED").Default("PENDING"),
		field.Int("error_count").Default(0),
		field.Int64("total_duration_ms").Optional().Nillable(),
		field.Bool("is_goal_completed").Default(false),
		field.Text("stop_reason").Optional().Nillable(),
	}
}

func (UxExecutionSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("batch", UxExecutionBatch.Type).Ref("sessions").Unique().Required().Field("batch_id"),
		edge.To("steps", UxExecutionStep.Type),
		edge.To("screenshots", UxScreenshotRecord.Type),
		edge.To("recording", UxRecordingRecord.Type).Unique(),
		edge.To("decisions", UxModelDecisionRecord.Type),
		edge.To("errors", UxErrorRecord.Type),
		edge.To("answers", UxQuestionnaireAnswer.Type),
		edge.To("evaluation", UxSubjectiveEvaluation.Type).Unique(),
		edge.To("suggestions", UxImprovementSuggestion.Type),
	}
}

func (UxExecutionSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("batch_id", "status", "started_at"),
		index.Fields("task_id", "profile_id"),
		index.Fields("model_session_id"),
	}
}
