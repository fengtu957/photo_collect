package service

import (
	"os"
	"time"

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
	// 生成私有空间的授权URL，有效期1小时
	deadline := time.Now().Add(time.Hour).Unix() // 1小时（秒）
	return storage.MakePrivateURLv2(s.mac, s.domain, key, deadline)
}
