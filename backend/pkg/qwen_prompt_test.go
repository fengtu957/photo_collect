package pkg

import (
	"strings"
	"testing"
)

func TestBuildPhotoEvaluationPromptDefinesAdmissionPriority(t *testing.T) {
	prompt := buildPhotoEvaluationPrompt("照片类型：标准证件照；目标比例：7:9", "")

	requiredText := []string{
		"证件照不要求全身入镜",
		"face_complete",
		"人脸未完整入镜",
		"不得只提示光线问题",
		"目标比例：7:9",
	}
	for _, text := range requiredText {
		if !strings.Contains(prompt, text) {
			t.Fatalf("prompt does not contain %q", text)
		}
	}
}
