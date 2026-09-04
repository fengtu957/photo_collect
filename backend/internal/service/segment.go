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

type SegmentService struct {
	oss    *AliyunOSSService
	aliyun *AliyunImageSegService
	taskUC *biz.TaskUsecase
	vipUC  *biz.VIPUsecase
}

func NewSegmentService(oss *AliyunOSSService, aliyun *AliyunImageSegService, taskUC *biz.TaskUsecase, vipUC *biz.VIPUsecase) *SegmentService {
	return &SegmentService{oss: oss, aliyun: aliyun, taskUC: taskUC, vipUC: vipUC}
}

type SegmentRequest struct {
	TaskID string `json:"task_id"`
	OSSKey string `json:"oss_key"`
}

func (s *SegmentService) validateTaskAccess(taskID string) (*data.Task, error) {
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		return nil, errors.New("task_id 不能为空")
	}
	if s == nil || s.taskUC == nil || s.vipUC == nil {
		return nil, errors.New("自动换背景服务未配置")
	}

	task, err := s.taskUC.GetTask(context.Background(), normalizedTaskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}
	if !task.IsBackgroundReplacementEnabled() {
		return nil, errors.New("当前任务未开启自动换背景")
	}

	entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), task.UserID)
	if err != nil {
		return nil, err
	}
	if entitlements == nil || !entitlements.IsVIP || !entitlements.Limits.CanUseBackgroundReplacement {
		return nil, errors.New("当前任务创建者未激活VIP，无法使用自动换背景")
	}
	return task, nil
}

func (s *SegmentService) Segment(w http.ResponseWriter, r *http.Request) {
	var req SegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 2013, err.Error())
		return
	}
	task, err := s.validateTaskAccess(req.TaskID)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	key := strings.TrimSpace(req.OSSKey)
	userID, _ := r.Context().Value(UserIDKey).(string)
	if s == nil || s.oss == nil || task == nil || !s.oss.IsOwnedTemporaryKey(userID, task.ID.Hex(), key) {
		Error(w, 2013, "photo_key 无效")
		return
	}
	if s.aliyun == nil {
		Error(w, 2014, "人体分割服务未配置")
		return
	}
	ossURL, err := s.oss.GetFileURLWithTTL(key, 10*time.Minute)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	if _, err := s.oss.ProbeObject(key); err != nil {
		Error(w, 2014, err.Error())
		return
	}
	resultURL, err := s.aliyun.SegmentBody(ossURL)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	Success(w, map[string]interface{}{"result_url": resultURL, "expires_in": 1800})
}
