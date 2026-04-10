package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type QwenClient struct {
	apiKey string
	apiURL string
	model  string
}

func NewQwenClient() *QwenClient {
	apiKey := os.Getenv("QWEN_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}

	model := os.Getenv("QWEN_MODEL")
	if model == "" {
		model = "qwen3-vl-flash"
	}

	return &QwenClient{
		apiKey: apiKey,
		apiURL: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		model:  model,
	}
}

func (c *QwenClient) Model() string {
	return c.model
}

type QwenRequest struct {
	Model          string        `json:"model"`
	Messages       []QwenMessage `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
	EnableThinking bool `json:"enable_thinking"`
}

type QwenMessage struct {
	Role    string               `json:"role"`
	Content []QwenMessageContent `json:"content"`
}

type QwenMessageContent struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *QwenImageURL  `json:"image_url,omitempty"`
}

type QwenImageURL struct {
	URL string `json:"url"`
}

type QwenResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func extractResponseContent(raw json.RawMessage) string {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	var asArray []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &asArray); err == nil {
		parts := make([]string, 0, len(asArray))
		for _, item := range asArray {
			if item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}

	return strings.TrimSpace(string(raw))
}

func (c *QwenClient) EvaluatePhoto(imageURL, photoSpec string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("QWEN_API_KEY 未配置")
	}

	prompt := fmt.Sprintf(`你是专业的证件照质量评估专家。请评估这张证件照的质量。

证件照规格要求：%s

请从以下维度进行评估（每项 0-100 分）：
1. 人脸清晰度 2. 光线质量 3. 人脸角度 4. 背景干净度 5. 表情规范性 6. 构图合理性

只返回严格 JSON，不要 markdown，不要代码块，不要额外说明。
返回格式：
{"score":85,"breakdown":{"clarity":90,"lighting":85,"angle":88,"background":80,"expression":85,"composition":87},"issues":["光线略显不足"],"suggestions":["建议在自然光充足的环境重拍"]}`, photoSpec)

	req := QwenRequest{
		Model:          c.model,
		EnableThinking: false,
		Messages: []QwenMessage{
			{
				Role: "user",
				Content: []QwenMessageContent{
					{
						Type: "image_url",
						ImageURL: &QwenImageURL{
							URL: imageURL,
						},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}
	req.ResponseFormat.Type = "json_object"

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 40 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		if qwenResp.Error != nil && qwenResp.Error.Message != "" {
			return "", fmt.Errorf(qwenResp.Error.Message)
		}
		return "", fmt.Errorf("百炼调用失败: %s", resp.Status)
	}

	if qwenResp.Error != nil && qwenResp.Error.Message != "" {
		return "", fmt.Errorf(qwenResp.Error.Message)
	}
	if len(qwenResp.Choices) > 0 {
		content := extractResponseContent(qwenResp.Choices[0].Message.Content)
		if content != "" {
			return content, nil
		}
	}
	return "", fmt.Errorf("未获取到模型返回内容")
}
