// Package kb 是 JasperKB 知识库的 MCP 客户端（零第三方依赖）：
// 以 JSON-RPC 2.0 over HTTP 调用知识库的 /mcp，把「数字分身改博客」的能力
// （kb_search / kb_get_doc / kb_create_doc / kb_update_doc / kb_publish_doc …）
// 接给数字分身对外暴露。
package kb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client JasperKB MCP 客户端。
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New 构造客户端；baseURL 为空表示未配置（调用时会给出明确提示）。
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// Configured 是否已配置知识库地址。
func (c *Client) Configured() bool { return c != nil && c.baseURL != "" }

// CallTool 调用知识库的 MCP 工具（tools/call），返回文本结果。
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("未配置 JasperKB 知识库地址（请设置 KB_URL，默认 http://localhost:8123）")
	}
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接 JasperKB 失败（%s）：%v", c.baseURL, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return "", fmt.Errorf("JasperKB 鉴权失败：请检查 KB_TOKEN 是否与服务端一致")
	default:
		return "", fmt.Errorf("JasperKB 返回 HTTP %d：%s", resp.StatusCode, clip(string(data), 200))
	}
	var r struct {
		Result *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("JasperKB 响应解析失败：%v", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("JasperKB 错误（%d）：%s", r.Error.Code, r.Error.Message)
	}
	if r.Result == nil {
		return "", fmt.Errorf("JasperKB 响应缺少 result")
	}
	var sb strings.Builder
	for _, item := range r.Result.Content {
		if item.Text != "" {
			sb.WriteString(item.Text)
			sb.WriteString("\n")
		}
	}
	text := strings.TrimSpace(sb.String())
	if r.Result.IsError {
		if text == "" {
			text = "知识库工具执行失败"
		}
		return "", fmt.Errorf("%s", text)
	}
	if text == "" {
		text = "（知识库返回空结果）"
	}
	return text, nil
}

// clip 截断过长文本（错误回显用）。
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
