package biz

import (
	"context"
	"errors"
	"fmt"
	"photo-backend/internal/data"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaskUsecase struct {
	repo    *data.TaskRepo
	subRepo *data.SubmissionRepo
	vipUC   *VIPUsecase
}

const taskCodeLength = 5
const maxVerificationCodeLength = 32
const taskCodeGenerateRetries = 32

func NewTaskUsecase(repo *data.TaskRepo, subRepo *data.SubmissionRepo, vipUC *VIPUsecase) *TaskUsecase {
	return &TaskUsecase{repo: repo, subRepo: subRepo, vipUC: vipUC}
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func validateTask(task *data.Task) error {
	if task == nil {
		return errors.New("任务不能为空")
	}
	task.VerificationCode = strings.TrimSpace(task.VerificationCode)
	if task.AIAnalysisEnabled == nil {
		enabled := true
		task.AIAnalysisEnabled = &enabled
	}
	if task.VerificationCodeEnabled && task.VerificationCode == "" {
		return errors.New("开启校验码后必须填写数字校验码")
	}
	if task.VerificationCode != "" && !isDigitsOnly(task.VerificationCode) {
		return errors.New("校验码只能填写数字")
	}
	if len(task.VerificationCode) > maxVerificationCodeLength {
		return errors.New(fmt.Sprintf("校验码长度不能超过%d位", maxVerificationCodeLength))
	}
	if !task.VerificationCodeEnabled {
		task.VerificationCode = ""
	}
	if task.PhotoSpec.MaxSizeKB < 0 {
		return errors.New("文件大小限制不能小于 0")
	}
	if !task.StartTime.IsZero() && !task.EndTime.IsZero() && task.StartTime.After(task.EndTime) {
		return errors.New("开始时间不能晚于截止时间")
	}
	return nil
}

func validateTaskOpenDurationLimit(task *data.Task, maxDays int) error {
	if task == nil || task.EndTime.IsZero() || maxDays <= 0 {
		return nil
	}
	openStart := task.StartTime
	if openStart.IsZero() {
		openStart = time.Now()
	}
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	if task.EndTime.Sub(openStart) > maxDuration {
		return errors.New(fmt.Sprintf("开放时间最多只能设置%d天", maxDays))
	}
	return nil
}

func generateTaskCode() (string, error) {
	segment, err := randomCodeSegment("0123456789", taskCodeLength)
	if err != nil {
		return "", err
	}

	return segment, nil
}

func (uc *TaskUsecase) ensureTaskCode(ctx context.Context, task *data.Task) error {
	if task == nil {
		return nil
	}
	if strings.TrimSpace(task.TaskCode) != "" {
		task.TaskCode = strings.TrimSpace(task.TaskCode)
		if len(task.TaskCode) != taskCodeLength || !isDigitsOnly(task.TaskCode) {
			return errors.New(fmt.Sprintf("任务码必须是固定%d位数字", taskCodeLength))
		}
		return nil
	}

	for i := 0; i < taskCodeGenerateRetries; i++ {
		taskCode, err := generateTaskCode()
		if err != nil {
			return err
		}

		existing, err := uc.repo.FindByTaskCode(ctx, taskCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		task.TaskCode = taskCode
		return nil
	}

	return errors.New("生成任务码失败，请重试")
}

func (uc *TaskUsecase) CreateTask(ctx context.Context, task *data.Task) error {
	task.TaskCode = ""
	if err := validateTask(task); err != nil {
		return err
	}
	if err := uc.ensureTaskCode(ctx, task); err != nil {
		return err
	}
	if uc.vipUC != nil {
		entitlements, err := uc.vipUC.GetUserEntitlements(ctx, task.UserID)
		if err != nil {
			return err
		}
		if !entitlements.IsVIP {
			activeCount, err := uc.repo.CountActiveByUserID(ctx, task.UserID)
			if err != nil {
				return err
			}
			if entitlements.Limits.MaxActiveTasks > 0 && int(activeCount) >= entitlements.Limits.MaxActiveTasks {
				return errors.New(fmt.Sprintf("普通用户最多创建%d个未结束任务，激活VIP后不受限制", entitlements.Limits.MaxActiveTasks))
			}
			if err := validateTaskOpenDurationLimit(task, entitlements.Limits.MaxOpenDurationDays); err != nil {
				return err
			}
			if task.AIAnalysisEnabled != nil && *task.AIAnalysisEnabled {
				return errors.New("AI分析仅限VIP开启")
			}
			task.MaxSubmissions = entitlements.Limits.MaxSubmissionsPerTask
		} else {
			if err := validateTaskOpenDurationLimit(task, entitlements.Limits.MaxOpenDurationDays); err != nil {
				return err
			}
			task.MaxSubmissions = 0
		}
	}
	task.Enabled = true
	task.Stats = data.TaskStats{TotalSubmissions: 0}
	for i := 0; i < 3; i++ {
		err := uc.repo.Create(ctx, task)
		if err == nil {
			return nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		task.TaskCode = ""
		if genErr := uc.ensureTaskCode(ctx, task); genErr != nil {
			return genErr
		}
	}

	return errors.New("创建任务失败，请重试")
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
	task.TaskCode = existing.TaskCode
	task.StartTime = existing.StartTime
	task.MaxSubmissions = existing.MaxSubmissions
	if task.AIAnalysisEnabled == nil {
		task.AIAnalysisEnabled = existing.AIAnalysisEnabled
	}
	if err := validateTask(task); err != nil {
		return err
	}
	if err := uc.ensureTaskCode(ctx, task); err != nil {
		return err
	}
	if uc.vipUC != nil {
		entitlements, err := uc.vipUC.GetUserEntitlements(ctx, userID)
		if err != nil {
			return err
		}
		if err := validateTaskOpenDurationLimit(task, entitlements.Limits.MaxOpenDurationDays); err != nil {
			return err
		}
		if entitlements.IsVIP {
			task.MaxSubmissions = 0
		} else {
			task.MaxSubmissions = entitlements.Limits.MaxSubmissionsPerTask
		}
		existingAIEnabled := existing.AIAnalysisEnabled != nil && *existing.AIAnalysisEnabled
		nextAIEnabled := task.AIAnalysisEnabled != nil && *task.AIAnalysisEnabled
		if !entitlements.IsVIP && nextAIEnabled && !existingAIEnabled {
			return errors.New("AI分析仅限VIP开启")
		}
	}

	return uc.repo.Update(ctx, id, task)
}

func (uc *TaskUsecase) GetTask(ctx context.Context, id string) (*data.Task, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TaskUsecase) GetTaskByCode(ctx context.Context, taskCode string) (*data.Task, error) {
	normalized := strings.TrimSpace(taskCode)
	if len(normalized) != taskCodeLength || !isDigitsOnly(normalized) {
		return nil, errors.New(fmt.Sprintf("任务码必须是固定%d位数字", taskCodeLength))
	}

	task, err := uc.repo.FindByTaskCode(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}

	return task, nil
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
