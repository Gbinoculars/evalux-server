package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"evalux-server/ent"
	"evalux-server/ent/sysrole"
	"evalux-server/ent/sysuser"
	"evalux-server/ent/sysuserrole"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// isNoRows 判断是否为 "sql: no rows in result set" 错误
// ent 的 OnConflict().DoNothing() 在 PostgreSQL 中冲突时会返回此错误，属于正常幂等行为
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// seedIgnoreConflict 封装 OnConflict DoNothing 的错误处理
func seedIgnoreConflict(err error) error {
	if err == nil || isNoRows(err) || ent.IsConstraintError(err) {
		return nil
	}
	return err
}

// SeedBaseData 初始化系统角色、组织角色、项目角色、根模块等基础数据（幂等）
func SeedBaseData(client *ent.Client) error {
	ctx := context.Background()

	// ========== 1. 四种系统角色（含 PROJECT_ADMIN） ==========
	roles := []struct {
		Code  string
		Name  string
		Perms []string
		Desc  string
	}{
		{
			"ADMIN", "管理员",
			[]string{"*"},
			"超级管理员，permission_codes=[*]，匹配任意权限码",
		},
		{
			"USER_ADMIN", "用户管理员",
			[]string{"USER:LIST", "USER:CREATE", "USER:EDIT", "USER:DELETE"},
			"负责用户账号维护、角色绑定与用户状态管理",
		},
		{
			"PROJECT_ADMIN", "项目管理员",
			[]string{"PROJECT:LIST", "PROJECT:VIEW", "PROJECT:EDIT", "PROJECT:DELETE", "PROJECT:MANAGE_MEMBER"},
			"全局项目管理员，可查看/编辑/删除所有项目及管理所有项目成员",
		},
		{
			"MEMBER", "普通成员",
			[]string{"PROJECT:CREATE"},
			"基础身份，可登录系统、创建组织和创建项目",
		},
	}
	for _, r := range roles {
		err := client.SysRole.Create().
			SetRoleCode(r.Code).
			SetRoleName(r.Name).
			SetPermissionCodes(r.Perms).
			SetDescription(r.Desc).
			OnConflictColumns("role_code").
			UpdatePermissionCodes().
			UpdateRoleName().
			Exec(ctx)
		if err = seedIgnoreConflict(err); err != nil {
			return err
		}
	}

	// ========== 2. 四种基础权限（保留，供旧模块树兼容） ==========
	perms := []struct {
		Code string
		Name string
		Desc string
	}{
		{"VIEW", "查看", "允许读取模块或资源内容"},
		{"EDIT", "编辑", "允许修改模块或资源内容"},
		{"CREATE", "创建", "允许在当前模块下新增子模块或资源"},
		{"DELETE", "删除", "允许删除当前模块或资源"},
	}
	for _, p := range perms {
		err := client.SysPermission.Create().
			SetPermissionCode(p.Code).
			SetPermissionName(p.Name).
			SetDescription(p.Desc).
			OnConflictColumns("permission_code").DoNothing().
			Exec(ctx)
		if err = seedIgnoreConflict(err); err != nil {
			return err
		}
	}

	// ========== 3. 模块树（保留旧结构兼容） ==========
	_ = seedIgnoreConflict(client.SysModule.Create().
		SetModuleCode("ALL").SetModuleName("全部模块").SetModuleType("ROOT").SetStatus("ACTIVE").
		OnConflictColumns("module_code").DoNothing().Exec(ctx))

	allModule, err := client.SysModule.Query().Where().First(ctx)
	if err == nil && allModule != nil {
		_ = seedIgnoreConflict(client.SysModule.Create().
			SetModuleCode("USER_MANAGEMENT").SetModuleName("用户管理模块").SetModuleType("CATEGORY").
			SetStatus("ACTIVE").SetParentModuleID(allModule.ID).
			OnConflictColumns("module_code").DoNothing().Exec(ctx))

		_ = seedIgnoreConflict(client.SysModule.Create().
			SetModuleCode("PROJECT_MANAGEMENT").SetModuleName("项目管理模块").SetModuleType("CATEGORY").
			SetStatus("ACTIVE").SetParentModuleID(allModule.ID).
			OnConflictColumns("module_code").DoNothing().Exec(ctx))

		// ADMIN → ALL 节点上的全部权限
		allPerms, _ := client.SysPermission.Query().All(ctx)
		permMap := make(map[string]uuid.UUID)
		for _, p := range allPerms {
			permMap[p.PermissionCode] = p.ID
		}
		allRoles, _ := client.SysRole.Query().All(ctx)
		roleMap := make(map[string]uuid.UUID)
		for _, r := range allRoles {
			roleMap[r.RoleCode] = r.ID
		}
		allModules, _ := client.SysModule.Query().All(ctx)
		moduleMap := make(map[string]uuid.UUID)
		for _, m := range allModules {
			moduleMap[m.ModuleCode] = m.ID
		}

		for _, pCode := range []string{"VIEW", "EDIT", "CREATE", "DELETE"} {
			_ = seedIgnoreConflict(client.SysRolePermission.Create().
				SetRoleID(roleMap["ADMIN"]).SetModuleID(moduleMap["ALL"]).SetPermissionID(permMap[pCode]).
				OnConflictColumns("role_id", "module_id", "permission_id").DoNothing().Exec(ctx))
		}

		// USER_ADMIN → USER_MANAGEMENT 上全部权限
		for _, pCode := range []string{"VIEW", "EDIT", "CREATE", "DELETE"} {
			_ = seedIgnoreConflict(client.SysRolePermission.Create().
				SetRoleID(roleMap["USER_ADMIN"]).SetModuleID(moduleMap["USER_MANAGEMENT"]).SetPermissionID(permMap[pCode]).
				OnConflictColumns("role_id", "module_id", "permission_id").DoNothing().Exec(ctx))
		}
	}

	// ========== 4. 组织角色 ==========
	orgRoles := []struct {
		Code  string
		Name  string
		Perms []string
		Desc  string
	}{
		{"ORG_OWNER", "组织所有者", []string{"ORG_VIEW", "ORG_EDIT", "ORG_MANAGE_MEMBER", "ORG_MANAGE_CHILD", "ORG_MANAGE_PROJECT", "ORG_CREATE_PROJECT"}, "组织最高权限，权限沿树向下继承"},
		{"ORG_ADMIN", "组织管理员", []string{"ORG_VIEW", "ORG_EDIT", "ORG_MANAGE_MEMBER", "ORG_MANAGE_PROJECT", "ORG_CREATE_PROJECT"}, "可管理成员和项目，权限沿树向下继承"},
		{"ORG_MEMBER", "组织成员", []string{"ORG_VIEW"}, "仅可查看组织信息，项目权限需单独授予"},
	}
	for _, r := range orgRoles {
		err := client.UxOrgRole.Create().
			SetRoleCode(r.Code).
			SetRoleName(r.Name).
			SetPermissionCodes(r.Perms).
			SetDescription(r.Desc).
			OnConflictColumns("role_code").
			UpdatePermissionCodes().
			UpdateRoleName().
			Exec(ctx)
		if err = seedIgnoreConflict(err); err != nil {
			return err
		}
	}

	// ========== 5. 项目角色 ==========
	projRoles := []struct {
		Code  string
		Name  string
		Perms []string
		Desc  string
	}{
		{"OWNER", "项目所有者", []string{"VIEW", "EDIT", "EXECUTE", "DELETE", "MANAGE_MEMBER"}, "项目最高权限"},
		{"ADMIN", "项目管理员", []string{"VIEW", "EDIT", "EXECUTE", "MANAGE_MEMBER"}, "除删除项目外的全部权限"},
		{"EDITOR", "编辑者", []string{"VIEW", "EDIT", "EXECUTE"}, "可编辑内容和执行评估"},
		{"VIEWER", "查看者", []string{"VIEW"}, "仅可查看"},
	}
	for _, r := range projRoles {
		err := client.UxProjectRole.Create().
			SetRoleCode(r.Code).
			SetRoleName(r.Name).
			SetPermissionCodes(r.Perms).
			SetDescription(r.Desc).
			OnConflictColumns("role_code").
			UpdatePermissionCodes().
			UpdateRoleName().
			Exec(ctx)
		if err = seedIgnoreConflict(err); err != nil {
			return err
		}
	}

	// ========== 数据迁移：将 RESEARCHER 角色重命名为普通成员 ==========
	_ = client.SysRole.Update().
		Where(sysrole.RoleCode("RESEARCHER")).
		SetRoleName("普通成员").
		Exec(ctx)

	log.Println("基础数据初始化完成（ent seed）")
	return nil
}

