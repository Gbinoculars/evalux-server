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

// UxRunProfileQuestionnaireSnapshot 画像收尾问卷绑定快照。
type UxRunProfileQuestionnaireSnapshot struct{ ent.Schema }

func (UxRunProfileQuestionnaireSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_profile_questionnaire_snapshot"}}
}

func (UxRunProfileQuestionnaireSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("source_binding_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("profile_id_ref", uuid.UUID{}),
		field.UUID("template_id_ref", uuid.UUID{}),
		field.String("template_name_snapshot").MaxLen(128),
		field.Int("question_order").Default(0),
	}
}

func (UxRunProfileQuestionnaireSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("profile_questionnaire_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunProfileQuestionnaireSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "profile_id_ref", "template_id_ref").Unique(),
		index.Fields("run_id", "profile_id_ref"),
	}
}
