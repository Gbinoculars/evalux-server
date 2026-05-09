package handler

import (
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type InvitationHandler struct {
	invService *service.InvitationService
}

func NewInvitationHandler(invService *service.InvitationService) *InvitationHandler {
	return &InvitationHandler{invService: invService}
}

// Invite 发起邀请
// POST /api/projects/:id/invitations
func (h *InvitationHandler) Invite(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var req model.InviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	inv, err := h.invService.Invite(c.Request.Context(), operatorID, projectID, req)
	if err != nil {
		if err.Error() == "无权邀请成员" {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "邀请已发送", inv)
}

// ListByProject 查询项目邀请列表
// GET /api/projects/:id/invitations
func (h *InvitationHandler) ListByProject(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	list, err := h.invService.ListByProject(c.Request.Context(), operatorID, projectID)
	if err != nil {
		if err.Error() == "无权查看邀请列表" {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// ListMyPending 查询我的待处理邀请
// GET /api/invitations/pending
func (h *InvitationHandler) ListMyPending(c *gin.Context) {
	operatorID := getOperatorID(c)
	list, err := h.invService.ListMyPending(c.Request.Context(), operatorID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, list)
}

// Respond 接受/拒绝邀请
// POST /api/invitations/:id/respond
func (h *InvitationHandler) Respond(c *gin.Context) {
	operatorID := getOperatorID(c)
	invitationID := c.Param("id")
	var req model.RespondInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.invService.Respond(c.Request.Context(), operatorID, invitationID, req.Accept); err != nil {
		msg := err.Error()
		if msg == "无权操作此邀请" {
			response.Forbidden(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}
	response.OKMsg(c, "操作成功", nil)
}
