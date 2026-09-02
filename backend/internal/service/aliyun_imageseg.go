package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// AliyunImageSegService calls Aliyun ImageSeg with a remote URL. The image
// bytes are fetched by Aliyun directly; this service only handles JSON.
type AliyunImageSegService struct {
	accessKeyID     string
	accessKeySecret string
	endpoint        string
	ossBucket       string
	ossEndpoint     string
	ossPrefix       string
	client          *http.Client
}

type aliyunSegmentResponse struct {
	Data struct {
		ImageURL string `json:"ImageURL"`
	} `json:"Data"`
	Message string `json:"Message"`
}

func NewAliyunImageSegService() *AliyunImageSegService {
	endpoint := strings.TrimSpace(os.Getenv("ALIYUN_IMAGESEG_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://imageseg.cn-shanghai.aliyuncs.com/"
	}
	return &AliyunImageSegService{
		accessKeyID:     strings.TrimSpace(os.Getenv("ALIYUN_ACCESS_KEY_ID")),
		accessKeySecret: strings.TrimSpace(os.Getenv("ALIYUN_ACCESS_KEY_SECRET")),
		endpoint:        endpoint,
		ossBucket:       strings.TrimSpace(os.Getenv("ALIYUN_OSS_BUCKET")),
		ossEndpoint:     strings.TrimSpace(os.Getenv("ALIYUN_OSS_ENDPOINT")),
		ossPrefix:       strings.Trim(strings.TrimSpace(os.Getenv("ALIYUN_OSS_PREFIX")), "/"),
		client:          &http.Client{Timeout: 30 * time.Second},
	}
}

type OSSUploadPolicy struct {
	UploadURL string            `json:"upload_url"`
	Key       string            `json:"key"`
	Fields    map[string]string `json:"fields"`
	ExpiresIn int               `json:"expires_in"`
}

func (s *AliyunImageSegService) CreateOSSUploadPolicy(userID string) (*OSSUploadPolicy, error) {
	if s == nil || s.accessKeyID == "" || s.accessKeySecret == "" || s.ossBucket == "" {
		return nil, errors.New("阿里云上海 OSS 未配置")
	}
	prefix := s.ossPrefix
	if prefix == "" {
		prefix = "photo-temp"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	userHash := fmt.Sprintf("%x", sha256.Sum256([]byte(userID)))[:16]
	key := fmt.Sprintf("%s%s/%d.jpg", prefix, userHash, time.Now().UnixNano())
	expires := time.Now().Add(10 * time.Minute).UTC()
	policyJSON := fmt.Sprintf(`{"expiration":"%s","conditions":[["starts-with","$key","%s"],["content-length-range",0,10485760]]}`, expires.Format("2006-01-02T15:04:05.000Z"), prefix)
	policy := base64.StdEncoding.EncodeToString([]byte(policyJSON))
	h := hmac.New(sha1.New, []byte(s.accessKeySecret))
	_, _ = h.Write([]byte(policy))
	host := s.ossHost()
	return &OSSUploadPolicy{
		UploadURL: "https://" + host,
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

func (s *AliyunImageSegService) GetOSSFileURL(key string, ttl time.Duration) (string, error) {
	return s.getOSSFileURL(http.MethodGet, key, ttl)
}

func (s *AliyunImageSegService) getOSSFileURL(method string, key string, ttl time.Duration) (string, error) {
	if s == nil || s.accessKeyID == "" || s.accessKeySecret == "" || s.ossBucket == "" {
		return "", errors.New("阿里云上海 OSS 未配置")
	}
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" || strings.Contains(key, "..") || strings.Contains(key, "://") {
		return "", errors.New("OSS 文件 key 无效")
	}
	expires := time.Now().Add(ttl).Unix()
	resource := "/" + s.ossBucket + "/" + key
	stringToSign := fmt.Sprintf("%s\n\n\n%d\n%s", method, expires, resource)
	h := hmac.New(sha1.New, []byte(s.accessKeySecret))
	_, _ = h.Write([]byte(stringToSign))
	u := url.URL{Scheme: "https", Host: s.ossHost(), Path: "/" + key}
	query := u.Query()
	query.Set("Expires", fmt.Sprintf("%d", expires))
	query.Set("OSSAccessKeyId", s.accessKeyID)
	query.Set("Signature", base64.StdEncoding.EncodeToString(h.Sum(nil)))
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (s *AliyunImageSegService) ProbeOSSFile(key string) error {
	fileURL, err := s.getOSSFileURL(http.MethodHead, key, 10*time.Minute)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodHead, fileURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("[aliyun-imageseg] oss probe status=%s content_type=%s content_length=%s", resp.Status, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OSS 文件不可访问: %s", resp.Status)
	}
	return nil
}

func (s *AliyunImageSegService) ossHost() string {
	endpoint := strings.TrimSpace(s.ossEndpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.Trim(endpoint, "/")
	if endpoint == "" {
		endpoint = "oss-cn-shanghai.aliyuncs.com"
	}
	if strings.HasPrefix(endpoint, s.ossBucket+".") {
		return endpoint
	}
	return s.ossBucket + "." + endpoint
}

func (s *AliyunImageSegService) SegmentBody(imageURL string) (string, error) {
	if s == nil || s.accessKeyID == "" || s.accessKeySecret == "" {
		return "", errors.New("阿里云人体分割未配置")
	}
	if strings.TrimSpace(imageURL) == "" {
		return "", errors.New("图片地址不能为空")
	}

	params := map[string]string{
		"Action":           "SegmentBody",
		"AccessKeyId":      s.accessKeyID,
		"Format":           "JSON",
		"ImageURL":         imageURL,
		"RegionId":         "cn-shanghai",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2019-12-30",
	}
	params["Signature"] = aliyunRPCSignature(http.MethodPost, params, s.accessKeySecret)

	query := make([]string, 0, len(params))
	for key, value := range params {
		query = append(query, aliyunPercentEncode(key)+"="+aliyunPercentEncode(value))
	}
	sort.Strings(query)
	requestBody := strings.Join(query, "&")
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.endpoint, "?"), strings.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(request)
	if err != nil {
		log.Printf("[aliyun-imageseg] request failed endpoint=%s err=%v", s.endpoint, err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		log.Printf("[aliyun-imageseg] response read failed status=%s err=%v", resp.Status, err)
		return "", err
	}
	log.Printf("[aliyun-imageseg] response status=%s body=%s", resp.Status, truncateLogBody(body, 2000))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("阿里云人体分割失败: %s (%s)", resp.Status, aliyunErrorMessage(body))
	}
	var result aliyunSegmentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析阿里云人体分割结果失败: %w", err)
	}
	if strings.TrimSpace(result.Data.ImageURL) == "" {
		if result.Message != "" {
			return "", errors.New("阿里云人体分割失败: " + result.Message)
		}
		return "", errors.New("阿里云人体分割未返回结果")
	}
	return result.Data.ImageURL, nil
}

func truncateLogBody(body []byte, max int) string {
	text := strings.TrimSpace(string(body))
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}

func aliyunErrorMessage(body []byte) string {
	var payload struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && (payload.Code != "" || payload.Message != "") {
		return strings.TrimSpace(payload.Code + ": " + payload.Message)
	}
	return truncateLogBody(body, 500)
}

func aliyunRPCSignature(method string, params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	canonical := strings.Join(pairs, "&")
	stringToSign := method + "&%2F&" + aliyunPercentEncode(canonical)
	h := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	encoded = strings.ReplaceAll(encoded, "%2A", "*")
	return encoded
}
