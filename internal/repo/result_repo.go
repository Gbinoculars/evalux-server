package repo

import (
	"context"
	"fmt"
	"time"

	"evalux-server/ent"
	"evalux-server/ent/uxexecutionbatch"
	"evalux-server/ent/uxexecutionsession"
	"evalux-server/ent/uximprovementsuggestion"
	"evalux-server/ent/uxprojectaireport"
	"evalux-server/ent/uxquestionnaireanswer"
	"evalux-server/ent/uxsubjectiveevaluation"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type ResultRepo struct {
	client *ent.Client
}

func NewResultRepo(client *ent.Client) *ResultRepo {
	return &ResultRepo{client: client}
}

// GetProjectOverview 计算项目级统计概览（按 batch->run->project 反查）
func (r *ResultRepo) GetProjectOverview(ctx context.Context, projectID string) (*model.ProjectResultOverview, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	// 取项目下所有 batch_id
	batches, err := r.client.UxExecutionBatch.Query().
		WithRun().All(ctx)
	if err != nil {
		return nil, err
	}
	batchIDs := make([]string, 0, len(batches))
	for _, b := range batches {
		if run := b.Edges.Run; run != nil && run.ProjectID == pid {
			batchIDs = append(batchIDs, b.ID)
		}
	}
	overview := &model.ProjectResultOverview{ProjectID: projectID}
	if len(batchIDs) == 0 {
		return overview, nil
	}

	sessions, err := r.client.UxExecutionSession.Query().
		Where(uxexecutionsession.BatchIDIn(batchIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	overview.TotalSessions = len(sessions)

	var totalDuration int64
	var totalErrors int
	durationCount := 0
	for _, s := range sessions {
		switch string(s.Status) {
		case "COMPLETED":
			overview.CompletedCount++
		case "FAILED", "TIMEOUT":
			overview.FailedCount++
		}
		totalErrors += s.ErrorCount
		if s.TotalDurationMs != nil {
			totalDuration += *s.TotalDurationMs
			durationCount++
		}
	}
	if overview.TotalSessions > 0 {
		overview.CompletionRate = float64(overview.CompletedCount) / float64(overview.TotalSessions) * 100
		overview.AvgErrorCount = float64(totalErrors) / float64(overview.TotalSessions)
	}
	if durationCount > 0 {
		overview.AvgDurationMs = totalDuration / int64(durationCount)
	}
	return overview, nil
}

// SaveAnswer 保存问卷回答（新签名）。
//   - origin = AFTER_TASK 时 taskID 必填，AFTER_ALL_PER_PROFILE 时 taskID 传空字符串
//   - sourceBindingID 指向 run 内绑定快照行 id（可选）
func (r *ResultRepo) SaveAnswer(ctx context.Context, sessionID, profileID, templateID, questionID, answerType string,
	origin string, taskID, sourceBindingID string,
	score float64, option []string, text string) error {
	sid, _ := uuid.Parse(sessionID)
	pid, _ := uuid.Parse(profileID)
	tmplID, _ := uuid.Parse(templateID)
	qid, _ := uuid.Parse(questionID)

	b := r.client.UxQuestionnaireAnswer.Create().
		SetSessionID(sid).
		SetProfileID(pid).
		SetTemplateID(tmplID).
		SetQuestionID(qid).
		SetAnswerType(answerType)
	if origin != "" {
		// 显式枚举
		b = b.SetAnswerOrigin(parseAnswerOrigin(origin))
	}
	if taskID != "" {
		if tid, err := uuid.Parse(taskID); err == nil {
			b = b.SetTaskID(tid)
		}
	}
	if sourceBindingID != "" {
		if sb, err := uuid.Parse(sourceBindingID); err == nil {
			b = b.SetSourceBindingID(sb)
		}
	}
	if score > 0 {
		b = b.SetAnswerScore(score)
	}
	if option != nil {
		b = b.SetAnswerOption(option)
	}
	if text != "" {
		b = b.SetAnswerText(text)
	}
	return b.OnConflictColumns("session_id", "question_id").DoNothing().Exec(ctx)
}

// SaveSubjectiveEval 保存主观评价
func (r *ResultRepo) SaveSubjectiveEval(ctx context.Context, sessionID string, score float64, summary string, snapshot map[string]interface{}) error {
	sid, _ := uuid.Parse(sessionID)
	b := r.client.UxSubjectiveEvaluation.Create().
		SetSessionID(sid).
		SetNillableOverallScore(&score).
		SetSummaryText(summary)
	if snapshot != nil {
		b.SetBasedOnSnapshot(snapshot)
	}
	return b.OnConflictColumns("session_id").DoNothing().Exec(ctx)
}

// SaveSuggestion 保存改进建议
func (r *ResultRepo) SaveSuggestion(ctx context.Context, sessionID, suggestionType, priority, text string) error {
	sid, _ := uuid.Parse(sessionID)
	return r.client.UxImprovementSuggestion.Create().
		SetSessionID(sid).
		SetSuggestionType(suggestionType).
		SetPriorityLevel(uximprovementsuggestion.PriorityLevel(priority)).
		SetSuggestionText(text).
		Exec(ctx)
}

// GetAnswersBySessionID 查询会话的问卷回答
func (r *ResultRepo) GetAnswersBySessionID(ctx context.Context, sessionID string) ([]model.AnswerDetail, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	answers, err := r.client.UxQuestionnaireAnswer.Query().
		Where(uxquestionnaireanswer.SessionID(sid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapAnswers(answers), nil
}

// GetAnswersBySessionIDs 批量查询多个会话的问卷回答
func (r *ResultRepo) GetAnswersBySessionIDs(ctx context.Context, sessionIDs []string) ([]model.AnswerDetail, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	sids := make([]uuid.UUID, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		sid, err := uuid.Parse(id)
		if err == nil {
			sids = append(sids, sid)
		}
	}
	answers, err := r.client.UxQuestionnaireAnswer.Query().
		Where(uxquestionnaireanswer.SessionIDIn(sids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapAnswers(answers), nil
}

func mapAnswers(answers []*ent.UxQuestionnaireAnswer) []model.AnswerDetail {
	list := make([]model.AnswerDetail, 0, len(answers))
	for _, a := range answers {
		d := model.AnswerDetail{
			AnswerID:     a.ID.String(),
			SessionID:    a.SessionID.String(),
			AnswerOrigin: string(a.AnswerOrigin),
			ProfileID:    a.ProfileID.String(),
			TemplateID:   a.TemplateID.String(),
			QuestionID:   a.QuestionID.String(),
			AnswerType:   a.AnswerType,
			CreatedAt:    a.CreatedAt,
		}
		if a.SourceBindingID != nil {
			d.SourceBindingID = a.SourceBindingID.String()
		}
		if a.TaskID != nil {
			d.TaskID = a.TaskID.String()
		}
		if a.AnswerScore != nil {
			d.AnswerScore = *a.AnswerScore
		}
		if a.AnswerOption != nil {
			optMap := make(map[string]interface{})
			for i, opt := range a.AnswerOption {
				optMap[fmt.Sprintf("%d", i)] = opt
			}
			d.AnswerOption = optMap
		}
		if a.AnswerText != nil {
			d.AnswerText = *a.AnswerText
		}
		list = append(list, d)
	}
	return list
}

func (r *ResultRepo) GetSubjectiveEval(ctx context.Context, sessionID string) (*model.SubjectiveEvalDetail, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	eval, err := r.client.UxSubjectiveEvaluation.Query().
		Where(uxsubjectiveevaluation.SessionID(sid)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	d := &model.SubjectiveEvalDetail{
		EvaluationID: eval.ID.String(),
		SessionID:    eval.SessionID.String(),
		SummaryText:  eval.SummaryText,
		CreatedAt:    eval.CreatedAt,
	}
	if eval.OverallScore != nil {
		d.OverallScore = *eval.OverallScore
	}
	if eval.BasedOnSnapshot != nil {
		d.BasedOnSnapshot = eval.BasedOnSnapshot
	}
	return d, nil
}

// GetSuggestions 查询会话的改进建议
func (r *ResultRepo) GetSuggestions(ctx context.Context, sessionID string) ([]model.SuggestionDetail, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	suggestions, err := r.client.UxImprovementSuggestion.Query().
		Where(uximprovementsuggestion.SessionID(sid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.SuggestionDetail, 0, len(suggestions))
	for _, s := range suggestions {
		list = append(list, model.SuggestionDetail{
			SuggestionID:   s.ID.String(),
			SessionID:      s.SessionID.String(),
			SuggestionType: s.SuggestionType,
			PriorityLevel:  string(s.PriorityLevel),
			SuggestionText: s.SuggestionText,
			CreatedAt:      s.CreatedAt,
		})
	}
	return list, nil
}

// SaveResultSnapshot 保存项目级结果快照（按 run 维度）
// 第 1 轮先按"项目最新一次 run"做兜底；第 2 轮 service 层完整接入。
func (r *ResultRepo) SaveResultSnapshot(ctx context.Context, projectID string, overview *model.ProjectResultOverview) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	// 取该项目最新一次 run + 它的第一个 batch
	runs, err := r.client.UxExecutionRun.Query().
		WithBatches().
		All(ctx)
	if err != nil {
		return err
	}
	var runID uuid.UUID
	var batchID string
	for _, run := range runs {
		if run.ProjectID == pid {
			runID = run.ID
			if bs := run.Edges.Batches; len(bs) > 0 {
				batchID = bs[0].ID
			}
			break
		}
	}
	if runID == uuid.Nil || batchID == "" {
		return nil
	}
	metricPayload := map[string]interface{}{
		"total_sessions":  overview.TotalSessions,
		"completed_count": overview.CompletedCount,
		"failed_count":    overview.FailedCount,
		"completion_rate": overview.CompletionRate,
		"avg_duration_ms": overview.AvgDurationMs,
		"avg_error_count": overview.AvgErrorCount,
		"avg_score":       overview.AvgScore,
		"generated_at":    time.Now(),
	}
	return r.client.UxResultSnapshot.Create().
		SetRunID(runID).
		SetBatchID(batchID).
		SetScopeType("OVERALL").
		SetTotalSessions(overview.TotalSessions).
		SetCompletedSessions(overview.CompletedCount).
		SetFailedSessions(overview.FailedCount).
		SetCompletionRate(overview.CompletionRate).
		SetNillableAvgDurationMs(&overview.AvgDurationMs).
		SetMetricPayload(metricPayload).
		Exec(ctx)
}

// SaveAIReport 保存项目级 AI 分析报告
func (r *ResultRepo) SaveAIReport(ctx context.Context, projectID, modelChannel string, sessionIDs []string, summary string, strengths, weaknesses, recommendations []string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	if strengths == nil {
		strengths = []string{}
	}
	if weaknesses == nil {
		weaknesses = []string{}
	}
	if recommendations == nil {
		recommendations = []string{}
	}
	if sessionIDs == nil {
		sessionIDs = []string{}
	}
	return r.client.UxProjectAiReport.Create().
		SetProjectID(pid).
		SetModelChannel(modelChannel).
		SetSessionIds(sessionIDs).
		SetAiSummary(summary).
		SetAiStrengths(strengths).
		SetAiWeaknesses(weaknesses).
		SetAiRecommendations(recommendations).
		Exec(ctx)
}

// GetLatestAIReport 获取项目最新的 AI 分析报告
func (r *ResultRepo) GetLatestAIReport(ctx context.Context, projectID string) (*model.AIReportResult, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	rec, err := r.client.UxProjectAiReport.Query().
		Where(
			uxprojectaireport.ProjectID(pid),
			uxprojectaireport.AiSummaryNEQ(""),
		).
		Order(ent.Desc(uxprojectaireport.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return nil, err
	}
	return &model.AIReportResult{
		ReportID:          rec.ID.String(),
		ProjectID:         rec.ProjectID.String(),
		AISummary:         rec.AiSummary,
		AIStrengths:       rec.AiStrengths,
		AIWeaknesses:      rec.AiWeaknesses,
		AIRecommendations: rec.AiRecommendations,
		ModelChannel:      rec.ModelChannel,
		SessionIDs:        rec.SessionIds,
		CreatedAt:         rec.CreatedAt,
	}, nil
}

// SaveHTMLReport 保存 AI 生成的 HTML 可视化报告
func (r *ResultRepo) SaveHTMLReport(ctx context.Context, projectID, modelChannel, htmlContent string, sessionIDs []string) error {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return err
	}
	if sessionIDs == nil {
		sessionIDs = []string{}
	}
	return r.client.UxProjectAiReport.Create().
		SetProjectID(pid).
		SetModelChannel(modelChannel).
		SetSessionIds(sessionIDs).
		SetAiSummary("").
		SetAiStrengths([]string{}).
		SetAiWeaknesses([]string{}).
		SetAiRecommendations([]string{}).
		SetHTMLContent(htmlContent).
		Exec(ctx)
}

// GetLatestHTMLReport 获取项目最新的 HTML 可视化报告
func (r *ResultRepo) GetLatestHTMLReport(ctx context.Context, projectID string) (*model.HTMLReportResult, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	rec, err := r.client.UxProjectAiReport.Query().
		Where(
			uxprojectaireport.ProjectID(pid),
			uxprojectaireport.HTMLContentNEQ(""),
		).
		Order(ent.Desc(uxprojectaireport.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		return nil, err
	}
	return &model.HTMLReportResult{
		ReportID:     rec.ID.String(),
		ProjectID:    rec.ProjectID.String(),
		HTML:         rec.HTMLContent,
		ModelChannel: rec.ModelChannel,
		SessionIDs:   rec.SessionIds,
		CreatedAt:    rec.CreatedAt,
	}, nil
}

// 强类型枚举转换辅助
func parseAnswerOrigin(s string) uxquestionnaireanswer.AnswerOrigin {
	if s == "AFTER_ALL_PER_PROFILE" {
		return uxquestionnaireanswer.AnswerOriginAFTER_ALL_PER_PROFILE
	}
	return uxquestionnaireanswer.AnswerOriginAFTER_TASK
}

// keep imports
var _ = uxexecutionbatch.HasRun
