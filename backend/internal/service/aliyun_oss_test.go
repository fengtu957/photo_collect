package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAliyunOSSPhotoKeysAreScopedByUserAndTask(t *testing.T) {
	service := &AliyunOSSService{
		tempPrefix:  "photo-temp",
		photoPrefix: "photos",
	}
	userID := "user-a"
	taskID := "task-a"

	temporaryKey := "photo-temp/" + ossUserHash(userID) + "/" + taskID + "/1.jpg"
	if !service.IsOwnedTemporaryKey(userID, taskID, temporaryKey) {
		t.Fatalf("expected temporary key %q to belong to current user and task", temporaryKey)
	}
	if service.IsOwnedTemporaryKey("user-b", taskID, temporaryKey) {
		t.Fatal("temporary key must not be accepted for another user")
	}
	if service.IsOwnedTemporaryKey(userID, "task-b", temporaryKey) {
		t.Fatal("temporary key must not be accepted for another task")
	}

	finalKey := service.NewFinalPhotoKey(userID, taskID)
	if !service.IsOwnedFinalKey(userID, taskID, finalKey) {
		t.Fatalf("expected final key %q to belong to current user and task", finalKey)
	}
	if service.IsOwnedFinalKey("user-b", taskID, finalKey) {
		t.Fatal("final key must not be accepted for another user")
	}
}

func TestAliyunOSSUploadPolicyIsBoundToExactKey(t *testing.T) {
	service := &AliyunOSSService{
		accessKeyID:     "access-key",
		accessKeySecret: "secret-key",
		bucket:          "bucket",
		endpoint:        "oss-cn-shanghai.aliyuncs.com",
		tempPrefix:      "photo-temp",
	}

	policy, err := service.CreateTemporaryUploadPolicy("user-a", "task-a")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(policy.Fields["policy"])
	if err != nil {
		t.Fatal(err)
	}
	wantCondition := `["eq","$key","` + policy.Key + `"]`
	if !strings.Contains(string(decoded), wantCondition) {
		t.Fatalf("policy %s does not contain exact-key condition %s", decoded, wantCondition)
	}
}
