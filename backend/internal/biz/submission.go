package biz

import (
	"context"
	"errors"
	"fmt"
	"photo-backend/internal/data"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type SubmissionUsecase struct {
	repo     *data.SubmissionRepo
	taskRepo *data.TaskRepo
	vipUC    *VIPUsecase
}

func NewSubmissionUsecase(repo *data.SubmissionRepo, taskRepo *data.TaskRepo, vipUC *VIPUsecase) *SubmissionUsecase {
	return &SubmissionUsecase{repo: repo, taskRepo: taskRepo, vipUC: vipUC}
}

func validateTaskAvailableForSubmission(task *data.Task) error {
	if task == nil {
		return errors.New("任务不存在")
	}
	if !task.Enabled {
		return errors.New("任务已停用")
	}

	now := time.Now()
	if !task.StartTime.IsZero() && now.Before(task.StartTime) {
		return errors.New("任务尚未开始")
	}
	if !task.EndTime.IsZero() && now.After(task.EndTime) {
		return errors.New("任务已截止")
	}

	return nil
}

func validateSubmissionPhoto(task *data.Task, sub *data.Submission) error {
	if task == nil || sub == nil {
		return nil
	}
	if task.PhotoSpec.MaxSizeKB <= 0 {
		return nil
	}
	limitBytes := int64(task.PhotoSpec.MaxSizeKB) * 1024
	if sub.Photo.FileSize > 0 && sub.Photo.FileSize > limitBytes {
		return errors.New("照片大小超过任务限制")
	}
	return nil
}

func isSupportedUniqueFieldType(fieldType string) bool {
	return fieldType == "text" || fieldType == "number" || fieldType == "select"
}

func normalizeCustomFieldValue(fieldType string, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if fieldType == "text" || fieldType == "number" || fieldType == "select" {
			return strings.TrimSpace(v)
		}
		return value
	case []string:
		if fieldType != "multiselect" {
			return value
		}
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, strings.TrimSpace(item))
		}
		return result
	case []interface{}:
		if fieldType != "multiselect" {
			return value
		}
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, strings.TrimSpace(s))
				continue
			}
			result = append(result, item)
		}
		return result
	default:
		return value
	}
}

func normalizeSubmissionCustomData(task *data.Task, customData map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})
	for key, value := range customData {
		normalized[key] = value
	}
	if task == nil {
		return normalized
	}
	for _, field := range task.CustomFields {
		value, ok := normalized[field.ID]
		if !ok {
			continue
		}
		normalized[field.ID] = normalizeCustomFieldValue(field.Type, value)
	}
	return normalized
}

