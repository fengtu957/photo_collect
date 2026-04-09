package biz

import (
	"context"
	"errors"
	"photo-backend/internal/data"

	"go.mongodb.org/mongo-driver/mongo"
)

type SubmissionUsecase struct {
	repo     *data.SubmissionRepo
	taskRepo *data.TaskRepo
}

func NewSubmissionUsecase(repo *data.SubmissionRepo, taskRepo *data.TaskRepo) *SubmissionUsecase {
	return &SubmissionUsecase{repo: repo, taskRepo: taskRepo}
}

func (uc *SubmissionUsecase) CreateSubmission(ctx context.Context, sub *data.Submission) error {
	// 查任务，判断当前用户是否为创建者
	task, err := uc.taskRepo.FindByID(ctx, sub.TaskID.Hex())
	if err != nil {
		return err
	}

	// 非创建者限制唯一提交
	if task.UserID != sub.UserID {
		existing, err := uc.repo.FindOneByTaskIDAndUserID(ctx, sub.TaskID.Hex(), sub.UserID)
		if err == nil && existing != nil {
			return errors.New("该任务已提交，请使用更新接口")
		}
	}

	sub.Status = "submitted"
	sub.AIEvaluation = data.AIEvaluation{Status: "pending", Score: 0}
	return uc.repo.Create(ctx, sub)
}

func (uc *SubmissionUsecase) UpdateSubmission(ctx context.Context, id string, userID string, sub *data.Submission) error {
	// 获取原有提交记录
	existing, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("提交记录不存在")
		}
		return err
	}

	// 获取任务信息
	task, err := uc.taskRepo.FindByID(ctx, existing.TaskID.Hex())
	if err != nil {
		return err
	}

	// 权限检查：只有提交者本人或任务创建者可以更新
	if existing.UserID != userID && task.UserID != userID {
		return errors.New("无权限更新此提交")
	}

	// 保留原有的 ID、TaskID、UserID、CreatedAt
	sub.ID = existing.ID
	sub.TaskID = existing.TaskID
	sub.UserID = existing.UserID
	sub.CreatedAt = existing.CreatedAt

	// 更新时重置 AI 评估状态
	sub.Status = "submitted"
	sub.AIEvaluation = data.AIEvaluation{Status: "pending", Score: 0}

	return uc.repo.Update(ctx, id, sub)
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
