package biz

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPhotoSourceAndProportionDecisions(t *testing.T) {
	for _, tc := range []struct {
		name          string
		recapture     interface{}
		sizeOK        bool
		wantError     bool
		wantAdmission bool
		wantPass      bool
	}{
		{"electronic original", false, true, false, true, true},
		{"screen or printed photo despite high scores", true, true, false, false, false},
		{"small portrait despite high scores", false, false, false, true, false},
		{"missing source observation", nil, true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"person_count": 1, "real_person": true, "is_recapture": tc.recapture,
				"face_detected": true, "face_complete": true, "head_complete": true,
				"shoulders_visible": true, "face_centered": true, "face_size_appropriate": tc.sizeOK,
				"breakdown": map[string]int{"clarity": 95, "lighting": 95, "angle": 95, "background": 95, "expression": 95, "composition": 95},
				"issues":    []string{}, "suggestions": []string{},
			}
			content, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			result, err := parseEvaluationContent(string(content))
			if tc.wantError {
				if err == nil {
					t.Fatal("missing source observation must trigger retry")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.AdmissionPassed != tc.wantAdmission || result.Passed != tc.wantPass || result.CanSubmit != tc.wantPass {
				t.Fatalf("unexpected decision: %+v", result)
			}
			if tc.recapture == true && (result.RealPerson || result.Score != 0 || result.Issues[0] != "请勿翻拍屏幕或纸质照片") {
				t.Fatalf("recapture must override other observations: %+v", result)
			}
			if !tc.sizeOK && (len(result.HardFailures) != 0 || result.Breakdown["composition"] != 59 || result.Issues[0] != "人脸大小不合适") {
				t.Fatalf("proportion must produce quality feedback: %+v", result)
			}
		})
	}
}

func TestParseEvaluationContentPrioritizesIncompleteFace(t *testing.T) {
	content := `{
		"passed": false,
		"admission_passed": false,
		"person_count": 1,
		"real_person": true, "is_recapture": false,
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
		"real_person": true, "is_recapture": false,
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
		"real_person": true, "is_recapture": false,
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

func TestParseEvaluationContentDerivesAdmissionAndPassResult(t *testing.T) {
	content := `{"person_count":1,"real_person":true,"is_recapture":false,"face_detected":true,"face_complete":true,"head_complete":true,"shoulders_visible":true,"face_centered":true,"face_size_appropriate":true,"breakdown":{"clarity":80,"lighting":70,"angle":80,"background":30,"expression":80,"composition":80},"issues":["背景色不符合白色要求"],"suggestions":["使用白色背景重拍"]}`

	result, err := parseEvaluationContent(content)
	if err != nil {
		t.Fatalf("parseEvaluationContent() error = %v", err)
	}
	if !result.AdmissionPassed {
		t.Fatal("AdmissionPassed = false, want true from hard fields")
	}
	if result.Passed || result.CanSubmit {
		t.Fatal("background score below 60 must not pass")
	}
	if len(result.HardFailures) != 0 {
		t.Fatalf("HardFailures = %#v, want empty", result.HardFailures)
	}
}
