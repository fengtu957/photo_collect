package biz

import (
	"reflect"
	"testing"
)

func TestParseEvaluationContentPrioritizesIncompleteFace(t *testing.T) {
	content := `{
		"passed": false,
		"admission_passed": false,
		"person_count": 1,
		"real_person": true,
		"face_detected": true,
		"face_complete": false,
		"head_complete": true,
		"shoulders_visible": false,
		"face_centered": false,
		"face_size_appropriate": false,
		"score": 58,
		"breakdown": {
			"clarity": 75,
			"lighting": 20,
			"angle": 70,
			"background": 65,
			"expression": 70,
			"composition": 50
		},
		"hard_failures": ["光线过强"],
		"issues": ["光线过强"],
		"suggestions": ["避免顶光直射"]
	}`

	result, err := parseEvaluationContent(content)
	if err != nil {
		t.Fatalf("parseEvaluationContent() error = %v", err)
	}

	if result.AdmissionPassed {
		t.Fatal("AdmissionPassed = true, want false")
	}
	if result.Passed || result.CanSubmit {
		t.Fatal("incomplete face must not pass or be submittable")
	}
	if result.Score != 0 {
		t.Fatalf("Score = %d, want 0 for admission failure", result.Score)
	}
	wantIssues := []string{"人脸未完整入镜", "肩部未完整入镜"}
	if !reflect.DeepEqual(result.Issues, wantIssues) {
		t.Fatalf("Issues = %#v, want %#v", result.Issues, wantIssues)
	}
	wantSuggestions := []string{"拉远并拍摄完整面部", "拉远并露出双肩"}
	if !reflect.DeepEqual(result.Suggestions, wantSuggestions) {
		t.Fatalf("Suggestions = %#v, want %#v", result.Suggestions, wantSuggestions)
	}
	for dimension, score := range result.Breakdown {
		if score != 0 {
			t.Fatalf("Breakdown[%q] = %d, want 0", dimension, score)
		}
	}
}

func TestParseEvaluationContentPrioritizesCenteringBeforeLightingWithoutModelHardFailures(t *testing.T) {
	content := `{
		"passed": false,
		"admission_passed": true,
		"person_count": 1,
		"real_person": true,
		"face_detected": true,
		"face_complete": true,
		"head_complete": true,
		"shoulders_visible": true,
		"face_centered": false,
		"face_size_appropriate": true,
		"score": 75,
		"breakdown": {
			"clarity": 80,
			"lighting": 50,
			"angle": 80,
			"background": 80,
			"expression": 80,
			"composition": 80
		},
		"issues": ["光线过强"],
		"suggestions": ["避免顶光直射"]
	}`

	result, err := parseEvaluationContent(content)
	if err != nil {
		t.Fatalf("parseEvaluationContent() error = %v", err)
	}

	wantIssues := []string{"人脸未居中", "光线过强"}
	if !reflect.DeepEqual(result.Issues, wantIssues) {
		t.Fatalf("Issues = %#v, want %#v", result.Issues, wantIssues)
	}
	if result.Passed {
		t.Fatal("Passed = true, want false")
	}
}

func TestParseEvaluationContentPassesQualifiedPhoto(t *testing.T) {
	content := `{
		"passed": true,
		"admission_passed": true,
		"person_count": 1,
		"real_person": true,
		"face_detected": true,
		"face_complete": true,
		"head_complete": true,
		"shoulders_visible": true,
		"face_centered": true,
		"face_size_appropriate": true,
		"score": 85,
		"breakdown": {
			"clarity": 85,
			"lighting": 85,
			"angle": 85,
			"background": 85,
			"expression": 85,
			"composition": 85
		},
		"hard_failures": [],
		"issues": [],
		"suggestions": []
	}`

	result, err := parseEvaluationContent(content)
	if err != nil {
		t.Fatalf("parseEvaluationContent() error = %v", err)
	}

	if !result.AdmissionPassed || !result.Passed || !result.CanSubmit {
		t.Fatalf("qualified photo result = admission:%v passed:%v canSubmit:%v", result.AdmissionPassed, result.Passed, result.CanSubmit)
	}
	if result.Score != 85 {
		t.Fatalf("Score = %d, want 85", result.Score)
	}
	if len(result.Issues) != 0 || len(result.Suggestions) != 0 {
		t.Fatalf("qualified photo feedback = issues:%#v suggestions:%#v", result.Issues, result.Suggestions)
	}
}
