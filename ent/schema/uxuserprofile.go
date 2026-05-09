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

// UxUserProfile 纯画像定义：不再持有任何反向绑定边。
type UxUserProfile struct{ ent.Schema }

func (UxUserProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_user_profile"}}
}

func (UxUserProfile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("profile_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.String("profile_type").MaxLen(32).Default("normal").Comment("画像类型: normal=普通群众, expert=专家"),
		field.String("age_group").MaxLen(32).NotEmpty(),
		field.String("education_level").MaxLen(32).NotEmpty(),
		field.String("gender").MaxLen(16).NotEmpty(),
		field.JSON("custom_fields", map[string]interface{}{}).Optional(),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxUserProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", UxProject.Type).Ref("profiles").Unique().Required().Field("project_id"),
	}
}

func (UxUserProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "enabled"),
	}
}
