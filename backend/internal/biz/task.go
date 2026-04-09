package biz

import (
	"context"
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

func (uc *TaskUsecase) CreateTask(ctx context.Context, task *data.Task) error {
	task.Enabled = true
	task.Stats = data.TaskStats{TotalSubmissions: 0}
	return uc.repo.Create(ctx, task)
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
