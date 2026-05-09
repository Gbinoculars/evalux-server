package repo

import (
	"context"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxprojectinvitation"
	"evalux-server/ent/uxprojectrole"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type InvitationRepo struct {
	client *ent.Client
}

func NewInvitationRepo(client *ent.Client) *InvitationRepo {
	return &InvitationRepo{client: client}
}

// Create 创建邀请
func (r *InvitationRepo) Create(ctx context.Context, projectID, inviterID, inviteeID, roleCode string, message *string) (*model.ProjectInvitation, error) {
	pid, _ := uuid.Parse(projectID)
	irid, _ := uuid.Parse(inviterID)
	iiid, _ := uuid.Parse(inviteeID)

	role, err := r.client.UxProjectRole.Query().Where(uxprojectrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return nil, err
	}

	expiredAt := time.Now().Add(7 * 24 * time.Hour)
	builder := r.client.UxProjectInvitation.Create().
		SetProjectID(pid).
		SetInviterID(irid).
		SetInviteeID(iiid).
		SetProjectRoleID(role.ID).
		SetExpiredAt(expiredAt)
	if message != nil {
		builder.SetMessage(*message)
	}
	inv, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entInvToModel(inv, role.RoleName), nil
}

// ListByProject 查询某项目的全部邀请
func (r *InvitationRepo) ListByProject(ctx context.Context, projectID string) ([]model.ProjectInvitation, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	invs, err := r.client.UxProjectInvitation.Query().
		Where(uxprojectinvitation.ProjectID(pid)).
		WithRole().
		Order(ent.Desc(uxprojectinvitation.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ProjectInvitation, 0, len(invs))
	for _, inv := range invs {
		roleName := ""
		if inv.Edges.Role != nil {
			roleName = inv.Edges.Role.RoleName
		}
		// 查 invitee 用户信息
		m := entInvToModel(inv, roleName)
		invitee, _ := r.client.SysUser.Get(ctx, inv.InviteeID)
		if invitee != nil {
			m.InviteeAccount = invitee.Account
			m.InviteeNickname = invitee.Nickname
		}
		list = append(list, *m)
	}
	return list, nil
}

// ListPendingByInvitee 查询被邀请人待处理的所有邀请
func (r *InvitationRepo) ListPendingByInvitee(ctx context.Context, inviteeID string) ([]model.ProjectInvitation, error) {
	iiid, err := uuid.Parse(inviteeID)
	if err != nil {
		return nil, err
	}
	invs, err := r.client.UxProjectInvitation.Query().
		Where(
			uxprojectinvitation.InviteeID(iiid),
			uxprojectinvitation.StatusEQ(uxprojectinvitation.StatusPENDING),
		).
		WithRole().
		Order(ent.Desc(uxprojectinvitation.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ProjectInvitation, 0, len(invs))
	for _, inv := range invs {
		roleName := ""
		if inv.Edges.Role != nil {
			roleName = inv.Edges.Role.RoleName
		}
		list = append(list, *entInvToModel(inv, roleName))
	}
	return list, nil
}

// GetByID 查单条邀请
func (r *InvitationRepo) GetByID(ctx context.Context, invitationID string) (*model.ProjectInvitation, error) {
	iid, err := uuid.Parse(invitationID)
	if err != nil {
		return nil, err
	}
	inv, err := r.client.UxProjectInvitation.Query().
		Where(uxprojectinvitation.ID(iid)).
		WithRole().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	roleName := ""
	if inv.Edges.Role != nil {
		roleName = inv.Edges.Role.RoleName
	}
	return entInvToModel(inv, roleName), nil
}

// UpdateStatus 更新邀请状态
func (r *InvitationRepo) UpdateStatus(ctx context.Context, invitationID, status string) error {
	iid, err := uuid.Parse(invitationID)
	if err != nil {
		return err
	}
	return r.client.UxProjectInvitation.UpdateOneID(iid).
		SetStatus(uxprojectinvitation.Status(status)).
		Exec(ctx)
}

// AcceptInvitation 接受邀请：写入 project_member 并更新状态为 ACCEPTED
func (r *InvitationRepo) AcceptInvitation(ctx context.Context, invitationID string) error {
	iid, err := uuid.Parse(invitationID)
	if err != nil {
		return err
	}
	inv, err := r.client.UxProjectInvitation.Query().
		Where(uxprojectinvitation.ID(iid)).
		Only(ctx)
	if err != nil {
		return err
	}
	// 写入项目成员（幂等）
	err = r.client.UxProjectMember.Create().
		SetProjectID(inv.ProjectID).
		SetUserID(inv.InviteeID).
		SetProjectRoleID(inv.ProjectRoleID).
		OnConflictColumns("project_id", "user_id").DoNothing().
		Exec(ctx)
	if err != nil {
		return err
	}
	// 更新邀请状态
	return r.client.UxProjectInvitation.UpdateOneID(iid).
		SetStatus(uxprojectinvitation.StatusACCEPTED).
		Exec(ctx)
}

// ==================== helpers ====================

func entInvToModel(inv *ent.UxProjectInvitation, roleName string) *model.ProjectInvitation {
	m := &model.ProjectInvitation{
		InvitationID:  inv.ID.String(),
		ProjectID:     inv.ProjectID.String(),
		InviterID:     inv.InviterID.String(),
		InviteeID:     inv.InviteeID.String(),
		ProjectRoleID: inv.ProjectRoleID.String(),
		Status:        string(inv.Status),
		CreatedAt:     inv.CreatedAt,
		RoleName:      roleName,
	}
	if inv.Message != nil {
		m.Message = inv.Message
	}
	if inv.ExpiredAt != nil {
		m.ExpiredAt = inv.ExpiredAt
	}
	return m
}
