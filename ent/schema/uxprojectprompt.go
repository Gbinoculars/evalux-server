package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// UxProjectPrompt 项目级 AI 提示词自定义配置
type UxProjectPrompt struct{ ent.Schema }

func (UxProjectPrompt) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project_prompt"}}
}

func (UxProjectPrompt) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("prompt_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.String("prompt_key").MaxLen(64).NotEmpty().Comment("提示词标识 key，如 execution_system"),
		field.Text("prompt_content").NotEmpty().Comment("用户自定义的提示词内容"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UxProjectPrompt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "prompt_key").Unique(),
		index.Fields("project_id"),
	}
}
