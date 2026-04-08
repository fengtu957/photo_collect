package biz

import (
	"context"
	"encoding/json"
	"photo-backend/internal/data"
	"photo-backend/pkg"
)

type EvaluationUsecase struct {
	subRepo *data.SubmissionRepo
	qwen    *pkg.QwenClient
}

func NewEvaluationUsecase(subRepo *data.SubmissionRepo, qwen *pkg.QwenClient) *EvaluationUsecase {
	return &EvaluationUsecase{subRepo: subRepo, qwen: qwen}
}

func (uc *EvaluationUsecase) EvaluateSubmission(ctx context.Context, submissionID, photoURL, photoSpec string) error {
	content, err := uc.qwen.EvaluatePhoto(photoURL, photoSpec)
	if err != nil {
		return err
	}

	var result struct {
		Score       int            `json:"score"`
		Breakdown   map[string]int `json:"breakdown"`
		Issues      []string       `json:"issues"`
		Suggestions []string       `json:"suggestions"`
	}
	json.Unmarshal([]byte(content), &result)

	// 更新提交记录的 AI 评估结果
	// TODO: 实现更新逻辑
	return nil
}
