package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"photo-backend/internal/data"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskUsecase struct {
	repo    *data.TaskRepo
	subRepo *data.SubmissionRepo
	vipUC   *VIPUsecase
}

const taskCodeLength = 5
const maxVerificationCodeLength = 32
const taskCodeGenerateRetries = 32
const maxTaskOpenDurationDays = 30

const (
	TaskInvitationRoleAdmin        = "admin"
	TaskInvitationRoleCollaborator = "collaborator"
)

type TaskInvitationInfo struct {
	TaskID    string `json:"task_id"`
	TaskTitle string `json:"task_title"`
	Role      string `json:"role"`
	RoleText  string `json:"role_text"`
	Token     string `json:"token,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Valid     bool   `json:"valid"`
	Accepted  bool   `json:"accepted,omitempty"`
}

func normalizeTaskInvitationRole(role string) (string, bool) {
	normalized := strings.TrimSpace(role)
	if normalized == TaskInvitationRoleAdmin || normalized == TaskInvitationRoleCollaborator {
		return normalized, true
	}
	return normalized, false
}

func taskInvitationRoleText(role string) string {
	if role == TaskInvitationRoleAdmin {
		return "管理员"
	}
	return "协作员"
}

func findTaskInvitation(task *data.Task, token string) *data.TaskInvitation {
	if task == nil {
		return nil
	}
	for i := range task.Invitations {
		if task.Invitations[i].Token == token {
			return &task.Invitations[i]
		}
	}
	return nil
}

func generateTaskInvitationToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func normalizeReplacementBackgroundColor(value string) (string, bool) {
	normalized := strings.TrimSpace(value)
	if normalized == "白底" || normalized == "蓝底" || normalized == "红底" {
		return normalized, true
	}
	if len(normalized) == 7 && normalized[0] == '#' {
		for i := 1; i < len(normalized); i++ {
			char := normalized[i]
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return normalized, false
			}
		}
		return strings.ToUpper(normalized), true
	}
	return normalized, false
}

func NewTaskUsecase(repo *data.TaskRepo, subRepo *data.SubmissionRepo, vipUC *VIPUsecase) *TaskUsecase {
	return &TaskUsecase{repo: repo, subRepo: subRepo, vipUC: vipUC}
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func validateTask(task *data.Task) error {
	if task == nil {
		return errors.New("任务不能为空")
	}
	task.VerificationCode = strings.TrimSpace(task.VerificationCode)
	if task.AIAnalysisEnabled == nil {
		enabled := true
		task.AIAnalysisEnabled = &enabled
	}
	if task.BackgroundReplacementEnabled == nil {
		enabled := false
		task.BackgroundReplacementEnabled = &enabled
	}
	if task.IsBackgroundReplacementEnabled() {
		backgroundColor, valid := normalizeReplacementBackgroundColor(task.PhotoSpec.BackgroundColor)
		if !valid {
			return errors.New("开启自动换背景后，请选择白底、蓝底、红底或填写有效的 RGB 颜色")
		}
		task.PhotoSpec.BackgroundColor = backgroundColor
	}
	if task.VerificationCodeEnabled && task.VerificationCode == "" {
		return errors.New("开启校验码后必须填写数字校验码")
	}
	if task.VerificationCode != "" && !isDigitsOnly(task.VerificationCode) {
		return errors.New("校验码只能填写数字")
	}
	if len(task.VerificationCode) > maxVerificationCodeLength {
		return errors.New(fmt.Sprintf("校验码长度不能超过%d位", maxVerificationCodeLength))
	}
	if !task.VerificationCodeEnabled {
		task.VerificationCode = ""
	}
	if task.PhotoSpec.MaxSizeKB < 0 {
		return errors.New("文件大小限制不能小于 0")
	}
	if !task.StartTime.IsZero() && !task.EndTime.IsZero() && task.StartTime.After(task.EndTime) {
		return errors.New("开始时间不能晚于截止时间")
	}
	return nil
}

func validateTaskOpenDurationLimitFrom(task *data.Task, fallbackStart time.Time, maxDays int) error {
	if task == nil || task.EndTime.IsZero() || maxDays <= 0 {
		return nil
	}
	openStart := task.StartTime
	if openStart.IsZero() {
		openStart = fallbackStart
	}
	if openStart.IsZero() {
		openStart = time.Now()
	}
	if task.EndTime.Before(openStart) {
		return errors.New("截止时间不能早于开始时间")
	}
	maxDuration := time.Duration(maxDays) * 24 * time.Hour
	if task.EndTime.Sub(openStart) > maxDuration {
		return errors.New(fmt.Sprintf("开放时间最多只能设置%d天", maxDays))
	}
	return nil
}

func validateTaskOpenDurationLimit(task *data.Task, maxDays int) error {
	return validateTaskOpenDurationLimitFrom(task, time.Now(), maxDays)
}

func generateTaskCode() (string, error) {
	segment, err := randomCodeSegment("0123456789", taskCodeLength)
	if err != nil {
		return "", err
	}

	return segment, nil
}

func (uc *TaskUsecase) ensureTaskCode(ctx context.Context, task *data.Task) error {
	if task == nil {
		return nil
	}
	if strings.TrimSpace(task.TaskCode) != "" {
		task.TaskCode = strings.TrimSpace(task.TaskCode)
		if len(task.TaskCode) != taskCodeLength || !isDigitsOnly(task.TaskCode) {
			return errors.New(fmt.Sprintf("任务码必须是固定%d位数字", taskCodeLength))
		}
		return nil
	}

	for i := 0; i < taskCodeGenerateRetries; i++ {
		taskCode, err := generateTaskCode()
		if err != nil {
			return err
		}

		existing, err := uc.repo.FindByTaskCode(ctx, taskCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		task.TaskCode = taskCode
		return nil
	}

	return errors.New("生成任务码失败，请重试")
}

func (uc *TaskUsecase) CreateTask(ctx context.Context, task *data.Task) error {
	task.TaskCode = ""
	if err := validateTask(task); err != nil {
		return err
	}
	if err := validateTaskOpenDurationLimit(task, maxTaskOpenDurationDays); err != nil {
		return err
	}
	if err := uc.ensureTaskCode(ctx, task); err != nil {
		return err
	}
	if uc.vipUC != nil {
		entitlements, err := uc.vipUC.GetUserEntitlements(ctx, task.UserID)
		if err != nil {
			return err
		}
		if !entitlements.IsVIP {
			activeCount, err := uc.repo.CountActiveByUserID(ctx, task.UserID)
			if err != nil {
				return err
			}
			if entitlements.Limits.MaxActiveTasks > 0 && int(activeCount) >= entitlements.Limits.MaxActiveTasks {
				return errors.New(fmt.Sprintf("普通用户最多创建%d个未结束任务，激活VIP后不受限制", entitlements.Limits.MaxActiveTasks))
			}
			if err := validateTaskOpenDurationLimit(task, entitlements.Limits.MaxOpenDurationDays); err != nil {
				return err
			}
			if task.AIAnalysisEnabled != nil && *task.AIAnalysisEnabled {
				return errors.New("AI分析仅限VIP开启")
			}
			if task.IsBackgroundReplacementEnabled() {
				return errors.New("自动换背景仅限VIP开启")
			}
			task.MaxSubmissions = entitlements.Limits.MaxSubmissionsPerTask
		} else {
			if err := validateTaskOpenDurationLimit(task, entitlements.Limits.MaxOpenDurationDays); err != nil {
				return err
			}
			task.MaxSubmissions = 0
		}
	}
	task.Enabled = true
	task.Stats = data.TaskStats{TotalSubmissions: 0}
	for i := 0; i < 3; i++ {
		err := uc.repo.Create(ctx, task)
		if err == nil {
			return nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		task.TaskCode = ""
		if genErr := uc.ensureTaskCode(ctx, task); genErr != nil {
			return genErr
		}
	}

	return errors.New("创建任务失败，请重试")
}

func (uc *TaskUsecase) UpdateTask(ctx context.Context, id string, userID string, task *data.Task) error {
	existing, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("任务不存在")
	}
	if !existing.CanManage(userID) {
		return errors.New("无权限编辑此任务")
	}

	task.ID = existing.ID
	task.UserID = existing.UserID
	task.AdminUserIDs = existing.AdminUserIDs
	task.CollaboratorUserIDs = existing.CollaboratorUserIDs
	task.Enabled = existing.Enabled
	task.Stats = existing.Stats
	task.CreatedAt = existing.CreatedAt
	task.TaskCode = existing.TaskCode
	if existing.StartTime.IsZero() || !time.Now().Before(existing.StartTime) {
		task.StartTime = existing.StartTime
	}
	task.MaxSubmissions = existing.MaxSubmissions
	if task.AIAnalysisEnabled == nil {
		task.AIAnalysisEnabled = existing.AIAnalysisEnabled
	}
	if task.BackgroundReplacementEnabled == nil {
		task.BackgroundReplacementEnabled = existing.BackgroundReplacementEnabled
	}
	if err := validateTask(task); err != nil {
		return err
	}
	if err := validateTaskOpenDurationLimitFrom(task, existing.CreatedAt, maxTaskOpenDurationDays); err != nil {
		return err
	}
	if err := uc.ensureTaskCode(ctx, task); err != nil {
		return err
	}
	if uc.vipUC != nil {
		entitlements, err := uc.vipUC.GetUserEntitlements(ctx, existing.UserID)
		if err != nil {
			return err
		}
		if err := validateTaskOpenDurationLimitFrom(task, existing.CreatedAt, entitlements.Limits.MaxOpenDurationDays); err != nil {
			return err
		}
		if entitlements.IsVIP {
			task.MaxSubmissions = 0
		} else {
			task.MaxSubmissions = entitlements.Limits.MaxSubmissionsPerTask
		}
		existingAIEnabled := existing.AIAnalysisEnabled != nil && *existing.AIAnalysisEnabled
		nextAIEnabled := task.AIAnalysisEnabled != nil && *task.AIAnalysisEnabled
		if !entitlements.IsVIP && nextAIEnabled && !existingAIEnabled {
			return errors.New("AI分析仅限VIP开启")
		}
		existingBackgroundEnabled := existing.IsBackgroundReplacementEnabled()
		nextBackgroundEnabled := task.IsBackgroundReplacementEnabled()
		if !entitlements.IsVIP && nextBackgroundEnabled && !existingBackgroundEnabled {
			return errors.New("自动换背景仅限VIP开启")
		}
	}

	return uc.repo.Update(ctx, id, task)
}

func (uc *TaskUsecase) GetTask(ctx context.Context, id string) (*data.Task, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TaskUsecase) GetTaskByCode(ctx context.Context, taskCode string) (*data.Task, error) {
	normalized := strings.TrimSpace(taskCode)
	if len(normalized) != taskCodeLength || !isDigitsOnly(normalized) {
		return nil, errors.New(fmt.Sprintf("任务码必须是固定%d位数字", taskCodeLength))
	}

	task, err := uc.repo.FindByTaskCode(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}

	return task, nil
}

func (uc *TaskUsecase) ListTasks(ctx context.Context, userID string) ([]*data.Task, error) {
	// 1. 我创建的任务
	created, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. 我管理的任务
	managed, err := uc.repo.FindByAdminUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. 我协作的任务
	collaborating, err := uc.repo.FindByCollaboratorUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 4. 我参与的任务（有提交记录的任务ID）
	participatedIDs, err := uc.subRepo.FindDistinctTaskIDsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 5. 过滤掉已在创建、管理或协作列表中的任务ID，避免重复查询
	accessibleSet := make(map[string]bool)
	for _, t := range created {
		accessibleSet[t.ID.Hex()] = true
	}
	uniqueManaged := make([]*data.Task, 0, len(managed))
	for _, t := range managed {
		if accessibleSet[t.ID.Hex()] {
			continue
		}
		accessibleSet[t.ID.Hex()] = true
		uniqueManaged = append(uniqueManaged, t)
	}
	uniqueCollaborating := make([]*data.Task, 0, len(collaborating))
	for _, t := range collaborating {
		if accessibleSet[t.ID.Hex()] {
			continue
		}
		accessibleSet[t.ID.Hex()] = true
		uniqueCollaborating = append(uniqueCollaborating, t)
	}
	var newIDs []primitive.ObjectID
	for _, oid := range participatedIDs {
		if !accessibleSet[oid.Hex()] {
			newIDs = append(newIDs, oid)
		}
	}

	// 6. 批量查询参与的任务
	participated, err := uc.repo.FindByIDs(ctx, newIDs)
	if err != nil {
		return nil, err
	}

	// 7. 合并并按创建时间倒序排序
	all := make([]*data.Task, 0, len(created)+len(uniqueManaged)+len(uniqueCollaborating)+len(participated))
	all = append(all, created...)
	all = append(all, uniqueManaged...)
	all = append(all, uniqueCollaborating...)
	all = append(all, participated...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	// 8. 动态计算每个任务的提交数量
	for _, task := range all {
		count, err := uc.subRepo.CountByTaskID(ctx, task.ID.Hex())
		if err == nil {
			task.Stats.TotalSubmissions = int(count)
		}
	}

	return all, nil
}

func (uc *TaskUsecase) CreateTaskInvitation(ctx context.Context, taskID, userID, role string) (*TaskInvitationInfo, error) {
	normalizedRole, ok := normalizeTaskInvitationRole(role)
	if !ok {
		return nil, errors.New("邀请身份无效")
	}

	task, err := uc.repo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("任务不存在")
	}
	if !task.CanManage(userID) {
		return nil, errors.New("无权限创建协作邀请")
	}
	if !task.EndTime.IsZero() && time.Now().After(task.EndTime) {
		return nil, errors.New("任务已结束，不能继续邀请")
	}

	token, err := generateTaskInvitationToken()
	if err != nil {
		return nil, err
	}
	invitation := data.TaskInvitation{
		Token:         token,
		Role:          normalizedRole,
		InviterUserID: userID,
		CreatedAt:     time.Now(),
	}
	if err := uc.repo.AddInvitation(ctx, taskID, invitation); err != nil {
		return nil, err
	}

	return &TaskInvitationInfo{
		TaskID:    task.ID.Hex(),
		TaskTitle: task.Title,
		Role:      normalizedRole,
		RoleText:  taskInvitationRoleText(normalizedRole),
		Token:     token,
		Status:    "valid",
		Message:   "邀请待领取",
		Valid:     true,
	}, nil
}

func (uc *TaskUsecase) GetTaskInvitation(ctx context.Context, token, viewerUserID string) (*TaskInvitationInfo, error) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" || len(normalizedToken) > 128 {
		return nil, errors.New("邀请不存在或已失效")
	}

	task, err := uc.repo.FindByInvitationToken(ctx, normalizedToken)
	if err != nil {
		return nil, err
	}
	invitation := findTaskInvitation(task, normalizedToken)
	if task == nil || invitation == nil {
		return nil, errors.New("邀请不存在或已失效")
	}

	info := &TaskInvitationInfo{
		TaskID:    task.ID.Hex(),
		TaskTitle: task.Title,
		Role:      invitation.Role,
		RoleText:  taskInvitationRoleText(invitation.Role),
		Status:    "valid",
		Message:   "确认后即可获得相应权限",
		Valid:     true,
	}
	if invitation.UsedAt != nil {
		info.Valid = false
		if invitation.UsedBy == viewerUserID {
			info.Status = "accepted"
			info.Message = "你已领取该邀请"
			info.Accepted = true
		} else {
			info.Status = "used"
			info.Message = "邀请已失效"
		}
		return info, nil
	}
	if !task.EndTime.IsZero() && time.Now().After(task.EndTime) {
		info.Valid = false
		info.Status = "expired"
		info.Message = "任务已结束，邀请已失效"
	}
	return info, nil
}

func (uc *TaskUsecase) AcceptTaskInvitation(ctx context.Context, token, userID string) (*TaskInvitationInfo, error) {
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" || len(normalizedToken) > 128 {
		return nil, errors.New("邀请不存在或已失效")
	}

	task, err := uc.repo.FindByInvitationToken(ctx, normalizedToken)
	if err != nil {
		return nil, err
	}
	invitation := findTaskInvitation(task, normalizedToken)
	if task == nil || invitation == nil || invitation.UsedAt != nil {
		return nil, errors.New("邀请已失效")
	}
	if !task.EndTime.IsZero() && time.Now().After(task.EndTime) {
		return nil, errors.New("任务已结束，邀请已失效")
	}

	role, ok := normalizeTaskInvitationRole(invitation.Role)
	if !ok {
		return nil, errors.New("邀请身份无效")
	}
	if role == TaskInvitationRoleAdmin {
		if task.CanManage(userID) {
			return nil, errors.New("你已拥有管理员权限")
		}
	} else {
		if task.CanManage(userID) {
			return nil, errors.New("管理员无需领取协作员权限")
		}
		if task.IsCollaborator(userID) {
			return nil, errors.New("你已拥有协作员权限")
		}
	}

	acceptedAt := time.Now()
	accepted, err := uc.repo.AcceptInvitation(ctx, task.ID.Hex(), normalizedToken, role, userID, acceptedAt)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, errors.New("邀请已失效")
	}

	return &TaskInvitationInfo{
		TaskID:    task.ID.Hex(),
		TaskTitle: task.Title,
		Role:      role,
		RoleText:  taskInvitationRoleText(role),
		Status:    "accepted",
		Message:   "邀请领取成功",
		Valid:     false,
		Accepted:  true,
	}, nil
}

func (uc *TaskUsecase) DeleteTask(ctx context.Context, id string, userID string) error {
	task, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("任务不存在")
	}
	if !task.CanManage(userID) {
		return errors.New("无权限删除此任务")
	}
	if err := uc.subRepo.DeleteByTaskID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
