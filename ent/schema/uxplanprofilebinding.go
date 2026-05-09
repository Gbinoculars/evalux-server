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

// UxPlanProfileBinding 计划要让哪些画像各跑一遍。
type UxPlanProfileBinding struct{ ent.Schema }

func (UxPlanProfileBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_plan_profile_binding"}}
}

func (UxPlanProfileBinding) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("binding_id"),
		field.UUID("plan_id", uuid.UUID{}),
		field.UUID("profile_id", uuid.UUID{}),
		field.Int("execution_order").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxPlanProfileBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plan", UxExecutionPlan.Type).Ref("profile_bindings").Unique().Required().Field("plan_id"),
	}
}

func (UxPlanProfileBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plan_id", "profile_id").Unique(),
		index.Fields("plan_id", "enabled"),
	}
}
