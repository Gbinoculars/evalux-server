package service

import (
	"context"
	"errors"

	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

type PlanService struct {
	planRepo *repo.PlanRepo
	execRepo *repo.ExecutionRepo
	runRepo  *repo.ExecutionRunRepo
	permRepo *repo.UnifiedPermRepo
}

func NewPlanService(
	planRepo *repo.PlanRepo,
	execRepo *repo.ExecutionRepo,
	permRepo *repo.UnifiedPermRepo,
) *PlanService {
	return &PlanService{planRepo: planRepo, execRepo: execRepo, permRepo: permRepo}
}

// SetRunRepo 后续注入 ExecutionRunRepo
func (s *PlanService) SetRunRepo(runRepo *repo.ExecutionRunRepo) {
	s.runRepo = runRepo
}

// Create 创建执行计划
func (s *PlanService) Create(ctx context.Context, operatorID string, req model.CreatePlanRequest) (*model.PlanDetail, error) {
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
	if !canEdit {
		return nil, errors.New("无权在该项目下创建执行计划")
	}
	if req.PlanType != "NORMAL" && req.PlanType != "AB_TEST" && req.PlanType != "EXPERT" {
		return nil, errors.New("plan_type 只能为 NORMAL、AB_TEST 或 EXPERT")
	}
	return s.planRepo.Create(ctx, operatorID, req)
}

// ListByProject 列出项目下所有可用计划
func (s *PlanService) ListByProject(ctx context.Context, operatorID, projectID string) ([]*model.PlanDetail, error) {
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目")
	}
	return s.planRepo.ListByProject(ctx, projectID)
}

// GetByID 获取计划详情
func (s *PlanService) GetByID(ctx context.Context, operatorID, planID string) (*model.PlanDetail, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, errors.New("执行计划不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, plan.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该执行计划")
	}
	return plan, nil
}

// Update 更新执行计划
func (s *PlanService) Update(ctx context.Context, operatorID, planID string, req model.UpdatePlanRequest) (*model.PlanDetail, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, errors.New("执行计划不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, plan.ProjectID, "EDIT")
	if !canEdit {
		return nil, errors.New("无权修改该执行计划")
	}
	return s.planRepo.Update(ctx, planID, req)
}

// Delete 删除执行计划
func (s *PlanService) Delete(ctx context.Context, operatorID, planID string) error {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return errors.New("执行计划不存在")
	}
	canEdit, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, plan.ProjectID, "EDIT")
	if !canEdit {
		return errors.New("无权删除该执行计划")
	}
	return s.planRepo.Delete(ctx, planID)
}

// GetABTestResult 按 run_id 取 A/B 对比结果
// 第 1 轮先返回基础对照-实验对比；第 2 轮 service 完整实现报告聚合
func (s *PlanService) GetABTestResult(ctx context.Context, operatorID string, req model.ABTestStartRequest) (*model.ABTestResult, error) {
	if s.runRepo == nil {
		return nil, errors.New("执行运行仓储未注入")
	}
	run, err := s.runRepo.GetByID(ctx, req.RunID)
	if err != nil {
		return nil, errors.New("执行运行不存在")
	}
	canView, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, run.ProjectID, "VIEW")
	if !canView {
		return nil, errors.New("无权查看该项目")
	}
	if run.PlanType != "AB_TEST" {
		return nil, errors.New("该运行不是 A/B 测试运行")
	}

	var controlBatchID, treatmentBatchID string
	for _, b := range run.Batches {
		if b.BatchRole == "CONTROL" {
			controlBatchID = b.BatchID
		} else if b.BatchRole == "TREATMENT" {
			treatmentBatchID = b.BatchID
		}
	}
	controlSessions, _ := s.execRepo.ListSessionsByBatchID(ctx, controlBatchID)
	treatmentSessions, _ := s.execRepo.ListSessionsByBatchID(ctx, treatmentBatchID)

	control := buildGroupStats(controlSessions, controlBatchID, "CONTROL", "对照组")
	treatment := buildGroupStats(treatmentSessions, treatmentBatchID, "TREATMENT", "实验组")

	comparison := model.ABTestComparison{
		CompletionRateDiff: treatment.CompletionRate - control.CompletionRate,
		ErrorCountDiff:     treatment.AvgErrorCount - control.AvgErrorCount,
		DurationDiffMs:     treatment.AvgDurationMs - control.AvgDurationMs,
		ScoreDiff:          treatment.AvgScore - control.AvgScore,
	}
	diff := comparison.CompletionRateDiff
	scoreDiff := comparison.ScoreDiff
	if diff > 5 || scoreDiff > 0.5 {
		comparison.Winner = "TREATMENT"
	} else if diff < -5 || scoreDiff < -0.5 {
		comparison.Winner = "CONTROL"
	} else {
		comparison.Winner = "TIE"
	}

	return &model.ABTestResult{
		RunID:      run.RunID,
		PlanID:     run.PlanIDRef,
		PlanName:   run.PlanName,
		Hypothesis: run.Hypothesis,
		Control:    control,
		Treatment:  treatment,
		Comparison: comparison,
	}, nil
}

func buildGroupStats(sessions []model.SessionDetail, batchID, role, label string) model.ABTestGroupStats {
	stats := model.ABTestGroupStats{
		BatchID:      batchID,
		BatchRole:    role,
		Label:        label,
		SessionCount: len(sessions),
	}
	if len(sessions) == 0 {
		return stats
	}
	var completed int
	var totalErr int
	var totalDur int64
	for _, sess := range sessions {
		if sess.IsGoalCompleted {
			completed++
		}
		totalErr += sess.ErrorCount
		if sess.TotalDurationMs != nil {
			totalDur += *sess.TotalDurationMs
		}
	}
	n := float64(len(sessions))
	stats.CompletionRate = float64(completed) / n * 100
	stats.AvgErrorCount = float64(totalErr) / n
	stats.AvgDurationMs = totalDur / int64(len(sessions))
	return stats
}
