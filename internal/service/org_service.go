package service

import (
	"context"
	"errors"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

type OrgService struct {
	orgRepo  *repo.OrgRepo
	permRepo *repo.UnifiedPermRepo
}

func NewOrgService(orgRepo *repo.OrgRepo, permRepo *repo.UnifiedPermRepo) *OrgService {
	return &OrgService{orgRepo: orgRepo, permRepo: permRepo}
}

// ==================== 组织 CRUD ====================

// Create 创建顶级组织或子组织
// 顶级组织：仅系统管理员（ADMIN）可创建
// 子组织：在父组织中拥有 ORG_MANAGE_CHILD 权限即可
func (s *OrgService) Create(ctx context.Context, operatorID string, req model.CreateOrgRequest) (*model.Org, error) {
	if req.ParentID == nil {
		// 创建顶级组织：必须是系统管理员
		isAdmin, _ := s.permRepo.IsSystemAdmin(ctx, operatorID)
		if !isAdmin {
			return nil, errors.New("只有系统管理员才能创建顶级组织")
		}
	} else {
		// 创建子组织：需要在父组织中拥有 ORG_MANAGE_CHILD 权限
		ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, *req.ParentID, "ORG_MANAGE_CHILD")
		if !ok {
			return nil, errors.New("无权在该组织下创建子组织")
		}
	}
	org, err := s.orgRepo.Create(ctx, operatorID, req)
	if err != nil {
		return nil, errors.New("创建组织失败: " + err.Error())
	}
	// 创建者自动成为 ORG_OWNER
	_, _ = s.orgRepo.AddMember(ctx, org.OrgID, operatorID, "ORG_OWNER")
	return org, nil
}

// GetByID 查询组织详情（需要 ORG_VIEW 或系统管理员）
func (s *OrgService) GetByID(ctx context.Context, operatorID, orgID string) (*model.Org, error) {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_VIEW")
	if !ok {
		return nil, errors.New("无权查看该组织")
	}
	return s.orgRepo.FindByID(ctx, orgID)
}

// List 查询顶级组织列表（系统管理员返回所有，普通用户返回自己参与的）
func (s *OrgService) List(ctx context.Context, operatorID string, query model.OrgListQuery) (*model.OrgListResponse, error) {
	isAdmin, _ := s.permRepo.IsSystemAdmin(ctx, operatorID)
	if isAdmin {
		list, total, err := s.orgRepo.List(ctx, query)
		if err != nil {
			return nil, errors.New("查询组织列表失败")
		}
		return &model.OrgListResponse{Total: total, List: list}, nil
	}
	// 普通用户：返回其参与的组织
	list, err := s.orgRepo.ListByUserID(ctx, operatorID)
	if err != nil {
		return nil, errors.New("查询组织列表失败")
	}
	return &model.OrgListResponse{Total: int64(len(list)), List: list}, nil
}

// ListMine 查询当前用户参与的所有组织（管理员返回全量含子组织）
func (s *OrgService) ListMine(ctx context.Context, operatorID string) ([]model.Org, error) {
	isAdmin, _ := s.permRepo.IsSystemAdmin(ctx, operatorID)
	if isAdmin {
		return s.orgRepo.ListAll(ctx)
	}
	return s.orgRepo.ListByUserID(ctx, operatorID)
}

// ListChildren 查询子组织（需要 ORG_VIEW）
func (s *OrgService) ListChildren(ctx context.Context, operatorID, orgID string) ([]model.Org, error) {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_VIEW")
	if !ok {
		return nil, errors.New("无权查看该组织")
	}
	return s.orgRepo.ListChildren(ctx, orgID)
}

// Update 更新组织信息（需要 ORG_EDIT）
func (s *OrgService) Update(ctx context.Context, operatorID, orgID string, req model.UpdateOrgRequest) (*model.Org, error) {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_EDIT")
	if !ok {
		return nil, errors.New("无权编辑该组织")
	}
	org, err := s.orgRepo.Update(ctx, orgID, req)
	if err != nil {
		return nil, errors.New("更新组织失败")
	}
	return org, nil
}

// Delete 删除组织（需要 ORG_MANAGE_CHILD 对父组织，或系统管理员）
func (s *OrgService) Delete(ctx context.Context, operatorID, orgID string) error {
	isAdmin, _ := s.permRepo.IsSystemAdmin(ctx, operatorID)
	if !isAdmin {
		// 非系统管理员必须是该组织的 ORG_OWNER
		ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_MANAGE_CHILD")
		if !ok {
			return errors.New("无权删除该组织")
		}
	}
	return s.orgRepo.Delete(ctx, orgID)
}

// ==================== 成员管理 ====================

// ListMembers 查询组织成员（需要 ORG_VIEW）
func (s *OrgService) ListMembers(ctx context.Context, operatorID, orgID string) ([]model.OrgMember, error) {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_VIEW")
	if !ok {
		return nil, errors.New("无权查看该组织成员")
	}
	return s.orgRepo.ListMembers(ctx, orgID)
}

// AddMember 添加组织成员（需要 ORG_MANAGE_MEMBER）
func (s *OrgService) AddMember(ctx context.Context, operatorID, orgID string, req model.AddOrgMemberRequest) (*model.OrgMember, error) {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_MANAGE_MEMBER")
	if !ok {
		return nil, errors.New("无权管理该组织成员")
	}
	return s.orgRepo.AddMember(ctx, orgID, req.UserID, req.RoleCode)
}

// UpdateMemberRole 修改成员角色（需要 ORG_MANAGE_MEMBER）
func (s *OrgService) UpdateMemberRole(ctx context.Context, operatorID, orgID, userID string, req model.UpdateOrgMemberRoleRequest) error {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_MANAGE_MEMBER")
	if !ok {
		return errors.New("无权管理该组织成员")
	}
	return s.orgRepo.UpdateMemberRole(ctx, orgID, userID, req.RoleCode)
}

// RemoveMember 移除成员（需要 ORG_MANAGE_MEMBER）
func (s *OrgService) RemoveMember(ctx context.Context, operatorID, orgID, userID string) error {
	ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, orgID, "ORG_MANAGE_MEMBER")
	if !ok {
		return errors.New("无权管理该组织成员")
	}
	return s.orgRepo.RemoveMember(ctx, orgID, userID)
}

// ListRoles 查询所有组织角色
func (s *OrgService) ListRoles(ctx context.Context) ([]model.OrgRole, error) {
	return s.orgRepo.ListRoles(ctx)
}
