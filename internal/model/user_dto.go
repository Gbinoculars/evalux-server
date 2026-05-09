package model

// ==================== 用户管理请求 ====================

// CreateUserRequest 管理员创建用户
type CreateUserRequest struct {
	Account  string `json:"account"  binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname" binding:"required,min=1,max=64"`
	RoleCode string `json:"role_code" binding:"required,oneof=ADMIN USER_ADMIN PROJECT_ADMIN MEMBER"`
}

// UpdateUserRequest 编辑用户信息
type UpdateUserRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,min=1,max=64"`
	Status   *string `json:"status"   binding:"omitempty,oneof=ACTIVE DISABLED"`
}

// SetUserStatusRequest 启用/禁用用户
type SetUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=ACTIVE DISABLED"`
}

// ResetPasswordRequest 重置密码
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

// AssignRoleRequest 角色绑定
type AssignRoleRequest struct {
	RoleCode string `json:"role_code" binding:"required,oneof=ADMIN USER_ADMIN PROJECT_ADMIN MEMBER"`
}

// RemoveRoleRequest 角色解绑
type RemoveRoleRequest struct {
	RoleCode string `json:"role_code" binding:"required,oneof=ADMIN USER_ADMIN PROJECT_ADMIN MEMBER"`
}

// UserListQuery 用户列表查询参数
type UserListQuery struct {
	Page     int    `form:"page"     binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   string `form:"status"   binding:"omitempty,oneof=ACTIVE DISABLED"`
	RoleCode string `form:"role_code" binding:"omitempty,oneof=ADMIN USER_ADMIN PROJECT_ADMIN MEMBER"`
}

// ==================== 用户管理响应 ====================

// UserDetail 用户详情（含角色）
type UserDetail struct {
	UserWithRoles
}

// UserListResponse 用户列表分页响应
type UserListResponse struct {
	Total int64        `json:"total"`
	List  []UserDetail `json:"list"`
}
