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

type QuestionnaireService struct {
	qRepo         *repo.QuestionnaireRepo
	permRepo      *repo.UnifiedPermRepo
	projectRepo   *repo.ProjectRepo
	llmClient     *llm.Client
	promptService *PromptService
}

func NewQuestionnaireService(qRepo *repo.QuestionnaireRepo, permRepo *repo.UnifiedPermRepo) *QuestionnaireService {
	return &QuestionnaireService{qRepo: qRepo, permRepo: permRepo}
}

func NewQuestionnaireServiceWithLLM(qRepo *repo.QuestionnaireRepo, permRepo *repo.UnifiedPermRepo, projectRepo *repo.ProjectRepo, llmClient *llm.Client, promptService *PromptService) *QuestionnaireService {
	return &QuestionnaireService{qRepo: qRepo, permRepo: permRepo, projectRepo: projectRepo, llmClient: llmClient, promptService: promptService}
}

func (s *QuestionnaireService) CreateTemplate(ctx context.Context, operatorID string, req model.CreateQuestionnaireRequest) (*model.QuestionnaireDetail, error) {
	if req.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
		if !ok {
			return nil, errors.New("无权在该项目下创建问卷")
		}
	}
	return s.qRepo.CreateTemplate(ctx, req)
}

func (s *QuestionnaireService) ListTemplates(ctx context.Context, operatorID, projectID string, query model.QuestionnaireListQuery) (*model.QuestionnaireListResponse, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !ok {
		return nil, errors.New("无权查看该项目问卷")
	}
	list, total, err := s.qRepo.ListTemplatesByProjectID(ctx, projectID, query)
	if err != nil {
		return nil, errors.New("查询问卷列表失败")
	}
	return &model.QuestionnaireListResponse{Total: total, List: list}, nil
}

func (s *QuestionnaireService) GetTemplate(ctx context.Context, operatorID, templateID string) (*model.QuestionnaireDetail, error) {
	detail, err := s.qRepo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return nil, errors.New("问卷不存在")
	}
	if detail.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "VIEW")
		if !ok {
			return nil, errors.New("无权查看该问卷")
		}
	}
	return detail, nil
}

func (s *QuestionnaireService) UpdateTemplate(ctx context.Context, operatorID, templateID string, req model.UpdateQuestionnaireRequest) (*model.QuestionnaireDetail, error) {
	detail, err := s.qRepo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return nil, errors.New("问卷不存在")
	}
	if detail.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
		if !ok {
			return nil, errors.New("无权编辑该问卷")
		}
	}
	return s.qRepo.UpdateTemplate(ctx, templateID, req)
}

func (s *QuestionnaireService) DeleteTemplate(ctx context.Context, operatorID, templateID string) error {
	detail, err := s.qRepo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return errors.New("问卷不存在")
	}
	if detail.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
		if !ok {
			return errors.New("无权删除该问卷")
		}
	}
	return s.qRepo.DeleteTemplate(ctx, templateID)
}

func (s *QuestionnaireService) CreateQuestion(ctx context.Context, operatorID string, req model.CreateQuestionRequest) (*model.QuestionDetail, error) {
	tmpl, err := s.qRepo.FindTemplateByID(ctx, req.TemplateID)
	if err != nil {
		return nil, errors.New("问卷不存在")
	}
	if tmpl.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, tmpl.ProjectID, "EDIT")
		if !ok {
			return nil, errors.New("无权编辑该问卷题目")
		}
	}
	return s.qRepo.CreateQuestion(ctx, req)
}

func (s *QuestionnaireService) ListQuestions(ctx context.Context, operatorID, templateID string) ([]model.QuestionDetail, error) {
	tmpl, err := s.qRepo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return nil, errors.New("问卷不存在")
	}
	if tmpl.ProjectID != "" {
		ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, tmpl.ProjectID, "VIEW")
		if !ok {
			return nil, errors.New("无权查看该问卷题目")
		}
	}
	return s.qRepo.ListQuestionsByTemplateID(ctx, templateID)
}

func (s *QuestionnaireService) DeleteQuestion(ctx context.Context, operatorID, questionID string) error {
	return s.qRepo.DeleteQuestion(ctx, questionID)
}

