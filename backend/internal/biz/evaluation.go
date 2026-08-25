package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"photo-backend/pkg"
)

type EvaluationUsecase struct {
	qwen *pkg.QwenClient
}

type EvaluationResult struct {
	Model             string         `json:"model"`
	AnalysisStatus    string         `json:"analysis_status"`
	Available         bool           `json:"available"`
	CanSubmit         bool           `json:"can_submit"`
	Passed            bool           `json:"passed"`
	PersonCount       int            `json:"person_count"`
	FaceDetected      bool           `json:"face_detected"`
	Score             int            `json:"score"`
	Breakdown         map[string]int `json:"breakdown"`
	Issues            []string       `json:"issues"`
	Suggestions       []string       `json:"suggestions"`
}

type evaluationPayload struct {
	Passed       *bool                         `json:"passed"`
	PersonCount  json.RawMessage              `json:"person_count"`
	FaceDetected *bool                         `json:"face_detected"`
	Score        json.RawMessage              `json:"score"`
	Breakdown    map[string]json.RawMessage   `json:"breakdown"`
	Issues       *[]string                     `json:"issues"`
	Suggestions  *[]string                    `json:"suggestions"`
}

var evaluationDimensions = []string{
	"clarity",
	"lighting",
	"angle",
	"background",
	"expression",
	"composition",
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

func clampEvaluationScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func parseJSONInt(raw json.RawMessage) (int, error) {
	if value := strings.TrimSpace(string(raw)); value == "" || value == "null" {
		return 0, fmt.Errorf("数值为空")
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, fmt.Errorf("数值无效")
		}
		return int(math.Round(number)), nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, err
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("数值无效")
	}
	return int(math.Round(parsed)), nil
}

func parseEvaluationContent(content string) (*EvaluationResult, error) {
	normalizedContent := normalizeJSONString(content)
	objectStart := strings.Index(normalizedContent, "{")
	if objectStart < 0 {
		return nil, fmt.Errorf("模型结果中没有 JSON 对象")
	}

	var payload evaluationPayload
	decoder := json.NewDecoder(strings.NewReader(normalizedContent[objectStart:]))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Passed == nil || len(payload.PersonCount) == 0 || payload.FaceDetected == nil ||
		len(payload.Score) == 0 || payload.Breakdown == nil || payload.Issues == nil || payload.Suggestions == nil {
		return nil, fmt.Errorf("模型结果缺少必要字段")
	}
	personCount, err := parseJSONInt(payload.PersonCount)
	if err != nil {
		return nil, fmt.Errorf("person_count 无效: %w", err)
	}
	if personCount < 0 {
		return nil, fmt.Errorf("person_count 无效")
	}
	if _, err := parseJSONInt(payload.Score); err != nil {
		return nil, fmt.Errorf("score 无效: %w", err)
	}

	breakdown := make(map[string]int, len(evaluationDimensions))
	totalScore := 0
	allDimensionsPassed := true
	for _, dimension := range evaluationDimensions {
		rawValue, ok := payload.Breakdown[dimension]
		if !ok {
			return nil, fmt.Errorf("breakdown 缺少 %s", dimension)
		}
		value, err := parseJSONInt(rawValue)
		if err != nil {
			return nil, fmt.Errorf("breakdown.%s 无效: %w", dimension, err)
		}
		value = clampEvaluationScore(value)
		breakdown[dimension] = value
		totalScore += value
		if value < 60 {
			allDimensionsPassed = false
		}
	}

	score := (totalScore + len(evaluationDimensions)/2) / len(evaluationDimensions)
	passed := personCount == 1 && *payload.FaceDetected && score >= 70 && allDimensionsPassed
	return &EvaluationResult{
		AnalysisStatus: "success",
		Available:    true,
		CanSubmit:    passed,
		Passed:       passed,
		PersonCount:  personCount,
		FaceDetected: *payload.FaceDetected,
		Score:        score,
		Breakdown:    breakdown,
		Issues:       *payload.Issues,
		Suggestions:  *payload.Suggestions,
	}, nil
}

func (uc *EvaluationUsecase) UnavailableResult() *EvaluationResult {
	return &EvaluationResult{
		Model:          uc.qwen.Model(),
		AnalysisStatus: "unavailable",
		Available:      false,
		CanSubmit:      true,
		Passed:         false,
		Breakdown:      map[string]int{},
		Issues:         []string{},
		Suggestions:    []string{"请自行确认照片清晰、正面且仅有一人"},
	}
}

func (uc *EvaluationUsecase) EvaluatePhoto(ctx context.Context, photoURL, photoSpec string) (*EvaluationResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		var content string
		var err error
		if attempt == 0 {
			content, err = uc.qwen.EvaluatePhoto(photoURL, photoSpec)
		} else {
			content, err = uc.qwen.EvaluatePhotoRetry(photoURL, photoSpec)
		}
		if err != nil {
			lastErr = err
			log.Printf("[ai-evaluation] request failed model=%s attempt=%d err=%v", uc.qwen.Model(), attempt+1, err)
			continue
		}

		result, err := parseEvaluationContent(content)
		if err != nil {
			lastErr = err
			log.Printf("[ai-evaluation] parse failed model=%s attempt=%d err=%v content_bytes=%d", uc.qwen.Model(), attempt+1, err, len(content))
			continue
		}

		result.Model = uc.qwen.Model()
		log.Printf("[ai-evaluation] parsed result model=%s attempt=%d passed=%v person_count=%d face_detected=%v score=%d issues=%v suggestions=%v", result.Model, attempt+1, result.Passed, result.PersonCount, result.FaceDetected, result.Score, result.Issues, result.Suggestions)
		return result, nil
	}

	log.Printf("[ai-evaluation] unavailable after retry model=%s err=%v", uc.qwen.Model(), lastErr)
	return uc.UnavailableResult(), nil
}
