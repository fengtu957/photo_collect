package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const maxOSSUploadBytes = 10 * 1024 * 1024
const maxOSSControlObjectBytes = 2 * 1024 * 1024

var errOSSObjectNotFound = errors.New("OSS 文件不存在")

type AliyunOSSService struct {
	accessKeyID     string
	accessKeySecret string
	bucket          string
	endpoint        string
	tempPrefix      string
	photoPrefix     string
	exportPrefix    string
	exportJobPrefix string
	client          *http.Client
}

type OSSUploadPolicy struct {
	UploadURL string            `json:"upload_url"`
	Key       string            `json:"key"`
	Fields    map[string]string `json:"fields"`
	ExpiresIn int               `json:"expires_in"`
}

type OSSObjectInfo struct {
	Size        int64
	ContentType string
}

func NewAliyunOSSService() *AliyunOSSService {
	return &AliyunOSSService{
		accessKeyID:     strings.TrimSpace(os.Getenv("ALIYUN_ACCESS_KEY_ID")),
		accessKeySecret: strings.TrimSpace(os.Getenv("ALIYUN_ACCESS_KEY_SECRET")),
		bucket:          strings.TrimSpace(os.Getenv("ALIYUN_OSS_BUCKET")),
		endpoint:        strings.TrimSpace(os.Getenv("ALIYUN_OSS_ENDPOINT")),
		tempPrefix:      normalizeOSSPrefix(os.Getenv("ALIYUN_OSS_TEMP_PREFIX"), "photo-temp"),
		photoPrefix:     normalizeOSSPrefix(os.Getenv("ALIYUN_OSS_PHOTO_PREFIX"), "photos"),
		exportPrefix:    normalizeOSSPrefix(os.Getenv("ALIYUN_OSS_EXPORT_PREFIX"), "exports"),
		exportJobPrefix: normalizeOSSPrefix(os.Getenv("ALIYUN_OSS_EXPORT_JOB_PREFIX"), "export-jobs"),
		client:          &http.Client{Timeout: 60 * time.Second},
	}
}

func normalizeOSSPrefix(value string, fallback string) string {
	normalized := strings.Trim(strings.TrimSpace(value), "/")
	if normalized == "" {
		normalized = fallback
	}
	return normalized
}

func (s *AliyunOSSService) validateConfigured() error {
	if s == nil || s.accessKeyID == "" || s.accessKeySecret == "" || s.bucket == "" {
		return errors.New("阿里云 OSS 未配置")
	}
	return nil
}

func (s *AliyunOSSService) CreateTemporaryUploadPolicy(userID string, taskID string) (*OSSUploadPolicy, error) {
	key := fmt.Sprintf("%s/%s/%s/%d.jpg", s.tempPrefix, ossUserHash(userID), taskID, time.Now().UnixNano())
	return s.createUploadPolicy(key)
}

func (s *AliyunOSSService) CreateFinalUploadPolicy(userID string, taskID string) (*OSSUploadPolicy, error) {
	key := s.NewFinalPhotoKey(userID, taskID)
	return s.createUploadPolicy(key)
}

func (s *AliyunOSSService) createUploadPolicy(key string) (*OSSUploadPolicy, error) {
	if err := s.validateConfigured(); err != nil {
		return nil, err
	}
	if !isValidOSSKey(key) {
		return nil, errors.New("OSS 文件 key 无效")
	}

	expires := time.Now().Add(10 * time.Minute).UTC()
	policyJSON := fmt.Sprintf(
		`{"expiration":"%s","conditions":[["eq","$key","%s"],["content-length-range",1,%d]]}`,
		expires.Format("2006-01-02T15:04:05.000Z"),
		key,
		maxOSSUploadBytes,
	)
	policy := base64.StdEncoding.EncodeToString([]byte(policyJSON))
	h := hmac.New(sha1.New, []byte(s.accessKeySecret))
	_, _ = h.Write([]byte(policy))
	return &OSSUploadPolicy{
		UploadURL: "https://" + s.ossHost(),
		Key:       key,
		Fields: map[string]string{
			"key":                   key,
			"policy":                policy,
			"OSSAccessKeyId":        s.accessKeyID,
			"Signature":             base64.StdEncoding.EncodeToString(h.Sum(nil)),
			"success_action_status": "200",
		},
		ExpiresIn: 600,
	}, nil
}

func (s *AliyunOSSService) NewFinalPhotoKey(userID string, taskID string) string {
	return fmt.Sprintf("%s/%s/%s/%d.jpg", s.photoPrefix, taskID, ossUserHash(userID), time.Now().UnixNano())
}

func (s *AliyunOSSService) NewExportKey(taskID string, exportID string) string {
	return fmt.Sprintf("%s/%s/%s.zip", s.exportPrefix, taskID, exportID)
}

func (s *AliyunOSSService) NewExportManifestKey(taskID string, exportID string) string {
	return fmt.Sprintf("%s/%s/%s.json", s.exportJobPrefix, taskID, exportID)
}

func (s *AliyunOSSService) NewExportStatusKey(taskID string, exportID string) string {
	return fmt.Sprintf("%s/%s/%s.status.json", s.exportPrefix, taskID, exportID)
}

func (s *AliyunOSSService) IsOwnedTemporaryKey(userID string, taskID string, key string) bool {
	prefix := fmt.Sprintf("%s/%s/%s/", s.tempPrefix, ossUserHash(userID), taskID)
	return isValidOSSKey(key) && strings.HasPrefix(key, prefix)
}

