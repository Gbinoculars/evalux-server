package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxexecutionbatch"
	"evalux-server/ent/uxexecutionsession"
	"evalux-server/ent/uxexecutionstep"
	"evalux-server/ent/uxrunprofilesnapshot"
	"evalux-server/ent/uxrunquestionnaireoptionsnapshot"
	"evalux-server/ent/uxrunquestionnairequestionsnapshot"
	"evalux-server/ent/uxrunquestionnairetemplatesnapshot"
	"evalux-server/ent/uxruntasksnapshot"
	"evalux-server/ent/uxscreenshotrecord"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type ExecutionRepo struct {
	client *ent.Client
}

func NewExecutionRepo(client *ent.Client) *ExecutionRepo {
	return &ExecutionRepo{client: client}
}

// CreateSession 创建执行会话。session 必须挂在某个 batch 下，
// project_id 不再直接持有，需要时反查 batch->run 取 project_id。
func (r *ExecutionRepo) CreateSession(ctx context.Context, req model.StartExecutionRequest, modelSessionID string) (*model.SessionDetail, error) {
	taskID, _ := uuid.Parse(req.TaskID)
	profileID, _ := uuid.Parse(req.ProfileID)

	if req.BatchID == "" {
		// 无 batch_id 不允许创建会话（防止数据"孤岛"）
		return nil, errors.New("session 必须指定 batch_id")
	}

	b := r.client.UxExecutionSession.Create().
		SetBatchID(req.BatchID).
		SetTaskID(taskID).
		SetProfileID(profileID).
		SetModelSessionID(modelSessionID).
		SetNillableDeviceSerial(nilIfEmpty(req.DeviceSerial)).
		SetStartedAt(time.Now()).
		SetStatus("RUNNING").
		SetErrorCount(0).
		SetIsGoalCompleted(false)

	s, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	d, _ := r.entSessionWithContext(ctx, s)
	return d, nil
}

// GetSession 查询会话（含 batch / run / project 反查信息）
func (r *ExecutionRepo) GetSession(ctx context.Context, sessionID string) (*model.SessionDetail, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	s, err := r.client.UxExecutionSession.Query().
		Where(uxexecutionsession.ID(sid)).
		WithRecording().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	d, _ := r.entSessionWithContext(ctx, s)
	if rec := s.Edges.Recording; rec != nil {
		d.RecordingURL = rec.FilePath
	}
	return d, nil
}

// UpdateSession 更新会话状态
func (r *ExecutionRepo) UpdateSession(ctx context.Context, sessionID string, status string, isGoalCompleted *bool, stopReason *string) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}
	upd := r.client.UxExecutionSession.UpdateOneID(sid).
		SetStatus(uxexecutionsession.Status(status))
	if status == "COMPLETED" || status == "FAILED" || status == "TIMEOUT" || status == "CANCELLED" {
		now := time.Now()
		upd.SetEndedAt(now)
		s, getErr := r.client.UxExecutionSession.Get(ctx, sid)
		if getErr == nil {
			durationMs := now.Sub(s.StartedAt).Milliseconds()
			upd.SetTotalDurationMs(durationMs)
		}
	}
	if isGoalCompleted != nil {
		upd.SetIsGoalCompleted(*isGoalCompleted)
	}
	if stopReason != nil {
		upd.SetStopReason(*stopReason)
	}
	return upd.Exec(ctx)
}

// IncrementErrorCount 增加错误计数
func (r *ExecutionRepo) IncrementErrorCount(ctx context.Context, sessionID string) error {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return err
	}
	s, err := r.client.UxExecutionSession.Get(ctx, sid)
	if err != nil {
		return err
	}
	return r.client.UxExecutionSession.UpdateOneID(sid).
		SetErrorCount(s.ErrorCount + 1).
		Exec(ctx)
}

