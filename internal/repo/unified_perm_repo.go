package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/sysuserrole"
	"evalux-server/ent/uxorg"
	"evalux-server/ent/uxorgmember"
	"evalux-server/ent/uxproject"
	"evalux-server/ent/uxprojectmember"
	"evalux-server/ent/uxprojectrole"

	"github.com/google/uuid"
)

// UnifiedPermRepo 统一权限仓库，所有鉴权均基于 permission_codes 数据驱动
type UnifiedPermRepo struct {
	client *ent.Client
}

func NewUnifiedPermRepo(client *ent.Client) *UnifiedPermRepo {
	return &UnifiedPermRepo{client: client}
}

// hasSystemPermission 检查用户的系统角色 permission_codes 是否包含指定权限码（支持 * 通配）
func (r *UnifiedPermRepo) hasSystemPermission(ctx context.Context, userID string, requiredCode string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	roles, err := r.client.SysUserRole.Query().
		Where(sysuserrole.UserID(uid)).
		WithRole().
		All(ctx)
	if err != nil {
		return false, err
	}
	for _, ur := range roles {
		if ur.Edges.Role == nil {
			continue
		}
		for _, code := range ur.Edges.Role.PermissionCodes {
			if code == "*" || code == requiredCode {
				return true, nil
			}
		}
	}
	return false, nil
}

// IsSystemAdmin 检查用户是否为系统管理员（permission_codes 含 *）
func (r *UnifiedPermRepo) IsSystemAdmin(ctx context.Context, userID string) (bool, error) {
	return r.hasSystemPermission(ctx, userID, "*")
}

// CheckUserPerm 检查用户是否拥有指定的系统级用户管理权限码
func (r *UnifiedPermRepo) CheckUserPerm(ctx context.Context, userID, code string) (bool, error) {
	return r.hasSystemPermission(ctx, userID, code)
}

// CheckOrgAccess 组织权限两步检查
func (r *UnifiedPermRepo) CheckOrgAccess(ctx context.Context, userID, orgID, orgCode string) (bool, error) {
	isAdmin, err := r.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return false, err
	}
	membership, err := r.client.UxOrgMember.Query().
		Where(uxorgmember.OrgID(oid), uxorgmember.UserID(uid)).
		WithRole().
		Only(ctx)
	if err != nil {
		return false, nil
	}
	if membership.Edges.Role == nil {
		return false, nil
	}
	for _, code := range membership.Edges.Role.PermissionCodes {
		if code == orgCode {
			return true, nil
		}
	}
	return false, nil
}

