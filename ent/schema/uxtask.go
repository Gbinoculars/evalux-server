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

// UxTask 纯任务定义：仅包含目标和成功标准，不再持有任何绑定关系。
// 绑定关系（任务-画像、任务-问卷）全部上提到 ux_execution_plan 层。
type UxTask struct{ ent.Schema }

func (UxTask) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_task"}}
}

func (UxTask) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("task_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.String("task_name").MaxLen(128).NotEmpty(),
		field.Text("task_goal").NotEmpty(),
		field.Text("precondition").Optional().Nillable(),
		field.Text("execution_guide").Optional().Nillable(),
		field.JSON("step_constraints", []map[string]interface{}{}).Optional(),
		field.Text("success_criteria").NotEmpty(),
		field.Text("failure_rule").Optional().Nillable(),
		field.Int("timeout_seconds").Default(300),
		field.Int("min_steps").Optional().Nillable().Comment("正常操作最小步骤数，超过此步数计为错误次数+1"),
		field.Int("max_steps").Optional().Nillable().Comment("最大执行步骤数，超过此步数强制终止并标记为失败"),
		field.Int("sort_order").Default(0),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxTask) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("tasks").Unique().Required().Field("project_id"),
	}
}

func (UxTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "sort_order"),
		index.Fields("status"),
	}
}
