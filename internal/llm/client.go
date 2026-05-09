package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"evalux-server/internal/model"
)

// Message 统一消息结构（支持多模态）
type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64 编码的图片列表（用于 Ollama）
}

// ChatRequest 统一模型请求
type ChatRequest struct {
	Messages    []Message          `json:"messages"`
	Channel     string             `json:"-"`
	ModelConfig *model.ModelConfig `json:"-"` // 项目级大模型配置
	Multimodal  bool               `json:"-"`
}

// ChatResponse 统一模型响应
type ChatResponse struct {
	Content string `json:"content"`
}

// Client 统一模型调用适配层
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		// Timeout 设为 0 表示不限制，由调用方通过 ctx 控制超时
		// http.Client.Timeout 对流式长连接（SSE/ndjson）不友好：
		// 它在"开始读响应体"后仍在倒计时，推理时间长时会在流读取中途强制断开
		httpClient: &http.Client{Timeout: 0},
	}
}

// resolveChannel 解析最终使用的通道，优先请求指定，其次项目默认
func resolveChannel(req ChatRequest) (string, error) {
	if req.Channel != "" {
		return req.Channel, nil
	}
	if req.ModelConfig != nil && req.ModelConfig.DefaultChannel != "" {
		return req.ModelConfig.DefaultChannel, nil
	}
	return "", fmt.Errorf("未指定模型通道，请在项目配置中设置默认通道")
}

// Chat 非流式聊天
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var sb strings.Builder
	err := c.ChatStream(ctx, req, func(chunk string) {
		sb.WriteString(chunk)
	})
	if err != nil {
		return nil, err
	}
	return &ChatResponse{Content: sb.String()}, nil
}

// ChatStream 流式聊天
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, onChunk func(chunk string)) error {
	channel, err := resolveChannel(req)
	if err != nil {
		return err
	}
	switch channel {
	case "ollama":
		return c.streamOllama(ctx, req.Messages, req.ModelConfig, onChunk)
	case "openrouter":
		return c.streamOpenAI(ctx, req.Messages, req.ModelConfig, "openrouter", onChunk)
	case "openai_compatible":
		return c.streamOpenAI(ctx, req.Messages, req.ModelConfig, "openai_compatible", onChunk)
	default:
		return fmt.Errorf("不支持的模型通道: %s", channel)
	}
}

