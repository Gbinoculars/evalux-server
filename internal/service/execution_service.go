package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"evalux-server/ent"
	"evalux-server/internal/llm"
	"evalux-server/internal/model"
	"evalux-server/internal/repo"
	"evalux-server/internal/storage"

	"github.com/google/uuid"
)

type ExecutionService struct {
	entClient     *ent.Client
	execRepo      *repo.ExecutionRepo
	taskRepo      *repo.TaskRepo
	profileRepo   *repo.ProfileRepo
	projectRepo   *repo.ProjectRepo
	runRepo       *repo.ExecutionRunRepo
	resultRepo    *repo.ResultRepo
	permRepo      *repo.UnifiedPermRepo
	llmClient     *llm.Client
	store         *storage.CloudreveStorage
	promptService *PromptService
}

func NewExecutionService(
	entClient *ent.Client,
	execRepo *repo.ExecutionRepo,
	taskRepo *repo.TaskRepo,
	profileRepo *repo.ProfileRepo,
	projectRepo *repo.ProjectRepo,
	runRepo *repo.ExecutionRunRepo,
	resultRepo *repo.ResultRepo,
	permRepo *repo.UnifiedPermRepo,
	llmClient *llm.Client,
	store *storage.CloudreveStorage,
	promptService *PromptService,
) *ExecutionService {
	return &ExecutionService{
		entClient: entClient,
		execRepo:  execRepo, taskRepo: taskRepo, profileRepo: profileRepo,
		projectRepo: projectRepo, runRepo: runRepo, resultRepo: resultRepo,
		permRepo: permRepo, llmClient: llmClient, store: store,
		promptService: promptService,
	}
}

// client 返回内部 ent 客户端
func (s *ExecutionService) client() *ent.Client { return s.entClient }

// StartSession 启动一个评估会话：
//   - 若 req 携带 SessionID（session 已由 StartRun 预创建），仅把它从 PENDING 置为 RUNNING；
//   - 若 req 携带 BatchID 但无 SessionID，从该 batch 下取下一个 PENDING 会话。
func (s *ExecutionService) StartSession(ctx context.Context, operatorID string, req model.StartExecutionRequest) (*model.SessionDetail, error) {
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EXECUTE")
	if !canEdit {
		return nil, errors.New("无权在该项目下启动评估")
	}

	// 直接领取已有 PENDING 会话
	if req.SessionID != "" {
		if sd, err := s.execRepo.GetSession(ctx, req.SessionID); err == nil && sd != nil {
			_ = s.execRepo.UpdateSession(ctx, req.SessionID, "RUNNING", nil, nil)
			sd.Status = "RUNNING"
			return sd, nil
		}
	}
	// 按 batch_id 拉取下一条 PENDING 会话
	if req.BatchID != "" {
		sessions, _ := s.execRepo.ListSessionsByBatchID(ctx, req.BatchID)
		for _, sess := range sessions {
			if sess.Status == "PENDING" {
				_ = s.execRepo.UpdateSession(ctx, sess.SessionID, "RUNNING", nil, nil)
				sess.Status = "RUNNING"
				return &sess, nil
			}
		}
		return nil, errors.New("当前批次下无可用会话")
	}

	return nil, errors.New("请提供 session_id 或 batch_id 来领取会话")
}

