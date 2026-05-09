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

// UxRunProfileSnapshot 画像集合快照：本次 run 要用哪些画像及其语义。
type UxRunProfileSnapshot struct{ ent.Schema }

func (UxRunProfileSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_run_profile_snapshot"}}
}

func (UxRunProfileSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("snapshot_id"),
		field.UUID("run_id", uuid.UUID{}),
		field.UUID("source_binding_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("profile_id_ref", uuid.UUID{}),
		field.String("profile_type_snapshot").MaxLen(32).Default("normal"),
		field.String("age_group_snapshot").MaxLen(32),
		field.String("gender_snapshot").MaxLen(16),
		field.String("education_level_snapshot").MaxLen(32),
		field.JSON("custom_fields_snapshot", map[string]interface{}{}).Optional(),
		field.Int("execution_order").Default(0),
	}
}

func (UxRunProfileSnapshot) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("run", UxExecutionRun.Type).Ref("profile_snapshots").Unique().Required().Field("run_id"),
	}
}

func (UxRunProfileSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "profile_id_ref").Unique(),
		index.Fields("run_id", "execution_order"),
	}
}