// streamOllama 流式调用 Ollama API（支持多模态）
func (c *Client) streamOllama(ctx context.Context, messages []Message, mc *model.ModelConfig, onChunk func(string)) error {
	if mc == nil || mc.OllamaBaseURL == "" {
		return fmt.Errorf("Ollama 请求地址未配置，请在项目配置中设置")
	}
	if mc.OllamaModel == "" {
		return fmt.Errorf("Ollama 模型名称未配置，请在项目配置中设置")
	}

	url := fmt.Sprintf("%s/api/chat", strings.TrimRight(mc.OllamaBaseURL, "/"))

	type ollamaMsg struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images,omitempty"`
	}
	ollamaMsgs := make([]ollamaMsg, len(messages))
	for i, m := range messages {
		ollamaMsgs[i] = ollamaMsg{Role: m.Role, Content: m.Content, Images: m.Images}
	}

	body := map[string]interface{}{
		"model":    mc.OllamaModel,
		"messages": ollamaMsgs,
		"stream":   true,
		"options":  map[string]interface{}{"num_ctx": 16384},
		"think":    false,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Ollama 调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama 返回错误(%d): %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			onChunk(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}

// buildChatCompletionsURL 兼容两类 base_url 写法：
//   1) 用户填的是"基础地址"——按渠道默认规则追加 chat/completions 路径
//      OpenRouter:        https://openrouter.ai/api/v1               -> 追加 /chat/completions
//      OpenAI 兼容 (有 /v1):  http://host:port/v1                       -> 追加 /chat/completions
//      OpenAI 兼容 (无 /v1):  http://host:port                          -> 追加 /v1/chat/completions
//   2) 用户填的是"完整 chat completions URL"（已包含 /chat/completions）
//      例如 Gemini 的 OpenAI 兼容入口：
//      https://generativelanguage.googleapis.com/v1beta/openai/chat/completions
//      此时直接使用，不再追加任何路径。
func buildChatCompletionsURL(baseURL, provider string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	lower := strings.ToLower(trimmed)
	// 用户已经填了完整的 chat/completions 地址，直接使用
	if strings.HasSuffix(lower, "/chat/completions") {
		return trimmed
	}
	// 末尾已经是 /v1（或 /api/v1 等），直接拼 /chat/completions
	if strings.HasSuffix(lower, "/v1") || strings.HasSuffix(lower, "/api/v1") {
		return trimmed + "/chat/completions"
	}
	// OpenRouter 的 baseURL 一般是 https://openrouter.ai/api/v1，落到上面分支；
	// 兜底：openrouter 不再追加 /v1（它的 API 文档以 /api/v1 开头），openai_compatible 追加 /v1
	if provider == "openrouter" {
		return trimmed + "/chat/completions"
	}
	return trimmed + "/v1/chat/completions"
}

// streamOpenAI 流式调用 OpenAI 兼容 API（OpenRouter / OpenAI Compatible）
func (c *Client) streamOpenAI(ctx context.Context, messages []Message, mc *model.ModelConfig, provider string, onChunk func(string)) error {
	var apiURL, apiKey, modelName string
	isOpenRouter := false

	switch provider {
	case "openrouter":
		if mc == nil {
			return fmt.Errorf("OpenRouter 未配置，请在项目配置中设置")
		}
		baseURL := strings.TrimRight(mc.OpenRouterBaseURL, "/")
		apiKey = mc.OpenRouterAPIKey
		modelName = mc.OpenRouterModel
		if baseURL == "" {
			return fmt.Errorf("OpenRouter 请求地址未配置，请在项目配置中设置")
		}
		if apiKey == "" {
			return fmt.Errorf("OpenRouter API Key 未配置，请在项目配置中设置")
		}
		if modelName == "" {
			return fmt.Errorf("OpenRouter 模型名称未配置，请在项目配置中设置")
		}
		apiURL = buildChatCompletionsURL(baseURL, "openrouter")
		isOpenRouter = true
	case "openai_compatible":
		if mc == nil {
			return fmt.Errorf("OpenAI 兼容 API 未配置，请在项目配置中设置")
		}
		baseURL := strings.TrimRight(mc.OpenAICompatibleBaseURL, "/")
		apiKey = mc.OpenAICompatibleAPIKey
		modelName = mc.OpenAICompatibleModel
		if baseURL == "" {
			return fmt.Errorf("OpenAI 兼容 API 请求地址未配置，请在项目配置中设置")
		}
		if modelName == "" {
			return fmt.Errorf("OpenAI 兼容 API 模型名称未配置，请在项目配置中设置")
		}
		apiURL = buildChatCompletionsURL(baseURL, "openai_compatible")
	}

	type imageURLContent struct {
		URL string `json:"url"`
	}
	type contentPart struct {
		Type     string           `json:"type"`
		Text     string           `json:"text,omitempty"`
		ImageURL *imageURLContent `json:"image_url,omitempty"`
	}
	type openaiMsg struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}

	openaiMsgs := make([]openaiMsg, len(messages))
	for i, m := range messages {
		if len(m.Images) > 0 {
			parts := []contentPart{}
			if m.Content != "" {
				parts = append(parts, contentPart{Type: "text", Text: m.Content})
			}
			for _, img := range m.Images {
				parts = append(parts, contentPart{
					Type:     "image_url",
					ImageURL: &imageURLContent{URL: fmt.Sprintf("data:image/png;base64,%s", img)},
				})
			}
			openaiMsgs[i] = openaiMsg{Role: m.Role, Content: parts}
		} else {
			openaiMsgs[i] = openaiMsg{Role: m.Role, Content: m.Content}
		}
	}

	body := map[string]interface{}{
		"model":    modelName,
		"messages": openaiMsgs,
		"stream":   true,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if isOpenRouter {
		httpReq.Header.Set("HTTP-Referer", "https://evalux.app")
		httpReq.Header.Set("X-OpenRouter-Title", "EvalUX")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s 调用失败: %w", provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s 返回错误(%d): %s", provider, resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	hasContent := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return fmt.Errorf("%s 流式返回错误: %s (code: %d)", provider, chunk.Error.Message, chunk.Error.Code)
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			hasContent = true
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return fmt.Errorf("%s 读取响应流失败: %w", provider, scanErr)
	}
	if !hasContent {
		return fmt.Errorf("%s 模型返回了空内容", provider)
	}
	return nil
}
