package model

import "time"

// ==================== 结果分析响应 ====================

// ProjectResultOverview 项目级结果总览
type ProjectResultOverview struct {
	ProjectID       string  `json:"project_id"`
	TotalSessions   int     `json:"total_sessions"`
	CompletedCount  int     `json:"completed_count"`
	FailedCount     int     `json:"failed_count"`
	CompletionRate  float64 `json:"completion_rate"`
	AvgDurationMs   int64   `json:"avg_duration_ms"`
	AvgErrorCount   float64 `json:"avg_error_count"`
	AvgScore        float64 `json:"avg_score"`
}

// SessionResult 单会话结果详情
type SessionResult struct {
	Session         SessionDetail          `json:"session"`
	Steps           []StepDetail           `json:"steps"`
	QuestionnaireAnswers []AnswerDetail     `json:"questionnaire_answers"`
	SubjectiveEval  *SubjectiveEvalDetail  `json:"subjective_evaluation"`
	Suggestions     []SuggestionDetail     `json:"suggestions"`
}

// AnswerDetail 问卷回答
type AnswerDetail struct {
	AnswerID        string                 `json:"answer_id"`
	SessionID       string                 `json:"session_id"`
	AnswerOrigin    string                 `json:"answer_origin"`              // AFTER_TASK | AFTER_ALL_PER_PROFILE
	SourceBindingID string                 `json:"source_binding_id,omitempty"` // 指向 run 内绑定快照行 id
	TaskID          string                 `json:"task_id,omitempty"`           // AFTER_TASK 必填，AFTER_ALL_PER_PROFILE 留空
	ProfileID       string                 `json:"profile_id"`
	TemplateID      string                 `json:"template_id"`
	QuestionID      string                 `json:"question_id"`
	AnswerType      string                 `json:"answer_type"`
	AnswerScore     float64                `json:"answer_score"`
	AnswerOption    map[string]interface{} `json:"answer_option"`
	AnswerText      string                 `json:"answer_text"`
	CreatedAt       time.Time              `json:"created_at"`
}

// SubjectiveEvalDetail 主观评价
type SubjectiveEvalDetail struct {
	EvaluationID    string                 `json:"evaluation_id"`
	SessionID       string                 `json:"session_id"`
	OverallScore    float64                `json:"overall_score"`
	SummaryText     string                 `json:"summary_text"`
	BasedOnSnapshot map[string]interface{} `json:"based_on_snapshot"`
	CreatedAt       time.Time              `json:"created_at"`
}

// SuggestionDetail 改进建议
type SuggestionDetail struct {
	SuggestionID   string    `json:"suggestion_id"`
	SessionID      string    `json:"session_id"`
	SuggestionType string    `json:"suggestion_type"`
	PriorityLevel  string    `json:"priority_level"`
	SuggestionText string    `json:"suggestion_text"`
	CreatedAt      time.Time `json:"created_at"`
}

// GenerateEvalRequest 请求为会话生成问卷回答和主观评价
type GenerateEvalRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	ModelChannel string `json:"model_channel" binding:"omitempty,oneof=ollama openrouter openai_compatible"`
}

// GenerateQuestionnaireRequest 请求为会话生成问卷回答
type GenerateQuestionnaireRequest struct {
	SessionID    string `json:"session_id" binding:"required"`
	ModelChannel string `json:"model_channel" binding:"omitempty,oneof=ollama openrouter openai_compatible"`
}

// ==================== 总体报告 ====================

// TaskStat 单任务维度统计
type TaskStat struct {
	TaskID          string  `json:"task_id"`
	TaskName        string  `json:"task_name"`
	TotalSessions   int     `json:"total_sessions"`
	SuccessCount    int     `json:"success_count"`   // is_goal_completed = true
	FailedCount     int     `json:"failed_count"`    // status = FAILED / TIMEOUT
	TotalErrors     int     `json:"total_errors"`
	AvgErrors       float64 `json:"avg_errors"`
	AvgDurationMs   int64   `json:"avg_duration_ms"`
	CompletionRate  float64 `json:"completion_rate"`
	AvgStepCount    float64 `json:"avg_step_count"`
}

// QuestionStat 单题统计
type QuestionStat struct {
	QuestionID    string             `json:"question_id"`
	QuestionNo    int                `json:"question_no"`
	QuestionType  string             `json:"question_type"`
	QuestionText  string             `json:"question_text"`
	DimensionCode string             `json:"dimension_code,omitempty"`
	AnswerCount   int                `json:"answer_count"`
	AvgScore      float64            `json:"avg_score,omitempty"`    // SCALE 类型
	MinScore      float64            `json:"min_score,omitempty"`
	MaxScore      float64            `json:"max_score,omitempty"`
	StdDev        float64            `json:"std_dev,omitempty"`
	OptionCounts  map[string]int     `json:"option_counts,omitempty"` // 选择题各选项次数
	TextAnswers  []string           `json:"text_answers,omitempty"`  // 开放题回答列表
}

