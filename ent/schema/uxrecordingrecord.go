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

type UxRecordingRecord struct{ ent.Schema }

func (UxRecordingRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_recording_record"}}
}

func (UxRecordingRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("recording_id"),
		field.UUID("session_id", uuid.UUID{}).Unique(),
		field.Text("file_path").NotEmpty(),
		field.Time("started_at").Default(time.Now),
		field.Time("ended_at").Optional().Nillable(),
		field.Int64("file_size_bytes").Optional().Nillable(),
		field.Enum("storage_status").Values("RECORDING", "SAVED", "FAILED").Default("RECORDING"),
	}
}

func (UxRecordingRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", UxExecutionSession.Type).Ref("recording").Unique().Required().Field("session_id"),
	}
}

func (UxRecordingRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("storage_status"),
	}
}
