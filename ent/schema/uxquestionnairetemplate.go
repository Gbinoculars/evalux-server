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

// UxQuestionnaireTemplate 纯问卷模板：不再持有 applicable_scope，
// 也不再有任务级反向绑定。问卷的"何时被填"由 plan 层四类绑定决定。
type UxQuestionnaireTemplate struct{ ent.Schema }

func (UxQuestionnaireTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_questionnaire_template"}}
}

func (UxQuestionnaireTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("template_id"),
		field.UUID("project_id", uuid.UUID{}).Optional().Nillable(),
		field.String("template_name").MaxLen(128).NotEmpty(),
		field.JSON("dimension_schema", []string{}).Optional(),
		field.Text("template_desc").Optional().Nillable(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxQuestionnaireTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("questionnaire_templates").Unique().Field("project_id"),
		edge.To("questions", UxQuestionnaireQuestion.Type),
	}
}

func (UxQuestionnaireTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id"),
		index.Fields("status"),
	}
}
