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
	Model               string         `json:"model"`
	AnalysisStatus      string         `json:"analysis_status"`
	Available           bool           `json:"available"`
	CanSubmit           bool           `json:"can_submit"`
	Passed              bool           `json:"passed"`
	AdmissionPassed     bool           `json:"admission_passed"`
	PersonCount         int            `json:"person_count"`
	RealPerson          bool           `json:"real_person"`
	FaceDetected        bool           `json:"face_detected"`
	FaceComplete        bool           `json:"face_complete"`
	HeadComplete        bool           `json:"head_complete"`
	ShouldersVisible    bool           `json:"shoulders_visible"`
	FaceCentered        bool           `json:"face_centered"`
	FaceSizeAppropriate bool           `json:"face_size_appropriate"`
	Score               int            `json:"score"`
	Breakdown           map[string]int `json:"breakdown"`
	HardFailures        []string       `json:"hard_failures"`
	Issues              []string       `json:"issues"`
	Suggestions         []string       `json:"suggestions"`
}

type evaluationPayload struct {
	Passed              *bool                      `json:"passed"`
	AdmissionPassed     *bool                      `json:"admission_passed"`
	PersonCount         json.RawMessage            `json:"person_count"`
	RealPerson          *bool                      `json:"real_person"`
	FaceDetected        *bool                      `json:"face_detected"`
	FaceComplete        *bool                      `json:"face_complete"`
	HeadComplete        *bool                      `json:"head_complete"`
	ShouldersVisible    *bool                      `json:"shoulders_visible"`
	FaceCentered        *bool                      `json:"face_centered"`
	FaceSizeAppropriate *bool                      `json:"face_size_appropriate"`
	Score               json.RawMessage            `json:"score"`
	Breakdown           map[string]json.RawMessage `json:"breakdown"`
	Issues              *[]string                  `json:"issues"`
	Suggestions         *[]string                  `json:"suggestions"`
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

func zeroEvaluationBreakdown() map[string]int {
	breakdown := make(map[string]int, len(evaluationDimensions))
	for _, dimension := range evaluationDimensions {
		breakdown[dimension] = 0
	}
	return breakdown
}

func appendUniqueLimited(items []string, value string, limit int) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(items) >= limit {
		return items
	}
	for _, item := range items {
		if item == trimmed {
			return items
		}
	}
	return append(items, trimmed)
}

func normalizeLimitedStrings(items []string, limit int) []string {
	result := make([]string, 0, limit)
	for _, item := range items {
		result = appendUniqueLimited(result, item, limit)
	}
	return result
}

func buildHardFailures(personCount int, payload evaluationPayload) []string {
	failures := make([]string, 0, 2)

	if personCount == 0 {
		failures = appendUniqueLimited(failures, "未检测到人物", 2)
	} else if personCount > 1 {
		failures = appendUniqueLimited(failures, "画面中存在多人", 2)
	} else if !*payload.RealPerson {
		failures = appendUniqueLimited(failures, "非真人证件照", 2)
	} else if !*payload.FaceDetected {
		failures = appendUniqueLimited(failures, "未检测到清晰人脸", 2)
	} else {
		if !*payload.FaceComplete {
			failures = appendUniqueLimited(failures, "人脸未完整入镜", 2)
		}
		if !*payload.HeadComplete {
			failures = appendUniqueLimited(failures, "头部未完整入镜", 2)
		}
		if !*payload.ShouldersVisible {
			failures = appendUniqueLimited(failures, "肩部未完整入镜", 2)
		}
		if !*payload.FaceSizeAppropriate {
			failures = appendUniqueLimited(failures, "人脸大小不合适", 2)
		}
	}

	if !*payload.AdmissionPassed && len(failures) == 0 {
		failures = append(failures, "不符合证件照准入要求")
	}
	return failures
}

func suggestionForIssue(issue string) string {
	switch issue {
	case "未检测到人物", "画面中存在多人":
		return "请上传单人证件照"
	case "非真人证件照":
		return "请上传真人现场照片"
	case "未检测到清晰人脸":
		return "请正对镜头重新拍摄"
	case "人脸未完整入镜":
		return "拉远并拍摄完整面部"
	case "头部未完整入镜":
		return "保留完整头部和头顶空间"
	case "肩部未完整入镜":
		return "拉远并露出双肩"
	case "人脸大小不合适":
		return "调整拍摄距离后重拍"
	case "人脸未居中":
		return "调整人脸至画面中央"
	case "人脸不够清晰":
		return "保持稳定并对焦面部"
	case "光线不符合要求":
		return "使用均匀柔和光线重拍"
	case "拍摄角度不规范":
		return "正对镜头保持头部端正"
	case "背景不符合要求":
		return "使用指定纯色背景重拍"
	case "表情不够规范":
		return "保持自然中性表情"
	case "证件照构图不规范":
		return "调整距离和构图后重拍"
	default:
		return "请按证件照要求重拍"
	}
}

