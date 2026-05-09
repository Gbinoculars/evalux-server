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

// UxQuestionnaireAnswer 问卷答案。
//   - answer_origin = AFTER_TASK: 任务后问卷，task_id 必填，profile_id 必填
//   - answer_origin = AFTER_ALL_PER_PROFILE: 画像收尾问卷，task_id 留空(NULL)，profile_id 必填
//
// 重要：template_id 和 question_id 均引用 run 快照表的 ID（snapshot_id），而非原始问卷表。
// 这样即使原始问卷被修改/删除，历史答案仍能精确关联到当时的题目版本。
// source_binding_id 指向 run 内 task_questionnaire_snapshot 或 profile_questionnaire_snapshot 的 id（弱引用，不建外键）。
type UxQuestionnaireAnswer struct{ ent.Schema }

func (UxQuestionnaireAnswer) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_questionnaire_answer"}}
}

func (UxQuestionnaireAnswer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("answer_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.Enum("answer_origin").Values("AFTER_TASK", "AFTER_ALL_PER_PROFILE").Default("AFTER_TASK"),
		field.UUID("source_binding_id", uuid.UUID{}).Optional().Nillable().Comment("指向 run 内绑定快照行（弱引用）"),
		field.UUID("task_id", uuid.UUID{}).Optional().Nillable().Comment("AFTER_TASK 必填，AFTER_ALL_PER_PROFILE 为空"),
		field.UUID("profile_id", uuid.UUID{}),
		field.UUID("template_id", uuid.UUID{}).Comment("引用 ux_run_questionnaire_template_snapshot.snapshot_id"),
		field.UUID("question_id", uuid.UUID{}).Comment("引用 ux_run_questionnaire_question_snapshot.snapshot_id"),
		field.String("answer_type").MaxLen(16).NotEmpty(),
		field.Float("answer_score").Optional().Nillable(),
		field.JSON("answer_option", []string{}).Optional(),
		field.Text("answer_text").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxQuestionnaireAnswer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("answers").Unique().Required().Field("session_id"),
	}
}

func (UxQuestionnaireAnswer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "question_id").Unique(),
		index.Fields("answer_origin"),
		index.Fields("task_id"),
		index.Fields("profile_id"),
		index.Fields("template_id"),
	}
}
