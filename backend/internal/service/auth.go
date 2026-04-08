package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	appID     string
	appSecret string
	jwtSecret string
}

func NewAuthService() *AuthService {
	return &AuthService{
		appID:     os.Getenv("WECHAT_APPID"),
		appSecret: os.Getenv("WECHAT_SECRET"),
		jwtSecret: os.Getenv("JWT_SECRET"),
	}
}

type WechatSession struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
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
