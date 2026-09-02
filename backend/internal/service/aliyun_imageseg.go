package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		client:          &http.Client{Timeout: 30 * time.Second},
	}
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
	params["Signature"] = aliyunRPCSignature(http.MethodGet, params, s.accessKeySecret)

	query := make([]string, 0, len(params))
	for key, value := range params {
		query = append(query, aliyunPercentEncode(key)+"="+aliyunPercentEncode(value))
	}
	sort.Strings(query)
	requestURL := strings.TrimRight(s.endpoint, "?") + "?" + strings.Join(query, "&")
	resp, err := s.client.Get(requestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("阿里云人体分割失败: %s", resp.Status)
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
