package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type OrgHandler struct {
	orgService *service.OrgService
}

func NewOrgHandler(orgService *service.OrgService) *OrgHandler {
	return &OrgHandler{orgService: orgService}
}

// List 顶级组织列表 / 管理员全量
// GET /api/orgs
func (h *OrgHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)
	var query model.OrgListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.orgService.List(c.Request.Context(), operatorID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// ListMine 当前用户参与的所有组织
// GET /api/orgs/mine
func (h *OrgHandler) ListMine(c *gin.Context) {
	operatorID := getOperatorID(c)
	list, err := h.orgService.ListMine(c.Request.Context(), operatorID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// GetByID 组织详情
// GET /api/orgs/:id
func (h *OrgHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	if orgID == "" {
		response.BadRequest(c, "组织ID不能为空")
		return
	}
	org, err := h.orgService.GetByID(c.Request.Context(), operatorID, orgID)
	if err != nil {
		if err.Error() == "无权查看该组织" {
			response.Forbidden(c, err.Error())
			return
		}
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, org)
}

// ListChildren 子组织列表
// GET /api/orgs/:id/children
func (h *OrgHandler) ListChildren(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	list, err := h.orgService.ListChildren(c.Request.Context(), operatorID, orgID)
	if err != nil {
		if err.Error() == "无权查看该组织" {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// Create 创建组织
// POST /api/orgs
func (h *OrgHandler) Create(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	org, err := h.orgService.Create(c.Request.Context(), operatorID, req)
	if err != nil {
		msg := err.Error()
		if msg == "无权在该组织下创建子组织" || msg == "只有系统管理员才能创建顶级组织" {
			response.Forbidden(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}
	response.OKMsg(c, "创建成功", org)
}

// Update 更新组织
// PUT /api/orgs/:id
func (h *OrgHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	if orgID == "" {
		response.BadRequest(c, "组织ID不能为空")
		return
	}
	var req model.UpdateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	org, err := h.orgService.Update(c.Request.Context(), operatorID, orgID, req)
	if err != nil {
		if err.Error() == "无权编辑该组织" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "更新成功", org)
}

// Delete 删除组织
// DELETE /api/orgs/:id
func (h *OrgHandler) Delete(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	if orgID == "" {
		response.BadRequest(c, "组织ID不能为空")
		return
	}
	if err := h.orgService.Delete(c.Request.Context(), operatorID, orgID); err != nil {
		if err.Error() == "无权删除该组织" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "删除成功", nil)
}

// ==================== 成员管理 ====================

// ListMembers 成员列表
// GET /api/orgs/:id/members
func (h *OrgHandler) ListMembers(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	list, err := h.orgService.ListMembers(c.Request.Context(), operatorID, orgID)
	if err != nil {
		if err.Error() == "无权查看该组织成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// AddMember 添加成员
// POST /api/orgs/:id/members
func (h *OrgHandler) AddMember(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	var req model.AddOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	member, err := h.orgService.AddMember(c.Request.Context(), operatorID, orgID, req)
	if err != nil {
		if err.Error() == "无权管理该组织成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "成员添加成功", member)
}

// UpdateMemberRole 修改成员角色
// PUT /api/orgs/:id/members/:userId
func (h *OrgHandler) UpdateMemberRole(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	userID := c.Param("userId")
	var req model.UpdateOrgMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.orgService.UpdateMemberRole(c.Request.Context(), operatorID, orgID, userID, req); err != nil {
		if err.Error() == "无权管理该组织成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "角色更新成功", nil)
}

// RemoveMember 移除成员
// DELETE /api/orgs/:id/members/:userId
func (h *OrgHandler) RemoveMember(c *gin.Context) {
	operatorID := getOperatorID(c)
	orgID := c.Param("id")
	userID := c.Param("userId")
	if err := h.orgService.RemoveMember(c.Request.Context(), operatorID, orgID, userID); err != nil {
		if err.Error() == "无权管理该组织成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "成员移除成功", nil)
}

// ListRoles 组织角色列表
// GET /api/orgs/roles
func (h *OrgHandler) ListRoles(c *gin.Context) {
	roles, err := h.orgService.ListRoles(c.Request.Context())
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, roles)
}