// ListSessionsByProjectID 按项目查询会话列表（通过 batch->run->project_id 反查过滤）
func (r *ExecutionRepo) ListSessionsByProjectID(ctx context.Context, projectID string, query model.SessionListQuery) ([]model.SessionDetail, int64, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, 0, err
	}
	// 找出该项目下所有 batch_id
	batchIDs, err := r.batchIDsOfProject(ctx, pid)
	if err != nil {
		return nil, 0, err
	}
	if len(batchIDs) == 0 {
		return []model.SessionDetail{}, 0, nil
	}
	q := r.client.UxExecutionSession.Query().Where(uxexecutionsession.BatchIDIn(batchIDs...))
	if query.Status != "" {
		q = q.Where(uxexecutionsession.StatusEQ(uxexecutionsession.Status(query.Status)))
	}
	if query.BatchIDs != "" {
		ids := strings.Split(query.BatchIDs, ",")
		q = q.Where(uxexecutionsession.BatchIDIn(ids...))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize
	sessions, err := q.Order(ent.Desc(uxexecutionsession.FieldStartedAt)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	list := make([]model.SessionDetail, 0, len(sessions))
	for _, s := range sessions {
		d, _ := r.entSessionWithContext(ctx, s)
		list = append(list, *d)
	}
	return list, int64(total), nil
}

// ListAllSessionsByProjectID 查询项目下所有会话（不分页，用于聚合报告）
func (r *ExecutionRepo) ListAllSessionsByProjectID(ctx context.Context, projectID string) ([]model.SessionDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	batchIDs, err := r.batchIDsOfProject(ctx, pid)
	if err != nil {
		return nil, err
	}
	if len(batchIDs) == 0 {
		return []model.SessionDetail{}, nil
	}
	sessions, err := r.client.UxExecutionSession.Query().
		Where(uxexecutionsession.BatchIDIn(batchIDs...)).
		WithSteps().
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.SessionDetail, 0, len(sessions))
	for _, s := range sessions {
		d, _ := r.entSessionWithContext(ctx, s)
		d.StepCount = len(s.Edges.Steps)
		list = append(list, *d)
	}
	return list, nil
}

