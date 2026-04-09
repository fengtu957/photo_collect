package service

import (
	"context"
	"encoding/json"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"

	"github.com/gorilla/mux"
)

type SubmissionService struct {
	uc *biz.SubmissionUsecase
}

func NewSubmissionService(uc *biz.SubmissionUsecase) *SubmissionService {
	return &SubmissionService{uc: uc}
}

func (s *SubmissionService) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	var sub data.Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		Error(w, 2001, err.Error())
		return
	}

	// 从 context 中获取用户 ID（由 JWT 中间件注入）
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

func (s *SubmissionService) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["taskId"]

	// 从 context 中获取用户 ID（由 JWT 中间件注入）
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 2003, "unauthorized")
		return
	}

	subs, err := s.uc.ListSubmissions(context.Background(), taskID, userID)
	if err != nil {
		Error(w, 2003, err.Error())
		return
	}

	Success(w, subs)
}
