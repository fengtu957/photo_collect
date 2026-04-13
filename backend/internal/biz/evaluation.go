package biz

import (
	"context"
	"encoding/json"
	"log"
	"photo-backend/pkg"
	"strings"
)

type EvaluationUsecase struct {
	qwen *pkg.QwenClient
}

type EvaluationResult struct {
	Model       string         `json:"model"`
	Passed      bool           `json:"passed"`
	PersonCount int            `json:"person_count"`
	FaceDetected bool          `json:"face_detected"`
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

	normalizedContent := normalizeJSONString(content)
	var result EvaluationResult
	if err := json.Unmarshal([]byte(normalizedContent), &result); err != nil {
		log.Printf("[ai-evaluation] parse failed model=%s err=%v raw_content=%s", uc.qwen.Model(), err, normalizedContent)
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
	log.Printf("[ai-evaluation] parsed result model=%s passed=%v person_count=%d face_detected=%v score=%d breakdown=%v issues=%v suggestions=%v", result.Model, result.Passed, result.PersonCount, result.FaceDetected, result.Score, result.Breakdown, result.Issues, result.Suggestions)
	return &result, nil
}
