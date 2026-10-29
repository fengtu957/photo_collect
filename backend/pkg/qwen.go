package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type QwenClient struct {
	apiKey        string
	apiURL        string
	model         string
	geometryModel string
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

	geometryModel := os.Getenv("QWEN_GEOMETRY_MODEL")
	if geometryModel == "" {
		geometryModel = "qwen3-vl-plus"
	}

	return &QwenClient{
		apiKey:        apiKey,
		apiURL:        "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		model:         model,
		geometryModel: geometryModel,
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
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *QwenImageURL `json:"image_url,omitempty"`
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

func isValidJSONObjectContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed != "" && json.Valid([]byte(trimmed))
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

		for i := len(parts) - 1; i >= 0; i-- {
			if isValidJSONObjectContent(parts[i]) {
				log.Printf("[qwen] extracted array content parts=%d strategy=last-valid-json", len(parts))
				return strings.TrimSpace(parts[i])
			}
		}

		longest := ""
		for _, part := range parts {
			if len(strings.TrimSpace(part)) > len(strings.TrimSpace(longest)) {
				longest = part
			}
		}
		if longest != "" {
			log.Printf("[qwen] extracted array content parts=%d strategy=longest-part", len(parts))
			return strings.TrimSpace(longest)
		}

		return ""
	}

	return strings.TrimSpace(string(raw))
}

func (c *QwenClient) EvaluatePhoto(imageURL, photoSpec string) (string, error) {
	return c.evaluatePhoto(imageURL, photoSpec, "")
}

// EvaluatePhotoRetry is used after a malformed model payload. The extra
// instruction makes the second attempt stricter without changing the public
// request shape used by the first attempt.
func (c *QwenClient) EvaluatePhotoRetry(imageURL, photoSpec string) (string, error) {
	return c.evaluatePhoto(imageURL, photoSpec, `

这是对上一次结果的重试。请只返回一份可被 JSON 解析器直接解析的完整 JSON，禁止 Markdown、代码块、前后解释文字或重复 JSON。
请严格只返回提示词列出的原始观察字段、breakdown、issues 和 suggestions，不要返回准入、总分、通过或硬失败字段。
无论硬性字段取值如何都必须填写完整 breakdown；背景、眼镜反光、光线、服装、表情、角度和构图不得改变人物与脸部相关布尔字段。`)
}

func (c *QwenClient) evaluatePhoto(imageURL, photoSpec, retryHint string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("QWEN_API_KEY 未配置")
	}

	prompt := buildPhotoEvaluationPrompt(photoSpec, retryHint)

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

	// Locate the head independently of quality scoring. Run both requests in
	// parallel so the existing two-attempt timeout budget remains unchanged.
	type geometryResponse struct {
		content string
		err     error
	}
	geometryResult := make(chan geometryResponse, 1)
	geometryReq := QwenRequest{Model: c.geometryModel, Messages: []QwenMessage{{Role: "user", Content: []QwenMessageContent{
		{Type: "image_url", ImageURL: &QwenImageURL{URL: imageURL}},
		{Type: "text", Text: `检测图中人物的头部（包括头发至下巴），输出 head_bbox：[左上x,左上y,右下x,右下y]。坐标使用0到1000的归一化整数，依据整张图片，不要先裁剪。并输出 source: direct或screen或print 判断是否翻拍屏幕或纸照。无可定位头部时 head_bbox 为 null。只输出JSON。`},
	}}}}
	geometryReq.ResponseFormat.Type = "json_object"
	go func() {
		content, err := c.sendEvaluationRequest(geometryReq)
		geometryResult <- geometryResponse{content, err}
	}()
	content, err := c.sendEvaluationRequest(req)
	geometry := <-geometryResult
	if err != nil {
		return "", err
	}
	if geometry.err != nil {
		return "", geometry.err
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", err
	}
	if result == nil || !json.Valid([]byte(geometry.content)) {
		return "", fmt.Errorf("模型定位结果无效")
	}
	result["geometry"] = json.RawMessage(geometry.content)
	merged, err := json.Marshal(result)
	return string(merged), err
}

func (c *QwenClient) sendEvaluationRequest(req QwenRequest) (string, error) {

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

	// Keep two-attempt worst case within the mini-program request timeout.
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	log.Printf("[qwen] evaluate photo model=%s status=%s response_bytes=%d", req.Model, resp.Status, len(respBody))

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		if qwenResp.Error != nil && qwenResp.Error.Message != "" {
			return "", fmt.Errorf("%s", qwenResp.Error.Message)
		}
		return "", fmt.Errorf("百炼调用失败: %s", resp.Status)
	}

	if qwenResp.Error != nil && qwenResp.Error.Message != "" {
		return "", fmt.Errorf("%s", qwenResp.Error.Message)
	}
	if len(qwenResp.Choices) > 0 {
		content := extractResponseContent(qwenResp.Choices[0].Message.Content)
		if content != "" {
			log.Printf("[qwen] extracted content model=%s content_bytes=%d", req.Model, len(content))
			return content, nil
		}
	}
	return "", fmt.Errorf("未获取到模型返回内容")
}
