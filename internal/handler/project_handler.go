package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

// Create 创建项目
// POST /api/projects
func (h *ProjectHandler) Create(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	project, err := h.projectService.Create(c.Request.Context(), operatorID, req)
	if err != nil {
		msg := err.Error()
		if msg == "无权创建项目" || msg == "无权在该组织下创建项目" {
			response.Forbidden(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}
	response.OKMsg(c, "创建成功", project)
}

// List 项目列表
// GET /api/projects
func (h *ProjectHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	var query model.ProjectListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.projectService.List(c.Request.Context(), operatorID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// GetByID 项目详情
// GET /api/projects/:id
func (h *ProjectHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	project, err := h.projectService.GetByID(c.Request.Context(), operatorID, projectID)
	if err != nil {
		if err.Error() == "无权查看该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, project)
}

// Update 更新项目
// PUT /api/projects/:id
func (h *ProjectHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	var req model.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	project, err := h.projectService.Update(c.Request.Context(), operatorID, projectID, req)
	if err != nil {
		if err.Error() == "无权编辑该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", project)
}

// Delete 删除项目
// DELETE /api/projects/:id
func (h *ProjectHandler) Delete(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	if projectID == "" {
		response.BadRequest(c, "项目ID不能为空")
		return
	}
	if err := h.projectService.Delete(c.Request.Context(), operatorID, projectID); err != nil {
		if err.Error() == "无权删除该项目" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// ==================== 成员管理 ====================

// ListMembers 项目成员列表
// GET /api/projects/:id/members
func (h *ProjectHandler) ListMembers(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	list, err := h.projectService.ListMembers(c.Request.Context(), operatorID, projectID)
	if err != nil {
		if err.Error() == "无权查看该项目成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// AddMember 添加项目成员
// POST /api/projects/:id/members
func (h *ProjectHandler) AddMember(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var req model.AddProjectMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.projectService.AddMember(c.Request.Context(), operatorID, projectID, req); err != nil {
		if err.Error() == "无权管理该项目成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "成员添加成功", nil)
}

// UpdateMemberRole 修改成员角色
// PUT /api/projects/:id/members/:userId
func (h *ProjectHandler) UpdateMemberRole(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	userID := c.Param("userId")
	var req model.UpdateProjectMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.projectService.UpdateMemberRole(c.Request.Context(), operatorID, projectID, userID, req); err != nil {
		if err.Error() == "无权管理该项目成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "角色更新成功", nil)
}

// RemoveMember 移除项目成员
// DELETE /api/projects/:id/members/:userId
func (h *ProjectHandler) RemoveMember(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	userID := c.Param("userId")
	if err := h.projectService.RemoveMember(c.Request.Context(), operatorID, projectID, userID); err != nil {
		if err.Error() == "无权管理该项目成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "成员移除成功", nil)
}

// ListRoles 项目角色列表
// GET /api/projects/roles
func (h *ProjectHandler) ListRoles(c *gin.Context) {
	roles, err := h.projectService.ListRoles(c.Request.Context())
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, roles)
}