// ReportStep 客户端上报一轮执行结果，后端调用模型返回下一步指令
// modelType: "multimodal" 多模态（发截图给AI） 或 "text" 文本模式（发UI元素文本）
func (s *ExecutionService) ReportStep(ctx context.Context, req model.ReportStepRequest, screenshotURL string, modelChannel string, modelType string, screenshotBase64 string) (*model.ReportStepResponse, error) {
	// 获取会话信息
	session, err := s.execRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, errors.New("执行会话不存在")
	}
	if session.Status != "RUNNING" {
		return nil, errors.New("会话已结束")
	}

	// 保存步骤记录
	step, err := s.execRepo.CreateStep(ctx, req.SessionID, req)
	if err != nil {
		return nil, errors.New("保存步骤记录失败")
	}

	// 保存截图记录（截图统一通过 multipart/form-data 文件上传，screenshotURL 由 handler 预先处理）
	if screenshotURL != "" {
		_ = s.execRepo.SaveScreenshot(ctx, req.SessionID, step.ID, screenshotURL, req.StepNo <= 1)
	}

	// 如果有错误，增加错误计数
	if req.ErrorMsg != "" {
		_ = s.execRepo.IncrementErrorCount(ctx, req.SessionID)
	}

	// 获取任务和画像信息（必须从 run snapshot 获取，不读原始表）
	var task *model.TaskDetail
	var profile *model.ProfileDetail
	if session.RunID != "" {
		runID, _ := uuid.Parse(session.RunID)
		taskUUID, _ := uuid.Parse(session.TaskID)
		profileUUID, _ := uuid.Parse(session.ProfileID)
		task = s.taskFromRunSnapshot(ctx, runID, taskUUID)
		profile = s.profileFromRunSnapshot(ctx, runID, profileUUID)
	}
	if task == nil {
		return nil, errors.New("无法从 run 快照中获取任务信息")
	}
	if profile == nil {
		return nil, errors.New("无法从 run 快照中获取画像信息")
	}

	// 检查 min_steps / max_steps 规则
	if task != nil {
		// 超过最大步骤数 → 强制终止，标记为失败
		if task.MaxSteps != nil && req.StepNo > *task.MaxSteps {
			reason := fmt.Sprintf("超过最大执行步骤限制(%d步)", *task.MaxSteps)
			f := false
			_ = s.execRepo.UpdateSession(ctx, req.SessionID, "FAILED", &f, &reason)
			go s.afterSessionFinished(req.SessionID)
			action := &model.ActionInstruction{
				ActionType:     "task_state",
				ActionParam:    map[string]interface{}{},
				DecisionReason: fmt.Sprintf("已超过最大执行步骤(%d步)，任务强制终止", *task.MaxSteps),
				TaskState:      "FAILED",
				NeedContinue:   false,
			}
			return &model.ReportStepResponse{
				Action:       action,
				SessionEnded: true,
				StopReason:   reason,
			}, nil
		}
		// 超过最小步骤数（操作路径偏长，效率低）→ 记一次错误
		if task.MinSteps != nil && req.StepNo > *task.MinSteps {
			_ = s.execRepo.IncrementErrorCount(ctx, req.SessionID)
		}
	}

	// 解析模型配置：从 batch 关联的 run_model_config 获取（保证 A/B 公平）
	var mc *model.ModelConfig
	resolvedChannel := modelChannel
	if session.BatchID != "" {
		if rmc, ch, err := s.ResolveRunModelConfig(ctx, session.BatchID); err == nil {
			mc = rmc
			if resolvedChannel == "" {
				resolvedChannel = ch
			}
		}
	}
	if mc == nil {
		return nil, fmt.Errorf("无法解析模型配置，请确认 run 已正确启动")
	}

	// 获取历史步骤：若存在 batch_id，则获取同批次同画像所有 session 的步骤，以实现跨任务 AI 上下文连贯
	// 这使得同一用户画像在执行多个任务时，AI 能够感知之前任务的操作历史
	var historySteps []model.StepDetail
	if session.BatchID != "" {
		historySteps, _ = s.execRepo.ListStepsByBatchAndProfile(ctx, session.BatchID, session.ProfileID)
	} else {
		historySteps, _ = s.execRepo.ListStepsBySessionID(ctx, req.SessionID)
	}

	isMultimodal := modelType == "multimodal"

	// 根据模型类型选择不同的 system prompt 和 user prompt
	// 优先使用用户自定义的提示词，回退到硬编码默认值
	var systemPrompt string
	var userPrompt string
	if isMultimodal {
		systemPrompt = s.promptService.GetPrompt(ctx, session.ProjectID, "multimodal_system")
		userPrompt = buildMultimodalPrompt(task, profile, historySteps, req)
	} else {
		systemPrompt = s.promptService.GetPrompt(ctx, session.ProjectID, "execution_system")
		userPrompt = buildExecutionPrompt(task, profile, historySteps, req, screenshotURL)
	}

	// 构建消息列表
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	if isMultimodal && screenshotBase64 != "" {
		// 多模态模式：user 消息附带图片
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: userPrompt,
			Images:  []string{screenshotBase64},
		})
	} else {
		// 文本模式：纯文本消息
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: userPrompt,
		})
	}

	// 调用模型（注入项目级配置）
	llmResp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages:    messages,
		Channel:     resolvedChannel,
		ModelConfig: mc,
		Multimodal:  isMultimodal,
	})

	if err != nil {
		// 模型调用失败，返回空指令，并把错误信息传给前端
		errMsg := "模型调用失败: " + err.Error()
		fmt.Printf("[ERROR] %s\n", errMsg)
		_ = s.execRepo.FinishStep(ctx, step.ID, errMsg)
		return &model.ReportStepResponse{
			Action:       nil,
			SessionEnded: false,
			StopReason:   "",
			RawLLMOutput: errMsg, // 传给前端显示具体错误
		}, nil
	}

	// 解析模型返回的动作指令
	action := parseActionFromLLM(llmResp.Content)

	// 如果解析失败，在日志中打印原始输出便于调试
	if action == nil {
		_ = s.execRepo.FinishStep(ctx, step.ID, llmResp.Content)
		fmt.Printf("[WARN] 模型返回内容无法解析为有效指令，原始输出（前500字符）:\n%.500s\n---\n", llmResp.Content)
		return &model.ReportStepResponse{
			Action:       nil,
			SessionEnded: false,
			RawLLMOutput: llmResp.Content, // 传给前端调试
		}, nil
	}

	// 解析成功，保存 AI 决策（action_type、action_param）到步骤记录
	_ = s.execRepo.FinishStepWithAction(ctx, step.ID, llmResp.Content, action.ActionType, action.ActionParam)

	// 判断是否需要结束会话
	if action != nil && !action.NeedContinue {
		isCompleted := action.TaskState == "COMPLETED"
		stopReason := action.DecisionReason
		_ = s.execRepo.UpdateSession(ctx, req.SessionID, "COMPLETED", &isCompleted, &stopReason)
		// 触发会话结束后的两类问卷
		go s.afterSessionFinished(req.SessionID)
		return &model.ReportStepResponse{
			Action:       action,
			SessionEnded: true,
			StopReason:   stopReason,
		}, nil
	}

	return &model.ReportStepResponse{
		Action:       action,
		SessionEnded: false,
	}, nil
}

