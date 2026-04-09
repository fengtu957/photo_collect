package service

import (
	"context"
	"encoding/json"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strconv"

	"github.com/gorilla/mux"
)

type SubmissionService struct {
	uc       *biz.SubmissionUsecase
	qiniuSvc *QiniuService
}

func NewSubmissionService(uc *biz.SubmissionUsecase, qiniuSvc *QiniuService) *SubmissionService {
	return &SubmissionService{uc: uc, qiniuSvc: qiniuSvc}
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
