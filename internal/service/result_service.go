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

type ResultService struct {
	resultRepo    *repo.ResultRepo
	execRepo      *repo.ExecutionRepo
	taskRepo      *repo.TaskRepo
	profileRepo   *repo.ProfileRepo
	projectRepo   *repo.ProjectRepo
	qRepo         *repo.QuestionnaireRepo
	permRepo      *repo.UnifiedPermRepo
	llmClient     *llm.Client
	promptService *PromptService
}

func NewResultService(
	resultRepo *repo.ResultRepo,
	execRepo *repo.ExecutionRepo,
	taskRepo *repo.TaskRepo,
	profileRepo *repo.ProfileRepo,
	projectRepo *repo.ProjectRepo,
	qRepo *repo.QuestionnaireRepo,
	permRepo *repo.UnifiedPermRepo,
	llmClient *llm.Client,
	promptService *PromptService,
) *ResultService {
	return &ResultService{
		resultRepo: resultRepo, execRepo: execRepo, taskRepo: taskRepo,
		profileRepo: profileRepo, projectRepo: projectRepo, qRepo: qRepo, permRepo: permRepo, llmClient: llmClient,
		promptService: promptService,
	}
}

// GetProjectOverview 获取项目级结果总览
func (s *ResultService) GetProjectOverview(ctx context.Context, operatorID, projectID string) (*model.ProjectResultOverview, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目结果")
	}
	return s.resultRepo.GetProjectOverview(ctx, projectID)
}

// GetSessionResult 获取单会话完整结果
func (s *ResultService) GetSessionResult(ctx context.Context, operatorID, sessionID string) (*model.SessionResult, error) {
	session, err := s.execRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, errors.New("会话不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该会话结果")
	}

	steps, _ := s.execRepo.ListStepsBySessionID(ctx, sessionID)
	answers, _ := s.resultRepo.GetAnswersBySessionID(ctx, sessionID)
	eval, _ := s.resultRepo.GetSubjectiveEval(ctx, sessionID)
	suggestions, _ := s.resultRepo.GetSuggestions(ctx, sessionID)

	return &model.SessionResult{
		Session:              *session,
		Steps:                steps,
		QuestionnaireAnswers: answers,
		SubjectiveEval:       eval,
		Suggestions:          suggestions,
	}, nil
}

// BatchGetAnswers 批量查询多个会话的问卷回答（用于 A/B 测试假设检验）
func (s *ResultService) BatchGetAnswers(ctx context.Context, operatorID, projectID string, sessionIDs []string) ([]model.AnswerDetail, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目")
	}
	if len(sessionIDs) == 0 {
		return []model.AnswerDetail{}, nil
	}
	answers, err := s.resultRepo.GetAnswersBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}
	if answers == nil {
		return []model.AnswerDetail{}, nil
	}
	return answers, nil
}

// GenerateEvaluation 为会话生成主观评价 + 改进建议（调用大模型）；问卷回答由 GenerateQuestionnaireAnswers 单独生成
func (s *ResultService) GenerateEvaluation(ctx context.Context, operatorID string, req model.GenerateEvalRequest) error {
	session, err := s.execRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return errors.New("会话不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "EXECUTE")
	if !canEdit {
		return errors.New("无权操作该会话")
	}

	// 获取任务、画像、步骤信息（全部从 run 快照取）
	var task *model.TaskDetail
	var profile *model.ProfileDetail
	if session.RunID != "" {
		snapTasks, _ := s.execRepo.ListSnapshotTasksByRunID(ctx, session.RunID)
		for _, st := range snapTasks {
			if st.TaskRef == session.TaskID {
				task = &model.TaskDetail{TaskID: st.TaskRef, TaskName: st.TaskName, TaskGoal: st.TaskGoal}
				break
			}
		}
		snapProfiles, _ := s.execRepo.ListSnapshotProfilesByRunID(ctx, session.RunID)
		for _, sp := range snapProfiles {
			if sp.ProfileRef == session.ProfileID {
				profile = &model.ProfileDetail{
					ProfileID: sp.ProfileRef, Gender: sp.Gender,
					AgeGroup: sp.AgeGroup, EducationLevel: sp.EducationLevel,
					CustomFields: sp.CustomFields,
				}
				break
			}
		}
	}
	steps, _ := s.execRepo.ListStepsBySessionID(ctx, req.SessionID)

	// 查询项目配置（大模型调用参数，需要明文 API Key）
	project, _ := s.projectRepo.FindByIDInternal(ctx, session.ProjectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}

	// 构造评估提示词
	prompt := buildEvalPrompt(task, profile, session, steps)

	// 调用模型生成评价（注入项目级配置）
	resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: s.promptService.GetPrompt(ctx, session.ProjectID, "eval_system")},
			{Role: "user", Content: prompt},
		},
		Channel:     req.ModelChannel,
		ModelConfig: mc,
	})
	if err != nil {
		return fmt.Errorf("模型调用失败: %w", err)
	}

	// 解析并保存评价结果
	return s.parseAndSaveEvalResult(ctx, session, resp.Content)
}

