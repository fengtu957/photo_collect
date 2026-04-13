package biz

import (
	"context"
	"errors"
	"photo-backend/internal/data"
	"sort"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaskUsecase struct {
	repo    *data.TaskRepo
	subRepo *data.SubmissionRepo
}

func NewTaskUsecase(repo *data.TaskRepo, subRepo *data.SubmissionRepo) *TaskUsecase {
	return &TaskUsecase{repo: repo, subRepo: subRepo}
}

func validateTask(task *data.Task) error {
	if task == nil {
		return errors.New("任务不能为空")
	}
	if task.AIAnalysisEnabled == nil {
		enabled := true
		task.AIAnalysisEnabled = &enabled
	}
	if task.PhotoSpec.MaxSizeKB < 0 {
		return errors.New("文件大小限制不能小于 0")
	}
	return nil
}

func (uc *TaskUsecase) CreateTask(ctx context.Context, task *data.Task) error {
	if err := validateTask(task); err != nil {
		return err
	}
	task.Enabled = true
	task.Stats = data.TaskStats{TotalSubmissions: 0}
	return uc.repo.Create(ctx, task)
}

func (uc *TaskUsecase) UpdateTask(ctx context.Context, id string, userID string, task *data.Task) error {
	existing, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("任务不存在")
	}
	if existing.UserID != userID {
		return errors.New("无权限编辑此任务")
	}

	task.ID = existing.ID
	task.UserID = existing.UserID
	task.Enabled = existing.Enabled
	task.Stats = existing.Stats
	task.CreatedAt = existing.CreatedAt
	if task.AIAnalysisEnabled == nil {
		task.AIAnalysisEnabled = existing.AIAnalysisEnabled
	}
	if err := validateTask(task); err != nil {
		return err
	}

	return uc.repo.Update(ctx, id, task)
}

func (uc *TaskUsecase) GetTask(ctx context.Context, id string) (*data.Task, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TaskUsecase) ListTasks(ctx context.Context, userID string) ([]*data.Task, error) {
	// 1. 我创建的任务
	created, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. 我参与的任务（有提交记录的任务ID）
	participatedIDs, err := uc.subRepo.FindDistinctTaskIDsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. 过滤掉已在创建列表中的任务ID，避免重复查询
	createdSet := make(map[string]bool)
	for _, t := range created {
		createdSet[t.ID.Hex()] = true
	}
	var newIDs []primitive.ObjectID
	for _, oid := range participatedIDs {
		if !createdSet[oid.Hex()] {
			newIDs = append(newIDs, oid)
		}
	}

	// 4. 批量查询参与的任务
	participated, err := uc.repo.FindByIDs(ctx, newIDs)
	if err != nil {
		return nil, err
	}

	// 5. 合并并按创建时间倒序排序
	all := append(created, participated...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	// 6. 动态计算每个任务的提交数量
	for _, task := range all {
		count, err := uc.subRepo.CountByTaskID(ctx, task.ID.Hex())
		if err == nil {
			task.Stats.TotalSubmissions = int(count)
		}
	}

	return all, nil
}

func (uc *TaskUsecase) DeleteTask(ctx context.Context, id string, userID string) error {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("任务不存在")
	}
	if task.UserID != userID {
		return errors.New("无权限删除此任务")
	}
	if err := uc.subRepo.DeleteByTaskID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
