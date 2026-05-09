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

// UxExecutionPlan 执行计划主表：仅承担"配置容器"职责，所有绑定通过子表挂在 plan 下。
// 强类型字段平铺，禁止使用 jsonb。
type UxExecutionPlan struct{ ent.Schema }

func (UxExecutionPlan) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_execution_plan"}}
}

func (UxExecutionPlan) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("plan_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.String("plan_name").MaxLen(128).NotEmpty(),
		field.Enum("plan_type").Values("NORMAL", "AB_TEST", "EXPERT").Default("NORMAL"),
		field.Int("max_concurrency").Default(1).Comment("设备并发上限"),
		field.Int("step_timeout_sec").Default(60).Comment("单步超时"),
		field.Int("session_timeout_sec").Default(300).Comment("单会话超时"),
		field.Int("retry_limit").Default(3).Comment("单步重试上限"),
		field.UUID("prompt_override_id", uuid.UUID{}).Optional().Nillable().Comment("提示词覆盖 prompt 表 ID"),
		field.Text("hypothesis").Optional().Nillable().Comment("A/B 测试假设说明"),
		field.Enum("status").Values("READY", "ARCHIVED").Default("READY"),
		field.UUID("created_by", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UxExecutionPlan) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("execution_plans").Unique().Required().Field("project_id"),
		edge.From("creator", SysUser.Type).Ref("execution_plans").Unique().Required().Field("created_by"),
		edge.To("model_configs", UxPlanModelConfig.Type),
		edge.To("task_bindings", UxPlanTaskBinding.Type),
		edge.To("profile_bindings", UxPlanProfileBinding.Type),
		edge.To("task_questionnaire_bindings", UxPlanTaskQuestionnaireBinding.Type),
		edge.To("profile_questionnaire_bindings", UxPlanProfileQuestionnaireBinding.Type),
	}
}

func (UxExecutionPlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "status"),
		index.Fields("created_at"),
	}
}
