package biz

import (
	"context"
	"photo-backend/internal/data"
)

type TaskUsecase struct {
	repo    *data.TaskRepo
	subRepo *data.SubmissionRepo
}

func NewTaskUsecase(repo *data.TaskRepo, subRepo *data.SubmissionRepo) *TaskUsecase {
	return &TaskUsecase{repo: repo, subRepo: subRepo}
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
	tasks, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 动态计算每个任务的提交数量
	for _, task := range tasks {
		count, err := uc.subRepo.CountByTaskID(ctx, task.ID.Hex())
		if err == nil {
			task.Stats.TotalSubmissions = int(count)
		}
	}

	return tasks, nil
}
