package model

import "time"

// ==================== 项目级大模型配置 ====================

// ModelConfig 项目级大模型配置
type ModelConfig struct {
	DefaultChannel           string `json:"default_channel,omitempty"`
	OllamaBaseURL            string `json:"ollama_base_url,omitempty"`
	OllamaModel              string `json:"ollama_model,omitempty"`
	OpenRouterBaseURL        string `json:"openrouter_base_url,omitempty"`
	OpenRouterAPIKey         string `json:"openrouter_api_key,omitempty"`
	OpenRouterModel          string `json:"openrouter_model,omitempty"`
	OpenAICompatibleBaseURL  string `json:"openai_compatible_base_url,omitempty"`
	OpenAICompatibleAPIKey   string `json:"openai_compatible_api_key,omitempty"`
	OpenAICompatibleModel    string `json:"openai_compatible_model,omitempty"`
	// 被测应用平台配置（用于任务结束后重置应用状态）
	Platform string `json:"platform,omitempty"` // android | ios
	AppID    string `json:"app_id,omitempty"`   // Android 包名，如 com.eg.android.AlipayGphone
	BundleID string `json:"bundle_id,omitempty"` // iOS Bundle ID，如 com.tencent.xin
	WdaURL   string `json:"wda_url,omitempty"`  // iOS WDA 服务地址，如 http://localhost:8100
}

// ==================== 项目管理请求 ====================

type CreateProjectRequest struct {
	OrgID        *string      `json:"org_id,omitempty"`
	ProjectName  string       `json:"project_name" binding:"required,min=1,max=128"`
	AppName      string       `json:"app_name" binding:"required,min=1,max=128"`
	AppVersion   string       `json:"app_version" binding:"omitempty,max=32"`
	ResearchGoal string       `json:"research_goal" binding:"required"`
	ProjectDesc  string       `json:"project_desc"`
	ModelConfig  *ModelConfig `json:"model_config,omitempty"`
}

type UpdateProjectRequest struct {
	ProjectName  *string      `json:"project_name" binding:"omitempty,min=1,max=128"`
	AppName      *string      `json:"app_name" binding:"omitempty,min=1,max=128"`
	AppVersion   *string      `json:"app_version" binding:"omitempty,max=32"`
	ResearchGoal *string      `json:"research_goal"`
	ProjectDesc  *string      `json:"project_desc"`
	Status       *string      `json:"status" binding:"omitempty,oneof=ACTIVE ARCHIVED"`
	ModelConfig  *ModelConfig `json:"model_config,omitempty"`
}

type ProjectListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   string `form:"status" binding:"omitempty,oneof=ACTIVE ARCHIVED"`
}

// ==================== 项目管理响应 ====================

type ProjectDetail struct {
	ProjectID    string       `json:"project_id"`
	OrgID        *string      `json:"org_id"`
	CreatedBy    string       `json:"created_by"`
	ProjectName  string       `json:"project_name"`
	AppName      string       `json:"app_name"`
	AppVersion   string       `json:"app_version"`
	ResearchGoal string       `json:"research_goal"`
	ProjectDesc  string       `json:"project_desc"`
	ModelConfig  *ModelConfig `json:"model_config,omitempty"`
	Status       string       `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	ArchivedAt   *time.Time   `json:"archived_at"`
}

type ProjectListResponse struct {
	Total int64           `json:"total"`
	List  []ProjectDetail `json:"list"`
}

// ==================== 项目成员 & 角色 ====================

type ProjectMember struct {
	ProjectMemberID string    `json:"project_member_id"`
	ProjectID       string    `json:"project_id"`
	UserID          string    `json:"user_id"`
	ProjectRoleID   string    `json:"project_role_id"`
	CreatedAt       time.Time `json:"created_at"`
	UserAccount     string    `json:"user_account,omitempty"`
	UserNickname    string    `json:"user_nickname,omitempty"`
	RoleCode        string    `json:"role_code,omitempty"`
	RoleName        string    `json:"role_name,omitempty"`
}

type ProjectRole struct {
	ProjectRoleID   string   `json:"project_role_id"`
	RoleCode        string   `json:"role_code"`
	RoleName        string   `json:"role_name"`
	PermissionCodes []string `json:"permission_codes"`
	Description     *string  `json:"description"`
}

type AddProjectMemberRequest struct {
	UserID   string `json:"user_id"   binding:"required"`
	RoleCode string `json:"role_code" binding:"required,oneof=OWNER ADMIN EDITOR VIEWER"`
}

type UpdateProjectMemberRoleRequest struct {
	RoleCode string `json:"role_code" binding:"required,oneof=OWNER ADMIN EDITOR VIEWER"`
}