// afterSessionFinished 会话结束（无论何种原因）后触发任务后问卷、画像收尾问卷、run 收尾
func (s *ExecutionService) afterSessionFinished(sessionID string) {
	ctx := context.Background()
	s.TriggerAfterTaskQuestionnaires(ctx, sessionID)
	s.TriggerAfterAllPerProfileQuestionnaires(ctx, sessionID)
	// 检查 run 是否全部完成
	sd, err := s.execRepo.GetSession(ctx, sessionID)
	if err == nil && sd != nil && sd.RunID != "" {
		runID, _ := uuid.Parse(sd.RunID)
		s.FinishRunIfDone(ctx, runID)
	}
}

// FinishSession 手动结束会话
func (s *ExecutionService) FinishSession(ctx context.Context, operatorID string, req model.FinishSessionRequest) error {
	session, err := s.execRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return errors.New("会话不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "EXECUTE")
	if !canEdit {
		return errors.New("无权操作该会话")
	}
	status := "COMPLETED"
	if !req.IsGoalCompleted {
		status = "CANCELLED"
	}
	if err := s.execRepo.UpdateSession(ctx, req.SessionID, status, &req.IsGoalCompleted, &req.StopReason); err != nil {
		return err
	}
	go s.afterSessionFinished(req.SessionID)
	return nil
}

// GetSession 查询会话详情
func (s *ExecutionService) GetSession(ctx context.Context, operatorID, sessionID string) (*model.SessionDetail, error) {
	session, err := s.execRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, errors.New("会话不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该会话")
	}
	return session, nil
}

// ListSessions 查询项目下的会话列表
func (s *ExecutionService) ListSessions(ctx context.Context, operatorID, projectID string, query model.SessionListQuery) (*model.SessionListResponse, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目会话")
	}
	list, total, err := s.execRepo.ListSessionsByProjectID(ctx, projectID, query)
	if err != nil {
		return nil, errors.New("查询会话列表失败")
	}
	return &model.SessionListResponse{Total: total, List: list}, nil
}

