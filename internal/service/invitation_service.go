package service

import (
	"context"
	"errors"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

type InvitationService struct {
	invRepo  *repo.InvitationRepo
	permRepo *repo.UnifiedPermRepo
}

func NewInvitationService(invRepo *repo.InvitationRepo, permRepo *repo.UnifiedPermRepo) *InvitationService {
	return &InvitationService{invRepo: invRepo, permRepo: permRepo}
}

// Invite 发起邀请（需要 MANAGE_MEMBER 权限）
func (s *InvitationService) Invite(ctx context.Context, operatorID, projectID string, req model.InviteRequest) (*model.ProjectInvitation, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "MANAGE_MEMBER")
	if !ok {
		return nil, errors.New("无权邀请成员")
	}
	inv, err := s.invRepo.Create(ctx, projectID, operatorID, req.InviteeID, req.RoleCode, req.Message)
	if err != nil {
		return nil, errors.New("发送邀请失败: " + err.Error())
	}
	return inv, nil
}

// ListByProject 查询项目的全部邀请（需要 MANAGE_MEMBER 权限）
func (s *InvitationService) ListByProject(ctx context.Context, operatorID, projectID string) ([]model.ProjectInvitation, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "MANAGE_MEMBER")
	if !ok {
		return nil, errors.New("无权查看邀请列表")
	}
	return s.invRepo.ListByProject(ctx, projectID)
}

// ListMyPending 查询当前用户待处理的邀请（无需鉴权，只能看自己的）
func (s *InvitationService) ListMyPending(ctx context.Context, operatorID string) ([]model.ProjectInvitation, error) {
	return s.invRepo.ListPendingByInvitee(ctx, operatorID)
}

// Respond 接受/拒绝邀请（只有被邀请人本人可操作）
func (s *InvitationService) Respond(ctx context.Context, operatorID, invitationID string, accept bool) error {
	inv, err := s.invRepo.GetByID(ctx, invitationID)
	if err != nil {
		return errors.New("邀请不存在")
	}
	if inv.InviteeID != operatorID {
		return errors.New("无权操作此邀请")
	}
	if inv.Status != "PENDING" {
		return errors.New("邀请已处理或已过期")
	}
	if accept {
		if err := s.invRepo.AcceptInvitation(ctx, invitationID); err != nil {
			return errors.New("接受邀请失败: " + err.Error())
		}
	} else {
		if err := s.invRepo.UpdateStatus(ctx, invitationID, "REJECTED"); err != nil {
			return errors.New("拒绝邀请失败")
		}
	}
	return nil
}
