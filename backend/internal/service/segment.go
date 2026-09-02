package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type SegmentService struct {
	qiniu  *QiniuService
	aliyun *AliyunImageSegService
}

func NewSegmentService(qiniu *QiniuService, aliyun *AliyunImageSegService) *SegmentService {
	return &SegmentService{qiniu: qiniu, aliyun: aliyun}
}

type SegmentRequest struct {
	OSSKey string `json:"oss_key"`
}

func (s *SegmentService) UploadPolicy(w http.ResponseWriter, r *http.Request) {
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
