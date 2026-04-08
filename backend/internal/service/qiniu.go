package service

import (
	"fmt"
	"os"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

type QiniuService struct {
	mac    *qbox.Mac
	bucket string
	domain string
}

func NewQiniuService() *QiniuService {
	accessKey := os.Getenv("QINIU_ACCESS_KEY")
	secretKey := os.Getenv("QINIU_SECRET_KEY")
	bucket := os.Getenv("QINIU_BUCKET")
	domain := os.Getenv("QINIU_DOMAIN")

	return &QiniuService{
		mac:    qbox.NewMac(accessKey, secretKey),
		bucket: bucket,
		domain: domain,
	}
}

func (s *QiniuService) GetUploadToken() string {
	putPolicy := storage.PutPolicy{Scope: s.bucket}
	return putPolicy.UploadToken(s.mac)
}

func (s *QiniuService) GetFileURL(key string) string {
	return fmt.Sprintf("https://%s/%s", s.domain, key)
}