// ListSteps 查询会话下的步骤列表
func (s *ExecutionService) ListSteps(ctx context.Context, operatorID, sessionID string) ([]model.StepDetail, error) {
	session, err := s.execRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, errors.New("会话不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该会话步骤")
	}
	return s.execRepo.ListStepsBySessionID(ctx, sessionID)
}

// UploadScreenshot 处理截图上传到Cloudreve，返回存储路径(cloudreve URI)
func (s *ExecutionService) UploadScreenshot(ctx context.Context, sessionID, filename, contentType string, data []byte) (string, error) {
	if s.store == nil {
		return "", errors.New("Cloudreve 未初始化")
	}
	reader := newBytesReader(data)
	return s.store.UploadFile(ctx, "screenshots", filename, reader, int64(len(data)), contentType)
}

// UploadRecording 处理录屏上传到Cloudreve，返回存储路径(cloudreve URI)
func (s *ExecutionService) UploadRecording(ctx context.Context, sessionID, filename, contentType string, data []byte) (string, error) {
	if s.store == nil {
		return "", errors.New("Cloudreve 未初始化")
	}
	reader := newBytesReader(data)
	storagePath, err := s.store.UploadFile(ctx, "recordings", filename, reader, int64(len(data)), contentType)
	if err != nil {
		return "", err
	}
	if err := s.execRepo.SaveRecording(ctx, sessionID, storagePath, int64(len(data))); err != nil {
		fmt.Printf("[WARN] SaveRecording failed for session %s: %v\n", sessionID, err)
	}
	return storagePath, nil
}

// GetFileURL 获取文件的临时访问 URL
func (s *ExecutionService) GetFileURL(ctx context.Context, storagePath string) (string, error) {
	if s.store == nil {
		return "", errors.New("Cloudreve 未初始化")
	}
	return s.store.GetFileURL(ctx, storagePath)
}

// ========== helpers ==========

// buildMultimodalPrompt 构建多模态模式的 user prompt（不包含UI元素文本，因为AI会看截图）
func buildMultimodalPrompt(task *model.TaskDetail, profile *model.ProfileDetail, history []model.StepDetail, current model.ReportStepRequest) string {
	prompt := "## 当前任务\n"
	if task != nil {
		prompt += fmt.Sprintf("任务名称: %s\n任务目标: %s\n完成条件: %s\n超时: %d秒\n\n",
			task.TaskName, task.TaskGoal, task.SuccessCriteria, task.TimeoutSeconds)
	}

	prompt += "## 用户画像（你需要模拟此用户的操作习惯）\n"
	if profile != nil {
		prompt += fmt.Sprintf("性别: %s, 年龄层次: %s, 教育程度: %s\n",
			profile.Gender, profile.AgeGroup, profile.EducationLevel)
		if profile.CustomFields != nil {
			dimensionLabels := map[string]string{
				"nickname": "昵称", "age": "年龄", "occupation": "职业",
				"city": "城市", "city_tier": "城市层级", "income_level": "收入水平",
				"phone_usage_years": "手机使用年限", "phone_usage_frequency": "手机使用频率",
				"personality": "性格特征", "typical_scenario": "使用场景",
				"tech_savviness": "技术熟练度",
			}
			for key, label := range dimensionLabels {
				if val, ok := profile.CustomFields[key]; ok && val != nil {
					prompt += fmt.Sprintf("%s: %v\n", label, val)
				}
			}
		}
		prompt += "\n"
	}

	if len(history) > 0 {
		prompt += fmt.Sprintf("## 历史步骤（共%d步，显示最近5步）\n", len(history))
		start := 0
		if len(history) > 5 {
			start = len(history) - 5
		}
		for _, h := range history[start:] {
			prompt += fmt.Sprintf("第%d步: 执行了 %s", h.StepNo, h.ActionType)
			if h.DecisionSummary != "" {
				desc := h.DecisionSummary
				if len(desc) > 80 {
					desc = desc[:80] + "..."
				}
				prompt += fmt.Sprintf(" | 理由: %s", desc)
			}
			prompt += "\n"
		}
		prompt += "\n"
	}

	prompt += fmt.Sprintf("## 当前界面（第 %d 步）\n", current.StepNo)
	prompt += "请观察附带的手机屏幕截图，分析当前界面状态。\n"
	if current.ErrorMsg != "" {
		prompt += fmt.Sprintf("\n⚠️ 上一步操作报错: %s\n", current.ErrorMsg)
	}

	prompt += "\n请根据截图画面和任务目标，返回下一步操作的JSON指令。根据截图中元素位置估算坐标，只返回JSON。"
	return prompt
}

