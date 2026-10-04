// Package mcp 把数字分身的能力以「最小但完整」的 Model Context Protocol 服务对外暴露，
// 支持外部宿主（Claude Desktop / Codex / Cherry / 自研插件）通过 stdio 或 HTTP 接入。
//
// 实现要点（零第三方依赖，纯标准库，与项目「小体量」一致）：
//   - JSON-RPC 2.0 信封（请求 / 响应 / 通知）
//   - MCP 生命周期：initialize → notifications/initialized
//   - 能力：tools（list / call）与 resources（list / read）
//   - 传输：stdio（行分隔 JSON）与 Streamable HTTP（POST → JSON 响应）
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"jasperlee/backend/internal/core"
)

const (
	protocolVersion = "2025-06-18"
	serverName      = "jasperlee"
	serverVersion   = "1.0.0"
)

// Server MCP 服务：把 core 的领域能力暴露为 MCP 工具/资源。
type Server struct {
	c *core.Core
}

// New 构造 MCP 服务（复用调用方传入的 core）。
func New(c *core.Core) *Server { return &Server{c: c} }

// ---- JSON-RPC 2.0 ----

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// 标准 JSON-RPC 错误码
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// ---- 传输：stdio ----

// ServeStdio 以行分隔 JSON 的循环提供 MCP stdio 传输。
func (s *Server) ServeStdio(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	w := bufio.NewWriter(out)
	defer w.Flush()
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		resp, ok := s.handle(line)
		if !ok || len(resp) == 0 {
			continue // 通知：无响应
		}
		if _, err := w.Write(append(resp, '\n')); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ---- 传输：Streamable HTTP ----

// ServeHTTP 处理单个 JSON-RPC 请求（POST body → JSON 响应）；通知返回 202。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	resp, ok := s.handle(bytes.TrimSpace(body))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !ok || len(resp) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	_, _ = w.Write(resp)
}

// ---- 分发 ----

// handle 处理一条原始 JSON-RPC 消息，返回 (响应字节, 是否应响应)。
func (s *Server) handle(raw []byte) ([]byte, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return marshal(rpcResponse{JSONRPC: "2.0", Error: &rpcError{codeParseError, "parse error"}}), true
	}
	if req.Method == "" {
		return marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{codeInvalidRequest, "method is required"}}), true
	}
	// 通知（无 id 或 notifications/*）：受理但不响应
	if len(req.ID) == 0 || strings.HasPrefix(req.Method, "notifications/") {
		return nil, false
	}
	result, rerr := s.dispatch(req.Method, req.Params)
	if rerr != nil {
		return marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: rerr}), true
	}
	return marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}), true
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.initialize(), nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		return s.callTool(params)
	case "resources/list":
		return s.listResources(), nil
	case "resources/read":
		return s.readResource(params)
	default:
		return nil, &rpcError{codeMethodNotFound, "method not found: " + method}
	}
}

// initialize MCP 握手结果。
func (s *Server) initialize() map[string]any {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"resources": map[string]any{"subscribe": false, "listChanged": false},
		},
		"serverInfo":   map[string]any{"name": serverName, "version": serverVersion},
		"instructions": "JasperLee 数字分身：可用工具查询个人主页、检索/阅读资料库、向收件箱投递、运行 Agent 任务。",
	}
}

func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
