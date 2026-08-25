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
		"后台会根据这些字段确定性计算",
		"不要返回 admission_passed、passed、score 或 hard_failures",
		"左右两侧均能看到肩线或肩部的一部分：shoulders_visible=true",
		"不能把上述硬性字段改为 false",
		"不得仅因不确定而默认 false",
		"无论硬性字段取值如何，都必须独立给出",
		"眼镜或镜片反光",
		"目标比例：7:9",
	}
	for _, text := range requiredText {
		if !strings.Contains(prompt, text) {
			t.Fatalf("prompt does not contain %q", text)
		}
	}
}

func TestBuildPhotoEvaluationPromptDoesNotAnchorToFailureResult(t *testing.T) {
	prompt := buildPhotoEvaluationPrompt("照片类型：标准证件照", "")

	anchoredFailure := `"hard_failures":["人脸未完整入镜","肩部未完整入镜"]`
	if strings.Contains(prompt, anchoredFailure) {
		t.Fatalf("prompt contains a fixed failure result that can anchor the model: %s", anchoredFailure)
	}
}