func buildExecutionPrompt(task *model.TaskDetail, profile *model.ProfileDetail, history []model.StepDetail, current model.ReportStepRequest, screenshotURL string) string {
	prompt := "## 当前任务\n"
	if task != nil {
		prompt += fmt.Sprintf("任务名称: %s\n任务目标: %s\n完成条件: %s\n超时: %d秒\n\n",
			task.TaskName, task.TaskGoal, task.SuccessCriteria, task.TimeoutSeconds)
	}

	prompt += "## 用户画像（你需要模拟此用户的操作习惯）\n"
	if profile != nil {
		prompt += fmt.Sprintf("性别: %s, 年龄层次: %s, 教育程度: %s\n",
			profile.Gender, profile.AgeGroup, profile.EducationLevel)
		if profile.CustomFields != nil {
			dimensionLabels := map[string]string{
				"nickname": "昵称", "age": "年龄", "occupation": "职业",
				"city": "城市", "city_tier": "城市层级", "income_level": "收入水平",
				"phone_usage_years": "手机使用年限", "phone_usage_frequency": "手机使用频率",
				"personality": "性格特征", "typical_scenario": "使用场景",
				"tech_savviness": "技术熟练度",
			}
			for key, label := range dimensionLabels {
				if val, ok := profile.CustomFields[key]; ok && val != nil {
					prompt += fmt.Sprintf("%s: %v\n", label, val)
				}
			}
		}
		prompt += "\n"
	}

	if len(history) > 0 {
		prompt += fmt.Sprintf("## 历史步骤（共%d步，显示最近5步）\n", len(history))
		start := 0
		if len(history) > 5 {
			start = len(history) - 5
		}
		for _, h := range history[start:] {
			prompt += fmt.Sprintf("第%d步: 执行了 %s", h.StepNo, h.ActionType)
			if h.ScreenDesc != "" {
				// 只取界面描述的前100字符避免过长
				desc := h.ScreenDesc
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				prompt += fmt.Sprintf(" | 当时界面: %s", desc)
			}
			prompt += "\n"
		}
		prompt += "\n"
	}

	prompt += fmt.Sprintf("## 当前界面（第 %d 步）\n", current.StepNo)
	if current.ScreenDesc != "" {
		prompt += fmt.Sprintf("%s\n", current.ScreenDesc)
	} else {
		prompt += "（无界面信息）\n"
	}
	if current.ErrorMsg != "" {
		prompt += fmt.Sprintf("\n⚠️ 上一步操作报错: %s\n", current.ErrorMsg)
	}

	prompt += "\n请根据当前界面元素列表和任务目标，返回下一步操作的JSON指令。记住：坐标必须从上面的元素列表中获取，只返回JSON。"
	return prompt
}

