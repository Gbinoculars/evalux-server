package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxexecutionbatch"
	"evalux-server/ent/uxexecutionplan"
	"evalux-server/ent/uxexecutionsession"
	"evalux-server/ent/uxexecutionrun"
	"evalux-server/ent/uxplanmodelconfig"
	"evalux-server/ent/uxplanprofilebinding"
	"evalux-server/ent/uxplanprofilequestionnairebinding"
	"evalux-server/ent/uxplantaskbinding"
	"evalux-server/ent/uxplantaskquestionnairebinding"
	"evalux-server/ent/uxquestionnairequestion"
	"evalux-server/ent/uxquestionnairetemplate"
	"evalux-server/ent/uxrunmodelconfig"
	"evalux-server/ent/uxrunprofilequestionnairesnapshot"
	"evalux-server/ent/uxrunprofilesnapshot"
	"evalux-server/ent/uxrunquestionnaireoptionsnapshot"
	"evalux-server/ent/uxrunquestionnairequestionsnapshot"
	"evalux-server/ent/uxrunquestionnairetemplatesnapshot"
	"evalux-server/ent/uxruntaskquestionnairesnapshot"
	"evalux-server/ent/uxruntasksnapshot"
	"evalux-server/ent/uxuserprofile"
	"evalux-server/internal/llm"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