func (s *QuestionnaireService) UpdateQuestion(ctx context.Context, operatorID, questionID string, req model.UpdateQuestionRequest) (*model.QuestionDetail, error) {
	return s.qRepo.UpdateQuestion(ctx, questionID, req)
}

func (s *QuestionnaireService) ReorderQuestions(ctx context.Context, operatorID, templateID string, questionIDs []string) error {
	return s.qRepo.ReorderQuestions(ctx, templateID, questionIDs)
}

// AIGenerateQuestionnaire 调用 LLM 生成问卷并存库
func (s *QuestionnaireService) AIGenerateQuestionnaire(ctx context.Context, operatorID string, req model.AIGenerateQuestionnaireRequest) (*model.QuestionnaireDetail, error) {
	if s.llmClient == nil || s.projectRepo == nil {
		return nil, errors.New("AI 生成功能未启用")
	}

	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
	if !ok {
		return nil, errors.New("无权在该项目下创建问卷")
	}

	// 获取项目模型配置（需要明文 API Key 用于 LLM 调用）
	project, _ := s.projectRepo.FindByIDInternal(ctx, req.ProjectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}

	channel := req.ModelChannel
	if channel == "" && mc != nil {
		channel = mc.DefaultChannel
	}
	if channel == "" {
		return nil, errors.New("未指定模型通道，请在项目配置中设置默认通道")
	}

	// 构建 prompt
	prompt := buildAIQuestionnairePrompt(req)

	var sb strings.Builder
	err := s.llmClient.ChatStream(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: s.promptService.GetPrompt(ctx, req.ProjectID, "questionnaire_ai_generate_system")},
			{Role: "user", Content: prompt},
		},
		Channel:     channel,
		ModelConfig: mc,
	}, func(chunk string) {
		sb.WriteString(chunk)
	})
	if err != nil {
		return nil, fmt.Errorf("AI 生成失败: %w", err)
	}

	// 解析 JSON
	rawContent := sb.String()
	// 尝试提取 ```json ... ``` 代码块
	if idx := strings.Index(rawContent, "```json"); idx >= 0 {
		rawContent = rawContent[idx+7:]
		if end := strings.Index(rawContent, "```"); end >= 0 {
			rawContent = rawContent[:end]
		}
	} else if idx := strings.Index(rawContent, "```"); idx >= 0 {
		rawContent = rawContent[idx+3:]
		if end := strings.Index(rawContent, "```"); end >= 0 {
			rawContent = rawContent[:end]
		}
	}
	rawContent = strings.TrimSpace(rawContent)

	type aiQuestion struct {
		Type          string   `json:"type"`
		Text          string   `json:"text"`
		Dimension     string   `json:"dimension"`
		ScaleMin      int      `json:"scale_min"`
		ScaleMax      int      `json:"scale_max"`
		Options       []string `json:"options"`
	}
	type aiResult struct {
		Questions []aiQuestion `json:"questions"`
	}

	var parsed aiResult
	if err := json.Unmarshal([]byte(rawContent), &parsed); err != nil {
		return nil, fmt.Errorf("AI 返回内容解析失败，请重试。原始内容前200字: %s", truncate(rawContent, 200))
	}

	// 创建问卷模板
	tmpl, err := s.qRepo.CreateTemplate(ctx, model.CreateQuestionnaireRequest{
		ProjectID:       req.ProjectID,
		TemplateName:    req.TemplateName,
		DimensionSchema: req.Aspects,
		TemplateDesc:    fmt.Sprintf("由 AI 根据「%s」理论基础自动生成", req.TheoryBasis),
	})
	if err != nil {
		return nil, fmt.Errorf("创建问卷模板失败: %w", err)
	}

	// 逐题创建
	for i, q := range parsed.Questions {
		createReq := model.CreateQuestionRequest{
			TemplateID:    tmpl.TemplateID,
			QuestionNo:    i + 1,
			QuestionType:  q.Type,
			QuestionText:  q.Text,
			DimensionCode: q.Dimension,
			IsRequired:    true,
		}
		if q.Type == "SCALE" {
			min, max := q.ScaleMin, q.ScaleMax
			if min == 0 { min = 1 }
			if max == 0 { max = req.ScaleOptions }
			createReq.ScoreRange = map[string]int{"min": min, "max": max}
			createReq.OptionList = []map[string]string{}
		} else if q.Type == "OPEN_ENDED" {
			createReq.OptionList = []map[string]string{}
			createReq.ScoreRange = map[string]int{}
		} else {
			opts := make([]map[string]string, 0, len(q.Options))
			for j, o := range q.Options {
				opts = append(opts, map[string]string{
					"label": o,
					"value": fmt.Sprintf("%d", j+1),
				})
			}
			createReq.OptionList = opts
			createReq.ScoreRange = map[string]int{}
		}
		if _, err := s.qRepo.CreateQuestion(ctx, createReq); err != nil {
			// 单题失败不影响整体，记录日志后继续
			continue
		}
	}

	return tmpl, nil
}

