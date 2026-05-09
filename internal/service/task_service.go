package service

import (
	"context"
	"errors"

	"evalux-server/internal/model"
	"evalux-server/internal/repo"
)

// TaskService 任务管理服务。本次重构后任务退化为纯任务定义，
// 不再持有"任务-画像"或"任务-问卷"绑定，绑定关系全部上提到 PlanService。
type TaskService struct {
	taskRepo *repo.TaskRepo
	permRepo *repo.UnifiedPermRepo
}

func NewTaskService(taskRepo *repo.TaskRepo, permRepo *repo.UnifiedPermRepo) *TaskService {
	return &TaskService{taskRepo: taskRepo, permRepo: permRepo}
}

func (s *TaskService) Create(ctx context.Context, operatorID string, req model.CreateTaskRequest) (*model.TaskDetail, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, req.ProjectID, "EDIT")
	if !ok {
		return nil, errors.New("无权在该项目下创建任务")
	}
	return s.taskRepo.Create(ctx, req)
}

func (s *TaskService) List(ctx context.Context, operatorID, projectID string, query model.TaskListQuery) (*model.TaskListResponse, error) {
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, projectID, "VIEW")
	if !ok {
		return nil, errors.New("无权查看该项目任务")
	}
	tasks, total, err := s.taskRepo.ListByProjectID(ctx, projectID, query)
	if err != nil {
		return nil, errors.New("查询任务列表失败")
	}
	return &model.TaskListResponse{Total: total, List: tasks}, nil
}

func (s *TaskService) GetByID(ctx context.Context, operatorID, taskID string) (*model.TaskDetail, error) {
	detail, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, errors.New("任务不存在")
	}
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "VIEW")
	if !ok {
		return nil, errors.New("无权查看该任务")
	}
	return detail, nil
}

func (s *TaskService) Update(ctx context.Context, operatorID, taskID string, req model.UpdateTaskRequest) (*model.TaskDetail, error) {
	detail, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, errors.New("任务不存在")
	}
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
	if !ok {
		return nil, errors.New("无权编辑该任务")
	}
	return s.taskRepo.Update(ctx, taskID, req)
}

func (s *TaskService) Delete(ctx context.Context, operatorID, taskID string) error {
	detail, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return errors.New("任务不存在")
	}
	ok, _ := s.permRepo.CheckProjectAccess(ctx, operatorID, detail.ProjectID, "EDIT")
	if !ok {
		return errors.New("无权删除该任务")
	}
	return s.taskRepo.Delete(ctx, taskID)
}
