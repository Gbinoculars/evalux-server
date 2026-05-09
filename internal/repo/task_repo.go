package repo

import (
	"context"

	"evalux-server/ent"
	"evalux-server/ent/uxtask"
	"evalux-server/internal/model"

	"github.com/google/uuid"
)

// TaskRepo 任务表仓储。本次重构后任务退化为纯任务定义，
// 不再持有任何"任务-画像""任务-问卷"绑定，绑定关系全部上提到 ux_execution_plan 层。
type TaskRepo struct {
	client *ent.Client
}

func NewTaskRepo(client *ent.Client) *TaskRepo {
	return &TaskRepo{client: client}
}

func (r *TaskRepo) Create(ctx context.Context, req model.CreateTaskRequest) (*model.TaskDetail, error) {
	pid, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return nil, err
	}
	b := r.client.UxTask.Create().
		SetProjectID(pid).
		SetTaskName(req.TaskName).
		SetTaskGoal(req.TaskGoal).
		SetSuccessCriteria(req.SuccessCriteria).
		SetTimeoutSeconds(req.TimeoutSeconds).
		SetSortOrder(req.SortOrder).
		SetStatus("ACTIVE")
	if req.Precondition != "" {
		b.SetPrecondition(req.Precondition)
	}
	if req.ExecutionGuide != "" {
		b.SetExecutionGuide(req.ExecutionGuide)
	}
	if req.StepConstraints != nil {
		b.SetStepConstraints(req.StepConstraints)
	}
	if req.FailureRule != "" {
		b.SetFailureRule(req.FailureRule)
	}
	if req.MinSteps != nil {
		b.SetMinSteps(*req.MinSteps)
	}
	if req.MaxSteps != nil {
		b.SetMaxSteps(*req.MaxSteps)
	}
	t, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entTaskToDetail(t), nil
}

func (r *TaskRepo) FindByID(ctx context.Context, taskID string) (*model.TaskDetail, error) {
	tid, err := uuid.Parse(taskID)
	if err != nil {
		return nil, err
	}
	t, err := r.client.UxTask.Get(ctx, tid)
	if err != nil {
		return nil, err
	}
	return entTaskToDetail(t), nil
}

func (r *TaskRepo) Update(ctx context.Context, taskID string, req model.UpdateTaskRequest) (*model.TaskDetail, error) {
	tid, err := uuid.Parse(taskID)
	if err != nil {
		return nil, err
	}
	upd := r.client.UxTask.UpdateOneID(tid)
	if req.TaskName != nil {
		upd.SetTaskName(*req.TaskName)
	}
	if req.TaskGoal != nil {
		upd.SetTaskGoal(*req.TaskGoal)
	}
	if req.Precondition != nil {
		upd.SetPrecondition(*req.Precondition)
	}
	if req.ExecutionGuide != nil {
		upd.SetExecutionGuide(*req.ExecutionGuide)
	}
	if req.StepConstraints != nil {
		upd.SetStepConstraints(req.StepConstraints)
	}
	if req.SuccessCriteria != nil {
		upd.SetSuccessCriteria(*req.SuccessCriteria)
	}
	if req.FailureRule != nil {
		upd.SetFailureRule(*req.FailureRule)
	}
	if req.TimeoutSeconds != nil {
		upd.SetTimeoutSeconds(*req.TimeoutSeconds)
	}
	if req.MinSteps != nil {
		upd.SetMinSteps(*req.MinSteps)
	}
	if req.MaxSteps != nil {
		upd.SetMaxSteps(*req.MaxSteps)
	}
	if req.SortOrder != nil {
		upd.SetSortOrder(*req.SortOrder)
	}
	if req.Status != nil {
		upd.SetStatus(uxtask.Status(*req.Status))
	}
	t, err := upd.Save(ctx)
	if err != nil {
		return nil, err
	}
	return entTaskToDetail(t), nil
}

func (r *TaskRepo) Delete(ctx context.Context, taskID string) error {
	tid, err := uuid.Parse(taskID)
	if err != nil {
		return err
	}
	return r.client.UxTask.DeleteOneID(tid).Exec(ctx)
}

func (r *TaskRepo) ListByProjectID(ctx context.Context, projectID string, query model.TaskListQuery) ([]model.TaskDetail, int64, error) {
	pid, err := uuid.Parse(projectID)
	if err != nil {
		return nil, 0, err
	}
	q := r.client.UxTask.Query().Where(uxtask.ProjectID(pid))
	if query.Status != "" {
		q = q.Where(uxtask.StatusEQ(uxtask.Status(query.Status)))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	offset := (page - 1) * pageSize
	tasks, err := q.Order(ent.Asc(uxtask.FieldSortOrder)).
		Limit(pageSize).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	list := make([]model.TaskDetail, 0, len(tasks))
	for _, t := range tasks {
		list = append(list, *entTaskToDetail(t))
	}
	return list, int64(total), nil
}

// FindByIDsActive 批量查询启用的任务定义（供 plan 启动时构造 run 快照使用）。
func (r *TaskRepo) FindByIDsActive(ctx context.Context, taskIDs []uuid.UUID) ([]*ent.UxTask, error) {
	if len(taskIDs) == 0 {
		return []*ent.UxTask{}, nil
	}
	return r.client.UxTask.Query().
		Where(uxtask.IDIn(taskIDs...)).
		All(ctx)
}

func entTaskToDetail(t *ent.UxTask) *model.TaskDetail {
	d := &model.TaskDetail{
		TaskID:          t.ID.String(),
		ProjectID:       t.ProjectID.String(),
		TaskName:        t.TaskName,
		TaskGoal:        t.TaskGoal,
		SuccessCriteria: t.SuccessCriteria,
		TimeoutSeconds:  t.TimeoutSeconds,
		SortOrder:       t.SortOrder,
		Status:          string(t.Status),
		CreatedAt:       t.CreatedAt,
	}
	if t.Precondition != nil {
		d.Precondition = *t.Precondition
	}
	if t.ExecutionGuide != nil {
		d.ExecutionGuide = *t.ExecutionGuide
	}
	if t.StepConstraints != nil {
		d.StepConstraints = t.StepConstraints
	}
	if t.FailureRule != nil {
		d.FailureRule = *t.FailureRule
	}
	if t.MinSteps != nil {
		d.MinSteps = t.MinSteps
	}
	if t.MaxSteps != nil {
		d.MaxSteps = t.MaxSteps
	}
	return d
}
