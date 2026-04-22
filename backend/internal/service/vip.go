package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strings"
)

type VIPService struct {
	vipUC    *biz.VIPUsecase
	taskRepo *data.TaskRepo
}

func NewVIPService(vipUC *biz.VIPUsecase, taskRepo *data.TaskRepo) *VIPService {
	return &VIPService{vipUC: vipUC, taskRepo: taskRepo}
}

func applyVIPContact(entitlements *biz.UserEntitlements) {
	if entitlements == nil {
		return
	}

	contactLabel := strings.TrimSpace(os.Getenv("VIP_CONTACT_LABEL"))
	contactValue := strings.TrimSpace(os.Getenv("VIP_CONTACT_VALUE"))
	entitlements.ContactLabel = contactLabel
	entitlements.ContactValue = contactValue
}

func (s *VIPService) GetEntitlements(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 3001, "unauthorized")
		return
	}

	entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), userID)
	if err != nil {
		Error(w, 3001, err.Error())
		return
	}

	count, err := s.taskRepo.CountActiveByUserID(context.Background(), userID)
	if err != nil {
		Error(w, 3001, err.Error())
		return
	}
	entitlements.Usage.ActiveTaskCount = int(count)
	applyVIPContact(entitlements)

	Success(w, entitlements)
}

func (s *VIPService) RedeemCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 3002, "unauthorized")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 3002, err.Error())
		return
	}

	entitlements, err := s.vipUC.RedeemCode(context.Background(), userID, req.Code)
	if err != nil {
		Error(w, 3003, err.Error())
		return
	}

	count, err := s.taskRepo.CountActiveByUserID(context.Background(), userID)
	if err != nil {
		Error(w, 3003, err.Error())
		return
	}
	entitlements.Usage.ActiveTaskCount = int(count)
	applyVIPContact(entitlements)

	Success(w, entitlements)
}