// GenerateQuestionnaireAnswers 第 1 轮简化：暂时不再走"任务-问卷绑定"路径。
// 任务后问卷的触发由调度侧（execution_service）按 run snapshot 在会话 FINISHED 时主动调用，
// 这里只保留接口签名以兼容老 handler，待第 2 轮完整接入。
func (s *ResultService) GenerateQuestionnaireAnswers(ctx context.Context, operatorID string, req model.GenerateEvalRequest) error {
	session, err := s.execRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return errors.New("会话不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, session.ProjectID, "EXECUTE")
	if !canEdit {
		return errors.New("无权操作该会话")
	}
	// 第 1 轮：暂留空，第 2 轮按 run snapshot 实现
	_ = req
	return nil
}

func buildQuestionnairePromptByName(task *model.TaskDetail, profile *model.ProfileDetail, session *model.SessionDetail, steps []model.StepDetail, templateName string, questions []model.QuestionDetail) string {
	var sb strings.Builder
	sb.WriteString("## 任务信息\n")
	if task != nil {
		sb.WriteString(fmt.Sprintf("任务: %s\n目标: %s\n完成条件: %s\n\n", task.TaskName, task.TaskGoal, task.SuccessCriteria))
	}
	sb.WriteString("## 用户画像\n")
	if profile != nil {
		sb.WriteString(fmt.Sprintf("年龄: %s, 教育: %s, 性别: %s\n\n", profile.AgeGroup, profile.EducationLevel, profile.Gender))
	}
	sb.WriteString("## 执行结果\n")
	sb.WriteString(fmt.Sprintf("状态: %s, 错误次数: %d, 是否完成: %v\n\n", session.Status, session.ErrorCount, session.IsGoalCompleted))
	sb.WriteString(fmt.Sprintf("## 执行步骤（共%d步）\n", len(steps)))
	for _, st := range steps {
		sb.WriteString(fmt.Sprintf("步骤%d: %s | %s\n", st.StepNo, st.ActionType, st.ScreenDesc))
	}
	sb.WriteString(fmt.Sprintf("\n## 问卷：%s\n", templateName))
	sb.WriteString("请根据以上执行过程，以该用户的视角逐题作答：\n\n")
	for _, q := range questions {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (question_id: %s)\n", q.QuestionNo, q.QuestionType, q.QuestionText, q.QuestionID))
		if len(q.OptionList) > 0 {
			sb.WriteString("   选项: ")
			for _, opt := range q.OptionList {
				for _, v := range opt {
					sb.WriteString(v + " / ")
				}
			}
			sb.WriteString("\n")
		}
		if q.ScoreRange != nil {
			if mn, ok := q.ScoreRange["min"]; ok {
				if mx, ok2 := q.ScoreRange["max"]; ok2 {
					sb.WriteString(fmt.Sprintf("   评分范围: %d ~ %d\n", mn, mx))
				}
			}
		}
	}
	return sb.String()
}

func (s *ResultService) parseAndSaveQuestionnaireAnswers(ctx context.Context, session *model.SessionDetail, taskID, templateID string, origin, sourceBindingID string, questions []model.QuestionDetail, content string) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 {
		return
	}
	jsonStr := content[start : end+1]

	var result struct {
		Answers []struct {
			QuestionID   string   `json:"question_id"`
			AnswerType   string   `json:"answer_type"`
			AnswerScore  float64  `json:"answer_score"`
			AnswerOption []string `json:"answer_option"`
			AnswerText   string   `json:"answer_text"`
		} `json:"answers"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return
	}

	for _, a := range result.Answers {
		_ = s.resultRepo.SaveAnswer(ctx,
			session.SessionID,
			session.ProfileID,
			templateID,
			a.QuestionID,
			a.AnswerType,
			origin,
			taskID,
			sourceBindingID,
			a.AnswerScore,
			a.AnswerOption,
			a.AnswerText,
		)
	}
}

// GenerateSnapshot 生成项目级结果快照
func (s *ResultService) GenerateSnapshot(ctx context.Context, operatorID, projectID string) error {
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "EDIT")
	if !canEdit {
		return errors.New("无权操作该项目")
	}
	overview, err := s.resultRepo.GetProjectOverview(ctx, projectID)
	if err != nil {
		return err
	}
	return s.resultRepo.SaveResultSnapshot(ctx, projectID, overview)
}

// filterSessionsByIDs 过滤会话列表，若 sessionIDs 为空则返回全部
func filterSessionsByIDs(all []model.SessionDetail, sessionIDs []string) []model.SessionDetail {
	if len(sessionIDs) == 0 {
		return all
	}
	idSet := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		idSet[id] = struct{}{}
	}
	result := make([]model.SessionDetail, 0, len(sessionIDs))
	for _, sess := range all {
		if _, ok := idSet[sess.SessionID]; ok {
			result = append(result, sess)
		}
	}
	return result
}

// computeOverview 基于 sessions 列表在内存中计算总览统计
func computeOverview(projectID string, sessions []model.SessionDetail) model.ProjectResultOverview {
	o := model.ProjectResultOverview{ProjectID: projectID, TotalSessions: len(sessions)}
	var totalDuration int64
	var totalErrors int
	durationCount := 0
	for _, sess := range sessions {
		switch sess.Status {
		case "COMPLETED":
			o.CompletedCount++
		case "FAILED", "TIMEOUT":
			o.FailedCount++
		}
		totalErrors += sess.ErrorCount
		if sess.TotalDurationMs != nil {
			totalDuration += *sess.TotalDurationMs
			durationCount++
		}
	}
	if o.TotalSessions > 0 {
		o.CompletionRate = float64(o.CompletedCount) / float64(o.TotalSessions) * 100
		o.AvgErrorCount = float64(totalErrors) / float64(o.TotalSessions)
	}
	if durationCount > 0 {
		o.AvgDurationMs = totalDuration / int64(durationCount)
	}
	return o
}

// aggregateReportData 聚合 task 统计、问卷统计，被报告方法共用。
// 以 runID 为锚点，题目信息完全从快照表获取，不再依赖原始问卷表。
func (s *ResultService) aggregateReportData(ctx context.Context, runID string, sessions []model.SessionDetail) (
	taskStatMap map[string]*model.TaskStat,
	taskStats []model.TaskStat,
	qStatGroupsResult []model.QuestionnaireStat,
) {
	// 预加载 run 快照的任务名和画像标签（一次查询，后续全用缓存）
	taskNameCache := map[string]string{}
	taskGoalCache := map[string]string{}
	snapTasks, _ := s.execRepo.ListSnapshotTasksByRunID(ctx, runID)
	for _, st := range snapTasks {
		taskNameCache[st.TaskRef] = st.TaskName
		taskGoalCache[st.TaskRef] = st.TaskGoal
	}
	profileLabelCache := map[string]string{}
	profileNickCache := map[string]string{}
	snapProfiles, _ := s.execRepo.ListSnapshotProfilesByRunID(ctx, runID)
	for _, sp := range snapProfiles {
		profileLabelCache[sp.ProfileRef] = fmt.Sprintf("%s·%s·%s", sp.Gender, sp.AgeGroup, sp.EducationLevel)
		profileNickCache[sp.ProfileRef] = sp.NickName
	}

	// 1. 按任务聚合统计
	taskStatMap = map[string]*model.TaskStat{}
	sessionIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sessionIDs = append(sessionIDs, sess.SessionID)
		ts, ok := taskStatMap[sess.TaskID]
		if !ok {
			taskName := taskNameCache[sess.TaskID]
			if taskName == "" {
				taskName = sess.TaskID
			}
			ts = &model.TaskStat{TaskID: sess.TaskID, TaskName: taskName}
			taskStatMap[sess.TaskID] = ts
		}
		ts.TotalSessions++
		ts.TotalErrors += sess.ErrorCount
		if sess.IsGoalCompleted {
			ts.SuccessCount++
		}
		if sess.Status == "FAILED" || sess.Status == "TIMEOUT" {
			ts.FailedCount++
		}
		if sess.TotalDurationMs != nil {
			ts.AvgDurationMs += *sess.TotalDurationMs
		}
		ts.AvgStepCount += float64(sess.StepCount)
	}
	taskStats = make([]model.TaskStat, 0, len(taskStatMap))
	for _, ts := range taskStatMap {
		if ts.TotalSessions > 0 {
			ts.AvgErrors = float64(ts.TotalErrors) / float64(ts.TotalSessions)
			ts.AvgDurationMs = ts.AvgDurationMs / int64(ts.TotalSessions)
			ts.CompletionRate = float64(ts.SuccessCount) / float64(ts.TotalSessions) * 100
			ts.AvgStepCount = ts.AvgStepCount / float64(ts.TotalSessions)
		}
		taskStats = append(taskStats, *ts)
	}

	// 2. 查询所有问卷回答
	allAnswers, _ := s.resultRepo.GetAnswersBySessionIDs(ctx, sessionIDs)

	// 3. 按 templateID+groupID 聚合问卷统计
	type answerGroup struct {
		templateID string
		groupBy    string
		groupID    string
		answers    []model.AnswerDetail
	}
	groupMap := map[string]*answerGroup{}
	for _, ans := range allAnswers {
		groupBy := "profile"
		groupID := ans.ProfileID
		if ans.AnswerOrigin == "AFTER_TASK" && ans.TaskID != "" {
			groupBy = "task"
			groupID = ans.TaskID
		}
		key := ans.TemplateID + "|" + groupID
		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &answerGroup{templateID: ans.TemplateID, groupBy: groupBy, groupID: groupID}
		}
		groupMap[key].answers = append(groupMap[key].answers, ans)
	}

	// 4. 对每组计算题目统计
	type qStatKey struct{ templateID, groupID, questionID string }
	qStatsCache := map[qStatKey]*model.QuestionStat{}
	for _, group := range groupMap {
		for _, ans := range group.answers {
			k := qStatKey{group.templateID, group.groupID, ans.QuestionID}
			qs, ok := qStatsCache[k]
			if !ok {
				qs = &model.QuestionStat{QuestionID: ans.QuestionID, QuestionType: ans.AnswerType, OptionCounts: map[string]int{}}
				qStatsCache[k] = qs
			}
			qs.AnswerCount++
			switch ans.AnswerType {
			case "SCALE":
				qs.AvgScore += ans.AnswerScore
				if qs.AnswerCount == 1 || ans.AnswerScore < qs.MinScore {
					qs.MinScore = ans.AnswerScore
				}
				if ans.AnswerScore > qs.MaxScore {
					qs.MaxScore = ans.AnswerScore
				}
			case "SINGLE_CHOICE", "MULTIPLE_CHOICE":
				for _, v := range ans.AnswerOption {
					qs.OptionCounts[fmt.Sprintf("%v", v)]++
				}
			case "OPEN_ENDED":
				if ans.AnswerText != "" {
					qs.TextAnswers = append(qs.TextAnswers, ans.AnswerText)
				}
			}
		}
	}
	for _, qs := range qStatsCache {
		if qs.AnswerCount > 0 && qs.QuestionType == "SCALE" {
			qs.AvgScore = qs.AvgScore / float64(qs.AnswerCount)
		}
	}

	// 5. 加载 run 快照的模板名（用于显示），缓存一次
	templateNameCache := map[string]string{}
	snapTemplates, _ := s.execRepo.ListSnapshotTemplatesByRunID(ctx, runID)
	for _, st := range snapTemplates {
		templateNameCache[st.TemplateRef] = st.TemplateName
	}

	// 6. 组装 QuestionnaireStat —— 题目元信息从快照表获取
	qStatGroupsResult = make([]model.QuestionnaireStat, 0)
	for _, group := range groupMap {
		tmplName := group.templateID
		if name, ok := templateNameCache[group.templateID]; ok {
			tmplName = name
		}
		groupLabel := group.groupID
		if group.groupBy == "task" {
			if ts, ok := taskStatMap[group.groupID]; ok {
				groupLabel = ts.TaskName
			}
		} else {
			if label, ok := profileLabelCache[group.groupID]; ok {
				groupLabel = label
			}
		}
		// 从快照表取题目（以 runID + template_id_ref 为键）
		questions, _ := s.execRepo.ListSnapshotQuestionsByRunAndTemplate(ctx, runID, group.templateID)
		qStatsForGroup := make([]model.QuestionStat, 0)
		for _, q := range questions {
			k := qStatKey{group.templateID, group.groupID, q.QuestionID}
			if qs, ok := qStatsCache[k]; ok {
				qs.QuestionNo = q.QuestionNo
				qs.QuestionText = q.QuestionText
				qs.DimensionCode = q.DimensionCode
				qStatsForGroup = append(qStatsForGroup, *qs)
			}
		}
		qStatGroupsResult = append(qStatGroupsResult, model.QuestionnaireStat{
			TemplateID:   group.templateID,
			TemplateName: tmplName,
			GroupBy:      group.groupBy,
			GroupLabel:   groupLabel,
			GroupID:      group.groupID,
			TotalAnswers: len(group.answers),
			Questions:    qStatsForGroup,
		})
	}
	return
}

// GetProjectReportStats 获取项目统计数据（不调用 AI，快速响应）
func (s *ResultService) GetProjectReportStats(ctx context.Context, operatorID string, req model.ReportStatsRequest) (*model.ProjectReportStats, error) {
	// 1. 通过 run_id 反查 project_id 做权限校验
	projectID, err := s.execRepo.GetRunProjectID(ctx, req.RunID)
	if err != nil {
		return nil, errors.New("run 不存在")
	}
	req.ProjectID = projectID
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目报告")
	}

	// 2. 按 run_id 查会话（可选 session_ids 进一步筛选）
	allSessions, _ := s.execRepo.ListSessionsByRunID(ctx, req.RunID)
	sessions := filterSessionsByIDs(allSessions, req.SessionIDs)

	// 3. 总览（基于筛选后的 sessions 计算）
	overview := computeOverview(projectID, sessions)

	// 4. 聚合 task + 问卷统计（从快照取题目）
	_, taskStats, qStatGroupsResult := s.aggregateReportData(ctx, req.RunID, sessions)

	// 5. 按画像聚合（从快照缓存取标签）
	snapProfiles, _ := s.execRepo.ListSnapshotProfilesByRunID(ctx, req.RunID)
	profileLabelMap := map[string]string{}
	profileNickMap := map[string]string{}
	for _, sp := range snapProfiles {
		profileLabelMap[sp.ProfileRef] = fmt.Sprintf("%s·%s·%s", sp.Gender, sp.AgeGroup, sp.EducationLevel)
		profileNickMap[sp.ProfileRef] = sp.NickName
	}
	profileStatMap := map[string]*model.ProfileStat{}
	for _, sess := range sessions {
		ps, ok := profileStatMap[sess.ProfileID]
		if !ok {
		label := profileLabelMap[sess.ProfileID]
		if label == "" {
			label = sess.ProfileID
		}
		nickName := profileNickMap[sess.ProfileID]
		ps = &model.ProfileStat{ProfileID: sess.ProfileID, ProfileLabel: label, NickName: nickName}
			profileStatMap[sess.ProfileID] = ps
		}
		ps.TotalSessions++
		if sess.IsGoalCompleted {
			ps.CompletedCount++
		}
		if sess.Status == "FAILED" || sess.Status == "TIMEOUT" {
			ps.FailedCount++
		}
		ps.AvgErrors += float64(sess.ErrorCount)
		if sess.TotalDurationMs != nil {
			ps.AvgDurationMs += *sess.TotalDurationMs
		}
		ps.AvgStepCount += float64(sess.StepCount)
	}
	profileStats := make([]model.ProfileStat, 0, len(profileStatMap))
	for _, ps := range profileStatMap {
		if ps.TotalSessions > 0 {
			ps.AvgErrors /= float64(ps.TotalSessions)
			ps.AvgDurationMs /= int64(ps.TotalSessions)
			ps.AvgStepCount /= float64(ps.TotalSessions)
		}
		profileStats = append(profileStats, *ps)
	}

	// 6. 拆分问卷（AFTER_TASK vs AFTER_ALL）
	taskQuestStats := make([]model.QuestionnaireStat, 0)
	globalQuestStats := make([]model.QuestionnaireStat, 0)
	for _, qs := range qStatGroupsResult {
		if qs.GroupBy == "task" {
			taskQuestStats = append(taskQuestStats, qs)
		} else {
			globalQuestStats = append(globalQuestStats, qs)
		}
	}

	return &model.ProjectReportStats{
		ProjectID:        projectID,
		Overview:         overview,
		TaskStats:        taskStats,
		ProfileStats:     profileStats,
		TaskQuestStats:   taskQuestStats,
		GlobalQuestStats: globalQuestStats,
	}, nil
}

// GetLatestAIReport 获取项目最新的已保存 AI 分析报告
func (s *ResultService) GetLatestAIReport(ctx context.Context, operatorID, projectID string) (*model.AIReportResult, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目报告")
	}
	return s.resultRepo.GetLatestAIReport(ctx, projectID)
}

// GetProjectReport 获取项目总体报告（聚合所有会话的任务统计、问卷统计、AI总结）
func (s *ResultService) GetProjectReport(ctx context.Context, operatorID string, req model.GenerateReportRequest) (*model.ProjectReport, error) {
	// 1. 通过 run_id 反查 project_id 做权限校验
	projectID, err := s.execRepo.GetRunProjectID(ctx, req.RunID)
	if err != nil {
		return nil, errors.New("run 不存在")
	}
	req.ProjectID = projectID
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目报告")
	}

	// 2. 按 run_id 查会话
	allSessions, _ := s.execRepo.ListSessionsByRunID(ctx, req.RunID)
	sessions := filterSessionsByIDs(allSessions, req.SessionIDs)

	// 3. 总览
	overview := computeOverview(projectID, sessions)

	// 4. 聚合统计（从快照取题目）
	_, taskStats, qStatGroupsResult := s.aggregateReportData(ctx, req.RunID, sessions)

	// 5. AI 总结
	aiSummary, aiStrengths, aiWeaknesses, aiRecommendations := s.generateAISummary(ctx, req, taskStats, qStatGroupsResult, &overview)

	// 6. 持久化保存 AI 分析结果（异步写库，不阻塞响应）
	go func() {
		_ = s.resultRepo.SaveAIReport(context.Background(), projectID, req.ModelChannel, req.SessionIDs,
			aiSummary, aiStrengths, aiWeaknesses, aiRecommendations)
	}()

	return &model.ProjectReport{
		ProjectID:          projectID,
		Overview:           overview,
		TaskStats:          taskStats,
		QuestionnaireStats: qStatGroupsResult,
		AISummary:          aiSummary,
		AIStrengths:        aiStrengths,
		AIWeaknesses:       aiWeaknesses,
		AIRecommendations:  aiRecommendations,
	}, nil
}

func (s *ResultService) generateAISummary(ctx context.Context, req model.GenerateReportRequest, taskStats []model.TaskStat, qStats []model.QuestionnaireStat, overview *model.ProjectResultOverview) (summary string, strengths, weaknesses, recommendations []string) {
	project, _ := s.projectRepo.FindByIDInternal(ctx, req.ProjectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 项目评估结果\n总会话: %d, 完成: %d, 失败: %d, 完成率: %.1f%%, 平均错误: %.1f\n\n",
		overview.TotalSessions, overview.CompletedCount, overview.FailedCount, overview.CompletionRate, overview.AvgErrorCount))
	sb.WriteString("## 各任务情况\n")
	for _, ts := range taskStats {
		sb.WriteString(fmt.Sprintf("- 「%s」: 总%d, 成功%d, 失败%d, 错误%d, 完成率%.1f%%\n",
			ts.TaskName, ts.TotalSessions, ts.SuccessCount, ts.FailedCount, ts.TotalErrors, ts.CompletionRate))
	}
	sb.WriteString("\n## 问卷统计\n")
	for _, qs := range qStats {
		sb.WriteString(fmt.Sprintf("### %s（%s:%s）\n", qs.TemplateName, qs.GroupBy, qs.GroupLabel))
		for _, q := range qs.Questions {
			sb.WriteString(fmt.Sprintf("  题%d[%s] %s: ", q.QuestionNo, q.QuestionType, q.QuestionText))
			if q.QuestionType == "SCALE" {
				sb.WriteString(fmt.Sprintf("均%.1f(%.1f~%.1f)\n", q.AvgScore, q.MinScore, q.MaxScore))
			} else if q.QuestionType == "OPEN_ENDED" {
				sb.WriteString(fmt.Sprintf("%d条回答\n", len(q.TextAnswers)))
			} else {
				for opt, cnt := range q.OptionCounts {
					sb.WriteString(fmt.Sprintf("%s×%d ", opt, cnt))
				}
				sb.WriteString("\n")
			}
		}
	}
	resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: s.promptService.GetPrompt(ctx, req.ProjectID, "report_system")},
			{Role: "user", Content: sb.String() + "\n请生成总体报告。"},
		},
		Channel:     req.ModelChannel,
		ModelConfig: mc,
	})
	if err != nil {
		return "AI总结生成失败: " + err.Error(), nil, nil, nil
	}
	start := strings.Index(resp.Content, "{")
	end := strings.LastIndex(resp.Content, "}")
	if start == -1 || end == -1 {
		return resp.Content, nil, nil, nil
	}
	var result struct {
		Summary         string   `json:"summary"`
		Strengths       []string `json:"strengths"`
		Weaknesses      []string `json:"weaknesses"`
		Recommendations []string `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(resp.Content[start:end+1]), &result); err != nil {
		return resp.Content, nil, nil, nil
	}
	return result.Summary, result.Strengths, result.Weaknesses, result.Recommendations
}

