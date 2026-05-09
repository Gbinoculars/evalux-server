package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/uxorg"
	"evalux-server/ent/uxorgmember"
	"evalux-server/ent/uxorgrole"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type OrgRepo struct {
	client *ent.Client
}

func NewOrgRepo(client *ent.Client) *OrgRepo {
	return &OrgRepo{client: client}
}

// Create 创建组织
func (r *OrgRepo) Create(ctx context.Context, createdBy string, req model.CreateOrgRequest) (*model.Org, error) {
	uid, err := uuid.Parse(createdBy)
	if err != nil {
		return nil, err
	}
	builder := r.client.UxOrg.Create().
		SetOrgName(req.OrgName).
		SetOrgType(req.OrgType).
		SetCreatedBy(uid).
		SetStatus("ACTIVE")
	if req.OrgDesc != "" {
		builder.SetOrgDesc(req.OrgDesc)
	}
	if req.ParentID != nil {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, err
		}
		builder.SetParentID(pid)
	}
	o, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entOrgToModel(o), nil
}

// FindByID 按 ID 查单个组织
func (r *OrgRepo) FindByID(ctx context.Context, orgID string) (*model.Org, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}
	o, err := r.client.UxOrg.Get(ctx, oid)
	if err != nil {
		return nil, err
	}
	return entOrgToModel(o), nil
}

// Update 更新组织信息
func (r *OrgRepo) Update(ctx context.Context, orgID string, req model.UpdateOrgRequest) (*model.Org, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxOrg.UpdateOneID(oid)
	if req.OrgName != nil {
		upd.SetOrgName(*req.OrgName)
	}
	if req.OrgType != nil {
		upd.SetOrgType(*req.OrgType)
	}
	if req.OrgDesc != nil {
		upd.SetOrgDesc(*req.OrgDesc)
	}
	if req.Status != nil {
		upd.SetStatus(uxorg.Status(*req.Status))
	}
	o, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entOrgToModel(o), nil
}

// Delete 删除组织
func (r *OrgRepo) Delete(ctx context.Context, orgID string) error {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return err
	}
	return r.client.UxOrg.DeleteOneID(oid).Exec(ctx)
}

// List 分页查询顶级组织（parent_id 为空）
func (r *OrgRepo) List(ctx context.Context, query model.OrgListQuery) ([]model.Org, int64, error) {
	q := r.client.UxOrg.Query().Where(uxorg.ParentIDIsNil())
	if query.Keyword != "" {
		q = q.Where(uxorg.OrgNameContainsFold(query.Keyword))
	}
	if query.Status != "" {
		q = q.Where(uxorg.StatusEQ(uxorg.Status(query.Status)))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	orgs, err := q.Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	list := make([]model.Org, 0, len(orgs))
	for _, o := range orgs {
		list = append(list, *entOrgToModel(o))
	}
	return list, int64(total), nil
}

// ListAll 查询所有组织（不过滤层级，用于管理员获取完整树）
func (r *OrgRepo) ListAll(ctx context.Context) ([]model.Org, error) {
	orgs, err := r.client.UxOrg.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.Org, 0, len(orgs))
	for _, o := range orgs {
		list = append(list, *entOrgToModel(o))
	}
	return list, nil
}

// ListChildren 查询指定组织的直接子组织
func (r *OrgRepo) ListChildren(ctx context.Context, parentOrgID string) ([]model.Org, error) {
	oid, err := uuid.Parse(parentOrgID)
	if err != nil {
		return nil, err
	}
	orgs, err := r.client.UxOrg.Query().
		Where(uxorg.ParentID(oid), uxorg.StatusEQ(uxorg.StatusACTIVE)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.Org, 0, len(orgs))
	for _, o := range orgs {
		list = append(list, *entOrgToModel(o))
	}
	return list, nil
}

// ListByUserID 查询用户参与的所有组织（通过 ux_org_member）
func (r *OrgRepo) ListByUserID(ctx context.Context, userID string) ([]model.Org, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	memberships, err := r.client.UxOrgMember.Query().
		Where(uxorgmember.UserID(uid)).
		WithOrg().
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.Org, 0, len(memberships))
	for _, m := range memberships {
		if m.Edges.Org != nil {
			list = append(list, *entOrgToModel(m.Edges.Org))
		}
	}
	return list, nil
}

