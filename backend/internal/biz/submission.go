package biz

import (
	"context"
	"photo-backend/internal/data"
)

type SubmissionUsecase struct {
	repo     *data.SubmissionRepo
	taskRepo *data.TaskRepo
}

func NewSubmissionUsecase(repo *data.SubmissionRepo, taskRepo *data.TaskRepo) *SubmissionUsecase {
	return &SubmissionUsecase{repo: repo, taskRepo: taskRepo}
}

func (uc *SubmissionUsecase) CreateSubmission(ctx context.Context, sub *data.Submission) error {
	sub.Status = "submitted"
	sub.AIEvaluation = data.AIEvaluation{Status: "pending", Score: 0}
	return uc.repo.Create(ctx, sub)
}

func (uc *SubmissionUsecase) ListSubmissions(ctx context.Context, taskID string, userID string) ([]*data.Submission, error) {
	// 获取任务信息，判断当前用户是否为创建者
	task, err := uc.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// 如果是任务创建者，返回所有提交
	if task.UserID == userID {
		return uc.repo.FindByTaskID(ctx, taskID)
	}

	// 如果不是创建者，只返回该用户自己的提交
	return uc.repo.FindByTaskIDAndUserID(ctx, taskID, userID)
}
