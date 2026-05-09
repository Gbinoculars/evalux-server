package service

import (
	"context"
	"errors"
	"evalux-server/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// UserRepository 定义 service 层需要的用户数据访问能力（接口）
type UserRepository interface {
	FindByAccount(ctx context.Context, account string) (*model.User, error)
	FindByID(ctx context.Context, userID string) (*model.User, error)
	Create(ctx context.Context, account, passwordHash, nickname string) (*model.User, error)
	AssignRole(ctx context.Context, userID, roleCode string) error
	GetRoles(ctx context.Context, userID string) ([]model.Role, error)
	UpdateLastLogin(ctx context.Context, userID string) error
}

type AuthService struct {
	userRepo UserRepository
}

func NewAuthService(userRepo UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) Register(ctx context.Context, account, password, nickname string) (*model.UserWithRoles, error) {
	if account == "admin" {
		return nil, errors.New("账号 admin 为系统保留账号，不允许注册")
	}
	existing, _ := s.userRepo.FindByAccount(ctx, account)
	if existing != nil {
		return nil, errors.New("账号已存在")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码处理失败")
	}

	user, err := s.userRepo.Create(ctx, account, string(hash), nickname)
	if err != nil {
		return nil, errors.New("注册失败")
	}

	// 默认分配普通成员角色
	_ = s.userRepo.AssignRole(ctx, user.UserID, "MEMBER")

	roles, _ := s.userRepo.GetRoles(ctx, user.UserID)
	return &model.UserWithRoles{User: *user, Roles: roles}, nil
}

func (s *AuthService) Login(ctx context.Context, account, password string) (*model.UserWithRoles, error) {
	user, err := s.userRepo.FindByAccount(ctx, account)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}

	if user.Status != "ACTIVE" {
		return nil, errors.New("账号已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("账号或密码错误")
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.UserID)

	roles, _ := s.userRepo.GetRoles(ctx, user.UserID)
	return &model.UserWithRoles{User: *user, Roles: roles}, nil
}

func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*model.UserWithRoles, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.New("用户不存在")
	}
	roles, _ := s.userRepo.GetRoles(ctx, user.UserID)
	return &model.UserWithRoles{User: *user, Roles: roles}, nil
}