func parseActionFromLLM(content string) *model.ActionInstruction {
	// 先移除模型可能输出的 <think>...</think> 思考标签（如 Qwen3.5 的默认思考模式）
	cleaned := content
	for {
		thinkStart := findIndex(cleaned, "<think>")
		if thinkStart == -1 {
			break
		}
		thinkEnd := findIndex(cleaned[thinkStart:], "</think>")
		if thinkEnd == -1 {
			// </think> 未闭合：说明输出被截断在思考块中，<think> 之后不存在有效 JSON
			// 只保留 <think> 之前的内容（通常为空），后续 JSON 提取会失败并返回 nil
			cleaned = cleaned[:thinkStart]
			break
		}
		// 移除完整的 <think>...</think> 块
		cleaned = cleaned[:thinkStart] + cleaned[thinkStart+thinkEnd+8:]
	}

	// 去除 markdown 代码块标记（有些模型会包裹 ```json ... ```）
	if idx := findIndex(cleaned, "```json"); idx != -1 {
		cleaned = cleaned[idx+7:]
		if endIdx := findIndex(cleaned, "```"); endIdx != -1 {
			cleaned = cleaned[:endIdx]
		}
	} else if idx := findIndex(cleaned, "```"); idx != -1 {
		cleaned = cleaned[idx+3:]
		if endIdx := findIndex(cleaned, "```"); endIdx != -1 {
			cleaned = cleaned[:endIdx]
		}
	}

	// 尝试提取JSON对象
	start := -1
	depth := 0
	end := -1
	for i, c := range cleaned {
		if c == '{' {
			if start == -1 {
				start = i
			}
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 && start != -1 {
				end = i
				break
			}
		}
	}
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	jsonStr := cleaned[start : end+1]

	// 将模型可能输出的分数坐标（如 115/480、430/1040）替换为对应的浮点数
	// 匹配 JSON 数值位置中的 整数/整数 形式
	jsonStr = replaceFractionCoords(jsonStr)

	var action model.ActionInstruction
	if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
		return nil
	}





	// 数据校验：确保 action_type 合法
	validTypes := map[string]bool{
		"tap": true, "input": true, "swipe": true,
		"back": true, "scroll": true, "wait": true,
	}
	// 状态类 action_type：AI 用这些表示任务状态变更（完成/失败），不是实际的 ADB 操作
	stateTypes := map[string]bool{
		"task_state": true, "complete": true, "completed": true,
		"finish": true, "done": true, "fail": true, "failed": true,
		"stop": true, "end": true, "none": true,
	}
	if stateTypes[action.ActionType] || stateTypes[strings.ToLower(action.ActionType)] {
		// 这是状态变更指令，不需要执行 ADB 操作
		// 根据 task_state 字段判断最终状态，统一 action_type 为 "task_state"
		action.ActionType = "task_state"
		// 确保 NeedContinue 为 false（既然 AI 说了状态变更）
		action.NeedContinue = false
	} else if !validTypes[action.ActionType] {
		// 尝试修正一些常见的变体
		switch action.ActionType {
		case "click", "press", "touch":
			action.ActionType = "tap"
		case "type", "enter", "text":
			action.ActionType = "input"
		case "slide", "drag":
			action.ActionType = "swipe"
		case "return", "go_back":
			action.ActionType = "back"
		default:
			return nil
		}
	}

	// 确保 action_param 不为 nil
	if action.ActionParam == nil {
		action.ActionParam = make(map[string]interface{})
	}

	return &action
}

// replaceFractionCoords 将 JSON 字符串中形如 120/1080 的分数坐标替换为数字。
// 某些模型（如 Qwen3.5）在返回坐标时会写成 "x": 120/1080 的分数形式，导致 JSON 解析失败。
// 规则：
//   - 分母 > 100 → 分子就是像素坐标（如 120/1080 → 120），直接取分子
//   - 分母 <= 100 → 这是比例分数（如 1/3 → 0.333333），做除法
var fractionRegexp = regexp.MustCompile(`\b(\d+)/(\d+)\b`)

func replaceFractionCoords(s string) string {
	return fractionRegexp.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, "/", 2)
		if len(parts) != 2 {
			return match
		}
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil || den == 0 {
			return match
		}
		// 分母 > 100 说明是 "像素/分辨率" 形式（如 120/1080），分子即像素坐标
		if den > 100 {
			return strconv.FormatFloat(num, 'f', 0, 64)
		}
		// 分母 <= 100 说明是比例分数（如 1/3），做除法
		return strconv.FormatFloat(num/den, 'f', 6, 64)
	})
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// base64DecodeScreenshot 解码 base64 截图数据（兼容带/不带 data URI 前缀的格式）
func base64DecodeScreenshot(b64 string) ([]byte, error) {
	// 移除可能的 data URI 前缀，如 "data:image/png;base64,"
	if idx := strings.Index(b64, ","); idx != -1 && idx < 100 {
		b64 = b64[idx+1:]
	}
	return base64.StdEncoding.DecodeString(b64)
}

type bytesReaderWrapper struct {
	data   []byte
	offset int
}

func newBytesReader(data []byte) *bytesReaderWrapper {
	return &bytesReaderWrapper{data: data}
}

func (r *bytesReaderWrapper) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
