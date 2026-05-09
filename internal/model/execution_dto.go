package model

import "time"

// ==================== 执行会话请求 ====================

// StartExecutionRequest 启动评估执行
type StartExecutionRequest struct {
	ProjectID    string `json:"project_id" binding:"required"`
	TaskID       string `json:"task_id"`
	ProfileID    string `json:"profile_id"`
	DeviceSerial string `json:"device_serial"`
	BatchID      string `json:"batch_id"`            // 同一次批量启动的标识
	SessionID    string `json:"session_id,omitempty"` // 由 StartRun 预创建的会话 ID（领取式启动）
}

// ReportStepRequest 客户端上报一轮执行结果（截图 + 状态）
type ReportStepRequest struct {
	SessionID   string `json:"session_id" binding:"required"`
	StepNo      int    `json:"step_no" binding:"required,min=1"`
	ScreenDesc  string `json:"screen_desc"`
	ActionType  string `json:"action_type"`
	ActionParam map[string]interface{} `json:"action_param"`
	ExecResult  map[string]interface{} `json:"exec_result"`
	ErrorMsg    string `json:"error_message"`
	RetryCount  int    `json:"retry_count"`
}

// ReportStepResponse 后端返回下一步动作指令
type ReportStepResponse struct {
	Action       *ActionInstruction `json:"action"`
	SessionEnded bool               `json:"session_ended"`
	StopReason   string             `json:"stop_reason,omitempty"`
	RawLLMOutput string             `json:"raw_llm_output,omitempty"` // 模型原始输出（调试用）
}

// ActionInstruction 模型生成的动作指令
type ActionInstruction struct {
	ActionType      string                 `json:"action_type"`
	ActionParam     map[string]interface{} `json:"action_param"`
	DecisionReason  string                 `json:"decision_reason"`
	TaskState       string                 `json:"task_state"`
	NeedContinue    bool                   `json:"need_continue"`
}

// FinishSessionRequest 结束会话
type FinishSessionRequest struct {
	SessionID      string `json:"session_id" binding:"required"`
	IsGoalCompleted bool  `json:"is_goal_completed"`
	StopReason     string `json:"stop_reason"`
}

// UploadRecordingRequest 上传录屏
type UploadRecordingRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// ==================== 执行会话响应 ====================

type SessionDetail struct {
	SessionID       string     `json:"session_id"`
	BatchID         string     `json:"batch_id"`
	BatchRole       string     `json:"batch_role,omitempty"`
	RunID           string     `json:"run_id,omitempty"`
	ProjectID       string     `json:"project_id,omitempty"`
	TaskID          string     `json:"task_id"`
	ProfileID       string     `json:"profile_id"`
	ModelSessionID  string     `json:"model_session_id"`
	DeviceSerial    string     `json:"device_serial"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	Status          string     `json:"status"`
	ErrorCount      int        `json:"error_count"`
	TotalDurationMs *int64     `json:"total_duration_ms"`
	IsGoalCompleted bool       `json:"is_goal_completed"`
	StopReason      string     `json:"stop_reason"`
	StepCount       int        `json:"step_count"`
	RecordingURL    string     `json:"recording_url,omitempty"`
}

type SessionListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=500"`
	Status   string `form:"status"`
	BatchIDs string `form:"batch_ids"` // 逗号分隔的 batch_id 列表，用于按批次过滤
}

type SessionListResponse struct {
	Total int64           `json:"total"`
	List  []SessionDetail `json:"list"`
}

type StepDetail struct {
	StepID          string                 `json:"step_id"`
	SessionID       string                 `json:"session_id"`
	StepNo          int                    `json:"step_no"`
	ScreenDesc      string                 `json:"screen_desc"`
	ActionType      string                 `json:"action_type"`
	ActionParam     map[string]interface{} `json:"action_param"`
	DecisionSummary string                 `json:"decision_summary"`
	ExecResult      map[string]interface{} `json:"exec_result"`
	ErrorMessage    string                 `json:"error_message"`
	RetryCount      int                    `json:"retry_count"`
	ScreenshotURL   string                 `json:"screenshot_url"`
	StartedAt       time.Time              `json:"started_at"`
	EndedAt         *time.Time             `json:"ended_at"`
}
