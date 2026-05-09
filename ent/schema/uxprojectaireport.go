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

// UxProjectAiReport 项目级 AI 分析报告持久化存储
type UxProjectAiReport struct{ ent.Schema }

func (UxProjectAiReport) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ux_project_ai_report"}}
}

func (UxProjectAiReport) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("report_id"),
		field.UUID("project_id", uuid.UUID{}),
		field.Text("ai_summary").Comment("AI 总体评价文本"),
		field.JSON("ai_strengths", []string{}).Comment("AI 识别的优点列表"),
		field.JSON("ai_weaknesses", []string{}).Comment("AI 识别的问题列表"),
		field.JSON("ai_recommendations", []string{}).Comment("AI 改进建议列表"),
		field.String("model_channel").MaxLen(32).Default("").Comment("生成所用的 AI 渠道"),
		field.JSON("session_ids", []string{}).Optional().Comment("本次分析覆盖的会话 ID 列表，空表示全量"),
		field.Text("html_content").Optional().Default("").Comment("AI 生成的 HTML 可视化报告内容"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (UxProjectAiReport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "created_at"),
	}
}
