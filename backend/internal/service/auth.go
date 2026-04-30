package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	appID     string
	appSecret string
	jwtSecret string
	envVersion string

	accessToken          string
	accessTokenExpiresAt time.Time
	accessTokenMu        sync.Mutex
}

func NewAuthService() *AuthService {
	envVersion := os.Getenv("WECHAT_MINI_PROGRAM_ENV_VERSION")
	if envVersion == "" {
		envVersion = "release"
	}

	return &AuthService{
		appID:     os.Getenv("WECHAT_APPID"),
		appSecret: os.Getenv("WECHAT_SECRET"),
		jwtSecret: os.Getenv("JWT_SECRET"),
		envVersion: envVersion,
	}
}

type WechatSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type WechatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type WechatMiniProgramCodeRequest struct {
	Scene     string `json:"scene"`
	Page      string `json:"page"`
	CheckPath bool   `json:"check_path"`
	EnvVersion string `json:"env_version,omitempty"`
	Width     int    `json:"width"`
	IsHyaline bool   `json:"is_hyaline"`
}

type WechatAPIError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (s *AuthService) Code2Session(code string) (*WechatSession, error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.appID, s.appSecret, code)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var session WechatSession
	json.Unmarshal(body, &session)

	if session.ErrCode != 0 {
		return nil, fmt.Errorf("wechat error: %s", session.ErrMsg)
	}
	return &session, nil
}

func (s *AuthService) GenerateToken(openID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"openid": openID,
		"exp":    time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) GetAccessToken() (string, error) {
	if s.appID == "" || s.appSecret == "" {
		return "", fmt.Errorf("WECHAT_APPID 或 WECHAT_SECRET 未配置")
	}

	s.accessTokenMu.Lock()
	defer s.accessTokenMu.Unlock()

	if s.accessToken != "" && time.Now().Before(s.accessTokenExpiresAt) {
		return s.accessToken, nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		s.appID, s.appSecret)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp WechatAccessTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.ErrCode != 0 {
		return "", fmt.Errorf("wechat error: %s", tokenResp.ErrMsg)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("微信 access_token 为空")
	}

	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn <= 10*time.Minute {
		expiresIn = 10 * time.Minute
	}
	s.accessToken = tokenResp.AccessToken
	s.accessTokenExpiresAt = time.Now().Add(expiresIn - 5*time.Minute)

	return s.accessToken, nil
}

func (s *AuthService) resetAccessToken() {
	s.accessTokenMu.Lock()
	defer s.accessTokenMu.Unlock()

	s.accessToken = ""
	s.accessTokenExpiresAt = time.Time{}
}

func (s *AuthService) requestMiniProgramCode(accessToken string, page string, scene string) ([]byte, string, *WechatAPIError, error) {
	reqBody, err := json.Marshal(WechatMiniProgramCodeRequest{
		Scene:      scene,
		Page:       page,
		CheckPath:  false,
		EnvVersion: s.envVersion,
		Width:      430,
		IsHyaline:  false,
	})
	if err != nil {
		return nil, "", nil, err
	}

	url := "https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=" + accessToken
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	trimmedBody := strings.TrimSpace(string(body))
	if strings.Contains(contentType, "application/json") || strings.HasPrefix(trimmedBody, "{") {
		var apiErr WechatAPIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, "", nil, err
		}
		if apiErr.ErrCode != 0 {
			return nil, "", &apiErr, nil
		}
		return nil, "", nil, fmt.Errorf("微信小程序码响应异常")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", nil, fmt.Errorf("微信小程序码接口返回异常状态: %s", resp.Status)
	}

	return body, contentType, nil, nil
}

func (s *AuthService) GetUnlimitedMiniProgramCode(page string, scene string) ([]byte, string, error) {
	accessToken, err := s.GetAccessToken()
	if err != nil {
		return nil, "", err
	}

	body, contentType, apiErr, err := s.requestMiniProgramCode(accessToken, page, scene)
	if err != nil {
		return nil, "", err
	}
	if apiErr == nil {
		return body, contentType, nil
	}

	if apiErr.ErrCode == 40001 || apiErr.ErrCode == 42001 {
		s.resetAccessToken()
		accessToken, err = s.GetAccessToken()
		if err != nil {
			return nil, "", err
		}

		body, contentType, apiErr, err = s.requestMiniProgramCode(accessToken, page, scene)
		if err != nil {
			return nil, "", err
		}
		if apiErr == nil {
			return body, contentType, nil
		}
	}

	return nil, "", fmt.Errorf("微信小程序码生成失败: %s", apiErr.ErrMsg)
}

func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	session, err := s.Code2Session(req.Code)
	if err != nil {
		Error(w, 1001, err.Error())
		return
	}

	token, _ := s.GenerateToken(session.OpenID)

	Success(w, map[string]string{
		"token":  token,
		"openid": session.OpenID,
	})
}
