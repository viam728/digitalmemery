package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/core"
)

// newTestServer 构造 mock 模型 + mock RAG + 临时数据目录的 MCP 服务（不发真实请求）。
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Port:          "0",
		DataDir:       t.TempDir(),
		RAGProvider:   "mock",
		ModelProvider: "mock",
		ModelBaseURL:  "https://api.deepseek.com",
		ModelName:     "mock-model",
		AdminPassword: "test",
	}
	return New(core.NewWithOptions(cfg, core.Options{Background: false}))
}

// call 走一次 handle 并解析响应；ok=false 表示通知（无响应）。
func call(t *testing.T, s *Server, req string) (map[string]any, bool) {
	t.Helper()
	raw, ok := s.handle([]byte(req))
	if !ok {
		return nil, false
	}
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("响应不是合法 JSON：%v（%s）", err, raw)
	}
	return resp, true
}

func resultOf(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 result：%v", resp)
	}
	return r
}

func toolText(t *testing.T, resp map[string]any) string {
	t.Helper()
	r := resultOf(t, resp)
	content, ok := r["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("缺少 content：%v", r)
	}
	item := content[0].(map[string]any)
	return item["text"].(string)
}

func TestInitialize(t *testing.T) {
	s := newTestServer(t)
	resp, ok := call(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if !ok {
		t.Fatal("initialize 应当有响应")
	}
	r := resultOf(t, resp)
	if r["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %v, want %s", r["protocolVersion"], protocolVersion)
	}
	info := r["serverInfo"].(map[string]any)
	if info["name"] != serverName {
		t.Fatalf("serverInfo.name = %v", info["name"])
	}
}

func TestToolsListHasExpectedTools(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	r := resultOf(t, resp)
	tools := r["tools"].([]any)
	if len(tools) != len(toolHandlers) {
		t.Fatalf("tools 数量 = %d, want %d", len(tools), len(toolHandlers))
	}
	names := map[string]bool{}
	for _, t0 := range tools {
		tm := t0.(map[string]any)
		names[tm["name"].(string)] = true
		if tm["inputSchema"] == nil {
			t.Fatalf("工具 %v 缺少 inputSchema", tm["name"])
		}
	}
	for _, want := range []string{"ask_jasper", "search_knowledge", "list_materials", "read_material", "get_profile", "submit_inbox", "run_task"} {
		if !names[want] {
			t.Fatalf("缺少工具 %s", want)
		}
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	s := newTestServer(t)
	if _, ok := call(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); ok {
		t.Fatal("通知不应产生响应")
	}
}

func TestUnknownMethod(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":9,"method":"no/such"}`)
	e := resp["error"].(map[string]any)
	if int(e["code"].(float64)) != codeMethodNotFound {
		t.Fatalf("错误码 = %v, want %d", e["code"], codeMethodNotFound)
	}
}

func TestParseError(t *testing.T) {
	s := newTestServer(t)
	if _, ok := call(t, s, `{not json`); !ok {
		t.Fatal("解析错误应当有响应")
	}
}

func TestGetProfile(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_profile","arguments":{}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, "李俊锋") || !strings.Contains(text, "Agent 工程师") {
		t.Fatalf("profile 内容异常：%s", text)
	}
}

func TestListAndReadMaterial(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_materials","arguments":{}}}`)
	list := toolText(t, resp)
	if !strings.Contains(list, "自我介绍.md") {
		t.Fatalf("资料库列表应包含种子文件：%s", list)
	}
	resp, _ = call(t, s, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_material","arguments":{"id":"f1"}}}`)
	body := toolText(t, resp)
	if !strings.Contains(body, "李俊锋") {
		t.Fatalf("f1 正文异常：%s", body)
	}
}

func TestSubmitInbox(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"submit_inbox","arguments":{"name":"外部插件","note":"hello","content":"JD 正文"}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, "已投递到收件箱") {
		t.Fatalf("投递返回异常：%s", text)
	}
	if len(s.c.Store.ListInbox()) != 1 {
		t.Fatalf("收件箱应有 1 条，实际 %d", len(s.c.Store.ListInbox()))
	}
}

func TestToolErrorIsFlagged(t *testing.T) {
	s := newTestServer(t)
	// 缺少必需参数 content → 工具错误应放进 result.isError
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"submit_inbox","arguments":{}}}`)
	r := resultOf(t, resp)
	if r["isError"] != true {
		t.Fatalf("期望 isError=true，got %v", r["isError"])
	}
}

func TestStdioRoundTrip(t *testing.T) {
	s := newTestServer(t)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := s.ServeStdio(strings.NewReader(input), &out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("期望 2 条响应（通知不响应），实际 %d: %v", len(lines), lines)
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &last); err != nil {
		t.Fatalf("第二行不是合法 JSON：%v", err)
	}
	if last["id"].(float64) != 2 {
		t.Fatalf("响应 id = %v, want 2", last["id"])
	}
}

func TestHTTPTransport(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, want 200", rec.Code)
	}
	// 通知 → 202
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("通知 HTTP 状态 = %d, want 202", rec2.Code)
	}
}

func TestResourcesListAndRead(t *testing.T) {
	s := newTestServer(t)
	resp, _ := call(t, s, `{"jsonrpc":"2.0","id":8,"method":"resources/list"}`)
	r := resultOf(t, resp)
	res := r["resources"].([]any)
	if len(res) == 0 {
		t.Fatal("resources 不应为空")
	}
	first := res[0].(map[string]any)
	uri := first["uri"].(string)
	if !strings.HasPrefix(uri, resourcePrefix) {
		t.Fatalf("uri 前缀异常：%s", uri)
	}
	resp, _ = call(t, s, `{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"`+uri+`"}}`)
	r = resultOf(t, resp)
	contents := r["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents 数量 = %d", len(contents))
	}
}
