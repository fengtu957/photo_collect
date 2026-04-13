package biz

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"photo-backend/internal/data"
	"strings"
	"time"
)

const (
	FreeMaxActiveTasks        = 3
	FreeMaxSubmissionsPerTask = 50
	FreeMaxOpenDurationDays   = 7
	VIPMaxOpenDurationDays    = 30
)

type UserEntitlementLimits struct {
	MaxActiveTasks        int  `json:"max_active_tasks"`
	MaxSubmissionsPerTask int  `json:"max_submissions_per_task"`
	MaxOpenDurationDays   int  `json:"max_open_duration_days"`
	CanUseAIAnalysis      bool `json:"can_use_ai_analysis"`
}

type UserEntitlementUsage struct {
	ActiveTaskCount int `json:"active_task_count"`
}

type UserEntitlements struct {
	IsVIP        bool                  `json:"is_vip"`
	PlanCode     string                `json:"plan_code,omitempty"`
	ExpireAt     *time.Time            `json:"expire_at,omitempty"`
	ContactLabel string                `json:"contact_label,omitempty"`
	ContactValue string                `json:"contact_value,omitempty"`
	Limits       UserEntitlementLimits `json:"limits"`
	Usage        UserEntitlementUsage  `json:"usage"`
}

type CreateActivationCodesRequest struct {
	PlanCode    string
	DurationDay int
	Count       int
	ExpireAt    time.Time
	Remark      string
}

type VIPUsecase struct {
	repo *data.VIPRepo
}

func NewVIPUsecase(repo *data.VIPRepo) *VIPUsecase {
	return &VIPUsecase{repo: repo}
}

func buildFreeEntitlements() *UserEntitlements {
	return &UserEntitlements{
		IsVIP: false,
		Limits: UserEntitlementLimits{
			MaxActiveTasks:        FreeMaxActiveTasks,
			MaxSubmissionsPerTask: FreeMaxSubmissionsPerTask,
			MaxOpenDurationDays:   FreeMaxOpenDurationDays,
			CanUseAIAnalysis:      false,
		},
	}
}

func (uc *VIPUsecase) GetMembership(ctx context.Context, userID string) (*data.VIPMembership, error) {
	return uc.repo.FindMembershipByUserID(ctx, userID)
}

func (uc *VIPUsecase) GetUserEntitlements(ctx context.Context, userID string) (*UserEntitlements, error) {
	entitlements := buildFreeEntitlements()
	membership, err := uc.repo.FindMembershipByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return entitlements, nil
	}

	expireAt := membership.ExpireAt
	entitlements.PlanCode = membership.PlanCode
	entitlements.ExpireAt = &expireAt

	now := time.Now()
	if membership.Status == "active" && membership.ExpireAt.After(now) {
		entitlements.IsVIP = true
		entitlements.Limits.MaxActiveTasks = 0
		entitlements.Limits.MaxSubmissionsPerTask = 0
		entitlements.Limits.MaxOpenDurationDays = VIPMaxOpenDurationDays
		entitlements.Limits.CanUseAIAnalysis = true
	}

	return entitlements, nil
}

func (uc *VIPUsecase) GrantVIP(ctx context.Context, userID string, planCode string, durationDay int, source string, remark string) (*data.VIPMembership, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user_id 不能为空")
	}
	if durationDay <= 0 {
		return nil, errors.New("duration_day 必须大于 0")
	}
	now := time.Now()
	existing, err := uc.repo.FindMembershipByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	startAt := now
	if existing != nil && existing.Status == "active" && existing.ExpireAt.After(now) {
		startAt = existing.StartAt
		now = existing.ExpireAt
	}

	membership := &data.VIPMembership{
		UserID:    userID,
		PlanCode:  strings.TrimSpace(planCode),
		Status:    "active",
		StartAt:   startAt,
		ExpireAt:  now.Add(time.Duration(durationDay) * 24 * time.Hour),
		Source:    source,
		Remark:    remark,
		CreatedAt: time.Time{},
	}
	if existing != nil {
		membership.ID = existing.ID
		membership.CreatedAt = existing.CreatedAt
	}

	if err := uc.repo.SaveMembership(ctx, membership); err != nil {
		return nil, err
	}
	return membership, nil
}

func (uc *VIPUsecase) RedeemCode(ctx context.Context, userID string, rawCode string) (*UserEntitlements, error) {
	if strings.TrimSpace(rawCode) == "" {
		return nil, errors.New("请输入激活码")
	}

	activationCode, err := uc.repo.ConsumeCode(ctx, rawCode, userID)
	if err != nil {
		return nil, err
	}
	if activationCode == nil {
		return nil, errors.New("激活码无效、已使用或已过期")
	}

	if _, err := uc.GrantVIP(ctx, userID, activationCode.PlanCode, activationCode.DurationDay, "code", activationCode.Code); err != nil {
		return nil, err
	}
	return uc.GetUserEntitlements(ctx, userID)
}

func (uc *VIPUsecase) CreateActivationCodes(ctx context.Context, req CreateActivationCodesRequest) ([]*data.VIPActivationCode, error) {
	if strings.TrimSpace(req.PlanCode) == "" {
		return nil, errors.New("plan_code 不能为空")
	}
	if req.DurationDay <= 0 {
		return nil, errors.New("duration_day 必须大于 0")
	}
	if req.Count <= 0 {
		return nil, errors.New("count 必须大于 0")
	}

	codes := make([]*data.VIPActivationCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		generatedCode, err := generateActivationCode(req.PlanCode)
		if err != nil {
			return nil, err
		}
		codes = append(codes, &data.VIPActivationCode{
			Code:        generatedCode,
			PlanCode:    strings.TrimSpace(req.PlanCode),
			DurationDay: req.DurationDay,
			Status:      "unused",
			ExpireAt:    req.ExpireAt,
			Remark:      req.Remark,
		})
	}

	if err := uc.repo.CreateActivationCodes(ctx, codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (uc *VIPUsecase) DisableCode(ctx context.Context, code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.New("code 不能为空")
	}
	return uc.repo.DisableCode(ctx, code)
}

func (uc *VIPUsecase) GetCode(ctx context.Context, code string) (*data.VIPActivationCode, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("code 不能为空")
	}
	return uc.repo.FindCodeByCode(ctx, code)
}

func generateActivationCode(planCode string) (string, error) {
	prefix := "VIP"
	normalizedPlanCode := strings.ToUpper(strings.TrimSpace(planCode))
	if normalizedPlanCode != "" {
		normalizedPlanCode = strings.ReplaceAll(normalizedPlanCode, "_", "")
		normalizedPlanCode = strings.ReplaceAll(normalizedPlanCode, "-", "")
		prefix = normalizedPlanCode
		if len(prefix) > 4 {
			prefix = prefix[:4]
		}
	}

	charset := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	groups := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		group, err := randomCodeSegment(charset, 4)
		if err != nil {
			return "", err
		}
		groups = append(groups, group)
	}

	return prefix + "-" + strings.Join(groups, "-"), nil
}

func randomCodeSegment(charset string, length int) (string, error) {
	var builder strings.Builder
	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		builder.WriteByte(charset[index.Int64()])
	}
	return builder.String(), nil
}
