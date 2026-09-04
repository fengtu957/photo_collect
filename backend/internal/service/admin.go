package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

const (
	maxTaskAdminCount        = 50
	maxTaskCollaboratorCount = 50
)

type adminTaskItem struct {
	*data.Task
	VIPStatusText string `json:"vip_status_text,omitempty"`
}

type AdminService struct {
	vipUC    *biz.VIPUsecase
	taskRepo *data.TaskRepo

	jwtSecret string
	username  string
	password  string
}

func NewAdminService(vipUC *biz.VIPUsecase, taskRepo *data.TaskRepo) *AdminService {
	return &AdminService{
		vipUC:     vipUC,
		taskRepo:  taskRepo,
		jwtSecret: os.Getenv("JWT_SECRET"),
		username:  os.Getenv("ADMIN_USERNAME"),
		password:  os.Getenv("ADMIN_PASSWORD"),
	}
}

func (s *AdminService) Login(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.jwtSecret) == "" {
		Error(w, 9103, "JWT_SECRET 未配置")
		return
	}

	if strings.TrimSpace(s.username) == "" || strings.TrimSpace(s.password) == "" {
		Error(w, 9104, "管理员账号未配置")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 9105, err.Error())
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password
	if subtle.ConstantTimeCompare([]byte(username), []byte(s.username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) != 1 {
		Error(w, 9106, "账号或密码错误")
		return
	}

	token, err := s.generateAdminToken(username)
	if err != nil {
		Error(w, 9107, "生成管理员令牌失败")
		return
	}

	Success(w, map[string]string{
		"token":    token,
		"username": username,
	})
}

func (s *AdminService) ListTasks(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))

	result, err := s.taskRepo.AdminListTasks(context.Background(), data.AdminTaskListQuery{
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		Error(w, 9108, err.Error())
		return
	}

	vipStatusByUserID := make(map[string]string)
	items := make([]*adminTaskItem, 0, len(result.Items))
	for _, task := range result.Items {
		if task == nil {
			continue
		}

		vipStatusText, exists := vipStatusByUserID[task.UserID]
		if !exists {
			entitlements, vipErr := s.vipUC.GetUserEntitlements(context.Background(), task.UserID)
			if vipErr == nil {
				vipStatusText = buildAdminVIPStatusText(entitlements)
			}
			vipStatusByUserID[task.UserID] = vipStatusText
		}

		items = append(items, &adminTaskItem{
			Task:          task,
			VIPStatusText: vipStatusText,
		})
	}

	Success(w, map[string]interface{}{
		"items":     items,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

func (s *AdminService) GrantVIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      string `json:"user_id"`
		PlanCode    string `json:"plan_code"`
		DurationDay int    `json:"duration_day"`
		Remark      string `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 9109, err.Error())
		return
	}

	planCode := strings.TrimSpace(req.PlanCode)
	if planCode == "" {
		planCode = "vip_admin"
	}

	membership, err := s.vipUC.GrantVIP(
		context.Background(),
		strings.TrimSpace(req.UserID),
		planCode,
		req.DurationDay,
		"admin",
		strings.TrimSpace(req.Remark),
	)
	if err != nil {
		Error(w, 9110, err.Error())
		return
	}

	entitlements, err := s.vipUC.GetUserEntitlements(context.Background(), membership.UserID)
	if err != nil {
		Error(w, 9111, err.Error())
		return
	}

	count, err := s.taskRepo.CountActiveByUserID(context.Background(), membership.UserID)
	if err != nil {
		Error(w, 9112, err.Error())
		return
	}
	entitlements.Usage.ActiveTaskCount = int(count)

	Success(w, map[string]interface{}{
		"membership":   membership,
		"entitlements": entitlements,
	})
}

func (s *AdminService) UpdateTaskAdmins(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(mux.Vars(r)["id"])
	var req struct {
		AdminUserIDs []string `json:"admin_user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 9113, err.Error())
		return
	}

	task, err := s.taskRepo.FindByID(r.Context(), taskID)
	if err != nil {
		Error(w, 9114, err.Error())
		return
	}
	if task == nil {
		Error(w, 9114, "任务不存在")
		return
	}

	adminUserIDs, err := normalizeTaskAdminUserIDs(task.UserID, req.AdminUserIDs)
	if err != nil {
		Error(w, 9113, err.Error())
		return
	}
	if err := s.taskRepo.UpdateAdminUserIDs(r.Context(), taskID, adminUserIDs); err != nil {
		Error(w, 9114, err.Error())
		return
	}

	Success(w, map[string]interface{}{
		"id":             taskID,
		"admin_user_ids": adminUserIDs,
	})
}

func (s *AdminService) UpdateTaskCollaborators(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(mux.Vars(r)["id"])
	var req struct {
		CollaboratorUserIDs []string `json:"collaborator_user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, 9113, err.Error())
		return
	}

	task, err := s.taskRepo.FindByID(r.Context(), taskID)
	if err != nil {
		Error(w, 9114, err.Error())
		return
	}
	if task == nil {
		Error(w, 9114, "任务不存在")
		return
	}

	collaboratorUserIDs, err := normalizeTaskCollaboratorUserIDs(task.UserID, task.AdminUserIDs, req.CollaboratorUserIDs)
	if err != nil {
		Error(w, 9113, err.Error())
		return
	}
	if err := s.taskRepo.UpdateCollaboratorUserIDs(r.Context(), taskID, collaboratorUserIDs); err != nil {
		Error(w, 9114, err.Error())
		return
	}

	Success(w, map[string]interface{}{
		"id":                    taskID,
		"collaborator_user_ids": collaboratorUserIDs,
	})
}

func normalizeTaskAdminUserIDs(ownerUserID string, values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		userID := strings.TrimSpace(value)
		if userID == "" || userID == ownerUserID || seen[userID] {
			continue
		}
		if len(userID) > 128 {
			return nil, errors.New("管理员 OpenID 长度不能超过128个字符")
		}
		if len(result) >= maxTaskAdminCount {
			return nil, errors.New("每个任务最多设置50个管理员")
		}
		seen[userID] = true
		result = append(result, userID)
	}
	return result, nil
}

func normalizeTaskCollaboratorUserIDs(ownerUserID string, adminUserIDs, values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	adminSet := make(map[string]bool, len(adminUserIDs))
	for _, adminUserID := range adminUserIDs {
		adminSet[adminUserID] = true
	}
	for _, value := range values {
		userID := strings.TrimSpace(value)
		if userID == "" || userID == ownerUserID || adminSet[userID] || seen[userID] {
			continue
		}
		if len(userID) > 128 {
			return nil, errors.New("协作者 OpenID 长度不能超过128个字符")
		}
		if len(result) >= maxTaskCollaboratorCount {
			return nil, errors.New("每个任务最多设置50个协作者")
		}
		seen[userID] = true
		result = append(result, userID)
	}
	return result, nil
}

func (s *AdminService) generateAdminToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":     "admin",
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(s.jwtSecret))
}

func buildAdminVIPStatusText(entitlements *biz.UserEntitlements) string {
	if entitlements == nil || !entitlements.IsVIP {
		return "当前状态：普通用户"
	}

	if entitlements.ExpireAt == nil || entitlements.ExpireAt.IsZero() {
		return "当前状态：VIP"
	}

	return "VIP 有效期至：" + entitlements.ExpireAt.Format("2006-01-02 15:04")
}