// StartRun 用户点"启动评估"，按 plan 在事务里写入 run + 8 张子快照表 + batch + sessions。
// 调度模型：本方法只把"配置侧"全部冻结到 run 子表，并准备好 sessions=PENDING；
// 客户端后续通过 /api/executions/start 拉取下一条 PENDING 会话开始执行。
func (s *ExecutionService) StartRun(ctx context.Context, operatorID, planID string) (*model.ExecutionRunDetail, error) {
	uid, err := uuid.Parse(operatorID)
	if err != nil {
		return nil, errors.New("无效的用户 ID")
	}
	planUUID, err := uuid.Parse(planID)
	if err != nil {
		return nil, errors.New("无效的计划 ID")
	}

	// 权限校验
	plan, err := s.client().UxExecutionPlan.Get(ctx, planUUID)
	if err != nil {
		return nil, errors.New("执行计划不存在")
	}
	canExec, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, plan.ProjectID.String(), "EXECUTE")
	if !canExec {
		return nil, errors.New("无权启动该项目下的评估")
	}
	if plan.Status != uxexecutionplan.StatusREADY {
		return nil, errors.New("计划已归档，无法启动")
	}

	// 读全部计划绑定（配置层）
	planMCs, err := s.client().UxPlanModelConfig.Query().
		Where(uxplanmodelconfig.PlanID(planUUID)).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(planMCs) == 0 {
		return nil, errors.New("计划未配置任何模型")
	}
	if plan.PlanType == uxexecutionplan.PlanTypeAB_TEST && len(planMCs) < 2 {
		return nil, errors.New("A/B 测试计划必须配置 CONTROL 与 TREATMENT 两份模型")
	}

	// 关键变更：plan 层只存"渠道引用"。从 project.model_config 解析出真实的
	// model_name / api_base_url / api_key（明文），稍后写入 run_model_config。
	projectDetail, err := s.projectRepo.FindByIDInternal(ctx, plan.ProjectID.String())
	if err != nil || projectDetail == nil || projectDetail.ModelConfig == nil {
		return nil, errors.New("项目模型配置缺失，无法启动评估，请先在'项目配置'中填写模型连接信息")
	}
	resolvedMC := make(map[string]struct {
		ModelName  string
		APIBaseURL string
		APIKey     string
	}, len(planMCs))
	for _, m := range planMCs {
		mn, base, key, ok := resolveChannelFromProject(projectDetail.ModelConfig, m.Channel)
		if !ok {
			return nil, fmt.Errorf("项目模型配置中未填写 [%s] 渠道的连接信息（model/base_url/api_key），请在'项目配置 → 模型配置'中补全后再启动评估", m.Channel)
		}
		resolvedMC[string(m.ModelRole)] = struct {
			ModelName  string
			APIBaseURL string
			APIKey     string
		}{ModelName: mn, APIBaseURL: base, APIKey: key}
	}


	taskBindings, err := s.client().UxPlanTaskBinding.Query().
		Where(uxplantaskbinding.PlanID(planUUID), uxplantaskbinding.Enabled(true)).
		Order(ent.Asc(uxplantaskbinding.FieldExecutionOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(taskBindings) == 0 {
		return nil, errors.New("计划未绑定任何任务")
	}
	profileBindings, err := s.client().UxPlanProfileBinding.Query().
		Where(uxplanprofilebinding.PlanID(planUUID), uxplanprofilebinding.Enabled(true)).
		Order(ent.Asc(uxplanprofilebinding.FieldExecutionOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(profileBindings) == 0 {
		return nil, errors.New("计划未绑定任何画像")
	}
	taskQBindings, _ := s.client().UxPlanTaskQuestionnaireBinding.Query().
		Where(uxplantaskquestionnairebinding.PlanID(planUUID), uxplantaskquestionnairebinding.Enabled(true)).
		Order(ent.Asc(uxplantaskquestionnairebinding.FieldQuestionOrder)).
		All(ctx)
	profileQBindings, _ := s.client().UxPlanProfileQuestionnaireBinding.Query().
		Where(uxplanprofilequestionnairebinding.PlanID(planUUID), uxplanprofilequestionnairebinding.Enabled(true)).
		Order(ent.Asc(uxplanprofilequestionnairebinding.FieldQuestionOrder)).
		All(ctx)

	// 把绑定里涉及到的 task/profile/template 全部预载（语义字段冻结源）
	taskIDSet := make(map[uuid.UUID]struct{}, len(taskBindings))
	for _, t := range taskBindings {
		taskIDSet[t.TaskID] = struct{}{}
	}
	taskIDs := make([]uuid.UUID, 0, len(taskIDSet))
	for k := range taskIDSet {
		taskIDs = append(taskIDs, k)
	}
	tasks, err := s.taskRepo.FindByIDsActive(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	taskMap := make(map[uuid.UUID]*ent.UxTask, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	profileIDSet := make(map[uuid.UUID]struct{}, len(profileBindings))
	for _, p := range profileBindings {
		profileIDSet[p.ProfileID] = struct{}{}
	}
	profileIDs := make([]uuid.UUID, 0, len(profileIDSet))
	for k := range profileIDSet {
		profileIDs = append(profileIDs, k)
	}
	profiles, err := s.client().UxUserProfile.Query().
		Where(uxuserprofile.IDIn(profileIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	profileMap := make(map[uuid.UUID]*ent.UxUserProfile, len(profiles))
	for _, p := range profiles {
		profileMap[p.ID] = p
	}

	templateIDSet := make(map[uuid.UUID]struct{})
	for _, b := range taskQBindings {
		templateIDSet[b.TemplateID] = struct{}{}
	}
	for _, b := range profileQBindings {
		templateIDSet[b.TemplateID] = struct{}{}
	}
	templateIDs := make([]uuid.UUID, 0, len(templateIDSet))
	for k := range templateIDSet {
		templateIDs = append(templateIDs, k)
	}
	templates, _ := s.client().UxQuestionnaireTemplate.Query().
		Where(uxquestionnairetemplate.IDIn(templateIDs...)).All(ctx)
	templateMap := make(map[uuid.UUID]*ent.UxQuestionnaireTemplate, len(templates))
	for _, t := range templates {
		templateMap[t.ID] = t
	}
	questions, _ := s.client().UxQuestionnaireQuestion.Query().
		Where(uxquestionnairequestion.TemplateIDIn(templateIDs...)).
		Order(ent.Asc(uxquestionnairequestion.FieldQuestionNo)).
		All(ctx)

	// 开启事务，写 run + 子快照 + batch + sessions
	tx, err := s.client().Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. run 主表
	runBuilder := tx.UxExecutionRun.Create().
		SetProjectID(plan.ProjectID).
		SetPlanIDRef(plan.ID).
		SetPlanNameSnapshot(plan.PlanName).
		SetPlanTypeSnapshot(uxexecutionrun.PlanTypeSnapshot(plan.PlanType)).
		SetMaxConcurrencySnapshot(plan.MaxConcurrency).
		SetStepTimeoutSecSnapshot(plan.StepTimeoutSec).
		SetSessionTimeoutSecSnapshot(plan.SessionTimeoutSec).
		SetRetryLimitSnapshot(plan.RetryLimit).
		SetStatus("RUNNING").
		SetStartedBy(uid).
		SetStartedAt(time.Now())
	if plan.Hypothesis != nil {
		runBuilder = runBuilder.SetHypothesisSnapshot(*plan.Hypothesis)
	}
	if plan.PromptOverrideID != nil {
		runBuilder = runBuilder.SetPromptOverrideIDSnapshot(*plan.PromptOverrideID)
	}
	run, err := runBuilder.Save(ctx)
	if err != nil {
		return nil, err
	}

	// 2. run_model_config 子表（plan 层只存渠道引用，运行层把真实连接信息从 project 解析后冻结）
	runMCMap := make(map[string]uuid.UUID, len(planMCs))
	for _, m := range planMCs {
		resolved := resolvedMC[string(m.ModelRole)]
		b := tx.UxRunModelConfig.Create().
			SetRunID(run.ID).
			SetModelRole(uxrunmodelconfig.ModelRole(m.ModelRole)).
			SetChannel(m.Channel).
			SetModelType(uxrunmodelconfig.ModelType(m.ModelType)).
			SetModelName(resolved.ModelName)
		if resolved.APIBaseURL != "" {
			b = b.SetAPIBaseURL(resolved.APIBaseURL)
		}
		if resolved.APIKey != "" {
			b = b.SetAPIKeyCipher(resolved.APIKey)
		}
		if m.Temperature != nil {
			b = b.SetTemperature(*m.Temperature)
		}
		if m.TopP != nil {
			b = b.SetTopP(*m.TopP)
		}
		if m.MaxTokens != nil {
			b = b.SetMaxTokens(*m.MaxTokens)
		}
		if m.ReasoningEffort != nil {
			b = b.SetReasoningEffort(*m.ReasoningEffort)
		}
		if m.ExtraParams != nil {
			b = b.SetExtraParams(*m.ExtraParams)
		}
		row, err := b.Save(ctx)
		if err != nil {
			return nil, err
		}
		runMCMap[string(m.ModelRole)] = row.ID
	}

	// 3. 任务快照
	for _, b := range taskBindings {
		t := taskMap[b.TaskID]
		if t == nil {
			continue
		}
		bldr := tx.UxRunTaskSnapshot.Create().
			SetRunID(run.ID).
			SetSourceBindingID(b.ID).
			SetTaskIDRef(b.TaskID).
			SetTaskNameSnapshot(t.TaskName).
			SetTaskGoalSnapshot(t.TaskGoal).
			SetSuccessCriteriaSnapshot(t.SuccessCriteria).
			SetTimeoutSecondsSnapshot(t.TimeoutSeconds).
			SetExecutionOrder(b.ExecutionOrder)
		if t.Precondition != nil {
			bldr = bldr.SetPreconditionSnapshot(*t.Precondition)
		}
		if t.ExecutionGuide != nil {
			bldr = bldr.SetExecutionGuideSnapshot(*t.ExecutionGuide)
		}
		if t.MinSteps != nil {
			bldr = bldr.SetMinStepsSnapshot(*t.MinSteps)
		}
		if t.MaxSteps != nil {
			bldr = bldr.SetMaxStepsSnapshot(*t.MaxSteps)
		}
		if _, err := bldr.Save(ctx); err != nil {
			return nil, err
		}
	}

	// 4. 画像快照
	for _, b := range profileBindings {
		p := profileMap[b.ProfileID]
		if p == nil {
			continue
		}
		bldr := tx.UxRunProfileSnapshot.Create().
			SetRunID(run.ID).
			SetSourceBindingID(b.ID).
			SetProfileIDRef(b.ProfileID).
			SetProfileTypeSnapshot(p.ProfileType).
			SetAgeGroupSnapshot(p.AgeGroup).
			SetGenderSnapshot(p.Gender).
			SetEducationLevelSnapshot(p.EducationLevel).
			SetExecutionOrder(b.ExecutionOrder)
		if p.CustomFields != nil {
			bldr = bldr.SetCustomFieldsSnapshot(p.CustomFields)
		}
		if _, err := bldr.Save(ctx); err != nil {
			return nil, err
		}
	}

	// 5. 任务后问卷绑定快照
	for _, b := range taskQBindings {
		tmpl := templateMap[b.TemplateID]
		name := ""
		if tmpl != nil {
			name = tmpl.TemplateName
		}
		if _, err := tx.UxRunTaskQuestionnaireSnapshot.Create().
			SetRunID(run.ID).
			SetSourceBindingID(b.ID).
			SetTaskIDRef(b.TaskID).
			SetTemplateIDRef(b.TemplateID).
			SetTemplateNameSnapshot(name).
			SetQuestionOrder(b.QuestionOrder).
			Save(ctx); err != nil {
			return nil, err
		}
	}

	// 6. 画像收尾问卷绑定快照
	for _, b := range profileQBindings {
		tmpl := templateMap[b.TemplateID]
		name := ""
		if tmpl != nil {
			name = tmpl.TemplateName
		}
		if _, err := tx.UxRunProfileQuestionnaireSnapshot.Create().
			SetRunID(run.ID).
			SetSourceBindingID(b.ID).
			SetProfileIDRef(b.ProfileID).
			SetTemplateIDRef(b.TemplateID).
			SetTemplateNameSnapshot(name).
			SetQuestionOrder(b.QuestionOrder).
			Save(ctx); err != nil {
			return nil, err
		}
	}

	// 7. 模板/题目/选项三层快照
	for _, tmpl := range templates {
		bldr := tx.UxRunQuestionnaireTemplateSnapshot.Create().
			SetRunID(run.ID).
			SetTemplateIDRef(tmpl.ID).
			SetTemplateNameSnapshot(tmpl.TemplateName)
		if tmpl.TemplateDesc != nil {
			bldr = bldr.SetTemplateDescSnapshot(*tmpl.TemplateDesc)
		}
		if _, err := bldr.Save(ctx); err != nil {
			return nil, err
		}
	}
	for _, q := range questions {
		bldr := tx.UxRunQuestionnaireQuestionSnapshot.Create().
			SetRunID(run.ID).
			SetTemplateIDRef(q.TemplateID).
			SetQuestionIDRef(q.ID).
			SetQuestionNo(q.QuestionNo).
			SetQuestionType(uxrunquestionnairequestionsnapshot.QuestionType(q.QuestionType)).
			SetQuestionText(q.QuestionText).
			SetIsRequired(q.IsRequired)
		if q.DimensionCode != nil {
			bldr = bldr.SetDimensionCode(*q.DimensionCode)
		}
		if q.ScoreRange != nil {
			if mn, ok := q.ScoreRange["min"]; ok {
				bldr = bldr.SetScoreMin(mn)
			}
			if mx, ok := q.ScoreRange["max"]; ok {
				bldr = bldr.SetScoreMax(mx)
			}
		}
		if _, err := bldr.Save(ctx); err != nil {
			return nil, err
		}
		// 选项快照
		if len(q.OptionList) > 0 {
			for idx, opt := range q.OptionList {
				v := opt["value"]
				lb := opt["label"]
				if v == "" && lb == "" {
					continue
				}
				if _, err := tx.UxRunQuestionnaireOptionSnapshot.Create().
					SetRunID(run.ID).
					SetQuestionIDRef(q.ID).
					SetOptionValue(safeStr(v, fmt.Sprintf("opt_%d", idx))).
					SetOptionLabel(safeStr(lb, v)).
					SetOptionOrder(idx).
					Save(ctx); err != nil {
					return nil, err
				}
			}
		}
	}

	// 8. batch（非 A/B 1 行 CONTROL；A/B 2 行）
	type batchInfo struct {
		ID   string
		Role string
	}
	roles := []string{"CONTROL"}
	if plan.PlanType == uxexecutionplan.PlanTypeAB_TEST {
		roles = []string{"CONTROL", "TREATMENT"}
	}
	batches := make([]batchInfo, 0, len(roles))
	shortRunID := run.ID.String()[:8]
	tsSuffix := time.Now().Format("20060102150405")
	for _, role := range roles {
		runMCID, ok := runMCMap[role]
		if !ok {
			return nil, fmt.Errorf("缺少 %s 角色的模型配置", role)
		}
		bid := fmt.Sprintf("run-%s-%s-%s", shortRunID, role, tsSuffix)
		if _, err := tx.UxExecutionBatch.Create().
			SetID(bid).
			SetRunID(run.ID).
			SetBatchRole(uxexecutionbatch.BatchRole(role)).
			SetRunModelConfigID(runMCID).
			Save(ctx); err != nil {
			return nil, err
		}
		batches = append(batches, batchInfo{ID: bid, Role: role})
	}

	// 9. 笛卡尔展开 sessions
	// A/B 测试：画像随机打乱后平分给两组（组间设计），确保分配公平；
	// 非 A/B 测试：所有画像归 CONTROL 批次。
	if plan.PlanType == uxexecutionplan.PlanTypeAB_TEST && len(batches) == 2 {
		// 随机打乱画像顺序
		shuffled := make([]*ent.UxPlanProfileBinding, len(profileBindings))
		copy(shuffled, profileBindings)
		rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		mid := (len(shuffled) + 1) / 2 // 奇数时 CONTROL 多一个
		controlProfiles := shuffled[:mid]
		treatmentProfiles := shuffled[mid:]
		for _, tb := range taskBindings {
			for _, pb := range controlProfiles {
				modelSessionID := fmt.Sprintf("session_%s_%s_%s",
					tb.TaskID.String()[:8], pb.ProfileID.String()[:8], uuid.New().String()[:8])
				if _, err := tx.UxExecutionSession.Create().
					SetBatchID(batches[0].ID).
					SetTaskID(tb.TaskID).
					SetProfileID(pb.ProfileID).
					SetModelSessionID(modelSessionID).
					SetStartedAt(time.Now()).
					SetStatus(uxexecutionsession.StatusPENDING).
					Save(ctx); err != nil {
					return nil, err
				}
			}
			for _, pb := range treatmentProfiles {
				modelSessionID := fmt.Sprintf("session_%s_%s_%s",
					tb.TaskID.String()[:8], pb.ProfileID.String()[:8], uuid.New().String()[:8])
				if _, err := tx.UxExecutionSession.Create().
					SetBatchID(batches[1].ID).
					SetTaskID(tb.TaskID).
					SetProfileID(pb.ProfileID).
					SetModelSessionID(modelSessionID).
					SetStartedAt(time.Now()).
					SetStatus(uxexecutionsession.StatusPENDING).
					Save(ctx); err != nil {
					return nil, err
				}
			}
		}
	} else {
		for _, b := range batches {
			for _, tb := range taskBindings {
				for _, pb := range profileBindings {
					modelSessionID := fmt.Sprintf("session_%s_%s_%s",
						tb.TaskID.String()[:8], pb.ProfileID.String()[:8], uuid.New().String()[:8])
					if _, err := tx.UxExecutionSession.Create().
						SetBatchID(b.ID).
						SetTaskID(tb.TaskID).
						SetProfileID(pb.ProfileID).
						SetModelSessionID(modelSessionID).
						SetStartedAt(time.Now()).
						SetStatus(uxexecutionsession.StatusPENDING).
						Save(ctx); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.runRepo.GetByID(ctx, run.ID.String())
}

// AbortRun 中止某次运行（把 RUNNING 的会话置为 CANCELLED，run 置 ABORTED）
func (s *ExecutionService) AbortRun(ctx context.Context, operatorID, runID string) error {
	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return errors.New("执行运行不存在")
	}
	canExec, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, run.ProjectID, "EXECUTE")
	if !canExec {
		return errors.New("无权操作该运行")
	}
	rid, _ := uuid.Parse(runID)
	now := time.Now()
	// 先把所有未结束会话置为 CANCELLED
	batchIDs := make([]string, 0, len(run.Batches))
	for _, b := range run.Batches {
		batchIDs = append(batchIDs, b.BatchID)
	}
	if len(batchIDs) > 0 {
		_, _ = s.client().UxExecutionSession.Update().
			Where(
				uxexecutionsession.BatchIDIn(batchIDs...),
				uxexecutionsession.StatusIn(uxexecutionsession.StatusPENDING, uxexecutionsession.StatusRUNNING, uxexecutionsession.StatusPAUSED),
			).
			SetStatus(uxexecutionsession.StatusCANCELLED).
			SetEndedAt(now).
			Save(ctx)
	}
	return s.client().UxExecutionRun.UpdateOneID(rid).
		SetStatus("ABORTED").
		SetFinishedAt(now).
		Exec(ctx)
}

// FinishRunIfDone 检查 run 是否所有 session 都已结束，是则置 FINISHED
func (s *ExecutionService) FinishRunIfDone(ctx context.Context, runID uuid.UUID) {
	run, err := s.client().UxExecutionRun.Get(ctx, runID)
	if err != nil || run.Status != "RUNNING" {
		return
	}
	batches, err := s.client().UxExecutionBatch.Query().
		Where(uxexecutionbatch.RunID(runID)).All(ctx)
	if err != nil || len(batches) == 0 {
		return
	}
	batchIDs := make([]string, 0, len(batches))
	for _, b := range batches {
		batchIDs = append(batchIDs, b.ID)
	}
	pending, err := s.client().UxExecutionSession.Query().
		Where(
			uxexecutionsession.BatchIDIn(batchIDs...),
			uxexecutionsession.StatusIn(uxexecutionsession.StatusPENDING, uxexecutionsession.StatusRUNNING, uxexecutionsession.StatusPAUSED),
		).Count(ctx)
	if err != nil || pending > 0 {
		return
	}
	_ = s.client().UxExecutionRun.UpdateOneID(runID).
		SetStatus("FINISHED").
		SetFinishedAt(time.Now()).
		Exec(ctx)
}

// ResolveRunModelConfig 取某个 batch 关联的 ux_run_model_config，并桥接成 llm 调用所需的 *model.ModelConfig
func (s *ExecutionService) ResolveRunModelConfig(ctx context.Context, batchID string) (*model.ModelConfig, string, error) {
	batch, err := s.client().UxExecutionBatch.Query().
		Where(uxexecutionbatch.ID(batchID)).Only(ctx)
	if err != nil {
		return nil, "", err
	}
	mc, err := s.client().UxRunModelConfig.Get(ctx, batch.RunModelConfigID)
	if err != nil {
		return nil, "", err
	}
	return runModelConfigToLLMConfig(mc), mc.Channel, nil
}

// runModelConfigToLLMConfig 把 RunModelConfig 投影成 LLM Client 期望的 ModelConfig
func runModelConfigToLLMConfig(m *ent.UxRunModelConfig) *model.ModelConfig {
	mc := &model.ModelConfig{DefaultChannel: m.Channel}
	apiBase := ""
	apiKey := ""
	if m.APIBaseURL != nil {
		apiBase = *m.APIBaseURL
	}
	if m.APIKeyCipher != nil {
		apiKey = *m.APIKeyCipher
	}
	switch m.Channel {
	case "ollama":
		mc.OllamaBaseURL = apiBase
		mc.OllamaModel = m.ModelName
	case "openrouter":
		mc.OpenRouterBaseURL = apiBase
		mc.OpenRouterAPIKey = apiKey
		mc.OpenRouterModel = m.ModelName
	case "openai_compatible":
		mc.OpenAICompatibleBaseURL = apiBase
		mc.OpenAICompatibleAPIKey = apiKey
		mc.OpenAICompatibleModel = m.ModelName
	}
	return mc
}

// resolveChannelFromProject 按 channel 从项目模型配置中取出 (model_name, base_url, api_key) 三元组。
// 这是"plan 引用项目配置"语义的解析入口：plan 层只存 channel；运行时由后端解析为完整连接信息。
// 第四返回值表示是否解析成功（即所选渠道在项目配置中已填写）。
func resolveChannelFromProject(mc *model.ModelConfig, channel string) (modelName, baseURL, apiKey string, ok bool) {
	if mc == nil {
		return "", "", "", false
	}
	switch channel {
	case "ollama":
		modelName = mc.OllamaModel
		baseURL = mc.OllamaBaseURL
		apiKey = ""
	case "openrouter":
		modelName = mc.OpenRouterModel
		baseURL = mc.OpenRouterBaseURL
		apiKey = mc.OpenRouterAPIKey
	case "openai_compatible":
		modelName = mc.OpenAICompatibleModel
		baseURL = mc.OpenAICompatibleBaseURL
		apiKey = mc.OpenAICompatibleAPIKey
	default:
		return "", "", "", false
	}
	// ollama 不需要 api_key，其它渠道至少需要 model + base_url
	if modelName == "" || baseURL == "" {
		return modelName, baseURL, apiKey, false
	}
	return modelName, baseURL, apiKey, true
}

// runProjectIDForBatch 反查 batch 所在 run 的 project_id
func (s *ExecutionService) runProjectIDForBatch(ctx context.Context, batchID string) string {
	batch, err := s.client().UxExecutionBatch.Query().
		Where(uxexecutionbatch.ID(batchID)).WithRun().Only(ctx)
	if err != nil {
		return ""
	}
	if run := batch.Edges.Run; run != nil {
		return run.ProjectID.String()
	}
	return ""
}

// runIDForBatch 反查 batch 所在 run_id
func (s *ExecutionService) runIDForBatch(ctx context.Context, batchID string) (uuid.UUID, error) {
	batch, err := s.client().UxExecutionBatch.Query().
		Where(uxexecutionbatch.ID(batchID)).Only(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return batch.RunID, nil
}

// taskFromRunSnapshot 取 run 内任务快照，封装为旧的 TaskDetail 形态供 prompt 使用
func (s *ExecutionService) taskFromRunSnapshot(ctx context.Context, runID, taskID uuid.UUID) *model.TaskDetail {
	t, err := s.client().UxRunTaskSnapshot.Query().
		Where(uxruntasksnapshot.RunID(runID), uxruntasksnapshot.TaskIDRef(taskID)).Only(ctx)
	if err != nil {
		return nil
	}
	d := &model.TaskDetail{
		TaskID:          t.TaskIDRef.String(),
		TaskName:        t.TaskNameSnapshot,
		TaskGoal:        t.TaskGoalSnapshot,
		SuccessCriteria: t.SuccessCriteriaSnapshot,
		TimeoutSeconds:  t.TimeoutSecondsSnapshot,
	}
	if t.PreconditionSnapshot != nil {
		d.Precondition = *t.PreconditionSnapshot
	}
	if t.ExecutionGuideSnapshot != nil {
		d.ExecutionGuide = *t.ExecutionGuideSnapshot
	}
	if t.MinStepsSnapshot != nil {
		d.MinSteps = t.MinStepsSnapshot
	}
	if t.MaxStepsSnapshot != nil {
		d.MaxSteps = t.MaxStepsSnapshot
	}
	return d
}

// profileFromRunSnapshot 取 run 内画像快照
func (s *ExecutionService) profileFromRunSnapshot(ctx context.Context, runID, profileID uuid.UUID) *model.ProfileDetail {
	p, err := s.client().UxRunProfileSnapshot.Query().
		Where(uxrunprofilesnapshot.RunID(runID), uxrunprofilesnapshot.ProfileIDRef(profileID)).Only(ctx)
	if err != nil {
		return nil
	}
	d := &model.ProfileDetail{
		ProfileID:      p.ProfileIDRef.String(),
		ProfileType:    p.ProfileTypeSnapshot,
		AgeGroup:       p.AgeGroupSnapshot,
		Gender:         p.GenderSnapshot,
		EducationLevel: p.EducationLevelSnapshot,
	}
	if p.CustomFieldsSnapshot != nil {
		d.CustomFields = p.CustomFieldsSnapshot
	}
	return d
}

// TriggerAfterTaskQuestionnaires 会话 FINISHED 时按 run snapshot 触发任务后问卷
func (s *ExecutionService) TriggerAfterTaskQuestionnaires(ctx context.Context, sessionID string) {
	sd, err := s.execRepo.GetSession(ctx, sessionID)
	if err != nil || sd == nil || sd.RunID == "" {
		return
	}
	runID, _ := uuid.Parse(sd.RunID)
	taskID, _ := uuid.Parse(sd.TaskID)
	bindings, err := s.client().UxRunTaskQuestionnaireSnapshot.Query().
		Where(uxruntaskquestionnairesnapshot.RunID(runID), uxruntaskquestionnairesnapshot.TaskIDRef(taskID)).
		Order(ent.Asc(uxruntaskquestionnairesnapshot.FieldQuestionOrder)).
		All(ctx)
	if err != nil || len(bindings) == 0 {
		return
	}
	mc, channel, err := s.ResolveRunModelConfig(ctx, sd.BatchID)
	if err != nil {
		return
	}
	task := s.taskFromRunSnapshot(ctx, runID, taskID)
	profileID, _ := uuid.Parse(sd.ProfileID)
	profile := s.profileFromRunSnapshot(ctx, runID, profileID)
	steps, _ := s.execRepo.ListStepsBySessionID(ctx, sessionID)

	for _, b := range bindings {
		questions := s.questionsFromRunSnapshot(ctx, runID, b.TemplateIDRef)
		if len(questions) == 0 {
			continue
		}
		prompt := buildRunQuestionnairePrompt(task, profile, sd, steps, b.TemplateNameSnapshot, questions)
		resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "system", Content: s.promptService.GetPrompt(ctx, sd.ProjectID, "questionnaire_system")},
				{Role: "user", Content: prompt},
			},
			Channel:     channel,
			ModelConfig: mc,
		})
		if err != nil {
			continue
		}
		s.saveAnswers(ctx, sd, "AFTER_TASK", sd.TaskID, b.ID.String(), b.TemplateIDRef.String(), questions, resp.Content)
	}
}

// TriggerAfterAllPerProfileQuestionnaires 检查"画像 × batch"是否全部完成，命中则触发收尾问卷
func (s *ExecutionService) TriggerAfterAllPerProfileQuestionnaires(ctx context.Context, sessionID string) {
	sd, err := s.execRepo.GetSession(ctx, sessionID)
	if err != nil || sd == nil || sd.RunID == "" || sd.BatchID == "" {
		fmt.Printf("[收尾问卷] session=%s 获取失败或缺少 runID/batchID\n", sessionID)
		return
	}
	runID, _ := uuid.Parse(sd.RunID)
	profileID, _ := uuid.Parse(sd.ProfileID)
	// run 下的总任务数
	totalTasks, err := s.client().UxRunTaskSnapshot.Query().
		Where(uxruntasksnapshot.RunID(runID)).Count(ctx)
	if err != nil || totalTasks == 0 {
		fmt.Printf("[收尾问卷] session=%s totalTasks=0 或查询失败\n", sessionID)
		return
	}
	// 该画像在该 batch 下已 FINISHED 的会话数
	finishedCount, err := s.client().UxExecutionSession.Query().
		Where(
			uxexecutionsession.BatchID(sd.BatchID),
			uxexecutionsession.ProfileID(profileID),
			uxexecutionsession.StatusIn(
				uxexecutionsession.StatusCOMPLETED,
				uxexecutionsession.StatusFAILED,
				uxexecutionsession.StatusTIMEOUT,
				uxexecutionsession.StatusCANCELLED,
			),
		).Count(ctx)
	fmt.Printf("[收尾问卷] session=%s profile=%s batch=%s finishedCount=%d totalTasks=%d\n",
		sessionID, sd.ProfileID, sd.BatchID, finishedCount, totalTasks)
	if err != nil || finishedCount < totalTasks {
		return
	}

	// 触发收尾问卷
	bindings, _ := s.client().UxRunProfileQuestionnaireSnapshot.Query().
		Where(uxrunprofilequestionnairesnapshot.RunID(runID), uxrunprofilequestionnairesnapshot.ProfileIDRef(profileID)).
		Order(ent.Asc(uxrunprofilequestionnairesnapshot.FieldQuestionOrder)).
		All(ctx)
	if len(bindings) == 0 {
		return
	}
	// 防重：如果该画像在该 batch 下已经写过 AFTER_ALL_PER_PROFILE 答案，跳过
	// 通过反查 ux_questionnaire_answer 即可（暂时不做严格防重，依赖 unique(session_id,question_id) 兜底）

	mc, channel, err := s.ResolveRunModelConfig(ctx, sd.BatchID)
	if err != nil {
		return
	}
	task := (*model.TaskDetail)(nil) // 收尾问卷不绑定单个任务
	profile := s.profileFromRunSnapshot(ctx, runID, profileID)
	steps, _ := s.execRepo.ListStepsByBatchAndProfile(ctx, sd.BatchID, sd.ProfileID)

	for _, b := range bindings {
		questions := s.questionsFromRunSnapshot(ctx, runID, b.TemplateIDRef)
		if len(questions) == 0 {
			continue
		}
		prompt := buildRunQuestionnairePrompt(task, profile, sd, steps, b.TemplateNameSnapshot, questions)
		resp, err := s.llmClient.Chat(ctx, llm.ChatRequest{
			Messages: []llm.Message{
				{Role: "system", Content: s.promptService.GetPrompt(ctx, sd.ProjectID, "questionnaire_system")},
				{Role: "user", Content: prompt},
			},
			Channel:     channel,
			ModelConfig: mc,
		})
		if err != nil {
			continue
		}
		// 收尾问卷 task_id 留空
		s.saveAnswers(ctx, sd, "AFTER_ALL_PER_PROFILE", "", b.ID.String(), b.TemplateIDRef.String(), questions, resp.Content)
	}
}

// questionsFromRunSnapshot 取 run 内某模板的所有题目快照（含选项）
func (s *ExecutionService) questionsFromRunSnapshot(ctx context.Context, runID, templateID uuid.UUID) []model.QuestionDetail {
	qs, err := s.client().UxRunQuestionnaireQuestionSnapshot.Query().
		Where(
			uxrunquestionnairequestionsnapshot.RunID(runID),
			uxrunquestionnairequestionsnapshot.TemplateIDRef(templateID),
		).
		Order(ent.Asc(uxrunquestionnairequestionsnapshot.FieldQuestionNo)).
		All(ctx)
	if err != nil {
		return nil
	}
	out := make([]model.QuestionDetail, 0, len(qs))
	for _, q := range qs {
		dto := model.QuestionDetail{
			QuestionID:   q.ID.String(),            // 使用快照表自身 ID（非原始表 ref）
			TemplateID:   q.TemplateIDRef.String(), // 保留 templateIDRef 用于按模板分组
			QuestionNo:   q.QuestionNo,
			QuestionType: string(q.QuestionType),
			QuestionText: q.QuestionText,
			IsRequired:   q.IsRequired,
		}
		if q.DimensionCode != nil {
			dto.DimensionCode = *q.DimensionCode
		}
		if q.ScoreMin != nil && q.ScoreMax != nil {
			dto.ScoreRange = map[string]int{"min": *q.ScoreMin, "max": *q.ScoreMax}
		}
		// 选项
		opts, _ := s.client().UxRunQuestionnaireOptionSnapshot.Query().
			Where(uxrunquestionnaireoptionsnapshot.RunID(runID), uxrunquestionnaireoptionsnapshot.QuestionIDRef(q.QuestionIDRef)).
			Order(ent.Asc(uxrunquestionnaireoptionsnapshot.FieldOptionOrder)).
			All(ctx)
		if len(opts) > 0 {
			ol := make([]map[string]string, 0, len(opts))
			for _, o := range opts {
				ol = append(ol, map[string]string{"value": o.OptionValue, "label": o.OptionLabel})
			}
			dto.OptionList = ol
		}
		out = append(out, dto)
	}
	return out
}

// saveAnswers 解析 LLM 返回的 JSON 并写入 ux_questionnaire_answer
func (s *ExecutionService) saveAnswers(ctx context.Context, sd *model.SessionDetail, origin, taskID, sourceBindingID, templateID string, questions []model.QuestionDetail, content string) {
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
	if err := jsonUnmarshalLoose([]byte(jsonStr), &result); err != nil {
		return
	}
	for _, a := range result.Answers {
		_ = s.resultRepo.SaveAnswer(ctx,
			sd.SessionID, sd.ProfileID, templateID, a.QuestionID, a.AnswerType,
			origin, taskID, sourceBindingID,
			a.AnswerScore, a.AnswerOption, a.AnswerText,
		)
	}
}

// 工具：把 ent.UxExecutionPlan.PlanType 转为 schema 上的字符串枚举值
func planTypeFromPlan(t uxexecutionplan.PlanType) string {
	return string(t)
}
var _ = planTypeFromPlan
var _ = uxrunquestionnairetemplatesnapshot.FieldRunID

func safeStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// jsonUnmarshalLoose 宽松 JSON 解析（容错 trailing commas 等场景未来扩展用，目前直接代理 json.Unmarshal）
func jsonUnmarshalLoose(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// buildRunQuestionnairePrompt 基于 run snapshot 构造问卷作答 prompt
func buildRunQuestionnairePrompt(task *model.TaskDetail, profile *model.ProfileDetail, sd *model.SessionDetail, steps []model.StepDetail, templateName string, questions []model.QuestionDetail) string {
	var sb strings.Builder
	if task != nil {
		sb.WriteString("## 任务信息\n")
		sb.WriteString(fmt.Sprintf("任务: %s\n目标: %s\n完成条件: %s\n\n", task.TaskName, task.TaskGoal, task.SuccessCriteria))
	} else {
		sb.WriteString("## 评估上下文\n本次问卷为画像收尾问卷，覆盖该画像在本批次下完成的所有任务。\n\n")
	}
	sb.WriteString("## 用户画像\n")
	if profile != nil {
		sb.WriteString(fmt.Sprintf("年龄: %s, 教育: %s, 性别: %s\n\n", profile.AgeGroup, profile.EducationLevel, profile.Gender))
	}
	sb.WriteString("## 执行结果\n")
	if sd != nil {
		sb.WriteString(fmt.Sprintf("状态: %s, 错误次数: %d, 是否完成: %v\n\n", sd.Status, sd.ErrorCount, sd.IsGoalCompleted))
	}
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
				if v, ok := opt["label"]; ok && v != "" {
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
