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

// UxRunQuestionnaireTemplateSnapshot 问卷模板元信息快照。
type UxRunQuestionnaireTemplateSnapshot struct{ ent.Schema }

func (UxRunQuestionnaireTemplateSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_questionnaire_template_snapshot"}}
}

func (UxRunQuestionnaireTemplateSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("template_id_ref", uuid.UUID{}),
		field.String("template_name_snapshot").MaxLen(128),
		field.Text("template_desc_snapshot").Optional().Nillable(),
	}
}

func (UxRunQuestionnaireTemplateSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("template_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunQuestionnaireTemplateSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "template_id_ref").Unique(),
	}
}