// QuestionnaireStat 单问卷模板汇总统计
type QuestionnaireStat struct {
	TemplateID    string         `json:"template_id"`
	TemplateName  string         `json:"template_name"`
	GroupBy       string         `json:"group_by"`      // "task" | "profile"
	GroupLabel    string         `json:"group_label"`   // 任务名 or 画像标签
	GroupID       string         `json:"group_id"`
	TotalAnswers  int            `json:"total_answers"`
	Questions     []QuestionStat `json:"questions"`
}

// GenerateReportRequest 生成总体报告请求
type GenerateReportRequest struct {
	RunID        string   `json:"run_id" binding:"required"`
	ProjectID    string   `json:"project_id"`  // 内部填充
	ModelChannel string   `json:"model_channel" binding:"omitempty,oneof=ollama openrouter openai_compatible"`
	SessionIDs   []string `json:"session_ids,omitempty"` // 可选：在 run 内进一步筛选
}

// ProjectReport 项目总体报告
type ProjectReport struct {
	ProjectID          string                `json:"project_id"`
	Overview           ProjectResultOverview `json:"overview"`
	TaskStats          []TaskStat            `json:"task_stats"`
	QuestionnaireStats []QuestionnaireStat   `json:"questionnaire_stats"`
	AISummary          string                `json:"ai_summary"`
	AIStrengths        []string              `json:"ai_strengths"`
	AIWeaknesses       []string              `json:"ai_weaknesses"`
	AIRecommendations  []string              `json:"ai_recommendations"`
}

// ReportStatsRequest 纯统计请求（不调 AI）
type ReportStatsRequest struct {
	RunID      string   `json:"run_id"`                     // 由 handler 从 URL 填充
	ProjectID  string   `json:"project_id"`                 // 内部填充，无需前端传
	SessionIDs []string `json:"session_ids,omitempty"`      // 可选：在 run 内进一步筛选
}

// AIReportResult 已持久化的 AI 分析报告
type AIReportResult struct {
	ReportID          string    `json:"report_id"`
	ProjectID         string    `json:"project_id"`
	AISummary         string    `json:"ai_summary"`
	AIStrengths       []string  `json:"ai_strengths"`
	AIWeaknesses      []string  `json:"ai_weaknesses"`
	AIRecommendations []string  `json:"ai_recommendations"`
	ModelChannel      string    `json:"model_channel"`
	SessionIDs        []string  `json:"session_ids"`
	CreatedAt         time.Time `json:"created_at"`
}

// ProfileStat 画像维度统计
type ProfileStat struct {
	ProfileID      string  `json:"profile_id"`
	ProfileLabel   string  `json:"profile_label"`  // 如 "男·青年·本科"
	NickName       string  `json:"nick_name"`       // 画像昵称，来自 custom_fields.nickname
	TotalSessions  int     `json:"total_sessions"`
	CompletedCount int     `json:"completed_count"`
	FailedCount    int     `json:"failed_count"`
	AvgErrors      float64 `json:"avg_errors"`
	AvgDurationMs  int64   `json:"avg_duration_ms"`
	AvgStepCount   float64 `json:"avg_step_count"`
}

// ProjectReportStats 纯统计响应（不含 AI 字段，快速返回）
type ProjectReportStats struct {
	ProjectID        string                `json:"project_id"`
	Overview         ProjectResultOverview `json:"overview"`
	TaskStats        []TaskStat            `json:"task_stats"`
	ProfileStats     []ProfileStat         `json:"profile_stats"`
	TaskQuestStats   []QuestionnaireStat   `json:"task_quest_stats"`   // AFTER_TASK 问卷（group_by="task"）
	GlobalQuestStats []QuestionnaireStat   `json:"global_quest_stats"` // AFTER_ALL 问卷（group_by="profile"）
}

// GenerateHTMLReportRequest 生成 HTML 可视化报告请求
type GenerateHTMLReportRequest struct {
	RunID        string   `json:"run_id" binding:"required"`
	ProjectID    string   `json:"project_id"` // 内部填充
	ModelChannel string   `json:"model_channel" binding:"omitempty,oneof=ollama openrouter openai_compatible"`
	SessionIDs   []string `json:"session_ids,omitempty"`
}

// HTMLReportResponse HTML 报告响应
type HTMLReportResponse struct {
	HTML string `json:"html"`
}

// HTMLReportResult 已持久化的 HTML 可视化报告
type HTMLReportResult struct {
	ReportID     string    `json:"report_id"`
	ProjectID    string    `json:"project_id"`
	HTML         string    `json:"html"`
	ModelChannel string    `json:"model_channel"`
	SessionIDs   []string  `json:"session_ids"`
	CreatedAt    time.Time `json:"created_at"`
}
