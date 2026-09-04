package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strings"
	"time"
)

type UploadService struct {
	oss    *AliyunOSSService
	taskUC *biz.TaskUsecase
	vipUC  *biz.VIPUsecase
}

type UploadPolicyRequest struct {
	TaskID            string `json:"task_id"`
	Purpose           string `json:"purpose"`
	SourceKey         string `json:"source_key"`
	VerificationToken string `json:"verification_token"`
}

type FinalizePhotoRequest struct {
	TaskID            string `json:"task_id"`
	SourceKey         string `json:"source_key"`
	FinalKey          string `json:"final_key"`
	VerificationToken string `json:"verification_token"`
}

type FinalizePhotoResponse struct {
	PhotoKey          string `json:"photo_key"`
	FileSize          int64  `json:"file_size"`
	VerificationToken string `json:"verification_token,omitempty"`
}

func NewUploadService(oss *AliyunOSSService, taskUC *biz.TaskUsecase, vipUC *biz.VIPUsecase) *UploadService {
	return &UploadService{oss: oss, taskUC: taskUC, vipUC: vipUC}
}

func (s *UploadService) CreateUploadPolicy(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || strings.TrimSpace(userID) == "" {
		Error(w, 2013, "unauthorized")
		return
	}

	var req UploadPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 2013, err.Error())
		return
	}

	task, err := s.getAvailableTask(req.TaskID)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}

	var policy *OSSUploadPolicy
	switch strings.TrimSpace(req.Purpose) {
	case "temporary":
		policy, err = s.oss.CreateTemporaryUploadPolicy(userID, task.ID.Hex())
	case "final":
		if !task.IsBackgroundReplacementEnabled() {
			// A task without AI or background replacement can upload directly to
			// its final key. AI tasks still need the temporary key for validation.
			if task.IsAIAnalysisEnabled() {
				Error(w, 2014, "当前任务需要先上传临时照片并完成 AI 检查")
				return
			}
			policy, err = s.oss.CreateFinalUploadPolicy(userID, task.ID.Hex())
			break
		}
		if err = s.validateBackgroundEntitlement(task); err != nil {
			Error(w, 2014, err.Error())
			return
		}
		if err = s.validateTemporarySource(task, userID, req.SourceKey, req.VerificationToken); err != nil {
			Error(w, 2014, err.Error())
			return
		}
		policy, err = s.oss.CreateFinalUploadPolicy(userID, task.ID.Hex())
	default:
		Error(w, 2013, "purpose 仅支持 temporary 或 final")
		return
	}
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	Success(w, policy)
}

func (s *UploadService) FinalizePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || strings.TrimSpace(userID) == "" {
		Error(w, 2013, "unauthorized")
		return
	}

	var req FinalizePhotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 2013, err.Error())
		return
	}
	task, err := s.getAvailableTask(req.TaskID)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	sourceKey := strings.TrimSpace(req.SourceKey)
	finalKey := strings.TrimSpace(req.FinalKey)
	if sourceKey == "" {
		// Direct final uploads are only allowed for tasks that do not require
		// AI verification or background processing.
		if task.IsAIAnalysisEnabled() || task.IsBackgroundReplacementEnabled() {
			Error(w, 2013, "source_key 不能为空")
			return
		}
		if !s.oss.IsOwnedFinalKey(userID, task.ID.Hex(), finalKey) {
			Error(w, 2013, "final_key 无效")
			return
		}
	} else {
		if err := s.validateTemporarySource(task, userID, sourceKey, req.VerificationToken); err != nil {
			Error(w, 2014, err.Error())
			return
		}
	}

	if task.IsBackgroundReplacementEnabled() {
		if err := s.validateBackgroundEntitlement(task); err != nil {
			Error(w, 2014, err.Error())
			return
		}
		if !s.oss.IsOwnedFinalKey(userID, task.ID.Hex(), finalKey) {
			Error(w, 2013, "final_key 无效")
			return
		}
	} else if sourceKey != "" {
		if finalKey != "" {
			Error(w, 2013, "当前任务的 final_key 必须为空")
			return
		}
		finalKey = s.oss.NewFinalPhotoKey(userID, task.ID.Hex())
		if err := s.oss.CopyObject(sourceKey, finalKey); err != nil {
			Error(w, 2014, err.Error())
			return
		}
	}

	objectInfo, err := s.oss.ProbeObject(finalKey)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	if err := validateOSSPhotoSize(task, objectInfo.Size); err != nil {
		Error(w, 2014, err.Error())
		return
	}

	response := FinalizePhotoResponse{PhotoKey: finalKey, FileSize: objectInfo.Size}
	if task.IsAIAnalysisEnabled() {
		response.VerificationToken = issuePhotoVerificationToken(userID, task.ID.Hex(), finalKey)
	}
	Success(w, response)
}

func (s *UploadService) getAvailableTask(taskID string) (*data.Task, error) {
	if s == nil || s.oss == nil || s.taskUC == nil {
		return nil, errors.New("OSS 上传服务未配置")
	}
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		return nil, errors.New("task_id 不能为空")
	}
	task, err := s.taskUC.GetTask(context.Background(), normalizedTaskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}
	if !task.Enabled {
		return nil, errors.New("任务已停用")
	}
	now := time.Now()
	if !task.StartTime.IsZero() && now.Before(task.StartTime) {
		return nil, errors.New("任务尚未开始")
	}
	if !task.EndTime.IsZero() && now.After(task.EndTime) {
		return nil, errors.New("任务已截止")
	}
	return task, nil
}

func (s *UploadService) validateTemporarySource(task *data.Task, userID string, sourceKey string, verificationToken string) error {
	key := strings.TrimSpace(sourceKey)
	if task == nil || !s.oss.IsOwnedTemporaryKey(userID, task.ID.Hex(), key) {
		return errors.New("source_key 无效")
	}
	if _, err := s.oss.ProbeObject(key); err != nil {
		return err
	}
	if task.IsAIAnalysisEnabled() {
		return verifyPhotoVerificationToken(verificationToken, userID, task.ID.Hex(), key)
	}
	return nil
}

func (s *UploadService) validateBackgroundEntitlement(task *data.Task) error {
	if task == nil || s.vipUC == nil {
		return errors.New("自动换背景服务未配置")
	}
	entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), task.UserID)
	if err != nil {
		return err
	}
	if entitlements == nil || !entitlements.IsVIP || !entitlements.Limits.CanUseBackgroundReplacement {
		return errors.New("当前任务创建者未激活VIP，无法使用自动换背景")
	}
	return nil
}

func validateOSSPhotoSize(task *data.Task, size int64) error {
	if size <= 0 {
		return errors.New("OSS 照片文件为空")
	}
	if task != nil && task.PhotoSpec.MaxSizeKB > 0 && size > int64(task.PhotoSpec.MaxSizeKB)*1024 {
		return errors.New("照片大小超过任务限制")
	}
	return nil
}