// CheckProjectAccess 统一三层项目权限检查
func (r *UnifiedPermRepo) CheckProjectAccess(ctx context.Context, userID, projectID, permissionCode string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return false, err
	}

	sysCode := "PROJECT:" + permissionCode
	hasSysPerm, err := r.hasSystemPermission(ctx, userID, sysCode)
	if err != nil {
		return false, err
	}
	if hasSysPerm {
		return true, nil
	}

	member, err := r.client.UxProjectMember.Query().
		Where(uxprojectmember.UserID(uid), uxprojectmember.ProjectID(pid)).
		WithRole().
		Only(ctx)
	if err == nil && member != nil && member.Edges.Role != nil {
		for _, code := range member.Edges.Role.PermissionCodes {
			if code == permissionCode {
				return true, nil
			}
		}
		return false, nil
	}

	project, err := r.client.UxProject.Get(ctx, pid)
	if err != nil {
		return false, err
	}
	if project.OrgID == nil {
		return false, nil
	}

	ancestorIDs, err := r.getAncestorOrgIDs(ctx, *project.OrgID)
	if err != nil {
		return false, err
	}

	for _, orgID := range ancestorIDs {
		membership, err := r.client.UxOrgMember.Query().
			Where(uxorgmember.OrgID(orgID), uxorgmember.UserID(uid)).
			WithRole().
			Only(ctx)
		if err != nil {
			continue
		}
		if membership != nil && membership.Edges.Role != nil {
			for _, code := range membership.Edges.Role.PermissionCodes {
				if code == "ORG_MANAGE_PROJECT" {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// ListAccessibleProjectIDs 查出操作者可访问的所有项目ID
func (r *UnifiedPermRepo) ListAccessibleProjectIDs(ctx context.Context, userID, permissionCode string) ([]string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	hasSysListPerm, err := r.hasSystemPermission(ctx, userID, "PROJECT:LIST")
	if err != nil {
		return nil, err
	}
	if hasSysListPerm {
		projects, err := r.client.UxProject.Query().
			Where(uxproject.StatusNEQ("ARCHIVED")).
			All(ctx)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(projects))
		for _, p := range projects {
			ids = append(ids, p.ID.String())
		}
		return ids, nil
	}

	idSet := make(map[string]bool)

	memberships, err := r.client.UxProjectMember.Query().
		Where(uxprojectmember.UserID(uid)).
		WithRole().
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range memberships {
		if m.Edges.Role != nil {
			for _, code := range m.Edges.Role.PermissionCodes {
				if code == permissionCode {
					idSet[m.ProjectID.String()] = true
					break
				}
			}
		}
	}

	orgMemberships, err := r.client.UxOrgMember.Query().
		Where(uxorgmember.UserID(uid)).
		WithRole().
		All(ctx)
	if err != nil {
		return nil, err
	}

	var manageOrgIDs []uuid.UUID
	for _, om := range orgMemberships {
		if om.Edges.Role != nil {
			for _, code := range om.Edges.Role.PermissionCodes {
				if code == "ORG_MANAGE_PROJECT" {
					manageOrgIDs = append(manageOrgIDs, om.OrgID)
					break
				}
			}
		}
	}

	for _, orgID := range manageOrgIDs {
		descendantIDs, err := r.getDescendantOrgIDs(ctx, orgID)
		if err != nil {
			continue
		}
		allOrgIDs := append(descendantIDs, orgID)
		projects, err := r.client.UxProject.Query().
			Where(uxproject.OrgIDIn(allOrgIDs...)).
			All(ctx)
		if err != nil {
			continue
		}
		for _, p := range projects {
			idSet[p.ID.String()] = true
		}
	}

	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids, nil
}

// AddProjectMember 添加项目成员
func (r *UnifiedPermRepo) AddProjectMember(ctx context.Context, projectID, userID, roleCode string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	role, err := r.client.UxProjectRole.Query().
		Where(uxprojectrole.RoleCode(roleCode)).
		Only(ctx)
	if err != nil {
		return err
	}
	return r.client.UxProjectMember.Create().
		SetProjectID(pid).
		SetUserID(uid).
		SetProjectRoleID(role.ID).
		OnConflictColumns("project_id", "user_id").DoNothing().
		Exec(ctx)
}

// GetUserRoleCodes 获取用户的所有系统角色编码
func (r *UnifiedPermRepo) GetUserRoleCodes(ctx context.Context, userID string) ([]string, error) {
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
	codes := make([]string, 0, len(urs))
	for _, ur := range urs {
		if ur.Edges.Role != nil {
			codes = append(codes, ur.Edges.Role.RoleCode)
		}
	}
	return codes, nil
}

// getAncestorOrgIDs 获取指定组织的所有祖先组织ID（含自身）
func (r *UnifiedPermRepo) getAncestorOrgIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	currentID := orgID
	for {
		ids = append(ids, currentID)
		org, err := r.client.UxOrg.Get(ctx, currentID)
		if err != nil {
			break
		}
		if org.ParentID == nil {
			break
		}
		currentID = *org.ParentID
	}
	return ids, nil
}

// getDescendantOrgIDs 获取指定组织的所有后代组织ID（不含自身）
func (r *UnifiedPermRepo) getDescendantOrgIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	var result []uuid.UUID
	children, err := r.client.UxOrg.Query().
		Where(
			uxorg.ParentID(orgID),
			uxorg.StatusEQ(uxorg.StatusACTIVE),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		result = append(result, child.ID)
		descendants, err := r.getDescendantOrgIDs(ctx, child.ID)
		if err != nil {
			continue
		}
		result = append(result, descendants...)
	}
	return result, nil
}
