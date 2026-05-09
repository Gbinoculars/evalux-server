package repo

import (
	"context"
	"strings"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxproject"
	"evalux-server/ent/uxprojectmember"
	"evalux-server/ent/uxprojectrole"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type ProjectRepo struct {
	client *ent.Client
}

func NewProjectRepo(client *ent.Client) *ProjectRepo {
	return &ProjectRepo{client: client}
}

func (r *ProjectRepo) Create(ctx context.Context, createdBy string, req model.CreateProjectRequest) (*model.ProjectDetail, error) {
	uid, err := uuid.Parse(createdBy)
	if err != nil {
		return nil, err
	}
	builder := r.client.UxProject.Create().
		SetCreatedBy(uid).
		SetProjectName(req.ProjectName).
		SetAppName(req.AppName).
		SetNillableAppVersion(nilIfEmpty(req.AppVersion)).
		SetResearchGoal(req.ResearchGoal).
		SetNillableProjectDesc(nilIfEmpty(req.ProjectDesc)).
		SetStatus("ACTIVE")
	if req.OrgID != nil && *req.OrgID != "" {
		oid, err := uuid.Parse(*req.OrgID)
		if err != nil {
			return nil, err
		}
		builder.SetOrgID(oid)
	}
	if req.ModelConfig != nil {
		builder.SetModelConfig(modelConfigToMap(req.ModelConfig))
	}
	p, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entProjectToDetail(p), nil
}

func (r *ProjectRepo) FindByID(ctx context.Context, projectID string) (*model.ProjectDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	p, err := r.client.UxProject.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	return entProjectToDetail(p), nil
}

// FindByIDInternal 获取项目完整信息（含明文 API Key），仅供后端内部 LLM 调用使用
func (r *ProjectRepo) FindByIDInternal(ctx context.Context, projectID string) (*model.ProjectDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	p, err := r.client.UxProject.Get(ctx, pid)
	if err != nil {
		return nil, err
	}
	return entProjectToDetailRaw(p), nil
}

func (r *ProjectRepo) Update(ctx context.Context, projectID string, req model.UpdateProjectRequest) (*model.ProjectDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxProject.UpdateOneID(pid)
	if req.ProjectName != nil {
		upd.SetProjectName(*req.ProjectName)
	}
	if req.AppName != nil {
		upd.SetAppName(*req.AppName)
	}
	if req.AppVersion != nil {
		upd.SetAppVersion(*req.AppVersion)
	}
	if req.ResearchGoal != nil {
		upd.SetResearchGoal(*req.ResearchGoal)
	}
	if req.ProjectDesc != nil {
		upd.SetProjectDesc(*req.ProjectDesc)
	}
	if req.Status != nil {
		upd.SetStatus(uxproject.Status(*req.Status))
		if *req.Status == "ARCHIVED" {
			now := time.Now()
			upd.SetArchivedAt(now)
		}
	}
	if req.ModelConfig != nil {
		// 如果前端传回的 API Key 是脱敏值（含 ****），保留数据库中的原始值
		if strings.Contains(req.ModelConfig.OpenRouterAPIKey, "****") || strings.Contains(req.ModelConfig.OpenAICompatibleAPIKey, "****") {
			existing, _ := r.client.UxProject.Get(ctx, pid)
			if existing != nil && existing.ModelConfig != nil {
				oldMc := mapToModelConfig(existing.ModelConfig)
				if oldMc != nil {
					if strings.Contains(req.ModelConfig.OpenRouterAPIKey, "****") {
						req.ModelConfig.OpenRouterAPIKey = oldMc.OpenRouterAPIKey
					}
					if strings.Contains(req.ModelConfig.OpenAICompatibleAPIKey, "****") {
						req.ModelConfig.OpenAICompatibleAPIKey = oldMc.OpenAICompatibleAPIKey
					}
				}
			}
		}
		upd.SetModelConfig(modelConfigToMap(req.ModelConfig))
	}
	p, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entProjectToDetail(p), nil
}

func (r *ProjectRepo) Delete(ctx context.Context, projectID string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	return r.client.UxProject.DeleteOneID(pid).Exec(ctx)
}

func (r *ProjectRepo) ListByIDs(ctx context.Context, projectIDs []string, query model.ProjectListQuery) ([]model.ProjectDetail, int64, error) {
	if len(projectIDs) == 0 {
		return nil, 0, nil
	}
	pids := make([]uuid.UUID, 0, len(projectIDs))
	for _, id := range projectIDs {
		pid, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}

	q := r.client.UxProject.Query().Where(uxproject.IDIn(pids...))
	if query.Keyword != "" {
		q = q.Where(uxproject.Or(
			uxproject.ProjectNameContainsFold(query.Keyword),
			uxproject.AppNameContainsFold(query.Keyword),
		))
	}
	if query.Status != "" {
		q = q.Where(uxproject.StatusEQ(uxproject.Status(query.Status)))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize

	projects, err := q.Order(ent.Desc(uxproject.FieldCreatedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	list := make([]model.ProjectDetail, 0, len(projects))
	for _, p := range projects {
		list = append(list, *entProjectToDetail(p))
	}
	return list, int64(total), nil
}

// ==================== 项目成员管理 ====================

func (r *ProjectRepo) ListMembers(ctx context.Context, projectID string) ([]model.ProjectMember, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	members, err := r.client.UxProjectMember.Query().
		Where(uxprojectmember.ProjectID(pid)).
		WithUser().
		WithRole().
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ProjectMember, 0, len(members))
	for _, m := range members {
		pm := model.ProjectMember{
			ProjectMemberID: m.ID.String(),
			ProjectID:       m.ProjectID.String(),
			UserID:          m.UserID.String(),
			ProjectRoleID:   m.ProjectRoleID.String(),
			CreatedAt:       m.CreatedAt,
		}
		if m.Edges.User != nil {
			pm.UserAccount = m.Edges.User.Account
			pm.UserNickname = m.Edges.User.Nickname
		}
		if m.Edges.Role != nil {
			pm.RoleCode = m.Edges.Role.RoleCode
			pm.RoleName = m.Edges.Role.RoleName
		}
		list = append(list, pm)
	}
	return list, nil
}

func (r *ProjectRepo) UpdateMemberRole(ctx context.Context, projectID, userID, roleCode string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	role, err := r.client.UxProjectRole.Query().Where(uxprojectrole.RoleCode(roleCode)).Only(ctx)
	if err != nil {
		return err
	}
	_, err = r.client.UxProjectMember.Update().
		Where(uxprojectmember.ProjectID(pid), uxprojectmember.UserID(uid)).
		SetProjectRoleID(role.ID).
		Save(ctx)
	return err
}

func (r *ProjectRepo) RemoveMember(ctx context.Context, projectID, userID string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = r.client.UxProjectMember.Delete().
		Where(uxprojectmember.ProjectID(pid), uxprojectmember.UserID(uid)).
		Exec(ctx)
	return err
}

func (r *ProjectRepo) ListRoles(ctx context.Context) ([]model.ProjectRole, error) {
	roles, err := r.client.UxProjectRole.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ProjectRole, 0, len(roles))
	for _, role := range roles {
		pr := model.ProjectRole{
			ProjectRoleID:   role.ID.String(),
			RoleCode:        role.RoleCode,
			RoleName:        role.RoleName,
			PermissionCodes: role.PermissionCodes,
		}
		if role.Description != "" {
			pr.Description = &role.Description
		}
		list = append(list, pr)
	}
	return list, nil
}

// ==================== helpers ====================

func entProjectToDetail(p *ent.UxProject) *model.ProjectDetail {
	d := &model.ProjectDetail{
		ProjectID:    p.ID.String(),
		CreatedBy:    p.CreatedBy.String(),
		ProjectName:  p.ProjectName,
		AppName:      p.AppName,
		ResearchGoal: p.ResearchGoal,
		Status:       string(p.Status),
		CreatedAt:    p.CreatedAt,
	}
	if p.OrgID != nil {
		s := p.OrgID.String()
		d.OrgID = &s
	}
	if p.AppVersion != nil {
		d.AppVersion = *p.AppVersion
	}
	if p.ProjectDesc != nil {
		d.ProjectDesc = *p.ProjectDesc
	}
	if p.ArchivedAt != nil {
		d.ArchivedAt = p.ArchivedAt
	}
	if p.ModelConfig != nil {
		mc := mapToModelConfig(p.ModelConfig)
		if mc != nil {
			// 脱敏：不向前端返回任何 API Key 明文
			if mc.OpenRouterAPIKey != "" {
				mc.OpenRouterAPIKey = maskKey(mc.OpenRouterAPIKey)
			}
			if mc.OpenAICompatibleAPIKey != "" {
				mc.OpenAICompatibleAPIKey = maskKey(mc.OpenAICompatibleAPIKey)
			}
		}
		d.ModelConfig = mc
	}
	return d
}

// entProjectToDetailRaw 不做 API Key 脱敏，供后端内部 LLM 调用使用
func entProjectToDetailRaw(p *ent.UxProject) *model.ProjectDetail {
	d := &model.ProjectDetail{
		ProjectID:    p.ID.String(),
		CreatedBy:    p.CreatedBy.String(),
		ProjectName:  p.ProjectName,
		AppName:      p.AppName,
		ResearchGoal: p.ResearchGoal,
		Status:       string(p.Status),
		CreatedAt:    p.CreatedAt,
	}
	if p.OrgID != nil {
		s := p.OrgID.String()
		d.OrgID = &s
	}
	if p.AppVersion != nil {
		d.AppVersion = *p.AppVersion
	}
	if p.ProjectDesc != nil {
		d.ProjectDesc = *p.ProjectDesc
	}
	if p.ArchivedAt != nil {
		d.ArchivedAt = p.ArchivedAt
	}
	if p.ModelConfig != nil {
		mc := mapToModelConfig(p.ModelConfig)
		d.ModelConfig = mc
	}
	return d
}

func modelConfigToMap(mc *model.ModelConfig) map[string]interface{} {
	return map[string]interface{}{
		"default_channel":            mc.DefaultChannel,
		"ollama_base_url":            mc.OllamaBaseURL,
		"ollama_model":               mc.OllamaModel,
		"openrouter_base_url":        mc.OpenRouterBaseURL,
		"openrouter_api_key":         mc.OpenRouterAPIKey,
		"openrouter_model":           mc.OpenRouterModel,
		"openai_compatible_base_url": mc.OpenAICompatibleBaseURL,
		"openai_compatible_api_key":  mc.OpenAICompatibleAPIKey,
		"openai_compatible_model":    mc.OpenAICompatibleModel,
		"platform":                   mc.Platform,
		"app_id":                     mc.AppID,
		"bundle_id":                  mc.BundleID,
		"wda_url":                    mc.WdaURL,
	}
}

func mapToModelConfig(m map[string]interface{}) *model.ModelConfig {
	mc := &model.ModelConfig{}
	if v, ok := m["default_channel"].(string); ok {
		mc.DefaultChannel = v
	}
	if v, ok := m["ollama_base_url"].(string); ok {
		mc.OllamaBaseURL = v
	}
	if v, ok := m["ollama_model"].(string); ok {
		mc.OllamaModel = v
	}
	if v, ok := m["openrouter_base_url"].(string); ok {
		mc.OpenRouterBaseURL = v
	}
	if v, ok := m["openrouter_api_key"].(string); ok {
		mc.OpenRouterAPIKey = v
	}
	if v, ok := m["openrouter_model"].(string); ok {
		mc.OpenRouterModel = v
	}
	if v, ok := m["openai_compatible_base_url"].(string); ok {
		mc.OpenAICompatibleBaseURL = v
	}
	if v, ok := m["openai_compatible_api_key"].(string); ok {
		mc.OpenAICompatibleAPIKey = v
	}
	if v, ok := m["openai_compatible_model"].(string); ok {
		mc.OpenAICompatibleModel = v
	}
	if v, ok := m["platform"].(string); ok {
		mc.Platform = v
	}
	if v, ok := m["app_id"].(string); ok {
		mc.AppID = v
	}
	if v, ok := m["bundle_id"].(string); ok {
		mc.BundleID = v
	}
	if v, ok := m["wda_url"].(string); ok {
		mc.WdaURL = v
	}
	if mc.DefaultChannel == "" && mc.OllamaBaseURL == "" && mc.OllamaModel == "" &&
		mc.OpenRouterBaseURL == "" && mc.OpenRouterAPIKey == "" && mc.OpenRouterModel == "" &&
		mc.OpenAICompatibleBaseURL == "" && mc.OpenAICompatibleAPIKey == "" && mc.OpenAICompatibleModel == "" &&
		mc.Platform == "" && mc.AppID == "" && mc.BundleID == "" && mc.WdaURL == "" {
		return nil
	}
	return mc
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// maskKey 对 API Key 做脱敏，仅保留前4位和后4位，中间用 **** 替代
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
