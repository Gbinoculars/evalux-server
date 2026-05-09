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

type UxScreenshotRecord struct{ ent.Schema }

func (UxScreenshotRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_screenshot_record"}}
}

func (UxScreenshotRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("screenshot_id"),
		field.UUID("session_id", uuid.UUID{}),
		field.UUID("step_id", uuid.UUID{}),
		field.Text("file_path").NotEmpty(),
		field.Time("shot_at").Default(time.Now),
		field.Int("width").Optional().Nillable(),
		field.Int("height").Optional().Nillable(),
		field.Bool("is_key_frame").Default(false),
	}
}

func (UxScreenshotRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("screenshots").Unique().Required().Field("session_id"),
		edge.From("step", UxExecutionStep.Type).Ref("screenshots").Unique().Required().Field("step_id"),
	}
}

func (UxScreenshotRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id", "step_id"),
		index.Fields("shot_at"),
		index.Fields("is_key_frame"),
	}
}
