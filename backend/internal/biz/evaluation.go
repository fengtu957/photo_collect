package biz

import (
	"context"
	"encoding/json"
	"photo-backend/pkg"
	"strings"
)

type EvaluationUsecase struct {
	qwen *pkg.QwenClient
}

type EvaluationResult struct {
	Model       string         `json:"model"`
	Score       int            `json:"score"`
	Breakdown   map[string]int `json:"breakdown"`
	Issues      []string       `json:"issues"`
	Suggestions []string       `json:"suggestions"`
}

func NewEvaluationUsecase(qwen *pkg.QwenClient) *EvaluationUsecase {
	return &EvaluationUsecase{qwen: qwen}
}

func normalizeJSONString(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func (uc *EvaluationUsecase) EvaluatePhoto(ctx context.Context, photoURL, photoSpec string) (*EvaluationResult, error) {
	content, err := uc.qwen.EvaluatePhoto(photoURL, photoSpec)
	if err != nil {
		return nil, err
	}

	var result EvaluationResult
	if err := json.Unmarshal([]byte(normalizeJSONString(content)), &result); err != nil {
		return nil, err
	}

	if result.Breakdown == nil {
		result.Breakdown = map[string]int{}
	}
	if result.Issues == nil {
		result.Issues = []string{}
	}
	if result.Suggestions == nil {
		result.Suggestions = []string{}
	}
	result.Model = uc.qwen.Model()
	return &result, nil
}
