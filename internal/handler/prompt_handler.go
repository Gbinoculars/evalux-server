package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type PromptHandler struct {
	promptService *service.PromptService
}

func NewPromptHandler(promptService *service.PromptService) *PromptHandler {
	return &PromptHandler{promptService: promptService}
}

// List 获取项目提示词列表
// GET /api/projects/:id/prompts
func (h *PromptHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	result, err := h.promptService.ListPrompts(c.Request.Context(), operatorID, projectID)
	if err != nil {
		if err.Error() == "无权查看该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// Update 保存自定义提示词
// PUT /api/projects/:id/prompts
func (h *PromptHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	var req model.UpdatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.promptService.UpdatePrompt(c.Request.Context(), operatorID, projectID, req); err != nil {
		if err.Error() == "无权编辑该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "保存成功", nil)
}

// Reset 重置提示词为默认值
// POST /api/projects/:id/prompts/reset
func (h *PromptHandler) Reset(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	var req model.ResetPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.promptService.ResetPrompt(c.Request.Context(), operatorID, projectID, req); err != nil {
		if err.Error() == "无权编辑该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "重置成功", nil)
}
