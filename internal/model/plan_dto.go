package model

import "time"

// ==================== 执行计划 DTO ====================

// PlanModelConfig 计划下的模型配置（CONTROL/TREATMENT 各一行）
//
// 重要：自 v1.x 起，plan 层模型配置改为"引用项目模型配置"语义：
//   - 前端只需提交 model_role + channel（CONTROL/TREATMENT 渠道选择）
//   - model_name / api_base_url / api_key_cipher 由后端在启动 run 时
//     从 project.model_config 中按 channel 解析填入，前端不再填写
//   - temperature / top_p / max_tokens / reasoning_effort / extra_params
//     仍允许前端覆盖（可选）
type PlanModelConfig struct {
	ConfigID        string  `json:"config_id,omitempty"`
	ModelRole       string  `json:"model_role" binding:"required,oneof=CONTROL TREATMENT"`
	Channel         string  `json:"channel" binding:"required,oneof=ollama openrouter openai_compatible"`
	ModelType       string  `json:"model_type" binding:"omitempty,oneof=multimodal text"`
	// 以下三项保留在 DTO 中以兼容历史接口，但前端不应再填写；后端写入 plan 表时会忽略
	ModelName       string  `json:"model_name,omitempty"`
	APIBaseURL      string  `json:"api_base_url,omitempty"`
	APIKeyCipher    string  `json:"api_key_cipher,omitempty"`
	Temperature     *float64 `json:"temperature"`
	TopP            *float64 `json:"top_p"`
	MaxTokens       *int    `json:"max_tokens"`
	ReasoningEffort string  `json:"reasoning_effort"`
	ExtraParams     string  `json:"extra_params"`
}

// PlanTaskBindingDTO 计划-任务绑定
type PlanTaskBindingDTO struct {
	BindingID      string `json:"binding_id,omitempty"`
	TaskID         string `json:"task_id" binding:"required"`
	ExecutionOrder int    `json:"execution_order"`
	Enabled        bool   `json:"enabled"`
	// 展示字段（可选，从 task 联表来）
	TaskName string `json:"task_name,omitempty"`
}

// PlanProfileBindingDTO 计划-画像绑定
type PlanProfileBindingDTO struct {
	BindingID      string `json:"binding_id,omitempty"`
	ProfileID      string `json:"profile_id" binding:"required"`
	ExecutionOrder int    `json:"execution_order"`
	Enabled        bool   `json:"enabled"`
}

// PlanTaskQuestionnaireBindingDTO 任务后问卷绑定
type PlanTaskQuestionnaireBindingDTO struct {
	BindingID     string `json:"binding_id,omitempty"`
	TaskID        string `json:"task_id" binding:"required"`
	TemplateID    string `json:"template_id" binding:"required"`
	QuestionOrder int    `json:"question_order"`
	Enabled       bool   `json:"enabled"`
	// 展示字段
	TemplateName string `json:"template_name,omitempty"`
}

// PlanProfileQuestionnaireBindingDTO 画像收尾问卷绑定
type PlanProfileQuestionnaireBindingDTO struct {
	BindingID     string `json:"binding_id,omitempty"`
	ProfileID     string `json:"profile_id" binding:"required"`
	TemplateID    string `json:"template_id" binding:"required"`
	QuestionOrder int    `json:"question_order"`
	Enabled       bool   `json:"enabled"`
	// 展示字段
	TemplateName string `json:"template_name,omitempty"`
}

// CreatePlanRequest 创建执行计划请求（一次性带上四类绑定）
type CreatePlanRequest struct {
	ProjectID                    string                              `json:"project_id" binding:"required"`
	PlanName                     string                              `json:"plan_name" binding:"required"`
	PlanType                     string                              `json:"plan_type" binding:"required,oneof=NORMAL AB_TEST EXPERT"`
	MaxConcurrency               int                                 `json:"max_concurrency"`
	StepTimeoutSec               int                                 `json:"step_timeout_sec"`
	SessionTimeoutSec            int                                 `json:"session_timeout_sec"`
	RetryLimit                   int                                 `json:"retry_limit"`
	PromptOverrideID             string                              `json:"prompt_override_id"`
	Hypothesis                   string                              `json:"hypothesis"`
	ModelConfigs                 []PlanModelConfig                   `json:"model_configs"`
	TaskBindings                 []PlanTaskBindingDTO                `json:"task_bindings"`
	ProfileBindings              []PlanProfileBindingDTO             `json:"profile_bindings"`
	TaskQuestionnaireBindings    []PlanTaskQuestionnaireBindingDTO   `json:"task_questionnaire_bindings"`
	ProfileQuestionnaireBindings []PlanProfileQuestionnaireBindingDTO `json:"profile_questionnaire_bindings"`
}

