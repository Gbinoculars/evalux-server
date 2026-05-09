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

type UxQuestionnaireQuestion struct{ ent.Schema }

func (UxQuestionnaireQuestion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_questionnaire_question"}}
}

func (UxQuestionnaireQuestion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("question_id"),
		field.UUID("template_id", uuid.UUID{}),
		field.Int("question_no"),
		field.Enum("question_type").Values("SCALE", "SINGLE_CHOICE", "MULTIPLE_CHOICE", "OPEN_ENDED"),
		field.Text("question_text").NotEmpty(),
		field.JSON("option_list", []map[string]string{}).Optional(),
		field.JSON("score_range", map[string]int{}).Optional(),
		field.String("dimension_code").MaxLen(32).Optional().Nillable(),
		field.Bool("is_required").Default(true),
	}
}

func (UxQuestionnaireQuestion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("template", UxQuestionnaireTemplate.Type).Ref("questions").Unique().Required().Field("template_id"),
	}
}

func (UxQuestionnaireQuestion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("template_id", "question_no").Unique(),
		index.Fields("question_type"),
	}
}
