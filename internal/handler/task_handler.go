package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskService *service.TaskService
}

func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{taskService: taskService}
}

// Create POST /api/tasks
func (h *TaskHandler) Create(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.taskService.Create(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "创建成功", detail)
}

// List GET /api/projects/:id/tasks
func (h *TaskHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var query model.TaskListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.taskService.List(c.Request.Context(), operatorID, projectID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GetByID GET /api/tasks/:id
func (h *TaskHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	detail, err := h.taskService.GetByID(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, detail)
}

// Update PUT /api/tasks/:id
func (h *TaskHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	detail, err := h.taskService.Update(c.Request.Context(), operatorID, c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", detail)
}

// Delete DELETE /api/tasks/:id
func (h *TaskHandler) Delete(c *gin.Context) {
	operatorID := getOperatorID(c)
	if err := h.taskService.Delete(c.Request.Context(), operatorID, c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}