// ========== helpers ==========

func buildEvalPrompt(task *model.TaskDetail, profile *model.ProfileDetail, session *model.SessionDetail, steps []model.StepDetail) string {
	var sb strings.Builder
	sb.WriteString("## 任务信息\n")
	if task != nil {
		sb.WriteString(fmt.Sprintf("任务: %s\n目标: %s\n完成条件: %s\n\n", task.TaskName, task.TaskGoal, task.SuccessCriteria))
	}
	sb.WriteString("## 用户画像\n")
	if profile != nil {
		sb.WriteString(fmt.Sprintf("年龄: %s, 教育: %s, 性别: %s\n\n", profile.AgeGroup, profile.EducationLevel, profile.Gender))
	}
	sb.WriteString("## 执行结果\n")
	sb.WriteString(fmt.Sprintf("状态: %s, 错误次数: %d, 是否完成: %v\n\n", session.Status, session.ErrorCount, session.IsGoalCompleted))
	sb.WriteString(fmt.Sprintf("## 执行步骤（共%d步）\n", len(steps)))
	for _, st := range steps {
		sb.WriteString(fmt.Sprintf("步骤%d: %s | %s", st.StepNo, st.ActionType, st.ScreenDesc))
		if st.ErrorMessage != "" {
			sb.WriteString(fmt.Sprintf(" [错误: %s]", st.ErrorMessage))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n请基于以上执行过程，生成主观评价和改进建议。")
	return sb.String()
}

func (s *ResultService) parseAndSaveEvalResult(ctx context.Context, session *model.SessionDetail, content string) error {
	// 提取JSON
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 {
		return errors.New("模型返回结果解析失败")
	}
	jsonStr := content[start : end+1]

	var result struct {
		OverallScore float64 `json:"overall_score"`
		SummaryText  string  `json:"summary_text"`
		Suggestions  []struct {
			SuggestionType string `json:"suggestion_type"`
			PriorityLevel  string `json:"priority_level"`
			SuggestionText string `json:"suggestion_text"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fmt.Errorf("评价结果JSON解析失败: %w", err)
	}

	// 保存主观评价
	_ = s.resultRepo.SaveSubjectiveEval(ctx, session.SessionID, result.OverallScore, result.SummaryText, nil)

	// 保存改进建议
	for _, sug := range result.Suggestions {
		_ = s.resultRepo.SaveSuggestion(ctx, session.SessionID, sug.SuggestionType, sug.PriorityLevel, sug.SuggestionText)
	}

	return nil
}

// GenerateHTMLReport 生成 HTML 可视化报告（调用 AI 生成完整 HTML 页面）
func (s *ResultService) GenerateHTMLReport(ctx context.Context, operatorID string, req model.GenerateHTMLReportRequest) (*model.HTMLReportResponse, error) {
	// 1. 通过 run_id 反查 project_id 做权限校验
	projectID, err := s.execRepo.GetRunProjectID(ctx, req.RunID)
	if err != nil {
		return nil, errors.New("run 不存在")
	}
	req.ProjectID = projectID
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目报告")
	}

	// 2. 获取项目信息（需要明文 API Key 用于 LLM 调用）
	project, _ := s.projectRepo.FindByIDInternal(ctx, projectID)
	var mc *model.ModelConfig
	if project != nil {
		mc = project.ModelConfig
	}

	// 3. 按 run_id 查会话
	allSessions, _ := s.execRepo.ListSessionsByRunID(ctx, req.RunID)
	sessions := filterSessionsByIDs(allSessions, req.SessionIDs)

	// 4. 总览
	overview := computeOverview(projectID, sessions)

	// 5. 聚合 task + 问卷统计（从快照取题目）
	_, taskStats, qStatGroupsResult := s.aggregateReportData(ctx, req.RunID, sessions)

	// 5. 画像统计（从快照取标签）
	snapProfiles2, _ := s.execRepo.ListSnapshotProfilesByRunID(ctx, req.RunID)
	profLabelMap := map[string]string{}
	profNickMap := map[string]string{}
	for _, sp := range snapProfiles2 {
		profLabelMap[sp.ProfileRef] = fmt.Sprintf("%s·%s·%s", sp.Gender, sp.AgeGroup, sp.EducationLevel)
		profNickMap[sp.ProfileRef] = sp.NickName
	}
	profileStatMap := map[string]*model.ProfileStat{}
	for _, sess := range sessions {
		ps, ok := profileStatMap[sess.ProfileID]
		if !ok {
			label := profLabelMap[sess.ProfileID]
			if label == "" {
				label = sess.ProfileID
			}
			nickName := profNickMap[sess.ProfileID]
			ps = &model.ProfileStat{ProfileID: sess.ProfileID, ProfileLabel: label, NickName: nickName}
			profileStatMap[sess.ProfileID] = ps
		}
		ps.TotalSessions++
		if sess.IsGoalCompleted {
			ps.CompletedCount++
		}
		if sess.Status == "FAILED" || sess.Status == "TIMEOUT" {
			ps.FailedCount++
		}
		ps.AvgErrors += float64(sess.ErrorCount)
		if sess.TotalDurationMs != nil {
			ps.AvgDurationMs += *sess.TotalDurationMs
		}
		ps.AvgStepCount += float64(sess.StepCount)
	}
	profileStats := make([]model.ProfileStat, 0, len(profileStatMap))
	for _, ps := range profileStatMap {
		if ps.TotalSessions > 0 {
			ps.AvgErrors /= float64(ps.TotalSessions)
			ps.AvgDurationMs /= int64(ps.TotalSessions)
			ps.AvgStepCount /= float64(ps.TotalSessions)
		}
		profileStats = append(profileStats, *ps)
	}

	// 6. 获取已有的 AI 分析结果
	aiReport, _ := s.resultRepo.GetLatestAIReport(ctx, req.ProjectID)

	// 7. 获取任务详情（从快照取名称+目标）
	type taskInfo struct {
		Name string
		Goal string
	}
	taskInfoMap := map[string]taskInfo{}
	snapTasks, _ := s.execRepo.ListSnapshotTasksByRunID(ctx, req.RunID)
	for _, st := range snapTasks {
		taskInfoMap[st.TaskRef] = taskInfo{Name: st.TaskName, Goal: st.TaskGoal}
	}

	// 8. 构造完整的数据摘要给 AI
	var sb strings.Builder
	sb.WriteString("# UX 评估报告数据摘要\n\n")

	// 项目基本信息
	sb.WriteString("## 应用基本信息\n")
	if project != nil {
		sb.WriteString(fmt.Sprintf("- 项目名称: %s\n- 应用名称: %s\n- 应用版本: %s\n- 研究目标: %s\n- 项目描述: %s\n",
			project.ProjectName, project.AppName, project.AppVersion, project.ResearchGoal, project.ProjectDesc))
	}
	sb.WriteString(fmt.Sprintf("- 评估会话数: %d\n- 评估日期: %s\n\n", len(sessions), "2026年4月"))

	// 总览
	sb.WriteString("## 总览\n")
	sb.WriteString(fmt.Sprintf("总会话: %d, 完成: %d, 失败: %d, 完成率: %.1f%%, 平均错误: %.1f, 平均耗时: %dms\n\n",
		overview.TotalSessions, overview.CompletedCount, overview.FailedCount, overview.CompletionRate, overview.AvgErrorCount, overview.AvgDurationMs))

	// 任务维度
	sb.WriteString("## 任务维度 - 客观数据\n")
	for _, ts := range taskStats {
		info := taskInfoMap[ts.TaskID]
		sb.WriteString(fmt.Sprintf("### 任务「%s」\n", ts.TaskName))
		sb.WriteString(fmt.Sprintf("- 目标: %s\n", info.Goal))
		sb.WriteString(fmt.Sprintf("- 总会话: %d, 成功: %d, 失败: %d, 完成率: %.1f%%\n", ts.TotalSessions, ts.SuccessCount, ts.FailedCount, ts.CompletionRate))
		sb.WriteString(fmt.Sprintf("- 总错误: %d, 平均错误: %.1f, 平均耗时: %dms, 平均步骤: %.1f\n\n", ts.TotalErrors, ts.AvgErrors, ts.AvgDurationMs, ts.AvgStepCount))
	}

	// 任务维度 - 问卷数据
	taskQuestStats := make([]model.QuestionnaireStat, 0)
	globalQuestStats := make([]model.QuestionnaireStat, 0)
	for _, qs := range qStatGroupsResult {
		if qs.GroupBy == "task" {
			taskQuestStats = append(taskQuestStats, qs)
		} else {
			globalQuestStats = append(globalQuestStats, qs)
		}
	}

	if len(taskQuestStats) > 0 {
		sb.WriteString("## 任务维度 - 问卷数据\n")
		for _, qs := range taskQuestStats {
			sb.WriteString(fmt.Sprintf("### 问卷「%s」（任务: %s，%d份回答）\n", qs.TemplateName, qs.GroupLabel, qs.TotalAnswers))
			for _, q := range qs.Questions {
				sb.WriteString(fmt.Sprintf("- Q%d [%s] %s: ", q.QuestionNo, q.QuestionType, q.QuestionText))
				if q.QuestionType == "SCALE" {
					sb.WriteString(fmt.Sprintf("均分%.1f (%.1f~%.1f)\n", q.AvgScore, q.MinScore, q.MaxScore))
				} else if q.QuestionType == "OPEN_ENDED" {
					sb.WriteString(fmt.Sprintf("%d条回答: %s\n", len(q.TextAnswers), strings.Join(q.TextAnswers, "; ")))
				} else {
					for opt, cnt := range q.OptionCounts {
						sb.WriteString(fmt.Sprintf("%s×%d ", opt, cnt))
					}
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	// 画像维度
	if len(profileStats) > 0 {
		sb.WriteString("## 画像维度 - 客观数据\n")
		for _, ps := range profileStats {
			name := ps.NickName
			if name == "" {
				name = ps.ProfileLabel
			}
			completionRate := float64(0)
			if ps.TotalSessions > 0 {
				completionRate = float64(ps.CompletedCount) / float64(ps.TotalSessions) * 100
			}
			sb.WriteString(fmt.Sprintf("### 画像「%s」(%s)\n", name, ps.ProfileLabel))
			sb.WriteString(fmt.Sprintf("- 总会话: %d, 完成: %d, 失败: %d, 完成率: %.1f%%\n", ps.TotalSessions, ps.CompletedCount, ps.FailedCount, completionRate))
			sb.WriteString(fmt.Sprintf("- 平均错误: %.1f, 平均耗时: %dms, 平均步骤: %.1f\n\n", ps.AvgErrors, ps.AvgDurationMs, ps.AvgStepCount))
		}
	}

	if len(globalQuestStats) > 0 {
		sb.WriteString("## 画像维度 - 问卷数据\n")
		for _, qs := range globalQuestStats {
			sb.WriteString(fmt.Sprintf("### 问卷「%s」（画像: %s，%d份回答）\n", qs.TemplateName, qs.GroupLabel, qs.TotalAnswers))
			for _, q := range qs.Questions {
				sb.WriteString(fmt.Sprintf("- Q%d [%s] %s: ", q.QuestionNo, q.QuestionType, q.QuestionText))
				if q.QuestionType == "SCALE" {
					sb.WriteString(fmt.Sprintf("均分%.1f (%.1f~%.1f)\n", q.AvgScore, q.MinScore, q.MaxScore))
				} else if q.QuestionType == "OPEN_ENDED" {
					sb.WriteString(fmt.Sprintf("%d条回答: %s\n", len(q.TextAnswers), strings.Join(q.TextAnswers, "; ")))
				} else {
					for opt, cnt := range q.OptionCounts {
						sb.WriteString(fmt.Sprintf("%s×%d ", opt, cnt))
					}
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	// AI 分析结果
	sb.WriteString("## AI 综合分析\n")
	if aiReport != nil {
		sb.WriteString(fmt.Sprintf("### 总体评价\n%s\n\n", aiReport.AISummary))
		if len(aiReport.AIStrengths) > 0 {
			sb.WriteString("### 做得好的地方\n")
			for _, s := range aiReport.AIStrengths {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
			}
			sb.WriteString("\n")
		}
		if len(aiReport.AIWeaknesses) > 0 {
			sb.WriteString("### 存在的问题\n")
			for _, s := range aiReport.AIWeaknesses {
				sb.WriteString(fmt.Sprintf("- %s\n", s))
			}
			sb.WriteString("\n")
		}
		if len(aiReport.AIRecommendations) > 0 {
			sb.WriteString("### 改进建议\n")
			for i, s := range aiReport.AIRecommendations {
				priority := "低"
				ratio := float64(i) / float64(len(aiReport.AIRecommendations))
				if ratio < 0.34 {
					priority = "高"
				} else if ratio < 0.67 {
					priority = "中"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", priority, s))
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("暂无 AI 分析数据。请根据客观数据和问卷数据自行分析总结。\n\n")
	}

	sb.WriteString("\n请根据以上全部数据，生成一份完整的、独立的、图文并茂的 HTML 可视化报告页面。")

	// 9. 调用 AI 生成 HTML
	resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: s.promptService.GetPrompt(ctx, req.ProjectID, "html_report_system")},
			{Role: "user", Content: sb.String()},
		},
		Channel:     req.ModelChannel,
		ModelConfig: mc,
	})
	if err != nil {
		return nil, fmt.Errorf("AI 生成 HTML 报告失败: %w", err)
	}

	// 10. 提取 HTML 内容（去除可能的 markdown 代码块包裹）
	html := resp.Content
	html = strings.TrimSpace(html)
	if strings.HasPrefix(html, "```html") {
		html = strings.TrimPrefix(html, "```html")
		if idx := strings.LastIndex(html, "```"); idx != -1 {
			html = html[:idx]
		}
	} else if strings.HasPrefix(html, "```") {
		html = strings.TrimPrefix(html, "```")
		if idx := strings.LastIndex(html, "```"); idx != -1 {
			html = html[:idx]
		}
	}
	html = strings.TrimSpace(html)

	// 11. 异步保存 HTML 报告到数据库
	go func() {
		_ = s.resultRepo.SaveHTMLReport(context.Background(), req.ProjectID, req.ModelChannel, html, req.SessionIDs)
	}()

	return &model.HTMLReportResponse{HTML: html}, nil
}

// GetLatestHTMLReport 获取项目最新的已保存 HTML 可视化报告
func (s *ResultService) GetLatestHTMLReport(ctx context.Context, operatorID, projectID string) (*model.HTMLReportResult, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目报告")
	}
	return s.resultRepo.GetLatestHTMLReport(ctx, projectID)
}
