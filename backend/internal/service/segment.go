package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"photo-backend/internal/biz"
	"strings"
	"time"
)

type SegmentService struct {
	qiniu  *QiniuService
	aliyun *AliyunImageSegService
	taskUC *biz.TaskUsecase
	vipUC  *biz.VIPUsecase
}

func NewSegmentService(qiniu *QiniuService, aliyun *AliyunImageSegService, taskUC *biz.TaskUsecase, vipUC *biz.VIPUsecase) *SegmentService {
	return &SegmentService{qiniu: qiniu, aliyun: aliyun, taskUC: taskUC, vipUC: vipUC}
}

type SegmentRequest struct {
	TaskID string `json:"task_id"`
	OSSKey string `json:"oss_key"`
}

func (s *SegmentService) validateTaskAccess(taskID string) error {
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		return errors.New("task_id 不能为空")
	}
	if s == nil || s.taskUC == nil || s.vipUC == nil {
		return errors.New("自动换背景服务未配置")
	}

	task, err := s.taskUC.GetTask(context.Background(), normalizedTaskID)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("任务不存在")
	}
	if !task.IsBackgroundReplacementEnabled() {
		return errors.New("当前任务未开启自动换背景")
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

func (s *SegmentService) UploadPolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.validateTaskAccess(r.URL.Query().Get("task_id")); err != nil {
		Error(w, 2014, err.Error())
		return
	}
	userID, _ := r.Context().Value(UserIDKey).(string)
	if s == nil || s.aliyun == nil {
		Error(w, 2014, "人体分割服务未配置")
		return
	}
	policy, err := s.aliyun.CreateOSSUploadPolicy(userID)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	Success(w, policy)
}

func (s *SegmentService) Segment(w http.ResponseWriter, r *http.Request) {
	var req SegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 2013, err.Error())
		return
	}
	if err := s.validateTaskAccess(req.TaskID); err != nil {
		Error(w, 2014, err.Error())
		return
	}
	key := strings.TrimSpace(req.OSSKey)
	if key == "" || strings.Contains(key, "..") || strings.Contains(key, "://") {
		Error(w, 2013, "photo_key 无效")
		return
	}
	if s == nil || s.qiniu == nil || s.aliyun == nil {
		Error(w, 2014, "人体分割服务未配置")
		return
	}
	ossURL, err := s.aliyun.GetOSSFileURL(key, 10*time.Minute)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	if err := s.aliyun.ProbeOSSFile(key); err != nil {
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
