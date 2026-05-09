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

// UxRunModelConfig 模型配置快照（被 ux_execution_batch.run_model_config_id 引用）。
type UxRunModelConfig struct{ ent.Schema }

func (UxRunModelConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_model_config"}}
}

func (UxRunModelConfig) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("config_id"),
		field.UUID("run_id", uuid.UUID{}),
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

func (UxRunModelConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("model_configs").Unique().Required().Field("run_id"),
	}
}

func (UxRunModelConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "model_role").Unique(),
	}
}
