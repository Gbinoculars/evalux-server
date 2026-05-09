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

type UxProject struct{ ent.Schema }

func (UxProject) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project"}}
}

func (UxProject) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("project_id"),
		field.UUID("created_by", uuid.UUID{}),
		field.UUID("org_id", uuid.UUID{}).Optional().Nillable().Comment("所属组织，允许为空以支持个人项目"),
		field.String("project_name").MaxLen(128).NotEmpty(),
		field.String("app_name").MaxLen(128).NotEmpty(),
		field.String("app_version").MaxLen(32).Optional().Nillable(),
		field.Text("research_goal").NotEmpty(),
		field.Text("project_desc").Optional().Nillable(),
		field.JSON("model_config", map[string]interface{}{}).Optional(),
		field.Enum("status").Values("ACTIVE", "DRAFT", "ARCHIVED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("archived_at").Optional().Nillable(),
	}
}

func (UxProject) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", SysUser.Type).Ref("projects").Unique().Required().Field("created_by"),
		edge.From("org", UxOrg.Type).Ref("projects").Unique().Field("org_id"),
		edge.To("profiles", UxUserProfile.Type),
		edge.To("tasks", UxTask.Type),
		edge.To("questionnaire_templates", UxQuestionnaireTemplate.Type),
		edge.To("members", UxProjectMember.Type),
		edge.To("invitations", UxProjectInvitation.Type),
		edge.To("execution_plans", UxExecutionPlan.Type),
	}
}

func (UxProject) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_by"),
		index.Fields("org_id"),
		index.Fields("project_name"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}
