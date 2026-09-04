package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type SubmissionService struct {
	uc      *biz.SubmissionUsecase
	taskUC  *biz.TaskUsecase
	vipUC   *biz.VIPUsecase
	evalUC  *biz.EvaluationUsecase
	ossSvc  *AliyunOSSService
	authSvc *AuthService
}

const submissionThumbnailProcess = "image/resize,m_lfit,w_320,h_320/quality,q_70"

type AnalyzePreviewRequest struct {
	TaskID string `json:"task_id"`
	Photo  struct {
		URL string `json:"url"`
	} `json:"photo"`
}

type RejectionNotificationRequest struct {
	ReviewStatus string `json:"review_status"`
	Prompt       string `json:"prompt"`
}

func (s *SubmissionService) validatePhotoVerification(r *http.Request, sub *data.Submission) error {
	if sub == nil || sub.TaskID.IsZero() || sub.Photo.URL == "" {
		return errors.New("照片不存在")
	}
	task, err := s.taskUC.GetTask(context.Background(), sub.TaskID.Hex())
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("任务不存在")
	}
	userID, _ := r.Context().Value(UserIDKey).(string)
	if s.ossSvc == nil || !s.ossSvc.IsOwnedFinalKey(userID, sub.TaskID.Hex(), sub.Photo.URL) {
		return errors.New("照片存储路径无效")
	}
	objectInfo, err := s.ossSvc.ProbeObject(sub.Photo.URL)
	if err != nil {
		return err
	}
	if err := validateOSSPhotoSize(task, objectInfo.Size); err != nil {
		return err
	}
	sub.Photo.FileSize = objectInfo.Size
	if task != nil && task.IsAIAnalysisEnabled() {
		if err := verifyPhotoVerificationToken(sub.VerificationToken, userID, sub.TaskID.Hex(), sub.Photo.URL); err != nil {
			return err
		}
	}
	return nil
}

func NewSubmissionService(uc *biz.SubmissionUsecase, taskUC *biz.TaskUsecase, vipUC *biz.VIPUsecase, evalUC *biz.EvaluationUsecase, ossSvc *AliyunOSSService, authSvc *AuthService) *SubmissionService {
	return &SubmissionService{uc: uc, taskUC: taskUC, vipUC: vipUC, evalUC: evalUC, ossSvc: ossSvc, authSvc: authSvc}
}