func buildHardFailureSuggestions(hardFailures []string) []string {
	suggestions := make([]string, 0, 2)
	for _, failure := range hardFailures {
		suggestions = appendUniqueLimited(suggestions, suggestionForIssue(failure), 2)
	}
	return suggestions
}

var evaluationIssueByDimension = map[string]string{
	"clarity":     "人脸不够清晰",
	"lighting":    "光线不符合要求",
	"angle":       "拍摄角度不规范",
	"background":  "背景不符合要求",
	"expression":  "表情不够规范",
	"composition": "证件照构图不规范",
}

func buildQualityFeedback(faceCentered bool, passed bool, breakdown map[string]int, modelIssues, modelSuggestions []string) ([]string, []string) {
	issues := make([]string, 0, 2)
	suggestions := make([]string, 0, 2)

	if !faceCentered {
		issues = appendUniqueLimited(issues, "人脸未居中", 2)
		suggestions = appendUniqueLimited(suggestions, suggestionForIssue("人脸未居中"), 2)
	}
	for _, issue := range modelIssues {
		issues = appendUniqueLimited(issues, issue, 2)
	}
	for _, suggestion := range modelSuggestions {
		suggestions = appendUniqueLimited(suggestions, suggestion, 2)
	}

	if !passed && len(issues) == 0 {
		for _, dimension := range evaluationDimensions {
			if breakdown[dimension] < 60 {
				issues = appendUniqueLimited(issues, evaluationIssueByDimension[dimension], 2)
			}
		}
	}
	if !passed && len(issues) == 0 {
		issues = append(issues, "证件照质量未达标")
	}
	for _, issue := range issues {
		if len(suggestions) >= 2 {
			break
		}
		suggestions = appendUniqueLimited(suggestions, suggestionForIssue(issue), 2)
	}

	return normalizeLimitedStrings(issues, 2), normalizeLimitedStrings(suggestions, 2)
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
	if payload.Passed == nil || payload.AdmissionPassed == nil || len(payload.PersonCount) == 0 ||
		payload.RealPerson == nil || payload.FaceDetected == nil || payload.FaceComplete == nil ||
		payload.HeadComplete == nil || payload.ShouldersVisible == nil || payload.FaceCentered == nil ||
		payload.FaceSizeAppropriate == nil || len(payload.Score) == 0 || payload.Breakdown == nil ||
		payload.Issues == nil || payload.Suggestions == nil {
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

	hardFailures := buildHardFailures(personCount, payload)
	admissionPassed := *payload.AdmissionPassed &&
		personCount == 1 &&
		*payload.RealPerson &&
		*payload.FaceDetected &&
		*payload.FaceComplete &&
		*payload.HeadComplete &&
		*payload.ShouldersVisible &&
		*payload.FaceSizeAppropriate &&
		len(hardFailures) == 0

	score := 0
	passed := false
	issues := []string{}
	suggestions := []string{}
	if !admissionPassed {
		breakdown = zeroEvaluationBreakdown()
		issues = hardFailures
		suggestions = buildHardFailureSuggestions(hardFailures)
	} else {
		score = (totalScore + len(evaluationDimensions)/2) / len(evaluationDimensions)
		passed = *payload.Passed && score >= 70 && allDimensionsPassed
		issues, suggestions = buildQualityFeedback(*payload.FaceCentered, passed, breakdown, *payload.Issues, *payload.Suggestions)
	}

	return &EvaluationResult{
		AnalysisStatus:      "success",
		Available:           true,
		CanSubmit:           passed,
		Passed:              passed,
		AdmissionPassed:     admissionPassed,
		PersonCount:         personCount,
		RealPerson:          *payload.RealPerson,
		FaceDetected:        *payload.FaceDetected,
		FaceComplete:        *payload.FaceComplete,
		HeadComplete:        *payload.HeadComplete,
		ShouldersVisible:    *payload.ShouldersVisible,
		FaceCentered:        *payload.FaceCentered,
		FaceSizeAppropriate: *payload.FaceSizeAppropriate,
		Score:               score,
		Breakdown:           breakdown,
		HardFailures:        hardFailures,
		Issues:              issues,
		Suggestions:         suggestions,
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
