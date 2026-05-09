package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"evalux-server/internal/llm"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

type ProfileService struct {
	profileRepo   *repo.ProfileRepo
	projectRepo   *repo.ProjectRepo
	permRepo      *repo.UnifiedPermRepo
	llmClient     *llm.Client
	promptService *PromptService
}

func NewProfileService(profileRepo *repo.ProfileRepo, projectRepo *repo.ProjectRepo, permRepo *repo.UnifiedPermRepo, llmClient *llm.Client, promptService *PromptService) *ProfileService {
	return &ProfileService{profileRepo: profileRepo, projectRepo: projectRepo, permRepo: permRepo, llmClient: llmClient, promptService: promptService}
}

// Generate 调用大模型批量生成画像（非流式，兼容保留）
func (s *ProfileService) Generate(ctx context.Context, operatorID string, req model.GenerateProfilesRequest) (*model.GenerateProfilesResponse, error) {
	content, err := s.callLLMForProfiles(ctx, operatorID, req)
	if err != nil {
		return nil, err
	}
	return s.saveProfilesFromContent(ctx, req.ProjectID, req.ProfileType, content)
}

// GenerateStream 流式生成画像，每收到 token 就通过 onChunk 回调
// 最终返回完整内容用于解析入库
func (s *ProfileService) GenerateStream(ctx context.Context, operatorID string, req model.GenerateProfilesRequest, onChunk func(chunk string)) (*model.GenerateProfilesResponse, error) {
	canCreate, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
	if !canCreate {
		return nil, errors.New("无权在该项目下生成画像")
	}

	project, _ := s.projectRepo.FindByIDInternal(ctx, req.ProjectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}

	prompt := buildProfileGenerationPrompt(req)

	systemPrompt := s.promptService.GetPrompt(ctx, req.ProjectID, "profile_normal_system")
	if req.ProfileType == "expert" {
		systemPrompt = s.promptService.GetPrompt(ctx, req.ProjectID, "profile_expert_system")
	}

	var fullContent strings.Builder
	err := s.llmClient.ChatStream(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		Channel:     req.ModelChannel,
		ModelConfig: mc,
	}, func(chunk string) {
		fullContent.WriteString(chunk)
		onChunk(chunk)
	})
	if err != nil {
		return nil, fmt.Errorf("模型调用失败: %w", err)
	}

	return s.saveProfilesFromContent(ctx, req.ProjectID, req.ProfileType, fullContent.String())
}

// callLLMForProfiles 内部调用 LLM（非流式）
func (s *ProfileService) callLLMForProfiles(ctx context.Context, operatorID string, req model.GenerateProfilesRequest) (string, error) {
	canCreate, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
	if !canCreate {
		return "", errors.New("无权在该项目下生成画像")
	}

	project, _ := s.projectRepo.FindByIDInternal(ctx, req.ProjectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}

	prompt := buildProfileGenerationPrompt(req)

	sysPrompt := s.promptService.GetPrompt(ctx, req.ProjectID, "profile_normal_system")
	if req.ProfileType == "expert" {
		sysPrompt = s.promptService.GetPrompt(ctx, req.ProjectID, "profile_expert_system")
	}

	resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: prompt},
		},
		Channel:     req.ModelChannel,
		ModelConfig: mc,
	})
	if err != nil {
		return "", fmt.Errorf("模型调用失败: %w", err)
	}
	return resp.Content, nil
}

// saveProfilesFromContent 解析模型返回内容并入库
func (s *ProfileService) saveProfilesFromContent(ctx context.Context, projectID, profileType, content string) (*model.GenerateProfilesResponse, error) {
	profiles, err := parseProfilesFromLLM(content)
	if err != nil {
		return nil, fmt.Errorf("模型返回结果解析失败: %w", err)
	}

	if profileType == "" {
		profileType = "normal"
	}

	result := &model.GenerateProfilesResponse{
		Profiles: make([]model.ProfileDetail, 0, len(profiles)),
	}
	for _, p := range profiles {
		detail, err := s.profileRepo.Create(ctx, projectID,
			profileType,
			getString(p, "age_group"),
			getString(p, "education_level"),
			getString(p, "gender"),
			getCustomFields(p),
		)
		if err != nil {
			continue
		}
		result.Profiles = append(result.Profiles, *detail)
		result.Generated++
	}

	return result, nil
}

