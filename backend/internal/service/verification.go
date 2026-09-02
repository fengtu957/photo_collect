package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

type photoVerificationClaims struct {
	UserID   string `json:"user_id"`
	TaskID   string `json:"task_id"`
	PhotoKey string `json:"photo_key"`
	Expires  int64  `json:"expires"`
}

func issuePhotoVerificationToken(userID, taskID, photoKey string) string {
	claims := photoVerificationClaims{UserID: userID, TaskID: taskID, PhotoKey: photoKey, Expires: time.Now().Add(10 * time.Minute).Unix()}
	payload, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + photoVerificationSignature(encoded)
}

func verifyPhotoVerificationToken(token, userID, taskID, photoKey string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(photoVerificationSignature(parts[0]))) {
		return errors.New("照片尚未通过检查，请重新检测")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("照片检查凭证无效")
	}
	var claims photoVerificationClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Expires < time.Now().Unix() || claims.UserID != userID || claims.TaskID != taskID || claims.PhotoKey != photoKey {
		return errors.New("照片检查凭证已失效，请重新检测")
	}
	return nil
}

func photoVerificationSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(os.Getenv("JWT_SECRET")))
	_, _ = h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
