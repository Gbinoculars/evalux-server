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

// UxRunQuestionnaireOptionSnapshot 问卷题目选项快照（单/多选题用）。
type UxRunQuestionnaireOptionSnapshot struct{ ent.Schema }

func (UxRunQuestionnaireOptionSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_questionnaire_option_snapshot"}}
}

func (UxRunQuestionnaireOptionSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("question_id_ref", uuid.UUID{}),
		field.String("option_value").MaxLen(128),
		field.String("option_label").MaxLen(256),
		field.Int("option_order").Default(0),
	}
}

func (UxRunQuestionnaireOptionSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("option_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunQuestionnaireOptionSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "question_id_ref", "option_value").Unique(),
		index.Fields("run_id", "question_id_ref", "option_order"),
	}
}
