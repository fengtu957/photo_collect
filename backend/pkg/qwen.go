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

	prompt := fmt.Sprintf(`你是专业的证件照质量评估专家。请根据输入图片，严格评估其是否符合证件照要求。

证件照规格要求：%s

任务分两步，必须严格按顺序执行：
第一步：准入判断
第二步：仅在准入通过后，再做质量评分

【一、准入判断规则】
先判断以下硬性条件：

1. 画面中必须恰好有 1 个人。
2. 必须能清晰识别到 1 张人脸。
3. 如果是空白图、纯背景图、风景图、物品图、卡通图、宠物图、截图、模糊到无法识别人脸的图片，必须判定不通过。
4. 如果出现多人、半张脸、严重遮挡导致无法确认人脸，必须判定不通过。
5. 如果人物不是主要主体，或人脸太小无法判断证件照质量，必须判定不通过。

准入不通过时，直接返回：
- passed=false
- score=0
- breakdown 六项全部为 0
不得继续做质量评分。

【二、质量评分规则】
只有在“恰好 1 人且能识别到 1 张清晰人脸”时，才继续评分。

评分维度（每项 0-100 分）：
1. clarity：人脸清晰度
2. lighting：光线质量
3. angle：人脸角度
4. background：背景干净度
5. expression：表情规范性
6. composition：构图合理性

评分要求：
- 0 分表示极差/完全不符合
- 60 分表示基本可用但存在明显问题
- 80 分表示良好
- 90 分以上表示优秀

总分 score 取六项的整数平均值（四舍五入）。

【三、passed 判定规则】
只有在准入通过的前提下，再按以下规则判断最终 passed：

当且仅当同时满足以下条件时，passed=true：
- score >= 70
- clarity >= 60
- lighting >= 60
- angle >= 60
- background >= 60
- expression >= 60
- composition >= 60

否则 passed=false。

【四、issues 和 suggestions 生成规则】
1. issues：列出当前图片存在的主要问题，最多 3 条，简洁明确，不要空泛。
2. suggestions：针对 issues 给出可执行建议，最多 3 条，必须具体。
3. 如果图片整体较好，也允许 issues 和 suggestions 返回空数组。

【五、输出格式要求】
只允许输出严格 JSON。
不要输出 markdown，不要输出代码块，不要输出解释，不要输出多余文字。
不要缺字段，不要增加字段。

固定返回格式如下：
{"passed":true,"person_count":1,"face_detected":true,"score":85,"breakdown":{"clarity":90,"lighting":85,"angle":88,"background":80,"expression":85,"composition":87},"issues":["光线略显不足"],"suggestions":["建议在自然光充足且光线均匀的环境重拍"]}

【六、特殊强制规则】
- 空白图 / 无人：person_count=0，face_detected=false，score=0，passed=false
- 多人时：person_count 返回实际识别人数，face_detected=true，score=0，passed=false
- 单人但无人脸时：person_count=1，face_detected=false，score=0，passed=false
- 以上不通过场景的 breakdown 都必须返回 {"clarity":0,"lighting":0,"angle":0,"background":0,"expression":0,"composition":0}
- 以上不通过场景的 issues 建议返回 ["不符合准入要求"]
- 以上不通过场景的 suggestions 建议返回 ["请上传恰好包含1名人物且可清晰识别人脸的证件照"]
- 无法判断时，从严处理，判定为不通过`, photoSpec)

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

	log.Printf("[qwen] evaluate photo model=%s status=%s raw_response=%s", c.model, resp.Status, strings.TrimSpace(string(respBody)))

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
			log.Printf("[qwen] extracted content model=%s content=%s", c.model, content)
			return content, nil
		}
	}
	return "", fmt.Errorf("未获取到模型返回内容")
}
