package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"jasperlee/backend/internal/models"
)

func tlsConfig() *tls.Config {
	//nolint:gosec
	return &tls.Config{InsecureSkipVerify: true}
}

// Usage 模型 token 用量
type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
}

// ChatMessage 一条发送给模型的消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamRequest 一次流式生成请求
type StreamRequest struct {
	Model string
	// Messages 完整对话上下文
	Messages []ChatMessage
	// Hits 命中的资料库片段（RAG 上下文）
	Hits []models.RagHit
}

// Client 模型客户端抽象：Mock（开发，零成本）| DeepSeek/GLM（OpenAI 兼容）
type Client struct {
	provider string
	apiKey   string
	baseURL  string
	hostIP   string
	model    string
}

// New 构造模型客户端
//
// provider: "mock" 或 "deepseek"/"glm"（OpenAI 兼容端点均可，BaseURL 可配）
// apiKey: 服务端配置的模型密钥
func New(provider, apiKey, baseURL string) *Client {
	return NewWithOptions(provider, apiKey, baseURL, "", "")
}

// NewWithOptions 构造模型客户端（带直连 IP 与默认模型）
func NewWithOptions(provider, apiKey, baseURL, hostIP, model string) *Client {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	return &Client{provider: provider, apiKey: apiKey, baseURL: baseURL, hostIP: hostIP, model: model}
}

// SetModel 切换默认模型（运行时切换）
func (c *Client) SetModel(m string) { c.model = m }

// Model 当前默认模型
func (c *Client) Model() string { return c.model }

// Stream 流式生成。emit 在每次产出增量时调用；返回总用量。
func (c *Client) Stream(ctx context.Context, req StreamRequest, emit func(string)) (Usage, error) {
	if c.provider == "mock" || c.apiKey == "" {
		return c.mockStream(ctx, req, emit)
	}
	return c.deepseekStream(ctx, req, emit)
}

// mockStream 开发用：分片模拟打字，并伪造用量
func (c *Client) mockStream(ctx context.Context, req StreamRequest, emit func(string)) (Usage, error) {
	var sb strings.Builder
	for _, m := range req.Messages {
		sb.WriteString(m.Content + "\n")
	}
	base := "我是 JasperLee（开发模式：未接入真实大模型，以下基于我的知识库作答）：\n\n"
	if len(req.Hits) > 0 {
		base += "根据我的资料库，相关内容如下：\n"
		for _, h := range req.Hits {
			base += "· " + strings.TrimSpace(h.Chunk) + "\n"
		}
	} else {
		base += "这个问题暂时没在我的知识库里找到答案，你可以换个问法，或看看我的个人主页。\n"
	}
	for _, r := range []rune(base) {
		select {
		case <-ctx.Done():
			return Usage{}, ctx.Err()
		case <-time.After(18 * time.Millisecond):
			emit(string(r))
		}
	}
	prompt := int64(len(sb.String()) / 4)
	return Usage{PromptTokens: prompt, CompletionTokens: int64(len(base) / 4)}, nil
}

// deepseekStream 通过 OpenAI 兼容接口流式生成（GLM 同协议）
func (c *Client) deepseekStream(ctx context.Context, req StreamRequest, emit func(string)) (Usage, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	type chatReq struct {
		Model    string        `json:"model"`
		Messages []ChatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
	}
	body, _ := json.Marshal(chatReq{
		Model:    model,
		Messages: req.Messages,
		Stream:   true,
	})

	url := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	host := ""
	if c.hostIP != "" {
		url = replaceHost(strings.TrimRight(c.baseURL, "/"), c.hostIP) + "/chat/completions"
		host = "open.bigmodel.cn"
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Usage{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if host != "" {
		hreq.Header.Set("Host", host)
		hreq.Host = host
	}

	client := c.httpClient()
	resp, err := client.Do(hreq)
	if err != nil {
		return Usage{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Usage{}, fmt.Errorf("llm status %d", resp.StatusCode)
	}

	var usage Usage
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			break
		}
		if chunk.Choices[0].Delta.Content != "" {
			emit(chunk.Choices[0].Delta.Content)
		}
		if chunk.Usage != nil {
			usage.PromptTokens, usage.CompletionTokens = chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens
		}
	}
	if err := sc.Err(); err != nil {
		log.Printf("llm stream scan: %v", err)
	}
	return usage, nil
}

// httpClient 直连 IP 时跳过主机名校验（仅用于本机 DNS 污染场景），否则用默认客户端
func (c *Client) httpClient() *http.Client {
	if c.hostIP == "" {
		return http.DefaultClient
	}
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: tlsConfig(),
	}}
}
