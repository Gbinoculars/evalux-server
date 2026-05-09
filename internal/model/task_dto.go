package model

import "time"

// ==================== 任务管理请求 ====================

type CreateTaskRequest struct {
	ProjectID       string                   `json:"project_id" binding:"required"`
	TaskName        string                   `json:"task_name" binding:"required,min=1,max=128"`
	TaskGoal        string                   `json:"task_goal" binding:"required"`
	Precondition    string                   `json:"precondition"`
	ExecutionGuide  string                   `json:"execution_guide"`
	StepConstraints []map[string]interface{} `json:"step_constraints"`
	SuccessCriteria string                   `json:"success_criteria" binding:"required"`
	FailureRule     string                   `json:"failure_rule"`
	TimeoutSeconds  int                      `json:"timeout_seconds" binding:"required,min=30"`
	MinSteps        *int                     `json:"min_steps"`
	MaxSteps        *int                     `json:"max_steps"`
	SortOrder       int                      `json:"sort_order"`
}

type UpdateTaskRequest struct {
	TaskName        *string                  `json:"task_name" binding:"omitempty,min=1,max=128"`
	TaskGoal        *string                  `json:"task_goal"`
	Precondition    *string                  `json:"precondition"`
	ExecutionGuide  *string                  `json:"execution_guide"`
	StepConstraints []map[string]interface{} `json:"step_constraints"`
	SuccessCriteria *string                  `json:"success_criteria"`
	FailureRule     *string                  `json:"failure_rule"`
	TimeoutSeconds  *int                     `json:"timeout_seconds" binding:"omitempty,min=30"`
	MinSteps        *int                     `json:"min_steps"`
	MaxSteps        *int                     `json:"max_steps"`
	SortOrder       *int                     `json:"sort_order"`
	Status          *string                  `json:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}

type TaskListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}

// ==================== 任务管理响应 ====================

type TaskDetail struct {
	TaskID          string                   `json:"task_id"`
	ProjectID       string                   `json:"project_id"`
	TaskName        string                   `json:"task_name"`
	TaskGoal        string                   `json:"task_goal"`
	Precondition    string                   `json:"precondition"`
	ExecutionGuide  string                   `json:"execution_guide"`
	StepConstraints []map[string]interface{} `json:"step_constraints"`
	SuccessCriteria string                   `json:"success_criteria"`
	FailureRule     string                   `json:"failure_rule"`
	TimeoutSeconds  int                      `json:"timeout_seconds"`
	MinSteps        *int                     `json:"min_steps"`
	MaxSteps        *int                     `json:"max_steps"`
	SortOrder       int                      `json:"sort_order"`
	Status          string                   `json:"status"`
	CreatedAt       time.Time                `json:"created_at"`
}

type TaskListResponse struct {
	Total int64        `json:"total"`
	List  []TaskDetail `json:"list"`
}

// ==================== 问卷管理请求 ====================

type CreateQuestionnaireRequest struct {
	ProjectID       string   `json:"project_id"`
	TemplateName    string   `json:"template_name" binding:"required,min=1,max=128"`
	DimensionSchema []string `json:"dimension_schema" binding:"required"`
	TemplateDesc    string   `json:"template_desc"`
}

type UpdateQuestionnaireRequest struct {
	TemplateName    *string  `json:"template_name" binding:"omitempty,min=1,max=128"`
	DimensionSchema []string `json:"dimension_schema"`
	TemplateDesc    *string  `json:"template_desc"`
	Status          *string  `json:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}

type QuestionnaireListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=ACTIVE DISABLED"`
}

// ==================== 问卷管理响应 ====================

type QuestionnaireDetail struct {
	TemplateID      string    `json:"template_id"`
	ProjectID       string    `json:"project_id"`
	TemplateName    string    `json:"template_name"`
	DimensionSchema []string  `json:"dimension_schema"`
	TemplateDesc    string    `json:"template_desc"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type QuestionnaireListResponse struct {
	Total int64                 `json:"total"`
	List  []QuestionnaireDetail `json:"list"`
}

// ==================== 问卷题目请求 ====================

type CreateQuestionRequest struct {
	TemplateID    string              `json:"template_id" binding:"required"`
	QuestionNo    int                 `json:"question_no" binding:"required,min=1"`
	QuestionType  string              `json:"question_type" binding:"required,oneof=SCALE SINGLE_CHOICE MULTIPLE_CHOICE OPEN_ENDED"`
	QuestionText  string              `json:"question_text" binding:"required"`
	OptionList    []map[string]string `json:"option_list"`
	ScoreRange    map[string]int      `json:"score_range"`
	DimensionCode string              `json:"dimension_code"`
	IsRequired    bool                `json:"is_required"`
}

type UpdateQuestionRequest struct {
	QuestionNo    *int                `json:"question_no"`
	QuestionType  *string             `json:"question_type" binding:"omitempty,oneof=SCALE SINGLE_CHOICE MULTIPLE_CHOICE OPEN_ENDED"`
	QuestionText  *string             `json:"question_text"`
	OptionList    []map[string]string `json:"option_list"`
	ScoreRange    map[string]int      `json:"score_range"`
	DimensionCode *string             `json:"dimension_code"`
	IsRequired    *bool               `json:"is_required"`
}

type ReorderQuestionsRequest struct {
	QuestionIDs []string `json:"question_ids" binding:"required,min=1"`
}

// ==================== AI 生成问卷 ====================

type AIGenerateQuestionnaireRequest struct {
	ProjectID      string   `json:"project_id"`
	TemplateName   string   `json:"template_name" binding:"required,min=1,max=128"`
	Aspects        []string `json:"aspects" binding:"required,min=1"`            // 评价的方面（维度）
	TheoryBasis    string   `json:"theory_basis" binding:"required"`             // 理论基础
	ScaleCount     int      `json:"scale_count" binding:"required,min=1,max=50"` // scale题目数量
	ScaleOptions   int      `json:"scale_options" binding:"required,min=2,max=10"` // scale题目选项数（1~N）
	OpenEndedCount int      `json:"open_ended_count" binding:"min=0,max=20"`     // 主观题数量
	ModelChannel   string   `json:"model_channel"`
}

type QuestionDetail struct {
	QuestionID    string              `json:"question_id"`
	TemplateID    string              `json:"template_id"`
	QuestionNo    int                 `json:"question_no"`
	QuestionType  string              `json:"question_type"`
	QuestionText  string              `json:"question_text"`
	OptionList    []map[string]string `json:"option_list"`
	ScoreRange    map[string]int      `json:"score_range"`
	DimensionCode string              `json:"dimension_code"`
	IsRequired    bool                `json:"is_required"`
}
