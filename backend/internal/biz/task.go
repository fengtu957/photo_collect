package biz

import (
	"context"
	"photo-backend/internal/data"
)

type TaskUsecase struct {
	repo *data.TaskRepo
}

func NewTaskUsecase(repo *data.TaskRepo) *TaskUsecase {
	return &TaskUsecase{repo: repo}
}

func (uc *TaskUsecase) CreateTask(ctx context.Context, task *data.Task) error {
	task.Enabled = true
	task.Stats = data.TaskStats{TotalSubmissions: 0}
	return uc.repo.Create(ctx, task)
}

func (uc *TaskUsecase) GetTask(ctx context.Context, id string) (*data.Task, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TaskUsecase) ListTasks(ctx context.Context, userID string) ([]*data.Task, error) {
	return uc.repo.FindByUserID(ctx, userID)
}