func (s *AliyunOSSService) IsOwnedFinalKey(userID string, taskID string, key string) bool {
	prefix := fmt.Sprintf("%s/%s/%s/", s.photoPrefix, taskID, ossUserHash(userID))
	return isValidOSSKey(key) && strings.HasPrefix(key, prefix)
}

func (s *AliyunOSSService) GetFileURL(key string) string {
	fileURL, _ := s.GetFileURLWithTTL(key, time.Hour)
	return fileURL
}

func (s *AliyunOSSService) GetFileURLWithTTL(key string, ttl time.Duration) (string, error) {
	return s.signedURL(http.MethodGet, key, ttl, nil)
}

func (s *AliyunOSSService) signedURL(method string, key string, ttl time.Duration, headers map[string]string) (string, error) {
	if err := s.validateConfigured(); err != nil {
		return "", err
	}
	key = strings.Trim(strings.TrimSpace(key), "/")
	if !isValidOSSKey(key) {
		return "", errors.New("OSS 文件 key 无效")
	}

	expires := time.Now().Add(ttl).Unix()
	canonicalHeaders := canonicalizeOSSHeaders(headers)
	resource := "/" + s.bucket + "/" + key
	stringToSign := fmt.Sprintf("%s\n\n\n%d\n%s%s", method, expires, canonicalHeaders, resource)
	h := hmac.New(sha1.New, []byte(s.accessKeySecret))
	_, _ = h.Write([]byte(stringToSign))
	u := url.URL{Scheme: "https", Host: s.ossHost(), Path: "/" + key}
	query := u.Query()
	query.Set("Expires", strconv.FormatInt(expires, 10))
	query.Set("OSSAccessKeyId", s.accessKeyID)
	query.Set("Signature", base64.StdEncoding.EncodeToString(h.Sum(nil)))
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func canonicalizeOSSHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	if value := strings.TrimSpace(headers["x-oss-copy-source"]); value != "" {
		return "x-oss-copy-source:" + value + "\n"
	}
	return ""
}

func (s *AliyunOSSService) ProbeObject(key string) (*OSSObjectInfo, error) {
	fileURL, err := s.signedURL(http.MethodHead, key, 10*time.Minute, nil)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodHead, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %s", errOSSObjectNotFound, key)
		}
		logOSSResponseError("probe", key, resp)
		return nil, fmt.Errorf("OSS 文件不可访问: %s", resp.Status)
	}
	return &OSSObjectInfo{Size: resp.ContentLength, ContentType: resp.Header.Get("Content-Type")}, nil
}

func isOSSObjectNotFound(err error) bool {
	return errors.Is(err, errOSSObjectNotFound)
}

func (s *AliyunOSSService) CopyObject(sourceKey string, destinationKey string) error {
	copySource := "/" + s.bucket + "/" + strings.Trim(strings.TrimSpace(sourceKey), "/")
	headers := map[string]string{"x-oss-copy-source": copySource}
	fileURL, err := s.signedURL(http.MethodPut, destinationKey, 10*time.Minute, headers)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPut, fileURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("x-oss-copy-source", copySource)
	resp, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logOSSResponseError("copy", destinationKey, resp)
		return fmt.Errorf("OSS 文件转存失败: %s", resp.Status)
	}
	return nil
}

func (s *AliyunOSSService) PutObject(key string, body []byte) error {
	if len(body) == 0 || len(body) > maxOSSControlObjectBytes {
		return errors.New("OSS 控制文件大小无效")
	}
	fileURL, err := s.signedURL(http.MethodPut, key, 10*time.Minute, nil)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.ContentLength = int64(len(body))
	resp, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logOSSResponseError("upload", key, resp)
		return fmt.Errorf("OSS 文件上传失败: %s", resp.Status)
	}
	return nil
}

func (s *AliyunOSSService) ReadSmallObject(key string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 || maxBytes > maxOSSControlObjectBytes {
		return nil, errors.New("OSS 控制文件读取大小无效")
	}
	fileURL, err := s.GetFileURLWithTTL(key, 30*time.Minute)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %s", errOSSObjectNotFound, key)
		}
		logOSSResponseError("read", key, resp)
		return nil, fmt.Errorf("OSS 文件下载失败: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errors.New("OSS 控制文件过大")
	}
	return body, nil
}

func (s *AliyunOSSService) ossHost() string {
	endpoint := strings.TrimSpace(s.endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.Trim(endpoint, "/")
	if endpoint == "" {
		endpoint = "oss-cn-shanghai.aliyuncs.com"
	}
	if strings.HasPrefix(endpoint, s.bucket+".") {
		return endpoint
	}
	return s.bucket + "." + endpoint
}

func ossUserHash(userID string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(userID)))[:16]
}

func isValidOSSKey(key string) bool {
	normalized := strings.Trim(strings.TrimSpace(key), "/")
	return normalized != "" && normalized == key && !strings.Contains(normalized, "..") && !strings.Contains(normalized, "://") && path.Clean(normalized) == normalized
}

func logOSSResponseError(operation string, key string, resp *http.Response) {
	if resp == nil {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	log.Printf(
		"[aliyun-oss] operation=%s key=%s status=%s request_id=%s body=%s",
		operation,
		key,
		resp.Status,
		resp.Header.Get("x-oss-request-id"),
		strings.TrimSpace(string(body)),
	)
}