func buildPhotoSpecText(task *data.Task) string {
	if task == nil {
		return "照片类型：标准证件照"
	}

	parts := make([]string, 0, 5)
	parts = append(parts, "照片类型：标准证件照")
	if task.PhotoSpec.Name != "" {
		parts = append(parts, "规格名称："+task.PhotoSpec.Name)
	}
	if task.PhotoSpec.Width > 0 && task.PhotoSpec.Height > 0 {
		parts = append(parts,
			"目标尺寸："+strconv.Itoa(task.PhotoSpec.Width)+"×"+strconv.Itoa(task.PhotoSpec.Height)+"毫米",
			"目标比例："+buildPhotoRatioText(task.PhotoSpec.Width, task.PhotoSpec.Height),
		)
	}
	if task.PhotoSpec.BackgroundColor != "" && !task.IsBackgroundReplacementEnabled() {
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

func (s *SubmissionService) evaluateTaskPhoto(userID string, taskID string, photoKey string) (*biz.EvaluationResult, error) {
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
	if s.ossSvc == nil || !s.ossSvc.IsOwnedTemporaryKey(userID, task.ID.Hex(), photoKey) {
		return nil, errors.New("照片临时存储路径无效")
	}
	objectInfo, err := s.ossSvc.ProbeObject(photoKey)
	if err != nil {
		return nil, err
	}
	if err := validateOSSPhotoSize(task, objectInfo.Size); err != nil {
		return nil, err
	}
	if s.vipUC != nil {
		entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), task.UserID)
		if err != nil {
			return nil, err
		}
		if entitlements == nil || !entitlements.IsVIP || !entitlements.Limits.CanUseAIAnalysis {
			return nil, errors.New("当前任务创建者未激活VIP，无法使用AI分析")
		}
	}

	photoURL, err := s.ossSvc.GetFileURLWithTTL(photoKey, 10*time.Minute)
	if err != nil {
		return nil, err
	}
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
	if err := s.validatePhotoVerification(r, &sub); err != nil {
		Error(w, 2002, err.Error())
		return
	}

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
	if sub.TaskID.IsZero() || sub.Photo.URL == "" {
		Error(w, 2005, "照片不存在")
		return
	}
	// Editing an unchanged existing photo does not need a new preview token.
	existing, existingErr := s.uc.GetSubmission(context.Background(), id, userID)
	if existingErr != nil || existing == nil || existing.Photo.URL != sub.Photo.URL {
		if err := s.validatePhotoVerification(r, &sub); err != nil {
			Error(w, 2005, err.Error())
			return
		}
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

func (s *SubmissionService) GetRejectionNotificationConfig(w http.ResponseWriter, r *http.Request) {
	templateID := ""
	if s.authSvc != nil {
		templateID = s.authSvc.RejectionTemplateID()
	}
	Success(w, map[string]interface{}{
		"enabled":     templateID != "",
		"template_id": templateID,
	})
}

func (s *SubmissionService) SendRejectionNotification(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2013, "unauthorized")
		return
	}
	if s.authSvc == nil {
		Error(w, 2014, "审核通知服务不可用")
		return
	}
	req := RejectionNotificationRequest{
		ReviewStatus: "图片不合格",
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		Error(w, 2014, "通知内容无效")
		return
	}

	submission, task, err := s.uc.GetSubmissionForTaskCreator(context.Background(), id, userID)
	if err != nil {
		Error(w, 2014, err.Error())
		return
	}
	if err := s.authSvc.SendRejectionSubscribeMessage(submission.UserID, task.ID.Hex(), submission.ID.Hex(), req.ReviewStatus, req.Prompt); err != nil {
		Error(w, 2014, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": id})
}

func (s *SubmissionService) AnalyzePreview(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
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

	result, err := s.evaluateTaskPhoto(userID, req.TaskID, req.Photo.URL)
	if err != nil {
		Error(w, 2012, err.Error())
		return
	}
	if result == nil {
		Error(w, 2012, "任务不存在")
		return
	}
	if result.CanSubmit && result.AnalysisStatus == "success" {
		resultMap := map[string]interface{}{}
		encoded, _ := json.Marshal(result)
		_ = json.Unmarshal(encoded, &resultMap)
		resultMap["verification_token"] = issuePhotoVerificationToken(userID, req.TaskID, req.Photo.URL)
		Success(w, resultMap)
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
		fileURL, err := s.ossSvc.GetFileURLWithTTL(submission.Photo.URL, time.Hour)
		if err != nil {
			Error(w, 2007, err.Error())
			return
		}
		submission.Photo.URL = fileURL
	}

	Success(w, submission)
}

func (s *SubmissionService) AuthorizeSubmissionPhotoLink(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2007, "unauthorized")
		return
	}
	if s.ossSvc == nil {
		Error(w, 2007, "图片下载服务不可用")
		return
	}

	submission, _, err := s.uc.GetSubmissionForTaskCreator(context.Background(), id, userID)
	if err != nil {
		Error(w, 2007, err.Error())
		return
	}
	photoKey := strings.TrimSpace(submission.Photo.URL)
	if photoKey == "" || submission.Photo.Deleted {
		Error(w, 2007, "该提交没有已上传的图片")
		return
	}

	downloadURL, err := s.ossSvc.GetFileURLWithTTL(photoKey, time.Hour)
	if err != nil {
		Error(w, 2007, err.Error())
		return
	}
	Success(w, map[string]interface{}{
		"download_url": downloadURL,
		"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
	})
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
			photoKey := result.List[i].Photo.URL
			fileURL, err := s.ossSvc.GetFileURLWithTTL(photoKey, time.Hour)
			if err != nil {
				Error(w, 2003, err.Error())
				return
			}
			thumbnailURL, err := s.ossSvc.GetProcessedImageURLWithTTL(photoKey, submissionThumbnailProcess, time.Hour)
			if err != nil {
				Error(w, 2003, err.Error())
				return
			}
			result.List[i].Photo.URL = fileURL
			result.List[i].Photo.ThumbnailURL = thumbnailURL
		}
	}

	Success(w, result)
}
