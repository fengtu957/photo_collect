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
