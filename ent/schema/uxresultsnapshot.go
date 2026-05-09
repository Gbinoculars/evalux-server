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

// UxResultSnapshot 结果快照：按 run/batch 维度落盘，不再按 project 维度模糊聚合。
// scope_type: TASK / PROFILE / OVERALL；scope_key 为 task_id / profile_id / NULL。
type UxResultSnapshot struct{ ent.Schema }

func (UxResultSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_result_snapshot"}}
}

func (UxResultSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.String("batch_id").MaxLen(128).NotEmpty(),
		field.Enum("scope_type").Values("TASK", "PROFILE", "OVERALL"),
		field.UUID("scope_key", uuid.UUID{}).Optional().Nillable().Comment("TASK 时为 task_id，PROFILE 时为 profile_id，OVERALL 时为 NULL"),
		field.Int("total_sessions").Default(0),
		field.Int("completed_sessions").Default(0),
		field.Int("failed_sessions").Default(0),
		field.Float("completion_rate").Default(0),
		field.Int64("avg_duration_ms").Optional().Nillable(),
		field.Float("avg_error_count").Optional().Nillable(),
		field.Float("avg_score").Optional().Nillable(),
		field.JSON("metric_payload", map[string]interface{}{}).Optional().Comment("扩展指标，运行域可用"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxResultSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("result_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxResultSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "batch_id"),
		index.Fields("scope_type"),
		index.Fields("created_at"),
	}
}
