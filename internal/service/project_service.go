package service

import (
	"context"
	"errors"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

type ProjectService struct {
	projectRepo *repo.ProjectRepo
	permRepo    *repo.UnifiedPermRepo
}

func NewProjectService(projectRepo *repo.ProjectRepo, permRepo *repo.UnifiedPermRepo) *ProjectService {
	return &ProjectService{projectRepo: projectRepo, permRepo: permRepo}
}

// Create 创建项目（已登录用户均可，自动成为 OWNER）
func (s *ProjectService) Create(ctx context.Context, operatorID string, req model.CreateProjectRequest) (*model.ProjectDetail, error) {
	// 若指定了 org_id，需要在该组织中拥有 ORG_CREATE_PROJECT 权限
	if req.OrgID != nil && *req.OrgID != "" {
		ok, _ := s.permRepo.CheckOrgAccess(ctx, operatorID, *req.OrgID, "ORG_CREATE_PROJECT")
		if !ok {
			return nil, errors.New("无权在该组织下创建项目")
		}
	}
	project, err := s.projectRepo.Create(ctx, operatorID, req)
	if err != nil {
		return nil, errors.New("创建项目失败")
	}
	_ = s.permRepo.AddProjectMember(ctx, project.ProjectID, operatorID, "OWNER")
	return project, nil
}

// GetByID 查询项目详情（需要 VIEW 权限）
func (s *ProjectService) GetByID(ctx context.Context, operatorID, projectID string) (*model.ProjectDetail, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !ok {
		return nil, errors.New("无权查看该项目")
	}
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}
	return project, nil
}

// List 分页查询项目列表（返回操作者有 VIEW 权限的项目）
func (s *ProjectService) List(ctx context.Context, operatorID string, query model.ProjectListQuery) (*model.ProjectListResponse, error) {
	accessibleIDs, err := s.permRepo.ListAccessibleProjectIDs(ctx, operatorID, "VIEW")
	if err != nil {
		return nil, errors.New("查询权限范围失败")
	}
	if len(accessibleIDs) == 0 {
		return &model.ProjectListResponse{Total: 0, List: []model.ProjectDetail{}}, nil
	}
	projects, total, err := s.projectRepo.ListByIDs(ctx, accessibleIDs, query)
	if err != nil {
		return nil, errors.New("查询项目列表失败")
	}
	return &model.ProjectListResponse{Total: total, List: projects}, nil
}

// Update 更新项目（需要 EDIT 权限）
func (s *ProjectService) Update(ctx context.Context, operatorID, projectID string, req model.UpdateProjectRequest) (*model.ProjectDetail, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "EDIT")
	if !ok {
		return nil, errors.New("无权编辑该项目")
	}
	_, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, errors.New("项目不存在")
	}
	return s.projectRepo.Update(ctx, projectID, req)
}

// Delete 删除项目（需要 DELETE 权限）
func (s *ProjectService) Delete(ctx context.Context, operatorID, projectID string) error {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "DELETE")
	if !ok {
		return errors.New("无权删除该项目")
	}
	_, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return errors.New("项目不存在")
	}
	return s.projectRepo.Delete(ctx, projectID)
}

// ==================== 成员管理 ====================

// ListMembers 查询项目成员（需要 VIEW 权限）
func (s *ProjectService) ListMembers(ctx context.Context, operatorID, projectID string) ([]model.ProjectMember, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !ok {
		return nil, errors.New("无权查看该项目成员")
	}
	return s.projectRepo.ListMembers(ctx, projectID)
}

// AddMember 添加项目成员（需要 MANAGE_MEMBER 权限）
func (s *ProjectService) AddMember(ctx context.Context, operatorID, projectID string, req model.AddProjectMemberRequest) error {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "MANAGE_MEMBER")
	if !ok {
		return errors.New("无权管理该项目成员")
	}
	return s.permRepo.AddProjectMember(ctx, projectID, req.UserID, req.RoleCode)
}

// UpdateMemberRole 修改成员角色（需要 MANAGE_MEMBER 权限）
func (s *ProjectService) UpdateMemberRole(ctx context.Context, operatorID, projectID, userID string, req model.UpdateProjectMemberRoleRequest) error {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "MANAGE_MEMBER")
	if !ok {
		return errors.New("无权管理该项目成员")
	}
	return s.projectRepo.UpdateMemberRole(ctx, projectID, userID, req.RoleCode)
}

// RemoveMember 移除成员（需要 MANAGE_MEMBER 权限）
func (s *ProjectService) RemoveMember(ctx context.Context, operatorID, projectID, userID string) error {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "MANAGE_MEMBER")
	if !ok {
		return errors.New("无权管理该项目成员")
	}
	return s.projectRepo.RemoveMember(ctx, projectID, userID)
}

// ListRoles 查询所有项目角色
func (s *ProjectService) ListRoles(ctx context.Context) ([]model.ProjectRole, error) {
	return s.projectRepo.ListRoles(ctx)
}

