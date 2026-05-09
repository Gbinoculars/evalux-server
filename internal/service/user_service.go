package service

import (
	"context"
	"errors"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"

	"golang.org/x/crypto/bcrypt"
)

// UserManageRepository 用户管理所需的数据访问能力
type UserManageRepository interface {
	FindByID(ctx context.Context, userID string) (*model.User, error)
	FindByAccount(ctx context.Context, account string) (*model.User, error)
	Create(ctx context.Context, account, passwordHash, nickname string) (*model.User, error)
	UpdateUser(ctx context.Context, userID string, nickname *string, status *string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	AssignRole(ctx context.Context, userID, roleCode string) error
	RemoveRole(ctx context.Context, userID, roleCode string) error
	GetRoles(ctx context.Context, userID string) ([]model.Role, error)
	ListUsers(ctx context.Context, query model.UserListQuery) ([]model.User, int64, error)
	ListUsersByIDs(ctx context.Context, accessibleIDs []string, query model.UserListQuery) ([]model.User, int64, error)
	CountByAccount(ctx context.Context, account string) (int64, error)
}

type UserService struct {
	userRepo UserManageRepository
	permRepo *repo.UnifiedPermRepo
}

func NewUserService(userRepo UserManageRepository, permRepo *repo.UnifiedPermRepo) *UserService {
	return &UserService{userRepo: userRepo, permRepo: permRepo}
}

// List 分页查询用户列表
// 权限：USER:LIST（ADMIN/*通配 或 USER_ADMIN）
func (s *UserService) List(ctx context.Context, operatorID string, query model.UserListQuery) (*model.UserListResponse, error) {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:LIST")
	if !ok {
		return nil, errors.New("无权查看用户列表")
	}
	users, total, err := s.userRepo.ListUsers(ctx, query)
	if err != nil {
		return nil, errors.New("查询用户列表失败")
	}
	list := make([]model.UserDetail, 0, len(users))
	for _, u := range users {
		roles, _ := s.userRepo.GetRoles(ctx, u.UserID)
		if roles == nil {
			roles = []model.Role{}
		}
		list = append(list, model.UserDetail{
			UserWithRoles: model.UserWithRoles{User: u, Roles: roles},
		})
	}
	return &model.UserListResponse{Total: total, List: list}, nil
}

// GetByID 查询单个用户详情
// 权限：USER:LIST（管理员/用户管理员）或本人
func (s *UserService) GetByID(ctx context.Context, operatorID, targetID string) (*model.UserDetail, error) {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:LIST")
	if !ok && operatorID != targetID {
		return nil, errors.New("无权查看该用户信息")
	}
	user, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	roles, _ := s.userRepo.GetRoles(ctx, user.UserID)
	if roles == nil {
		roles = []model.Role{}
	}
	return &model.UserDetail{
		UserWithRoles: model.UserWithRoles{User: *user, Roles: roles},
	}, nil
}

// CreateUser 创建用户
// 权限：USER:CREATE
func (s *UserService) CreateUser(ctx context.Context, operatorID string, req model.CreateUserRequest) (*model.UserDetail, error) {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:CREATE")
	if !ok {
		return nil, errors.New("无权创建用户")
	}
	if req.Account == "admin" {
		return nil, errors.New("账号 admin 为系统保留账号，不允许创建")
	}
	count, _ := s.userRepo.CountByAccount(ctx, req.Account)
	if count > 0 {
		return nil, errors.New("账号已存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码处理失败")
	}
	user, err := s.userRepo.Create(ctx, req.Account, string(hash), req.Nickname)
	if err != nil {
		return nil, errors.New("创建用户失败")
	}
	_ = s.userRepo.AssignRole(ctx, user.UserID, req.RoleCode)
	if req.RoleCode != "MEMBER" {
		_ = s.userRepo.AssignRole(ctx, user.UserID, "MEMBER")
	}
	roles, _ := s.userRepo.GetRoles(ctx, user.UserID)
	if roles == nil {
		roles = []model.Role{}
	}
	return &model.UserDetail{
		UserWithRoles: model.UserWithRoles{User: *user, Roles: roles},
	}, nil
}

// UpdateUser 编辑用户（昵称、状态）
// 权限：USER:EDIT 或本人（本人只能改昵称，不能改状态）
func (s *UserService) UpdateUser(ctx context.Context, operatorID, targetID string, req model.UpdateUserRequest) (*model.UserDetail, error) {
	canEditAll, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:EDIT")
	isSelf := operatorID == targetID
	if !canEditAll && !isSelf {
		return nil, errors.New("无权编辑该用户")
	}
	if req.Status != nil && !canEditAll {
		return nil, errors.New("无权修改用户状态")
	}
	_, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	if err := s.userRepo.UpdateUser(ctx, targetID, req.Nickname, req.Status); err != nil {
		return nil, errors.New("更新用户失败")
	}
	return s.GetByID(ctx, operatorID, targetID)
}

// SetStatus 启用/禁用用户
// 权限：USER:EDIT，且不能操作自己
func (s *UserService) SetStatus(ctx context.Context, operatorID, targetID, status string) error {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:EDIT")
	if !ok {
		return errors.New("无权修改用户状态")
	}
	if operatorID == targetID {
		return errors.New("不能修改自己的账号状态")
	}
	_, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return errors.New("用户不存在")
	}
	return s.userRepo.UpdateUser(ctx, targetID, nil, &status)
}

// ResetPassword 重置密码
// 权限：USER:EDIT 或本人
func (s *UserService) ResetPassword(ctx context.Context, operatorID, targetID, newPassword string) error {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:EDIT")
	if !ok && operatorID != targetID {
		return errors.New("无权重置该用户密码")
	}
	_, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return errors.New("用户不存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码处理失败")
	}
	return s.userRepo.UpdatePassword(ctx, targetID, string(hash))
}

// AssignRole 绑定角色
// 权限：USER:EDIT
func (s *UserService) AssignRole(ctx context.Context, operatorID, targetID, roleCode string) error {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:EDIT")
	if !ok {
		return errors.New("无权管理用户角色")
	}
	_, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return errors.New("用户不存在")
	}
	return s.userRepo.AssignRole(ctx, targetID, roleCode)
}

// RemoveRole 解绑角色
// 权限：USER:EDIT
func (s *UserService) RemoveRole(ctx context.Context, operatorID, targetID, roleCode string) error {
	ok, _ := s.permRepo.CheckUserPerm(ctx, operatorID, "USER:EDIT")
	if !ok {
		return errors.New("无权管理用户角色")
	}
	_, err := s.userRepo.FindByID(ctx, targetID)
	if err != nil {
		return errors.New("用户不存在")
	}
	return s.userRepo.RemoveRole(ctx, targetID, roleCode)
}
