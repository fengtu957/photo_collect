package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type SubmissionService struct {
	uc       *biz.SubmissionUsecase
	taskUC   *biz.TaskUsecase
	vipUC    *biz.VIPUsecase
	evalUC   *biz.EvaluationUsecase
	qiniuSvc *QiniuService
}

type AnalyzePreviewRequest struct {
	TaskID string `json:"task_id"`
	Photo  struct {
		URL string `json:"url"`
	} `json:"photo"`
}

func NewSubmissionService(uc *biz.SubmissionUsecase, taskUC *biz.TaskUsecase, vipUC *biz.VIPUsecase, evalUC *biz.EvaluationUsecase, qiniuSvc *QiniuService) *SubmissionService {
	return &SubmissionService{uc: uc, taskUC: taskUC, vipUC: vipUC, evalUC: evalUC, qiniuSvc: qiniuSvc}
}

func buildPhotoSpecText(task *data.Task) string {
	if task == nil {
		return ""
	}

	parts := make([]string, 0, 3)
	if task.PhotoSpec.Name != "" {
		parts = append(parts, "规格名称："+task.PhotoSpec.Name)
	}
	if task.PhotoSpec.Width > 0 && task.PhotoSpec.Height > 0 {
		parts = append(parts, "照片比例："+buildPhotoRatioText(task.PhotoSpec.Width, task.PhotoSpec.Height))
	}
	if task.PhotoSpec.BackgroundColor != "" {
		parts = append(parts, "背景色要求："+task.PhotoSpec.BackgroundColor)
	}

	return strings.Join(parts, "；")
}

func buildPhotoRatioText(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	divisor := greatestCommonDivisor(width, height)
	if divisor <= 0 {
		return strconv.Itoa(width) + ":" + strconv.Itoa(height)
	}

	return strconv.Itoa(width/divisor) + ":" + strconv.Itoa(height/divisor)
}

func greatestCommonDivisor(a int, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}

	for b != 0 {
		a, b = b, a%b
	}

	if a == 0 {
		return 1
	}

	return a
}

func (s *SubmissionService) evaluateTaskPhoto(taskID string, photoKey string) (*biz.EvaluationResult, error) {
	task, err := s.taskUC.GetTask(context.Background(), taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, nil
	}
	if !task.IsAIAnalysisEnabled() {
		return nil, errors.New("当前任务未开启AI分析")
	}
	if s.vipUC != nil {
		entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), task.UserID)
		if err != nil {
			return nil, err
		}
		if !entitlements.IsVIP {
			return nil, errors.New("当前任务创建者未开通VIP，无法使用AI分析")
		}
	}

	photoURL := s.qiniuSvc.GetFileURLWithTTL(photoKey, 10*time.Minute)
	return s.evalUC.EvaluatePhoto(context.Background(), photoURL, buildPhotoSpecText(task))
}

func (s *SubmissionService) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	var sub data.Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		Error(w, 2001, err.Error())
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2001, "unauthorized")
		return
	}
	sub.UserID = userID

	if err := s.uc.CreateSubmission(context.Background(), &sub); err != nil {
		Error(w, 2002, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": sub.ID.Hex()})
}

func (s *SubmissionService) UpdateSubmission(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var sub data.Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		Error(w, 2004, err.Error())
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2004, "unauthorized")
		return
	}

	if err := s.uc.UpdateSubmission(context.Background(), id, userID, &sub); err != nil {
		Error(w, 2005, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": id})
}

func (s *SubmissionService) DeleteSubmission(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2008, "unauthorized")
		return
	}

	if err := s.uc.DeleteSubmission(context.Background(), id, userID); err != nil {
		Error(w, 2009, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": id})
}

func (s *SubmissionService) AnalyzePreview(w http.ResponseWriter, r *http.Request) {
	_, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2011, "unauthorized")
		return
	}

	var req AnalyzePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 2011, err.Error())
		return
	}
	if req.TaskID == "" {
		Error(w, 2011, "task_id 不能为空")
		return
	}
	if req.Photo.URL == "" {
		Error(w, 2011, "照片不存在")
		return
	}

	result, err := s.evaluateTaskPhoto(req.TaskID, req.Photo.URL)
	if err != nil {
		Error(w, 2012, err.Error())
		return
	}
	if result == nil {
		Error(w, 2012, "任务不存在")
		return
	}

	Success(w, result)
}

func (s *SubmissionService) GetSubmission(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2006, "unauthorized")
		return
	}

	submission, err := s.uc.GetSubmission(context.Background(), id, userID)
	if err != nil {
		Error(w, 2007, err.Error())
		return
	}

	if submission.Photo.URL != "" {
		submission.Photo.URL = s.qiniuSvc.GetFileURL(submission.Photo.URL)
	}

	Success(w, submission)
}

func (s *SubmissionService) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["taskId"]

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2003, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, err := s.uc.ListSubmissions(context.Background(), taskID, userID, page, limit)
	if err != nil {
		Error(w, 2003, err.Error())
		return
	}

	// 转换 photo.url 从 key 到完整的签名 URL
	for i := range result.List {
		if result.List[i].Photo.URL != "" {
			result.List[i].Photo.URL = s.qiniuSvc.GetFileURL(result.List[i].Photo.URL)
		}
	}

	Success(w, result)
}
