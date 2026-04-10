package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)

type QiniuService struct {
	mac    *qbox.Mac
	bucket string
	domain string
	pipeline string
}

type PfopStatusResult struct {
	ID    string             `json:"id"`
	Code  int                `json:"code"`
	Desc  string             `json:"desc"`
	Items []PfopStatusItem   `json:"items"`
}

type PfopStatusItem struct {
	Cmd   string               `json:"cmd"`
	Code  int                  `json:"code"`
	Desc  string               `json:"desc"`
	Key   string               `json:"key"`
	Items []PfopStatusSubItem  `json:"items"`
}

type PfopStatusSubItem struct {
	Cmd  string `json:"cmd"`
	Code int    `json:"code"`
	Desc string `json:"desc"`
	Key  string `json:"key"`
}

func NewQiniuService() *QiniuService {
	accessKey := os.Getenv("QINIU_ACCESS_KEY")
	secretKey := os.Getenv("QINIU_SECRET_KEY")
	bucket := os.Getenv("QINIU_BUCKET")
	domain := os.Getenv("QINIU_DOMAIN")
	pipeline := os.Getenv("QINIU_PIPELINE")

	return &QiniuService{
		mac:    qbox.NewMac(accessKey, secretKey),
		bucket: bucket,
		domain: domain,
		pipeline: pipeline,
	}
}

func (s *QiniuService) GetUploadToken() string {
	putPolicy := storage.PutPolicy{Scope: s.bucket}
	return putPolicy.UploadToken(s.mac)
}

func (s *QiniuService) GetUploadTokenForKey(key string) string {
	putPolicy := storage.PutPolicy{Scope: fmt.Sprintf("%s:%s", s.bucket, key)}
	return putPolicy.UploadToken(s.mac)
}

func (s *QiniuService) GetFileURL(key string) string {
	// 生成私有空间的授权URL，有效期1小时
	deadline := time.Now().Add(time.Hour).Unix() // 1小时（秒）
	return storage.MakePrivateURLv2(s.mac, s.domain, key, deadline)
}

func (s *QiniuService) GetFileURLWithTTL(key string, ttl time.Duration) string {
	deadline := time.Now().Add(ttl).Unix()
	return storage.MakePrivateURLv2(s.mac, s.domain, key, deadline)
}

func (s *QiniuService) UploadFile(localPath string, key string) error {
	cfg := storage.Config{}
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}
	uploadToken := s.GetUploadTokenForKey(key)
	return formUploader.PutFile(context.Background(), &ret, uploadToken, key, localPath, &putExtra)
}

func (s *QiniuService) StartMkzipJob(indexKey string, exportKey string) (string, error) {
	cfg := storage.Config{}
	operationManager := storage.NewOperationManager(s.mac, &cfg)
	fops := fmt.Sprintf("mkzip/4|saveas/%s", storage.EncodedEntry(s.bucket, exportKey))
	return operationManager.Pfop(s.bucket, indexKey, fops, s.pipeline, "", true)
}

func (s *QiniuService) QueryPfop(persistentID string) (*PfopStatusResult, error) {
	url := "https://api.qiniu.com/status/get/prefop?id=" + persistentID
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.mac.SignRequest(req)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询七牛导出状态失败: %s", resp.Status)
	}

	var result PfopStatusResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
