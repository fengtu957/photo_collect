package service

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"photo-backend/internal/data"
)

func TestPhotoEvaluationSurvivesFinalizationToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	evaluation := &data.AIEvaluation{
		Status: "success", Score: 76, Issues: []string{"清晰度偏低"},
		Breakdown: map[string]int{"clarity": 70}, EvaluatedAt: time.Now().UTC(),
	}
	preview := issuePhotoVerificationToken("user", "task", "temporary/photo", evaluation)
	claims, err := readPhotoVerificationToken(preview, "user", "task", "temporary/photo")
	if err != nil {
		t.Fatal(err)
	}
	final := issuePhotoVerificationToken("user", "task", "final/photo", claims.Evaluation)
	got, err := readPhotoVerificationToken(final, "user", "task", "final/photo")
	if err != nil || !reflect.DeepEqual(got.Evaluation, evaluation) {
		t.Fatalf("evaluation lost during finalization: claims=%+v err=%v", got, err)
	}
	if err := verifyPhotoVerificationToken(final, "user", "task", "another/photo"); err == nil {
		t.Fatal("evaluation token must be bound to its photo")
	}
	parts := strings.Split(final, ".")
	got.Evaluation.Score = 100
	payload, _ := json.Marshal(got)
	tampered := base64.RawURLEncoding.EncodeToString(payload) + "." + parts[1]
	if err := verifyPhotoVerificationToken(tampered, "user", "task", "final/photo"); err == nil {
		t.Fatal("client must not be able to change the signed score")
	}
}

func TestLegacyPhotoTokenHasNoEvaluation(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token := issuePhotoVerificationToken("user", "task", "photo")
	claims, err := readPhotoVerificationToken(token, "user", "task", "photo")
	if err != nil || claims.Evaluation != nil {
		t.Fatalf("old tokens should not acquire a zero-score evaluation: claims=%+v err=%v", claims, err)
	}
}

func TestBuildPhotoSpecTextIncludesIDPhotoDimensions(t *testing.T) {
	task := &data.Task{
		PhotoSpec: data.PhotoSpec{
			Name:            "一寸",
			Width:           25,
			Height:          35,
			BackgroundColor: "白底",
		},
	}

	got := buildPhotoSpecText(task)
	wantParts := []string{
		"照片类型：标准证件照",
		"规格名称：一寸",
		"目标尺寸：25×35毫米",
		"目标比例：5:7",
		"背景色要求：白底",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("buildPhotoSpecText() = %q, want it to contain %q", got, part)
		}
	}
}

func TestBuildPhotoSpecTextOmitsBackgroundBeforeAutomaticReplacement(t *testing.T) {
	enabled := true
	task := &data.Task{
		PhotoSpec: data.PhotoSpec{
			Name:            "一寸",
			Width:           25,
			Height:          35,
			BackgroundColor: "白底",
		},
		BackgroundReplacementEnabled: &enabled,
	}

	got := buildPhotoSpecText(task)
	if strings.Contains(got, "背景色要求") {
		t.Fatalf("buildPhotoSpecText() = %q, background requirement must be omitted before replacement", got)
	}
}

func TestBuildRejectionSubscribeMessageLinksToSubmissionEditor(t *testing.T) {
	authSvc := &AuthService{
		envVersion:           "trial",
		rejectionTemplateID:  "template-id",
		rejectionResultField: "phrase4",
		rejectionRemarkField: "thing7",
	}

	message := authSvc.buildRejectionSubscribeMessage("submitter-openid", "task-id", "submission-id", "待修改", "请重新拍摄并提交")
	if message.ToUser != "submitter-openid" {
		t.Fatalf("ToUser = %q", message.ToUser)
	}
	if message.Page != "pages/photo-upload/photo-upload?taskId=task-id&submissionId=submission-id" {
		t.Fatalf("Page = %q", message.Page)
	}
	if message.MiniProgramState != "trial" {
		t.Fatalf("MiniProgramState = %q", message.MiniProgramState)
	}
	if len(message.Data) != 2 {
		t.Fatalf("data length = %d", len(message.Data))
	}
	if message.Data["phrase4"].Value != "待修改" {
		t.Fatalf("result = %q", message.Data["phrase4"].Value)
	}
	if message.Data["thing7"].Value != "请重新拍摄并提交" {
		t.Fatalf("remark = %q", message.Data["thing7"].Value)
	}
}
