package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// getOperatorID 从上下文获取当前登录用户ID
func getOperatorID(c *gin.Context) string {
	uid, _ := c.Get("user_id")
	if uid == nil {
		return ""
	}
	return uid.(string)
}

// List 用户列表（分页、搜索、筛选）
// GET /api/users?page=1&page_size=20&keyword=xxx&status=ACTIVE&role_code=RESEARCHER
func (h *UserHandler) List(c *gin.Context) {
	operatorID := getOperatorID(c)

	var query model.UserListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	result, err := h.userService.List(c.Request.Context(), operatorID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	response.OK(c, result)
}

// GetByID 用户详情
// GET /api/users/:id
func (h *UserHandler) GetByID(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	detail, err := h.userService.GetByID(c.Request.Context(), operatorID, targetID)
	if err != nil {
		if err.Error() == "无权查看该用户信息" {
			response.Forbidden(c, err.Error())
			return
		}
		response.NotFound(c, err.Error())
		return
	}

	response.OK(c, detail)
}

// Create 创建用户
// POST /api/users
func (h *UserHandler) Create(c *gin.Context) {
	operatorID := getOperatorID(c)

	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	detail, err := h.userService.CreateUser(c.Request.Context(), operatorID, req)
	if err != nil {
		if err.Error() == "无权创建用户" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OKMsg(c, "创建成功", detail)
}

// Update 编辑用户
// PUT /api/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	detail, err := h.userService.UpdateUser(c.Request.Context(), operatorID, targetID, req)
	if err != nil {
		msg := err.Error()
		if msg == "无权编辑该用户" || msg == "无权修改用户状态" {
			response.Forbidden(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}

	response.OKMsg(c, "更新成功", detail)
}

// SetStatus 启用/禁用用户
// PUT /api/users/:id/status
func (h *UserHandler) SetStatus(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req model.SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.SetStatus(c.Request.Context(), operatorID, targetID, req.Status); err != nil {
		msg := err.Error()
		if msg == "无权修改用户状态" || msg == "不能修改自己的账号状态" {
			response.Forbidden(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}

	response.OKMsg(c, "状态更新成功", nil)
}

// ResetPassword 重置密码
// PUT /api/users/:id/password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.ResetPassword(c.Request.Context(), operatorID, targetID, req.NewPassword); err != nil {
		if err.Error() == "无权重置该用户密码" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OKMsg(c, "密码重置成功", nil)
}

// AssignRole 绑定角色
// POST /api/users/:id/roles
func (h *UserHandler) AssignRole(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req model.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.AssignRole(c.Request.Context(), operatorID, targetID, req.RoleCode); err != nil {
		if err.Error() == "无权管理用户角色" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OKMsg(c, "角色绑定成功", nil)
}

// RemoveRole 解绑角色
// DELETE /api/users/:id/roles
func (h *UserHandler) RemoveRole(c *gin.Context) {
	operatorID := getOperatorID(c)
	targetID := c.Param("id")
	if targetID == "" {
		response.BadRequest(c, "用户ID不能为空")
		return
	}

	var req model.RemoveRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	if err := h.userService.RemoveRole(c.Request.Context(), operatorID, targetID, req.RoleCode); err != nil {
		if err.Error() == "无权管理用户角色" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.OKMsg(c, "角色解绑成功", nil)
}
