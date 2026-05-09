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

// UxPlanModelConfig 计划下的模型配置（CONTROL/TREATMENT 各一行）。
// 非 A/B 计划仅 1 行 CONTROL；A/B 计划 2 行。
type UxPlanModelConfig struct{ ent.Schema }

func (UxPlanModelConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_plan_model_config"}}
}

func (UxPlanModelConfig) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("config_id"),
		field.UUID("plan_id", uuid.UUID{}),
		field.Enum("model_role").Values("CONTROL", "TREATMENT").Default("CONTROL"),
		field.String("channel").MaxLen(64).NotEmpty(),
		field.Enum("model_type").Values("multimodal", "text").Default("multimodal"),
		field.String("model_name").MaxLen(128).NotEmpty(),
		field.String("api_base_url").MaxLen(512).Optional().Nillable(),
		field.String("api_key_cipher").MaxLen(1024).Optional().Nillable(),
		field.Float("temperature").Optional().Nillable(),
		field.Float("top_p").Optional().Nillable(),
		field.Int("max_tokens").Optional().Nillable(),
		field.String("reasoning_effort").MaxLen(32).Optional().Nillable(),
		field.String("extra_params").MaxLen(2048).Optional().Nillable(),
	}
}

func (UxPlanModelConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", UxExecutionPlan.Type).Ref("model_configs").Unique().Required().Field("plan_id"),
	}
}

func (UxPlanModelConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "model_role").Unique(),
	}
}