func normalizeUniqueComparableValue(fieldType string, value interface{}) string {
	if !isSupportedUniqueFieldType(fieldType) || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (uc *SubmissionUsecase) validateUniqueCustomFields(ctx context.Context, task *data.Task, sub *data.Submission, excludeSubmissionID string) error {
	if task == nil || sub == nil || len(task.CustomFields) == 0 {
		return nil
	}

	hasUniqueField := false
	for _, field := range task.CustomFields {
		if field.Unique && isSupportedUniqueFieldType(field.Type) {
			hasUniqueField = true
			break
		}
	}
	if !hasUniqueField {
		return nil
	}

	submissions, err := uc.repo.FindAllByTaskID(ctx, sub.TaskID.Hex())
	if err != nil {
		return err
	}

	for _, field := range task.CustomFields {
		if !field.Unique || !isSupportedUniqueFieldType(field.Type) {
			continue
		}

		candidate := normalizeUniqueComparableValue(field.Type, sub.CustomData[field.ID])
		if candidate == "" {
			continue
		}

		for _, existing := range submissions {
			if existing == nil {
				continue
			}
			if excludeSubmissionID != "" && existing.ID.Hex() == excludeSubmissionID {
				continue
			}
			if normalizeUniqueComparableValue(field.Type, existing.CustomData[field.ID]) == candidate {
				return errors.New(field.Label + "已被占用，请更换后再提交")
			}
		}
	}

	return nil
}

func (uc *SubmissionUsecase) CreateSubmission(ctx context.Context, sub *data.Submission) error {
	// 查任务，判断当前用户是否为创建者
	task, err := uc.taskRepo.FindByID(ctx, sub.TaskID.Hex())
	if err != nil {
		return err
	}
	if err := validateTaskAvailableForSubmission(task); err != nil {
		return err
	}

	// 非创建者限制唯一提交
	if task.UserID != sub.UserID {
		existing, err := uc.repo.FindOneByTaskIDAndUserID(ctx, sub.TaskID.Hex(), sub.UserID)
		if err == nil && existing != nil {
			return errors.New("该任务已提交，请使用更新接口")
		}
	}
	if err := validateSubmissionPhoto(task, sub); err != nil {
		return err
	}
	if uc.vipUC != nil {
		entitlements, err := uc.vipUC.GetUserEntitlements(ctx, task.UserID)
		if err != nil {
			return err
		}
		if !entitlements.IsVIP && entitlements.Limits.MaxSubmissionsPerTask > 0 {
			total, err := uc.repo.CountByTaskID(ctx, sub.TaskID.Hex())
			if err != nil {
				return err
			}
			if int(total) >= entitlements.Limits.MaxSubmissionsPerTask {
				return errors.New(fmt.Sprintf("当前任务已达到免费版%d人收集上限，开通VIP后可继续收集", entitlements.Limits.MaxSubmissionsPerTask))
			}
		}
	}
	sub.CustomData = normalizeSubmissionCustomData(task, sub.CustomData)
	if err := uc.validateUniqueCustomFields(ctx, task, sub, ""); err != nil {
		return err
	}

	sub.Status = "submitted"
	sub.AIEvaluation = data.AIEvaluation{}
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
	if err := validateTaskAvailableForSubmission(task); err != nil {
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
	if sub.Photo.FileSize == 0 {
		sub.Photo.FileSize = existing.Photo.FileSize
	}
	if sub.Photo.Width == 0 {
		sub.Photo.Width = existing.Photo.Width
	}
	if sub.Photo.Height == 0 {
		sub.Photo.Height = existing.Photo.Height
	}
	if err := validateSubmissionPhoto(task, sub); err != nil {
		return err
	}
	sub.CustomData = normalizeSubmissionCustomData(task, sub.CustomData)
	if err := uc.validateUniqueCustomFields(ctx, task, sub, existing.ID.Hex()); err != nil {
		return err
	}

	// 当前 AI 分析为即时返回，不再持久化写回 submission
	sub.Status = "submitted"
	sub.AIEvaluation = data.AIEvaluation{}

	return uc.repo.Update(ctx, id, sub)
}

func (uc *SubmissionUsecase) GetSubmission(ctx context.Context, id string, userID string) (*data.Submission, error) {
	submission, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("提交记录不存在")
		}
		return nil, err
	}

	task, err := uc.taskRepo.FindByID(ctx, submission.TaskID.Hex())
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}

	if submission.UserID != userID && task.UserID != userID {
		return nil, errors.New("无权限查看此提交")
	}

	return submission, nil
}

func (uc *SubmissionUsecase) DeleteSubmission(ctx context.Context, id string, userID string) error {
	submission, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("提交记录不存在")
		}
		return err
	}

	task, err := uc.taskRepo.FindByID(ctx, submission.TaskID.Hex())
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("任务不存在")
	}
	if task.UserID != userID {
		return errors.New("只有创建者可以删除提交记录")
	}

	return uc.repo.Delete(ctx, id)
}

type SubmissionListResult struct {
	List    []*data.Submission `json:"list"`
	Total   int64              `json:"total"`
	HasMore bool               `json:"has_more"`
}

func (uc *SubmissionUsecase) ListSubmissions(ctx context.Context, taskID string, userID string, page, limit int) (*SubmissionListResult, error) {
	task, err := uc.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}

	var list []*data.Submission
	var total int64

	if task.UserID == userID {
		// 创建者：返回所有提交
		list, err = uc.repo.FindByTaskID(ctx, taskID, page, limit)
		if err != nil {
			return nil, err
		}
		total, err = uc.repo.CountByTaskID(ctx, taskID)
	} else {
		// 非创建者：只返回自己的提交
		list, err = uc.repo.FindByTaskIDAndUserID(ctx, taskID, userID, page, limit)
		if err != nil {
			return nil, err
		}
		total, err = uc.repo.CountByTaskIDAndUserID(ctx, taskID, userID)
	}
	if err != nil {
		return nil, err
	}

	return &SubmissionListResult{
		List:    list,
		Total:   total,
		HasMore: int64(page*limit) < total,
	}, nil
}
