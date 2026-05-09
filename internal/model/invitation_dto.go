package model

import "time"

// ==================== 邀请模型 ====================

type ProjectInvitation struct {
	InvitationID  string     `json:"invitation_id"`
	ProjectID     string     `json:"project_id"`
	InviterID     string     `json:"inviter_id"`
	InviteeID     string     `json:"invitee_id"`
	ProjectRoleID string     `json:"project_role_id"`
	Status        string     `json:"status"` // PENDING / ACCEPTED / REJECTED / EXPIRED
	Message       *string    `json:"message"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiredAt     *time.Time `json:"expired_at"`
	// JOIN 展开
	InviteeAccount  string `json:"invitee_account,omitempty"`
	InviteeNickname string `json:"invitee_nickname,omitempty"`
	RoleName        string `json:"role_name,omitempty"`
}

// ==================== 邀请请求 ====================

type InviteRequest struct {
	InviteeID string  `json:"invitee_id" binding:"required"`
	RoleCode  string  `json:"role_code"  binding:"required,oneof=ADMIN EDITOR VIEWER"`
	Message   *string `json:"message"`
}

type RespondInvitationRequest struct {
	Accept bool `json:"accept"`
}
