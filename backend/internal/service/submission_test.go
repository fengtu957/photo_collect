package service

import (
	"strings"
	"testing"

	"photo-backend/internal/data"
)

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