// SeedAdminUser 若系统中不存在任何拥有 ADMIN 角色的用户，则自动创建一个（幂等）
func SeedAdminUser(client *ent.Client, account, password, nickname string) error {
	ctx := context.Background()

	// 查询 ADMIN 角色
	adminRole, err := client.SysRole.Query().
		Where(sysrole.RoleCode("ADMIN")).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("查询 ADMIN 角色失败: %w", err)
	}

	// 检查是否已有绑定 ADMIN 角色的用户
	count, err := client.SysUserRole.Query().
		Where(sysuserrole.RoleID(adminRole.ID)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("查询管理员用户失败: %w", err)
	}
	if count > 0 {
		log.Println("已存在系统管理员，跳过自动创建")
		return nil
	}

	// 检查账号是否已存在（防止账号冲突）
	exists, _ := client.SysUser.Query().
		Where(sysuser.Account(account)).
		Exist(ctx)
	if exists {
		// 账号存在但没有 ADMIN 角色，直接绑定
		u, err := client.SysUser.Query().Where(sysuser.Account(account)).Only(ctx)
		if err != nil {
			return fmt.Errorf("查询已有账号失败: %w", err)
		}
		err = client.SysUserRole.Create().
			SetUserID(u.ID).
			SetRoleID(adminRole.ID).
			OnConflictColumns("user_id", "role_id").DoNothing().
			Exec(ctx)
		if err = seedIgnoreConflict(err); err != nil {
			return fmt.Errorf("绑定 ADMIN 角色失败: %w", err)
		}
		log.Printf("已将账号 %q 提升为系统管理员", account)
		return nil
	}

	// 创建新管理员用户
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	// 查询 MEMBER 角色（注册时默认分配）
	memberRole, err := client.SysRole.Query().
		Where(sysrole.RoleCode("MEMBER")).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("查询 MEMBER 角色失败: %w", err)
	}

	u, err := client.SysUser.Create().
		SetAccount(account).
		SetPasswordHash(string(hash)).
		SetNickname(nickname).
		SetStatus(sysuser.StatusACTIVE).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("创建管理员用户失败: %w", err)
	}

	// 绑定 ADMIN 角色
	if err = seedIgnoreConflict(client.SysUserRole.Create().
		SetUserID(u.ID).SetRoleID(adminRole.ID).
		OnConflictColumns("user_id", "role_id").DoNothing().Exec(ctx)); err != nil {
		return fmt.Errorf("绑定 ADMIN 角色失败: %w", err)
	}
	// 同时绑定 MEMBER 角色
	_ = seedIgnoreConflict(client.SysUserRole.Create().
		SetUserID(u.ID).SetRoleID(memberRole.ID).
		OnConflictColumns("user_id", "role_id").DoNothing().Exec(ctx))

	log.Printf("系统管理员已自动创建 账号=%q 昵称=%q（请及时修改默认密码）", account, nickname)
	return nil
}
