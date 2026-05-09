package model

import "time"

// ==================== 画像管理请求 ====================

type GenerateProfilesRequest struct {
	ProjectID    string                 `json:"project_id" binding:"required"`
	Count        int                    `json:"count" binding:"required,min=1,max=50"`
	ProfileType  string                 `json:"profile_type" binding:"omitempty,oneof=normal expert"` // normal=普通群众, expert=专家
	FieldDefs    []ProfileFieldDef      `json:"field_defs"`
	CustomFields map[string]interface{} `json:"custom_fields"`
	ModelChannel string                 `json:"model_channel" binding:"omitempty,oneof=ollama openrouter openai_compatible"`
	// 维度筛选条件：用户可以限定生成画像的范围
	Filters []ProfileDimensionFilter `json:"filters"`
}

type ProfileFieldDef struct {
	FieldName   string   `json:"field_name" binding:"required"`
	FieldType   string   `json:"field_type" binding:"required,oneof=enum range text"`
	Candidates  []string `json:"candidates"`
	RangeMin    string   `json:"range_min"`
	RangeMax    string   `json:"range_max"`
	Description string   `json:"description"`
}

// ProfileDimensionFilter 维度筛选条件
type ProfileDimensionFilter struct {
	Dimension string   `json:"dimension"` // 维度名，如 education_level, age_group, gender 等
	Values    []string `json:"values"`    // 允许的值列表，如 ["高中","初中"]
}

type UpdateProfileRequest struct {
	AgeGroup       *string                `json:"age_group"`
	EducationLevel *string                `json:"education_level"`
	Gender         *string                `json:"gender"`
	CustomFields   map[string]interface{} `json:"custom_fields"`
	Enabled        *bool                  `json:"enabled"`
}

type ProfileListQuery struct {
	Page        int    `form:"page" binding:"omitempty,min=1"`
	PageSize    int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Enabled     string `form:"enabled" binding:"omitempty,oneof=true false"`
	ProfileType string `form:"profile_type" binding:"omitempty,oneof=normal expert"`
}

// BatchDeleteProfilesRequest 批量删除画像请求
type BatchDeleteProfilesRequest struct {
	ProfileIDs []string `json:"profile_ids" binding:"required,min=1"`
}

// ==================== 画像管理响应 ====================

type ProfileDetail struct {
	ProfileID      string                 `json:"profile_id"`
	ProjectID      string                 `json:"project_id"`
	ProfileType    string                 `json:"profile_type"`
	AgeGroup       string                 `json:"age_group"`
	EducationLevel string                 `json:"education_level"`
	Gender         string                 `json:"gender"`
	CustomFields   map[string]interface{} `json:"custom_fields"`
	Enabled        bool                   `json:"enabled"`
	CreatedAt      time.Time              `json:"created_at"`
}

type ProfileListResponse struct {
	Total int64           `json:"total"`
	List  []ProfileDetail `json:"list"`
}

type GenerateProfilesResponse struct {
	Generated int             `json:"generated"`
	Profiles  []ProfileDetail `json:"profiles"`
}
