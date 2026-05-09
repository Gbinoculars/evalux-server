package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type QuestionnaireHandler struct {
	qService *service.QuestionnaireService
}

func NewQuestionnaireHandler(qService *service.QuestionnaireService) *QuestionnaireHandler {
	return &QuestionnaireHandler{qService: qService}
}

// CreateTemplate POST /api/questionnaires
func (h *QuestionnaireHandler) CreateTemplate(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreateQuestionnaireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.qService.CreateTemplate(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "创建成功", detail)
}

// ListTemplates GET /api/projects/:id/questionnaires
func (h *QuestionnaireHandler) ListTemplates(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var query model.QuestionnaireListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.qService.ListTemplates(c.Request.Context(), operatorID, projectID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GetTemplate GET /api/questionnaires/:id
func (h *QuestionnaireHandler) GetTemplate(c *gin.Context) {
	operatorID := getOperatorID(c)
	detail, err := h.qService.GetTemplate(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, detail)
}

// UpdateTemplate PUT /api/questionnaires/:id
func (h *QuestionnaireHandler) UpdateTemplate(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.UpdateQuestionnaireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.qService.UpdateTemplate(c.Request.Context(), operatorID, c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", detail)
}

// DeleteTemplate DELETE /api/questionnaires/:id
func (h *QuestionnaireHandler) DeleteTemplate(c *gin.Context) {
	operatorID := getOperatorID(c)
	if err := h.qService.DeleteTemplate(c.Request.Context(), operatorID, c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// CreateQuestion POST /api/questionnaires/:id/questions
func (h *QuestionnaireHandler) CreateQuestion(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	req.TemplateID = c.Param("id")
	detail, err := h.qService.CreateQuestion(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "创建成功", detail)
}

// ListQuestions GET /api/questionnaires/:id/questions
func (h *QuestionnaireHandler) ListQuestions(c *gin.Context) {
	operatorID := getOperatorID(c)
	list, err := h.qService.ListQuestions(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// DeleteQuestion DELETE /api/questions/:id
func (h *QuestionnaireHandler) DeleteQuestion(c *gin.Context) {
	operatorID := getOperatorID(c)
	if err := h.qService.DeleteQuestion(c.Request.Context(), operatorID, c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// UpdateQuestion PUT /api/questions/:id
func (h *QuestionnaireHandler) UpdateQuestion(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.UpdateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.qService.UpdateQuestion(c.Request.Context(), operatorID, c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", detail)
}

// ReorderQuestions POST /api/questionnaires/:id/reorder
func (h *QuestionnaireHandler) ReorderQuestions(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.ReorderQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.qService.ReorderQuestions(c.Request.Context(), operatorID, c.Param("id"), req.QuestionIDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "排序成功", nil)
}

// AIGenerateQuestionnaire POST /api/projects/:id/questionnaires/ai-generate
func (h *QuestionnaireHandler) AIGenerateQuestionnaire(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.AIGenerateQuestionnaireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	req.ProjectID = c.Param("id")
	detail, err := h.qService.AIGenerateQuestionnaire(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "AI 生成成功", detail)
}