// List 查询画像列表
func (s *ProfileService) List(ctx context.Context, operatorID, projectID string, query model.ProfileListQuery) (*model.ProfileListResponse, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目画像")
	}
	profiles, total, err := s.profileRepo.ListByProjectID(ctx, projectID, query)
	if err != nil {
		return nil, errors.New("查询画像列表失败")
	}
	return &model.ProfileListResponse{Total: total, List: profiles}, nil
}

// GetByID 查询画像详情
func (s *ProfileService) GetByID(ctx context.Context, operatorID, profileID string) (*model.ProfileDetail, error) {
	detail, err := s.profileRepo.FindByID(ctx, profileID)
	if err != nil {
		return nil, errors.New("画像不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该画像")
	}
	return detail, nil
}

// Update 更新画像
func (s *ProfileService) Update(ctx context.Context, operatorID, profileID string, req model.UpdateProfileRequest) (*model.ProfileDetail, error) {
	detail, err := s.profileRepo.FindByID(ctx, profileID)
	if err != nil {
		return nil, errors.New("画像不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
	if !canEdit {
		return nil, errors.New("无权编辑该画像")
	}
	return s.profileRepo.Update(ctx, profileID, req)
}

// Delete 删除画像
func (s *ProfileService) Delete(ctx context.Context, operatorID, profileID string) error {
	detail, err := s.profileRepo.FindByID(ctx, profileID)
	if err != nil {
		return errors.New("画像不存在")
	}
	canDelete, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
	if !canDelete {
		return errors.New("无权删除该画像")
	}
	return s.profileRepo.Delete(ctx, profileID)
}

// ========== helpers ==========

func buildProfileGenerationPrompt(req model.GenerateProfilesRequest) string {
	if req.ProfileType == "expert" {
		return buildExpertProfilePrompt(req)
	}
	return buildNormalProfilePrompt(req)
}

// buildNormalProfilePrompt 普通群众画像 Prompt
func buildNormalProfilePrompt(req model.GenerateProfilesRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请生成 %d 个虚拟用户画像，返回JSON数组。\n\n", req.Count))

	sb.WriteString("每个画像必须包含以下所有字段（JSON key 用英文）：\n")
	sb.WriteString("- nickname: 昵称（中文，2-4个字的自然人名，如\"张明\"\"李小红\"）\n")
	sb.WriteString("- gender: 性别（男/女）\n")
	sb.WriteString("- age: 年龄（具体数字，18-70之间）\n")
	sb.WriteString("- age_group: 年龄层次（青年18-35/中年36-55/老年56+）\n")
	sb.WriteString("- education_level: 学历（小学/初中/高中/大专/本科/硕士/博士）\n")
	sb.WriteString("- occupation: 职业/行业（如学生、教师、程序员、设计师、销售、退休、自由职业等）\n")
	sb.WriteString("- city_tier: 所在城市层级（一线城市/新一线城市/二线城市/三四线城市/县城乡镇）\n")
	sb.WriteString("- city: 所在城市（具体城市名，如北京、成都、威海等，与city_tier匹配）\n")
	sb.WriteString("- income_level: 收入水平（低收入<5k/中等收入5k-15k/中高收入15k-30k/高收入>30k，月薪）\n")
	sb.WriteString("- phone_usage_years: 智能手机使用年限（具体数字，1-15年）\n")
	sb.WriteString("- phone_usage_frequency: 手机使用频率（轻度用户<2h/天、中度用户2-5h/天、重度用户>5h/天）\n")
	sb.WriteString("- personality: 性格特征（从谨慎、果断、耐心、急躁、粗心、细心中选1-2个，用逗号分隔）\n")
	sb.WriteString("- typical_scenario: 典型使用场景（从居家、办公室、通勤、公共场所、夜间中选1-2个，用逗号分隔）\n")
	sb.WriteString("- tech_savviness: 技术熟练度（新手/普通/熟练/极客）\n")
	sb.WriteString("- avatar_desc: 头像描述（用一句简短的自然语言描述这个人的外貌特征，用于后续AI生成头像图片，如\"30岁左右的职业女性，短发，戴眼镜，温和的笑容\"）\n\n")

	// 处理筛选条件
	appendFilters(&sb, req)
	appendFieldDefs(&sb, req)
	appendNormalRequirements(&sb)
	return sb.String()
}

// buildExpertProfilePrompt 专家画像 Prompt
func buildExpertProfilePrompt(req model.GenerateProfilesRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请生成 %d 个UX评估专家画像，返回JSON数组。\n\n", req.Count))

	sb.WriteString("每个专家画像必须包含以下所有字段（JSON key 用英文）：\n")
	sb.WriteString("- nickname: 专家姓名（中文，2-4个字的自然人名，如\"王博文\"\"陈思睿\"）\n")
	sb.WriteString("- gender: 性别（男/女）\n")
	sb.WriteString("- age: 年龄（具体数字，25-60之间）\n")
	sb.WriteString("- age_group: 年龄层次（青年25-35/中年36-55/资深56+）\n")
	sb.WriteString("- education_level: 学历（本科/硕士/博士）\n")
	sb.WriteString("- specialty_domain: 专业领域（从交互设计/可用性工程/UI设计/用户研究/信息架构/无障碍设计中选1-2个，用逗号分隔）\n")
	sb.WriteString("- work_years: 从业年限（具体数字，3-25年）\n")
	sb.WriteString("- affiliation: 所属机构（如\"某科技公司UX团队\"\"高校人机交互实验室\"\"独立UX咨询师\"等）\n")
	sb.WriteString("- eval_methodology: 评估方法论偏好（从启发式评估/认知走查/SUS问卷/尼尔森十原则/用户访谈/A/B测试/眼动追踪中选1-3个，用逗号分隔）\n")
	sb.WriteString("- focus_area: 关注重点（从可学习性/效率/可记忆性/容错性/满意度/一致性/反馈及时性/视觉层次中选2-3个，用逗号分隔）\n")
	sb.WriteString("- eval_style: 评估风格（严谨系统型/直觉体验型/数据驱动型/用户共情型）\n")
	sb.WriteString("- strictness: 评估严格程度（宽松/适中/严格/非常严格）\n")
	sb.WriteString("- personality: 专业素养特征（从严谨、洞察力强、注重细节、擅长沟通、理论功底深、实战经验丰富中选1-2个，用逗号分隔）\n")
	sb.WriteString("- avatar_desc: 头像描述（用一句简短的自然语言描述这个专家的外貌特征，如\"35岁左右的男性学者，戴细框眼镜，穿深色休闲西装，目光锐利\"）\n\n")

	// 处理筛选条件
	appendFilters(&sb, req)
	appendFieldDefs(&sb, req)

	sb.WriteString("要求：\n")
	sb.WriteString("1. 专家之间必须有足够差异性，覆盖不同专业领域、从业年限、评估风格的组合\n")
	sb.WriteString("2. 每个专家的 avatar_desc 要与其年龄、性别等特征一致\n")
	sb.WriteString("3. 评估方法论和关注重点要与专业领域匹配\n")
	sb.WriteString("4. 只返回JSON数组，不要有任何其他文字说明\n")
	return sb.String()
}

func appendFilters(sb *strings.Builder, req model.GenerateProfilesRequest) {
	if len(req.Filters) > 0 {
		sb.WriteString("【重要约束】生成的画像必须满足以下条件：\n")
		for _, f := range req.Filters {
			dimensionLabel := getDimensionLabel(f.Dimension)
			sb.WriteString(fmt.Sprintf("- %s 仅限: %s\n", dimensionLabel, strings.Join(f.Values, "、")))
		}
		sb.WriteString("\n")
	}
}

func appendFieldDefs(sb *strings.Builder, req model.GenerateProfilesRequest) {
	if len(req.FieldDefs) > 0 {
		sb.WriteString("还需包含以下额外自定义字段：\n")
		for _, f := range req.FieldDefs {
			sb.WriteString(fmt.Sprintf("- %s (%s)", f.FieldName, f.FieldType))
			if len(f.Candidates) > 0 {
				sb.WriteString(fmt.Sprintf("，候选值: %s", strings.Join(f.Candidates, "、")))
			}
			if f.RangeMin != "" || f.RangeMax != "" {
				sb.WriteString(fmt.Sprintf("，范围: %s ~ %s", f.RangeMin, f.RangeMax))
			}
			if f.Description != "" {
				sb.WriteString(fmt.Sprintf("，说明: %s", f.Description))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}

func appendNormalRequirements(sb *strings.Builder) {
	sb.WriteString("要求：\n")
	sb.WriteString("1. 画像之间必须有足够差异性，覆盖不同年龄、性别、职业、地域、性格的组合\n")
	sb.WriteString("2. 每个人的 avatar_desc 要与其年龄、性别、职业等特征一致\n")
	sb.WriteString("3. city 要与 city_tier 匹配（如一线城市只能是北京/上海/广州/深圳）\n")
	sb.WriteString("4. 只返回JSON数组，不要有任何其他文字说明\n")
}

// getDimensionLabel 获取维度的中文标签
func getDimensionLabel(dim string) string {
	labels := map[string]string{
		"gender":                "性别",
		"age_group":             "年龄层次",
		"education_level":       "学历",
		"occupation":            "职业",
		"city_tier":             "城市层级",
		"income_level":          "收入水平",
		"phone_usage_frequency": "手机使用频率",
		"personality":           "性格特征",
		"typical_scenario":      "使用场景",
		"tech_savviness":        "技术熟练度",
		// 专家维度
		"specialty_domain":  "专业领域",
		"eval_methodology":  "评估方法论",
		"focus_area":        "关注重点",
		"eval_style":        "评估风格",
		"strictness":        "严格程度",
	}
	if label, ok := labels[dim]; ok {
		return label
	}
	return dim
}

// BatchDelete 批量删除画像
func (s *ProfileService) BatchDelete(ctx context.Context, operatorID string, req model.BatchDeleteProfilesRequest) (int, error) {
	if len(req.ProfileIDs) == 0 {
		return 0, errors.New("画像ID列表不能为空")
	}
	// 检查第一个画像的权限（同一项目下的画像）
	detail, err := s.profileRepo.FindByID(ctx, req.ProfileIDs[0])
	if err != nil {
		return 0, errors.New("画像不存在")
	}
	canDelete, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
	if !canDelete {
		return 0, errors.New("无权删除该项目的画像")
	}
	return s.profileRepo.BatchDelete(ctx, req.ProfileIDs)
}

func parseProfilesFromLLM(content string) ([]map[string]interface{}, error) {
	// 尝试从内容中提取JSON数组
	content = strings.TrimSpace(content)
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("未找到JSON数组")
	}
	jsonStr := content[start : end+1]

	var profiles []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &profiles); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return profiles, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getCustomFields(m map[string]interface{}) map[string]interface{} {
	// age_group, education_level, gender 是独立数据库字段，其余全部存入 custom_fields
	coreFields := map[string]bool{
		"age_group": true, "education_level": true, "gender": true,
	}
	custom := make(map[string]interface{})
	for k, v := range m {
		if !coreFields[k] {
			custom[k] = v
		}
	}
	if len(custom) == 0 {
		return nil
	}
	return custom
}
