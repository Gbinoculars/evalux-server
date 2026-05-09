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

// UxExecutionBatch 一次 run 下的批次。
//   - 非 A/B 计划：1 行 batch_role=CONTROL
//   - A/B 计划：2 行（CONTROL + TREATMENT）
//
// batch_id 是 string 主键（跨服务一致的可读字符串）。
// run_model_config_id 指向当前 run 的模型配置快照行。
type UxExecutionBatch struct{ ent.Schema }

func (UxExecutionBatch) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_execution_batch"}}
}

func (UxExecutionBatch) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").MaxLen(128).NotEmpty().StorageKey("batch_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.Enum("batch_role").Values("CONTROL", "TREATMENT").Default("CONTROL"),
		field.UUID("run_model_config_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxExecutionBatch) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("batches").Unique().Required().Field("run_id"),
		edge.To("sessions", UxExecutionSession.Type),
	}
}

func (UxExecutionBatch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "batch_role").Unique(),
	}
}