func buildAIQuestionnairePrompt(req model.AIGenerateQuestionnaireRequest) string {
	aspectsStr := strings.Join(req.Aspects, "、")
	return fmt.Sprintf(`请根据以下要求，为移动应用用户体验评估设计一份问卷，并以纯JSON格式输出。

【问卷需求】
- 问卷名称：%s
- 评价维度：%s
- 理论基础：%s
- 量表题（SCALE）数量：%d 题，评分范围 1~%d
- 主观开放题（OPEN_ENDED）数量：%d 题

【推荐测量维度参考（来源：移动应用UX评估量表）】
若用户指定的评价维度包含以下关键词，请参考对应的标准化问题表述：
1. 吸引力：我认为使用和操作这款应用程序界面总体上是让人愉快的。
2. 有效性：我认为这款应用程序能够十分高效和快速的让我完成目标和任务。
3. 易懂性：我认为操控和使用这款应用界面是非常容易的。
4. 可控性：我认为这款应用对于我的操作的反馈是在我的意料之中并且是符合逻辑的。
5. 趣味性：我认为操作和使用这款应用界面不会让我感到烦躁。
6. 创新性：我认为这款应用界面的设计十分的具有创新性。
7. 美观性：我认为这款应用界面的视觉设计让人感觉十分的愉悦。
8. 易用性：我认为我可以很容易的学会使用这款应用并且不需要额外的学习或者培训。
9. 内容可信度：我认为使用这款应用是可靠的，它不会因为误操作或者表述不清而对我带来利益损害。
10. 内容质量：我认为这款应用界面提供的信息和描述是准确的并且值得信任。
11. 界面清晰度：我认为这款应用界面的设计、归类和布局结构是有条理的。
12. 触觉体验：我认为这款应用界面的操作十分流畅。
13. 价值给予：我会将这款应用推荐给别人。
14. 有用性：我依然会继续使用这款应用。
15. 操作过程有效性：在使用过程中，我能非常清楚的知道我下一步应该做什么。

请根据用户指定的维度从上述参考中选取或调整题目表述，确保量表题的专业性和可度量性。

【输出要求】
输出纯JSON，结构如下：
{
  "questions": [
    {
      "type": "SCALE",
      "text": "题目内容",
      "dimension": "对应维度（必须是评价维度之一）",
      "scale_min": 1,
      "scale_max": %d
    },
    {
      "type": "OPEN_ENDED",
      "text": "题目内容",
      "dimension": "对应维度（必须是评价维度之一）",
      "scale_min": 0,
      "scale_max": 0
    }
  ]
}

要求：
1. 量表题必须覆盖所有评价维度，尽量均匀分布
2. 主观题应引导用户对整体体验或具体维度进行定性描述
3. 题目表述清晰、专业、符合中文用户习惯
4. type 字段只能是 SCALE 或 OPEN_ENDED
5. dimension 字段必须从以下维度中选择：%s
6. 只输出JSON，不要有任何额外文字或解释`,
		req.TemplateName,
		aspectsStr,
		req.TheoryBasis,
		req.ScaleCount,
		req.ScaleOptions,
		req.OpenEndedCount,
		req.ScaleOptions,
		aspectsStr,
	)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
