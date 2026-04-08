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

func (uc *SubmissionUsecase) ListSubmissions(ctx context.Context, taskID string) ([]*data.Submission, error) {
	return uc.repo.FindByTaskID(ctx, taskID)
}
