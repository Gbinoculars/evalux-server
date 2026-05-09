package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type ResultHandler struct {
	resultService *service.ResultService
}

func NewResultHandler(resultService *service.ResultService) *ResultHandler {
	return &ResultHandler{resultService: resultService}
}

// GetProjectOverview GET /api/projects/:id/results/overview
func (h *ResultHandler) GetProjectOverview(c *gin.Context) {
	operatorID := getOperatorID(c)
	overview, err := h.resultService.GetProjectOverview(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, overview)
}

// GetSessionResult GET /api/executions/:id/result
func (h *ResultHandler) GetSessionResult(c *gin.Context) {
	operatorID := getOperatorID(c)
	result, err := h.resultService.GetSessionResult(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GenerateEvaluation POST /api/results/generate-eval
func (h *ResultHandler) GenerateEvaluation(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateEvalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.resultService.GenerateEvaluation(c.Request.Context(), operatorID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "评价生成成功", nil)
}

// GenerateQuestionnaireAnswers POST /api/results/generate-questionnaire
func (h *ResultHandler) GenerateQuestionnaireAnswers(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateEvalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.resultService.GenerateQuestionnaireAnswers(c.Request.Context(), operatorID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "问卷回答已生成", nil)
}

// GenerateSnapshot POST /api/projects/:id/results/snapshot
func (h *ResultHandler) GenerateSnapshot(c *gin.Context) {
	operatorID := getOperatorID(c)
	if err := h.resultService.GenerateSnapshot(c.Request.Context(), operatorID, c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "结果快照已生成", nil)
}

// GetProjectReportStats POST /api/runs/:runId/stats
func (h *ResultHandler) GetProjectReportStats(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.ReportStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	req.RunID = c.Param("runId")
	stats, err := h.resultService.GetProjectReportStats(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, stats)
}

// BatchGetAnswers POST /api/projects/:id/answers/batch
func (h *ResultHandler) BatchGetAnswers(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req struct {
		SessionIDs []string `json:"session_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	answers, err := h.resultService.BatchGetAnswers(c.Request.Context(), operatorID, c.Param("id"), req.SessionIDs)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, answers)
}

// GetProjectReport POST /api/runs/:runId/report
func (h *ResultHandler) GetProjectReport(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	req.RunID = c.Param("runId")
	report, err := h.resultService.GetProjectReport(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, report)
}

// GetLatestAIReport GET /api/projects/:id/results/report/latest
func (h *ResultHandler) GetLatestAIReport(c *gin.Context) {
	operatorID := getOperatorID(c)
	result, err := h.resultService.GetLatestAIReport(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		// 没有记录时返回 null，前端显示空态
		response.OK(c, nil)
		return
	}
	response.OK(c, result)
}

// GenerateHTMLReport POST /api/runs/:runId/report/html
func (h *ResultHandler) GenerateHTMLReport(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.GenerateHTMLReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	req.RunID = c.Param("runId")
	result, err := h.resultService.GenerateHTMLReport(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GetLatestHTMLReport GET /api/projects/:id/results/report/html/latest
func (h *ResultHandler) GetLatestHTMLReport(c *gin.Context) {
	operatorID := getOperatorID(c)
	result, err := h.resultService.GetLatestHTMLReport(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, result)
}
