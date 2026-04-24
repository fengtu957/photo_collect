package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