// ==================== 成员管理 ====================

// ListMembers 查询组织成员（含用户信息和角色信息）
func (r *OrgRepo) ListMembers(ctx context.Context, orgID string) ([]model.OrgMember, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}
	members, err := r.client.UxOrgMember.Query().
		Where(uxorgmember.OrgID(oid)).
		WithUser().
		WithRole().
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.OrgMember, 0, len(members))
	for _, m := range members {
		om := model.OrgMember{
			OrgMemberID: m.ID.String(),
			OrgID:       m.OrgID.String(),
			UserID:      m.UserID.String(),
			OrgRoleID:   m.OrgRoleID.String(),
			CreatedAt:   m.CreatedAt,
		}
		if m.Edges.User != nil {
			om.UserAccount = m.Edges.User.Account
			om.UserNickname = m.Edges.User.Nickname
		}
		if m.Edges.Role != nil {
			om.RoleCode = m.Edges.Role.RoleCode
			om.RoleName = m.Edges.Role.RoleName
		}
		list = append(list, om)
	}
	return list, nil
}

// AddMember 添加组织成员
func (r *OrgRepo) AddMember(ctx context.Context, orgID, userID, roleCode string) (*model.OrgMember, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	role, err := r.client.UxOrgRole.Query().Where(uxorgrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return nil, err
	}
	err = r.client.UxOrgMember.Create().
		SetOrgID(oid).
		SetUserID(uid).
		SetOrgRoleID(role.ID).
		OnConflictColumns("org_id", "user_id").DoNothing().
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	// 重新查询以获取完整记录
	m, err := r.client.UxOrgMember.Query().
		Where(uxorgmember.OrgID(oid), uxorgmember.UserID(uid)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return &model.OrgMember{
		OrgMemberID: m.ID.String(),
		OrgID:       m.OrgID.String(),
		UserID:      m.UserID.String(),
		OrgRoleID:   m.OrgRoleID.String(),
		CreatedAt:   m.CreatedAt,
		RoleCode:    roleCode,
	}, nil
}

// UpdateMemberRole 修改组织成员角色
func (r *OrgRepo) UpdateMemberRole(ctx context.Context, orgID, userID, roleCode string) error {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	role, err := r.client.UxOrgRole.Query().Where(uxorgrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return err
	}
	_, err = r.client.UxOrgMember.Update().
		Where(uxorgmember.OrgID(oid), uxorgmember.UserID(uid)).
		SetOrgRoleID(role.ID).
		Save(ctx)
	return err
}

// RemoveMember 移除组织成员
func (r *OrgRepo) RemoveMember(ctx context.Context, orgID, userID string) error {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = r.client.UxOrgMember.Delete().
		Where(uxorgmember.OrgID(oid), uxorgmember.UserID(uid)).
		Exec(ctx)
	return err
}

// ListRoles 查询所有组织角色
func (r *OrgRepo) ListRoles(ctx context.Context) ([]model.OrgRole, error) {
	roles, err := r.client.UxOrgRole.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.OrgRole, 0, len(roles))
	for _, role := range roles {
		or := model.OrgRole{
			OrgRoleID:       role.ID.String(),
			RoleCode:        role.RoleCode,
			RoleName:        role.RoleName,
			PermissionCodes: role.PermissionCodes,
		}
		if role.Description != "" {
			or.Description = &role.Description
		}
		list = append(list, or)
	}
	return list, nil
}

// ==================== helpers ====================

func entOrgToModel(o *ent.UxOrg) *model.Org {
	m := &model.Org{
		OrgID:     o.ID.String(),
		OrgName:   o.OrgName,
		OrgType:   o.OrgType,
		CreatedBy: o.CreatedBy.String(),
		Status:    string(o.Status),
		CreatedAt: o.CreatedAt,
	}
	if o.ParentID != nil {
		s := o.ParentID.String()
		m.ParentID = &s
	}
	if o.OrgDesc != nil {
		m.OrgDesc = o.OrgDesc
	}
	return m
}
