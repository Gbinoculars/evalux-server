package repo

import (
	"context"
	"evalux-server/ent"
	"evalux-server/ent/sysrole"
	"evalux-server/ent/sysuser"
	"evalux-server/ent/sysuserrole"
	"evalux-server/internal/model"
	"time"

	"github.com/google/uuid"
)

type UserRepo struct {
	client *ent.Client
}

func NewUserRepo(client *ent.Client) *UserRepo {
	return &UserRepo{client: client}
}

func (r *UserRepo) FindByAccount(ctx context.Context, account string) (*model.User, error) {
	u, err := r.client.SysUser.Query().Where(sysuser.Account(account)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return entUserToModel(u), nil
}

func (r *UserRepo) FindByID(ctx context.Context, userID string) (*model.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	u, err := r.client.SysUser.Get(ctx, uid)
	if err != nil {
		return nil, err
	}
	return entUserToModel(u), nil
}

func (r *UserRepo) Create(ctx context.Context, account, passwordHash, nickname string) (*model.User, error) {
	u, err := r.client.SysUser.Create().
		SetAccount(account).
		SetPasswordHash(passwordHash).
		SetNickname(nickname).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return entUserToModel(u), nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, userID string, nickname *string, status *string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	upd := r.client.SysUser.UpdateOneID(uid)
	if nickname != nil {
		upd.SetNickname(*nickname)
	}
	if status != nil {
		upd.SetStatus(sysuser.Status(*status))
	}
	return upd.Exec(ctx)
}

func (r *UserRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return r.client.SysUser.UpdateOneID(uid).SetPasswordHash(passwordHash).Exec(ctx)
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return r.client.SysUser.UpdateOneID(uid).SetLastLoginAt(time.Now()).Exec(ctx)
}

func (r *UserRepo) AssignRole(ctx context.Context, userID, roleCode string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	role, err := r.client.SysRole.Query().Where(sysrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return err
	}
	// OnConflict 幂等插入
	return r.client.SysUserRole.Create().
		SetUserID(uid).
		SetRoleID(role.ID).
		OnConflictColumns("user_id", "role_id").DoNothing().
		Exec(ctx)
}

func (r *UserRepo) RemoveRole(ctx context.Context, userID, roleCode string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	role, err := r.client.SysRole.Query().Where(sysrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return err
	}
	_, err = r.client.SysUserRole.Delete().
		Where(sysuserrole.UserID(uid), sysuserrole.RoleID(role.ID)).
		Exec(ctx)
	return err
}

func (r *UserRepo) GetRoles(ctx context.Context, userID string) ([]model.Role, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	urs, err := r.client.SysUserRole.Query().
		Where(sysuserrole.UserID(uid)).
		WithRole().
		All(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]model.Role, 0, len(urs))
	for _, ur := range urs {
		if ur.Edges.Role != nil {
			roles = append(roles, model.Role{
				RoleID:   ur.Edges.Role.ID.String(),
				RoleCode: ur.Edges.Role.RoleCode,
				RoleName: ur.Edges.Role.RoleName,
			})
		}
	}
	return roles, nil
}

func (r *UserRepo) CountByAccount(ctx context.Context, account string) (int64, error) {
	count, err := r.client.SysUser.Query().Where(sysuser.Account(account)).Count(ctx)
	return int64(count), err
}

func (r *UserRepo) ListUsers(ctx context.Context, query model.UserListQuery) ([]model.User, int64, error) {
	q := r.client.SysUser.Query()
	q = applyUserFilters(q, query)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize

	users, err := q.Order(ent.Desc(sysuser.FieldCreatedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return entUsersToModels(users), int64(total), nil
}

func (r *UserRepo) ListUsersByIDs(ctx context.Context, accessibleIDs []string, query model.UserListQuery) ([]model.User, int64, error) {
	if len(accessibleIDs) == 0 {
		return nil, 0, nil
	}
	uids := make([]uuid.UUID, 0, len(accessibleIDs))
	for _, id := range accessibleIDs {
		uid, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		uids = append(uids, uid)
	}

	q := r.client.SysUser.Query().Where(sysuser.IDIn(uids...))
	q = applyUserFilters(q, query)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize

	users, err := q.Order(ent.Desc(sysuser.FieldCreatedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return entUsersToModels(users), int64(total), nil
}

// ========== helpers ==========

func applyUserFilters(q *ent.SysUserQuery, query model.UserListQuery) *ent.SysUserQuery {
	if query.Keyword != "" {
		q = q.Where(
			sysuser.Or(
				sysuser.AccountContainsFold(query.Keyword),
				sysuser.NicknameContainsFold(query.Keyword),
			),
		)
	}
	if query.Status != "" {
		q = q.Where(sysuser.StatusEQ(sysuser.Status(query.Status)))
	}
	if query.RoleCode != "" {
		q = q.Where(sysuser.HasUserRolesWith(
			sysuserrole.HasRoleWith(sysrole.RoleCode(query.RoleCode)),
		))
	}
	return q
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

func entUserToModel(u *ent.SysUser) *model.User {
	m := &model.User{
		UserID:       u.ID.String(),
		Account:      u.Account,
		PasswordHash: u.PasswordHash,
		Nickname:     u.Nickname,
		Status:       string(u.Status),
		CreatedAt:    u.CreatedAt,
	}
	if u.LastLoginAt != nil {
		m.LastLoginAt = u.LastLoginAt
	}
	return m
}

func entUsersToModels(users []*ent.SysUser) []model.User {
	result := make([]model.User, 0, len(users))
	for _, u := range users {
		result = append(result, *entUserToModel(u))
	}
	return result
}
