package model

// PromptItem 单个提示词配置项
type PromptItem struct {
	PromptKey     string `json:"prompt_key"`
	PromptLabel   string `json:"prompt_label"`   // 中文展示名
	PromptDesc    string `json:"prompt_desc"`     // 简短说明
	PromptContent string `json:"prompt_content"`  // 当前有效的提示词内容
	IsCustom      bool   `json:"is_custom"`       // true=用户已自定义，false=使用默认值
}

// ListPromptsResponse 项目提示词列表响应
type ListPromptsResponse struct {
	Prompts []PromptItem `json:"prompts"`
}

// UpdatePromptRequest 更新单个提示词请求
type UpdatePromptRequest struct {
	PromptKey     string `json:"prompt_key" binding:"required"`
	PromptContent string `json:"prompt_content" binding:"required"`
}

// ResetPromptRequest 重置提示词为默认值请求
type ResetPromptRequest struct {
	PromptKey string `json:"prompt_key" binding:"required"`
}
