package model

import "time"

// ==================== 组织模型 ====================

type Org struct {
	OrgID     string     `json:"org_id"`
	ParentID  *string    `json:"parent_id"`
	OrgName   string     `json:"org_name"`
	OrgType   string     `json:"org_type"`
	OrgDesc   *string    `json:"org_desc"`
	CreatedBy string     `json:"created_by"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

type OrgMember struct {
	OrgMemberID  string    `json:"org_member_id"`
	OrgID        string    `json:"org_id"`
	UserID       string    `json:"user_id"`
	OrgRoleID    string    `json:"org_role_id"`
	CreatedAt    time.Time `json:"created_at"`
	// JOIN 展开
	UserAccount  string `json:"user_account,omitempty"`
	UserNickname string `json:"user_nickname,omitempty"`
	RoleCode     string `json:"role_code,omitempty"`
	RoleName     string `json:"role_name,omitempty"`
}

type OrgRole struct {
	OrgRoleID       string   `json:"org_role_id"`
	RoleCode        string   `json:"role_code"`
	RoleName        string   `json:"role_name"`
	PermissionCodes []string `json:"permission_codes"`
	Description     *string  `json:"description"`
}

// ==================== 组织管理请求 ====================

type CreateOrgRequest struct {
	OrgName  string  `json:"org_name"  binding:"required,min=1,max=128"`
	OrgType  string  `json:"org_type"  binding:"required,oneof=LAB GROUP TEAM"`
	OrgDesc  string  `json:"org_desc"`
	ParentID *string `json:"parent_id"`
}

type UpdateOrgRequest struct {
	OrgName *string `json:"org_name" binding:"omitempty,min=1,max=128"`
	OrgType *string `json:"org_type" binding:"omitempty,oneof=LAB GROUP TEAM"`
	OrgDesc *string `json:"org_desc"`
	Status  *string `json:"status"   binding:"omitempty,oneof=ACTIVE DISABLED"`
}

type OrgListQuery struct {
	Page     int    `form:"page"      binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword"`
	Status   string `form:"status"    binding:"omitempty,oneof=ACTIVE DISABLED"`
}

// ==================== 组织管理响应 ====================

type OrgListResponse struct {
	Total int64 `json:"total"`
	List  []Org `json:"list"`
}

// ==================== 组织成员管理请求 ====================

type AddOrgMemberRequest struct {
	UserID   string `json:"user_id"   binding:"required"`
	RoleCode string `json:"role_code" binding:"required,oneof=ORG_OWNER ORG_ADMIN ORG_MEMBER"`
}

type UpdateOrgMemberRoleRequest struct {
	RoleCode string `json:"role_code" binding:"required,oneof=ORG_OWNER ORG_ADMIN ORG_MEMBER"`
}
