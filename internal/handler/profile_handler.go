package handler

import (
	"encoding/json"
	"fmt"

	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: profileService}
}

// Generate AI生成画像（非流式，兼容保留）
// POST /api/profiles/generate
func (h *ProfileHandler) Generate(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateProfilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.profileService.Generate(c.Request.Context(), operatorID, req)
	if err != nil {
		if err.Error() == "无权在该项目下生成画像" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "生成成功", result)
}

// GenerateStream AI生成画像（流式 SSE）
// POST /api/profiles/generate-stream
func (h *ProfileHandler) GenerateStream(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateProfilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.SSEvent("error", "参数校验失败: "+err.Error())
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	// 流式生成，每个 token 通过 SSE 推送
	result, err := h.profileService.GenerateStream(
		c.Request.Context(),
		operatorID,
		req,
		func(chunk string) {
			c.SSEvent("chunk", chunk)
			c.Writer.Flush()
		},
	)

	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
		return
	}

	// 推送最终结果
	resultJSON, _ := json.Marshal(result)
	c.SSEvent("done", string(resultJSON))
	c.Writer.Flush()
}

// GenerateStreamGET GET 方式的流式端点（用于 EventSource）
// GET /api/profiles/generate-stream?project_id=xxx&count=5&model_channel=openai_compatible&filters=JSON
func (h *ProfileHandler) GenerateStreamGET(c *gin.Context) {
	operatorID := getOperatorID(c)
	count := 5
	if v := c.Query("count"); v != "" {
		fmt.Sscanf(v, "%d", &count)
	}
	req := model.GenerateProfilesRequest{
		ProjectID:    c.Query("project_id"),
		Count:        count,
		ProfileType:  c.DefaultQuery("profile_type", "normal"),
		ModelChannel: c.Query("model_channel"),
	}

	// 解析 filters JSON
	if filtersStr := c.Query("filters"); filtersStr != "" {
		var filters []model.ProfileDimensionFilter
		if err := json.Unmarshal([]byte(filtersStr), &filters); err == nil {
			req.Filters = filters
		}
	}

	// 先设置 SSE 响应头，确保所有事件（包括 error）都以 SSE 格式返回
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	if req.ProjectID == "" {
		c.SSEvent("error", "project_id 不能为空")
		c.Writer.Flush()
		return
	}

	result, err := h.profileService.GenerateStream(
		c.Request.Context(),
		operatorID,
		req,
		func(chunk string) {
			c.SSEvent("chunk", chunk)
			c.Writer.Flush()
		},
	)

	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
		return
	}

	resultJSON, _ := json.Marshal(result)
	c.SSEvent("done", string(resultJSON))
	c.Writer.Flush()
}

// List 画像列表
// GET /api/projects/:id/profiles
func (h *ProfileHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var query model.ProfileListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.profileService.List(c.Request.Context(), operatorID, projectID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GetByID 画像详情
// GET /api/profiles/:id
func (h *ProfileHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	profileID := c.Param("id")
	detail, err := h.profileService.GetByID(c.Request.Context(), operatorID, profileID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, detail)
}

// Update 更新画像
// PUT /api/profiles/:id
func (h *ProfileHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	profileID := c.Param("id")
	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.profileService.Update(c.Request.Context(), operatorID, profileID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", detail)
}

// Delete 删除画像
// DELETE /api/profiles/:id
func (h *ProfileHandler) Delete(c *gin.Context) {
	operatorID := getOperatorID(c)
	profileID := c.Param("id")
	if err := h.profileService.Delete(c.Request.Context(), operatorID, profileID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// BatchDelete 批量删除画像
// POST /api/profiles/batch-delete
func (h *ProfileHandler) BatchDelete(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.BatchDeleteProfilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	deleted, err := h.profileService.BatchDelete(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, fmt.Sprintf("成功删除 %d 个画像", deleted), map[string]int{"deleted": deleted})
}
