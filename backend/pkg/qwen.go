package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type QwenClient struct {
	apiKey string
	apiURL string
}

func NewQwenClient() *QwenClient {
	return &QwenClient{
		apiKey: os.Getenv("QWEN_API_KEY"),
		apiURL: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
	}
}

type QwenRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Image string `json:"image,omitempty"`
				Text  string `json:"text,omitempty"`
			} `json:"content"`
		} `json:"messages"`
	} `json:"input"`
}

type QwenResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

func (c *QwenClient) EvaluatePhoto(imageURL, photoSpec string) (string, error) {
	prompt := fmt.Sprintf(`你是专业的证件照质量评估专家。请评估这张证件照的质量。

证件照规格要求：%s

请从以下维度进行评估（每项 0-100 分）：
1. 人脸清晰度 2. 光线质量 3. 人脸角度 4. 背景干净度 5. 表情规范性 6. 构图合理性

返回严格的 JSON 格式：
{"score":85,"breakdown":{"clarity":90,"lighting":85,"angle":88,"background":80,"expression":85,"composition":87},"issues":["光线略显不足"],"suggestions":["建议在自然光充足的环境重拍"]}`, photoSpec)

	req := QwenRequest{Model: "qwen-vl-plus"}
	req.Input.Messages = []struct {
		Role    string `json:"role"`
		Content []struct {
			Image string `json:"image,omitempty"`
			Text  string `json:"text,omitempty"`
		} `json:"content"`
	}{{Role: "user", Content: []struct {
		Image string `json:"image,omitempty"`
		Text  string `json:"text,omitempty"`
	}{{Image: imageURL}, {Text: prompt}}}}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var qwenResp QwenResponse
	json.Unmarshal(respBody, &qwenResp)

	if len(qwenResp.Output.Choices) > 0 {
		return qwenResp.Output.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response")
}
