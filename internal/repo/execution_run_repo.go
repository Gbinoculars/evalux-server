package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/uxexecutionbatch"
	"evalux-server/ent/uxexecutionrun"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

type ExecutionRunRepo struct {
	client *ent.Client
}

func NewExecutionRunRepo(client *ent.Client) *ExecutionRunRepo {
	return &ExecutionRunRepo{client: client}
}

// ListByProject 列出项目下所有 run（按启动时间倒序），并附带 batch 列表。
func (r *ExecutionRunRepo) ListByProject(ctx context.Context, projectID string) ([]*model.ExecutionRunDetail, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, err
	}
	rows, err := r.client.UxExecutionRun.Query().
		Where(uxexecutionrun.ProjectID(pid)).
		WithBatches().
		Order(ent.Desc(uxexecutionrun.FieldStartedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ExecutionRunDetail, 0, len(rows))
	for _, row := range rows {
		result = append(result, entRunToDetail(row))
	}
	return result, nil
}

// GetByID 取指定 run 的详情
func (r *ExecutionRunRepo) GetByID(ctx context.Context, runID string) (*model.ExecutionRunDetail, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	row, err := r.client.UxExecutionRun.Query().
		Where(uxexecutionrun.ID(rid)).
		WithBatches().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return entRunToDetail(row), nil
}

// ListBatchesByRunID 按 run_id 取批次
func (r *ExecutionRunRepo) ListBatchesByRunID(ctx context.Context, runID string) ([]model.ExecutionBatchDTO, error) {
	rid, err := uuid.Parse(runID)
	if err != nil {
		return nil, err
	}
	batches, err := r.client.UxExecutionBatch.Query().
		Where(uxexecutionbatch.RunID(rid)).
		Order(ent.Asc(uxexecutionbatch.FieldBatchRole)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ExecutionBatchDTO, 0, len(batches))
	for _, b := range batches {
		list = append(list, model.ExecutionBatchDTO{
			BatchID:   b.ID,
			RunID:     b.RunID.String(),
			BatchRole: string(b.BatchRole),
			CreatedAt: b.CreatedAt,
		})
	}
	return list, nil
}

func entRunToDetail(row *ent.UxExecutionRun) *model.ExecutionRunDetail {
	d := &model.ExecutionRunDetail{
		RunID:      row.ID.String(),
		ProjectID:  row.ProjectID.String(),
		PlanName:   row.PlanNameSnapshot,
		PlanType:   string(row.PlanTypeSnapshot),
		Status:     string(row.Status),
		StartedBy:  row.StartedBy.String(),
		StartedAt:  row.StartedAt,
		FinishedAt: row.FinishedAt,
	}
	if row.PlanIDRef != nil {
		d.PlanIDRef = row.PlanIDRef.String()
	}
	if row.HypothesisSnapshot != nil {
		d.Hypothesis = *row.HypothesisSnapshot
	}
	if batches := row.Edges.Batches; batches != nil {
		bs := make([]model.ExecutionBatchDTO, 0, len(batches))
		for _, b := range batches {
			bs = append(bs, model.ExecutionBatchDTO{
				BatchID:   b.ID,
				RunID:     b.RunID.String(),
				BatchRole: string(b.BatchRole),
				CreatedAt: b.CreatedAt,
			})
		}
		d.Batches = bs
	}
	return d
}
