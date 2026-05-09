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

// UxRunQuestionnaireQuestionSnapshot 问卷题目快照（题面、题型、维度、量表范围）。
// 选项独立到 ux_run_questionnaire_option_snapshot，避免本表使用 jsonb。
type UxRunQuestionnaireQuestionSnapshot struct{ ent.Schema }

func (UxRunQuestionnaireQuestionSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_questionnaire_question_snapshot"}}
}

func (UxRunQuestionnaireQuestionSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("template_id_ref", uuid.UUID{}),
		field.UUID("question_id_ref", uuid.UUID{}),
		field.Int("question_no"),
		field.Enum("question_type").Values("SCALE", "SINGLE_CHOICE", "MULTIPLE_CHOICE", "OPEN_ENDED"),
		field.Text("question_text"),
		field.String("dimension_code").MaxLen(32).Optional().Nillable(),
		field.Bool("is_required").Default(true),
		field.Int("score_min").Optional().Nillable(),
		field.Int("score_max").Optional().Nillable(),
	}
}

func (UxRunQuestionnaireQuestionSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("question_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunQuestionnaireQuestionSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "template_id_ref", "question_id_ref").Unique(),
		index.Fields("run_id", "template_id_ref", "question_no"),
	}
}