// ListSessionsByBatchID 按 batch_id 查询所有会话（A/B 分组聚合）
func (r *ExecutionRepo) ListSessionsByBatchID(ctx context.Context, batchID string) ([]model.SessionDetail, error) {
	sessions, err := r.client.UxExecutionSession.Query().
		Where(uxexecutionsession.BatchID(batchID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.SessionDetail, 0, len(sessions))
	for _, s := range sessions {
		d, _ := r.entSessionWithContext(ctx, s)
		list = append(list, *d)
	}
	return list, nil
}

// ListSessionsByRunID 按 run_id 查询所有会话（结果聚合主入口）
func (r *ExecutionRepo) ListSessionsByRunID(ctx context.Context, runID string) ([]model.SessionDetail, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	batches, err := r.client.UxExecutionBatch.Query().
		Where(uxexecutionbatch.RunID(rid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(batches) == 0 {
		return []model.SessionDetail{}, nil
	}
	batchIDs := make([]string, 0, len(batches))
	for _, b := range batches {
		batchIDs = append(batchIDs, b.ID)
	}
	sessions, err := r.client.UxExecutionSession.Query().
		Where(uxexecutionsession.BatchIDIn(batchIDs...)).
		WithSteps().
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.SessionDetail, 0, len(sessions))
	for _, s := range sessions {
		d, _ := r.entSessionWithContext(ctx, s)
		d.StepCount = len(s.Edges.Steps)
		list = append(list, *d)
	}
	return list, nil
}

func (r *ExecutionRepo) CreateStep(ctx context.Context, sessionID string, req model.ReportStepRequest) (*ent.UxExecutionStep, error) {
	sid, _ := uuid.Parse(sessionID)
	b := r.client.UxExecutionStep.Create().
		SetSessionID(sid).
		SetStepNo(req.StepNo).
		SetRetryCount(req.RetryCount).
		SetStartedAt(time.Now())
	if req.ScreenDesc != "" {
		b.SetScreenDesc(req.ScreenDesc)
	}
	if req.ActionType != "" {
		b.SetActionType(req.ActionType)
	}
	if req.ActionParam != nil {
		b.SetActionParam(req.ActionParam)
	}
	if req.ExecResult != nil {
		b.SetExecutionResult(req.ExecResult)
	}
	if req.ErrorMsg != "" {
		b.SetErrorMessage(req.ErrorMsg)
	}
	return b.Save(ctx)
}

// FinishStep 完成步骤（保存AI决策结果）
func (r *ExecutionRepo) FinishStep(ctx context.Context, stepID uuid.UUID, decisionSummary string) error {
	now := time.Now()
	upd := r.client.UxExecutionStep.UpdateOneID(stepID).SetEndedAt(now)
	if decisionSummary != "" {
		upd.SetDecisionSummary(decisionSummary)
	}
	return upd.Exec(ctx)
}

// FinishStepWithAction 完成步骤并保存AI的决策动作信息
func (r *ExecutionRepo) FinishStepWithAction(ctx context.Context, stepID uuid.UUID, decisionSummary string, actionType string, actionParam map[string]interface{}) error {
	now := time.Now()
	upd := r.client.UxExecutionStep.UpdateOneID(stepID).SetEndedAt(now)
	if decisionSummary != "" {
		upd.SetDecisionSummary(decisionSummary)
	}
	if actionType != "" {
		upd.SetActionType(actionType)
	}
	if actionParam != nil {
		upd.SetActionParam(actionParam)
	}
	return upd.Exec(ctx)
}

// ListStepsBySessionID 查询会话下所有步骤
func (r *ExecutionRepo) ListStepsBySessionID(ctx context.Context, sessionID string) ([]model.StepDetail, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	steps, err := r.client.UxExecutionStep.Query().
		Where(uxexecutionstep.SessionID(sid)).
		Order(ent.Asc(uxexecutionstep.FieldStepNo)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]model.StepDetail, 0, len(steps))
	for _, st := range steps {
		d := model.StepDetail{
			StepID:     st.ID.String(),
			SessionID:  st.SessionID.String(),
			StepNo:     st.StepNo,
			RetryCount: st.RetryCount,
			StartedAt:  st.StartedAt,
			EndedAt:    st.EndedAt,
		}
		if st.ScreenDesc != nil {
			d.ScreenDesc = *st.ScreenDesc
		}
		if st.ActionType != nil {
			d.ActionType = *st.ActionType
		}
		if st.ActionParam != nil {
			d.ActionParam = st.ActionParam
		}
		if st.DecisionSummary != nil {
			d.DecisionSummary = *st.DecisionSummary
		}
		if st.ExecutionResult != nil {
			d.ExecResult = st.ExecutionResult
		}
		if st.ErrorMessage != nil {
			d.ErrorMessage = *st.ErrorMessage
		}
		// 查截图
		screenshots, _ := r.client.UxScreenshotRecord.Query().
			Where(uxscreenshotrecord.StepID(st.ID)).
			All(ctx)
		if len(screenshots) > 0 {
			d.ScreenshotURL = screenshots[0].FilePath
		}
		list = append(list, d)
	}
	return list, nil
}

// ListStepsByBatchAndProfile 同批次同画像所有 session 的步骤（跨任务上下文）
func (r *ExecutionRepo) ListStepsByBatchAndProfile(ctx context.Context, batchID string, profileID string) ([]model.StepDetail, error) {
	pid, err := uuid.Parse(profileID)
	if err != nil {
		return nil, err
	}
	sessions, err := r.client.UxExecutionSession.Query().
		Where(
			uxexecutionsession.BatchID(batchID),
			uxexecutionsession.ProfileID(pid),
		).
		Order(ent.Asc(uxexecutionsession.FieldStartedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var allSteps []model.StepDetail
	for _, sess := range sessions {
		steps, _ := r.ListStepsBySessionID(ctx, sess.ID.String())
		allSteps = append(allSteps, steps...)
	}
	return allSteps, nil
}

// SaveScreenshot 保存截图记录
func (r *ExecutionRepo) SaveScreenshot(ctx context.Context, sessionID string, stepID uuid.UUID, filePath string, isKeyFrame bool) error {
	sid, _ := uuid.Parse(sessionID)
	return r.client.UxScreenshotRecord.Create().
		SetSessionID(sid).
		SetStepID(stepID).
		SetFilePath(filePath).
		SetShotAt(time.Now()).
		SetIsKeyFrame(isKeyFrame).
		OnConflictColumns().DoNothing().
		Exec(ctx)
}

// SaveRecording 保存录屏记录
func (r *ExecutionRepo) SaveRecording(ctx context.Context, sessionID, filePath string, fileSize int64) error {
	sid, _ := uuid.Parse(sessionID)
	return r.client.UxRecordingRecord.Create().
		SetSessionID(sid).
		SetFilePath(filePath).
		SetStartedAt(time.Now()).
		SetStorageStatus("SAVED").
		SetNillableFileSizeBytes(&fileSize).
		OnConflictColumns("session_id").DoNothing().
		Exec(ctx)
}

// batchIDsOfProject 通过 run.project_id 反查项目下所有 batch_id
func (r *ExecutionRepo) batchIDsOfProject(ctx context.Context, projectID uuid.UUID) ([]string, error) {
	batches, err := r.client.UxExecutionBatch.Query().
		WithRun().
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(batches))
	for _, b := range batches {
		if run := b.Edges.Run; run != nil && run.ProjectID == projectID {
			ids = append(ids, b.ID)
		}
	}
	return ids, nil
}

// entSessionWithContext 把 session 转 DTO 并填充 batch_role / run_id / project_id
func (r *ExecutionRepo) entSessionWithContext(ctx context.Context, s *ent.UxExecutionSession) (*model.SessionDetail, error) {
	d := &model.SessionDetail{
		SessionID:       s.ID.String(),
		BatchID:         s.BatchID,
		TaskID:          s.TaskID.String(),
		ProfileID:       s.ProfileID.String(),
		ModelSessionID:  s.ModelSessionID,
		Status:          string(s.Status),
		ErrorCount:      s.ErrorCount,
		IsGoalCompleted: s.IsGoalCompleted,
		StartedAt:       s.StartedAt,
		EndedAt:         s.EndedAt,
		TotalDurationMs: s.TotalDurationMs,
	}
	if s.DeviceSerial != nil {
		d.DeviceSerial = *s.DeviceSerial
	}
	if s.StopReason != nil {
		d.StopReason = *s.StopReason
	}
	// 反查 batch -> run 取 batch_role / run_id / project_id
	if s.BatchID != "" {
		batch, err := r.client.UxExecutionBatch.Query().
			Where(uxexecutionbatch.ID(s.BatchID)).
			WithRun().
			Only(ctx)
		if err == nil {
			d.BatchRole = string(batch.BatchRole)
			d.RunID = batch.RunID.String()
			if run := batch.Edges.Run; run != nil {
				d.ProjectID = run.ProjectID.String()
			}
		}
	}
	return d, nil
}

// ==================== 快照查询方法 ====================

// SnapshotTemplateInfo 模板快照精简信息（报告聚合用）
type SnapshotTemplateInfo struct {
	SnapshotID   string // ux_run_questionnaire_template_snapshot.id
	TemplateRef  string // template_id_ref（原始模板 ID）
	TemplateName string
}

// ListSnapshotTemplatesByRunID 查询 run 下所有问卷模板快照
func (r *ExecutionRepo) ListSnapshotTemplatesByRunID(ctx context.Context, runID string) ([]SnapshotTemplateInfo, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.UxRunQuestionnaireTemplateSnapshot.Query().
		Where(uxrunquestionnairetemplatesnapshot.RunID(rid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]SnapshotTemplateInfo, 0, len(rows))
	for _, row := range rows {
		list = append(list, SnapshotTemplateInfo{
			SnapshotID:   row.ID.String(),
			TemplateRef:  row.TemplateIDRef.String(),
			TemplateName: row.TemplateNameSnapshot,
		})
	}
	return list, nil
}

// ListSnapshotQuestionsByRunAndTemplate 查询 run 下某个模板的所有题目快照
// templateIDRef 是 answer.template_id 存的快照 template_id_ref
func (r *ExecutionRepo) ListSnapshotQuestionsByRunAndTemplate(ctx context.Context, runID, templateIDRef string) ([]model.QuestionDetail, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	tplRef, err := uuid.Parse(templateIDRef)
	if err != nil {
		return nil, err
	}
	questions, err := r.client.UxRunQuestionnaireQuestionSnapshot.Query().
		Where(
			uxrunquestionnairequestionsnapshot.RunID(rid),
			uxrunquestionnairequestionsnapshot.TemplateIDRef(tplRef),
		).
		Order(ent.Asc(uxrunquestionnairequestionsnapshot.FieldQuestionNo)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// 批量查选项（同一 run + question_id_ref 下的选项）
	questionIDRefs := make([]uuid.UUID, 0, len(questions))
	for _, q := range questions {
		questionIDRefs = append(questionIDRefs, q.QuestionIDRef)
	}
	options, _ := r.client.UxRunQuestionnaireOptionSnapshot.Query().
		Where(
			uxrunquestionnaireoptionsnapshot.RunID(rid),
			uxrunquestionnaireoptionsnapshot.QuestionIDRefIn(questionIDRefs...),
		).
		Order(ent.Asc(uxrunquestionnaireoptionsnapshot.FieldOptionOrder)).
		All(ctx)
	// 按 question_id_ref 分组
	optionMap := map[uuid.UUID][]map[string]string{}
	for _, opt := range options {
		optionMap[opt.QuestionIDRef] = append(optionMap[opt.QuestionIDRef], map[string]string{
			"value": opt.OptionValue,
			"label": opt.OptionLabel,
		})
	}

	list := make([]model.QuestionDetail, 0, len(questions))
	for _, q := range questions {
		d := model.QuestionDetail{
			QuestionID:   q.ID.String(), // 快照自身的 snapshot_id
			TemplateID:   tplRef.String(),
			QuestionNo:   q.QuestionNo,
			QuestionType: string(q.QuestionType),
			QuestionText: q.QuestionText,
			IsRequired:   q.IsRequired,
			OptionList:   optionMap[q.QuestionIDRef],
		}
		if q.DimensionCode != nil {
			d.DimensionCode = *q.DimensionCode
		}
		if q.ScoreMin != nil && q.ScoreMax != nil {
			d.ScoreRange = map[string]int{"min": *q.ScoreMin, "max": *q.ScoreMax}
		}
		list = append(list, d)
	}
	return list, nil
}

// GetRunProjectID 通过 run_id 反查 project_id
func (r *ExecutionRepo) GetRunProjectID(ctx context.Context, runID string) (string, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return "", err
	}
	run, err := r.client.UxExecutionRun.Get(ctx, rid)
	if err != nil {
		return "", err
	}
	return run.ProjectID.String(), nil
}

// SnapshotTaskInfo 任务快照精简信息
type SnapshotTaskInfo struct {
	TaskRef  string // task_id_ref
	TaskName string
	TaskGoal string
}

// ListSnapshotTasksByRunID 查询 run 下所有任务快照（名称+目标）
func (r *ExecutionRepo) ListSnapshotTasksByRunID(ctx context.Context, runID string) ([]SnapshotTaskInfo, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.UxRunTaskSnapshot.Query().
		Where(uxruntasksnapshot.RunID(rid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]SnapshotTaskInfo, 0, len(rows))
	for _, row := range rows {
		list = append(list, SnapshotTaskInfo{
			TaskRef:  row.TaskIDRef.String(),
			TaskName: row.TaskNameSnapshot,
			TaskGoal: row.TaskGoalSnapshot,
		})
	}
	return list, nil
}

// SnapshotProfileInfo 画像快照精简信息
type SnapshotProfileInfo struct {
	ProfileRef     string // profile_id_ref
	Gender         string
	AgeGroup       string
	EducationLevel string
	NickName       string
	CustomFields   map[string]interface{}
}

// ListSnapshotProfilesByRunID 查询 run 下所有画像快照
func (r *ExecutionRepo) ListSnapshotProfilesByRunID(ctx context.Context, runID string) ([]SnapshotProfileInfo, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.UxRunProfileSnapshot.Query().
		Where(uxrunprofilesnapshot.RunID(rid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]SnapshotProfileInfo, 0, len(rows))
	for _, row := range rows {
		info := SnapshotProfileInfo{
			ProfileRef:     row.ProfileIDRef.String(),
			Gender:         row.GenderSnapshot,
			AgeGroup:       row.AgeGroupSnapshot,
			EducationLevel: row.EducationLevelSnapshot,
			CustomFields:   row.CustomFieldsSnapshot,
		}
		if row.CustomFieldsSnapshot != nil {
			if nn, ok := row.CustomFieldsSnapshot["nickname"]; ok {
				info.NickName = fmt.Sprintf("%v", nn)
			}
		}
		list = append(list, info)
	}
	return list, nil
}