// UpdatePlanRequest 更新执行计划（同样支持一次性整体覆盖四类绑定）
type UpdatePlanRequest struct {
	PlanName                     *string                              `json:"plan_name"`
	MaxConcurrency               *int                                 `json:"max_concurrency"`
	StepTimeoutSec               *int                                 `json:"step_timeout_sec"`
	SessionTimeoutSec            *int                                 `json:"session_timeout_sec"`
	RetryLimit                   *int                                 `json:"retry_limit"`
	PromptOverrideID             *string                              `json:"prompt_override_id"`
	Hypothesis                   *string                              `json:"hypothesis"`
	Status                       *string                              `json:"status" binding:"omitempty,oneof=READY ARCHIVED"`
	ModelConfigs                 []PlanModelConfig                    `json:"model_configs"`
	TaskBindings                 []PlanTaskBindingDTO                 `json:"task_bindings"`
	ProfileBindings              []PlanProfileBindingDTO              `json:"profile_bindings"`
	TaskQuestionnaireBindings    []PlanTaskQuestionnaireBindingDTO    `json:"task_questionnaire_bindings"`
	ProfileQuestionnaireBindings []PlanProfileQuestionnaireBindingDTO `json:"profile_questionnaire_bindings"`
}

// PlanDetail 执行计划详情（含全部子绑定）
type PlanDetail struct {
	PlanID                       string                               `json:"plan_id"`
	ProjectID                    string                               `json:"project_id"`
	PlanName                     string                               `json:"plan_name"`
	PlanType                     string                               `json:"plan_type"`
	MaxConcurrency               int                                  `json:"max_concurrency"`
	StepTimeoutSec               int                                  `json:"step_timeout_sec"`
	SessionTimeoutSec            int                                  `json:"session_timeout_sec"`
	RetryLimit                   int                                  `json:"retry_limit"`
	PromptOverrideID             string                               `json:"prompt_override_id"`
	Hypothesis                   string                               `json:"hypothesis"`
	Status                       string                               `json:"status"`
	CreatedBy                    string                               `json:"created_by"`
	CreatedAt                    time.Time                            `json:"created_at"`
	UpdatedAt                    time.Time                            `json:"updated_at"`
	ModelConfigs                 []PlanModelConfig                    `json:"model_configs"`
	TaskBindings                 []PlanTaskBindingDTO                 `json:"task_bindings"`
	ProfileBindings              []PlanProfileBindingDTO              `json:"profile_bindings"`
	TaskQuestionnaireBindings    []PlanTaskQuestionnaireBindingDTO    `json:"task_questionnaire_bindings"`
	ProfileQuestionnaireBindings []PlanProfileQuestionnaireBindingDTO `json:"profile_questionnaire_bindings"`
}

// ==================== A/B 测试结果 DTO ====================

type ABTestGroupStats struct {
	BatchID        string  `json:"batch_id"`
	BatchRole      string  `json:"batch_role"` // CONTROL / TREATMENT
	Label          string  `json:"label"`
	SessionCount   int     `json:"session_count"`
	CompletionRate float64 `json:"completion_rate"`
	AvgErrorCount  float64 `json:"avg_error_count"`
	AvgDurationMs  int64   `json:"avg_duration_ms"`
	AvgScore       float64 `json:"avg_score"`
}

type ABTestComparison struct {
	CompletionRateDiff float64 `json:"completion_rate_diff"` // Treatment - Control
	ErrorCountDiff     float64 `json:"error_count_diff"`
	DurationDiffMs     int64   `json:"duration_diff_ms"`
	ScoreDiff          float64 `json:"score_diff"`
	Winner             string  `json:"winner"` // CONTROL | TREATMENT | TIE
}

// ABTestResult 现在以 run_id 为根
type ABTestResult struct {
	RunID      string           `json:"run_id"`
	PlanID     string           `json:"plan_id"`
	PlanName   string           `json:"plan_name"`
	Hypothesis string           `json:"hypothesis"`
	Control    ABTestGroupStats `json:"control"`
	Treatment  ABTestGroupStats `json:"treatment"`
	Comparison ABTestComparison `json:"comparison"`
}

// ABTestStartRequest A/B 测试结果查询请求（按 run_id）
type ABTestStartRequest struct {
	RunID string `json:"run_id" binding:"required"`
}

// ==================== 执行运行记录 DTO ====================

// StartRunRequest 启动一次评估，仅传 plan_id 即可，后端按 plan 展开 batch/session
type StartRunRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
}

type ExecutionRunDetail struct {
	RunID        string             `json:"run_id"`
	ProjectID    string             `json:"project_id"`
	PlanIDRef    string             `json:"plan_id_ref,omitempty"`
	PlanName     string             `json:"plan_name"`
	PlanType     string             `json:"plan_type"`
	Status       string             `json:"status"`
	StartedBy    string             `json:"started_by"`
	StartedAt    time.Time          `json:"started_at"`
	FinishedAt   *time.Time         `json:"finished_at"`
	Hypothesis   string             `json:"hypothesis,omitempty"`
	Batches      []ExecutionBatchDTO `json:"batches,omitempty"`
}

type ExecutionBatchDTO struct {
	BatchID   string    `json:"batch_id"`
	RunID     string    `json:"run_id"`
	BatchRole string    `json:"batch_role"`
	CreatedAt time.Time `json:"created_at"`
}
