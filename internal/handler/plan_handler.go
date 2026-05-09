package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type PlanHandler struct {
	planService *service.PlanService
	execService *service.ExecutionService
	runRepo     *repo.ExecutionRunRepo
}

func NewPlanHandler(planService *service.PlanService, execService *service.ExecutionService, runRepo *repo.ExecutionRunRepo) *PlanHandler {
	return &PlanHandler{planService: planService, execService: execService, runRepo: runRepo}
}

// Create POST /api/plans
func (h *PlanHandler) Create(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.planService.Create(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "创建成功", detail)
}

// List GET /api/projects/:id/plans
func (h *PlanHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	list, err := h.planService.ListByProject(c.Request.Context(), operatorID, projectID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, list)
}

// GetByID GET /api/plans/:id
func (h *PlanHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	planID := c.Param("id")
	detail, err := h.planService.GetByID(c.Request.Context(), operatorID, planID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, detail)
}

// Update PUT /api/plans/:id
func (h *PlanHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	planID := c.Param("id")
	var req model.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.planService.Update(c.Request.Context(), operatorID, planID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", detail)
}

// Delete DELETE /api/plans/:id
func (h *PlanHandler) Delete(c *gin.Context) {
	operatorID := getOperatorID(c)
	planID := c.Param("id")
	if err := h.planService.Delete(c.Request.Context(), operatorID, planID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// GetABTestResult POST /api/plans/abtest-result
func (h *PlanHandler) GetABTestResult(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.ABTestStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.planService.GetABTestResult(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, result)
}

// ListExecutionRuns GET /api/projects/:id/execution-runs
func (h *PlanHandler) ListExecutionRuns(c *gin.Context) {
	projectID := c.Param("id")
	list, err := h.runRepo.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, list)
}

// GetExecutionRun GET /api/execution-runs/:id
func (h *PlanHandler) GetExecutionRun(c *gin.Context) {
	runID := c.Param("id")
	detail, err := h.runRepo.GetByID(c.Request.Context(), runID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, detail)
}

// StartRun POST /api/plans/:id/start
// 用户点"启动评估"，按 plan 在事务内冻结 run 快照、创建 batch、展开 sessions。
func (h *PlanHandler) StartRun(c *gin.Context) {
	operatorID := getOperatorID(c)
	planID := c.Param("id")
	detail, err := h.execService.StartRun(c.Request.Context(), operatorID, planID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "评估已启动", detail)
}

// AbortRun POST /api/execution-runs/:id/abort
func (h *PlanHandler) AbortRun(c *gin.Context) {
	operatorID := getOperatorID(c)
	runID := c.Param("id")
	if err := h.execService.AbortRun(c.Request.Context(), operatorID, runID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "已中止", nil)
}
