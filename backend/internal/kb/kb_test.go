package kb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallToolSuccess(t *testing.T) {
	var gotAuth, gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"已发布：Hello（slug=hello）"}],"isError":false}}`))
	}))
	defer ts.Close()

	c := New(ts.URL+"/", "tok-123")
	out, err := c.CallTool(context.Background(), "kb_publish_doc", map[string]any{"id": "n-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已发布") {
		t.Fatalf("结果异常：%s", out)
	}
	if gotAuth != "Bearer tok-123" {
		t.Fatalf("鉴权头异常：%s", gotAuth)
	}
	if !strings.Contains(gotBody, "kb_publish_doc") || !strings.Contains(gotBody, `"n-1"`) {
		t.Fatalf("请求体异常：%s", gotBody)
	}
}

func TestUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"无效的访问令牌"}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "bad").CallTool(context.Background(), "kb_list_spaces", nil)
	if err == nil || !strings.Contains(err.Error(), "鉴权失败") {
		t.Fatalf("应提示鉴权失败：%v", err)
	}
}

func TestToolIsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"文档不存在：n-x"}],"isError":true}}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "").CallTool(context.Background(), "kb_get_doc", map[string]any{"id": "n-x"})
	if err == nil || !strings.Contains(err.Error(), "文档不存在") {
		t.Fatalf("应透传工具错误：%v", err)
	}
}

func TestRPCError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"unknown tool: nope"}}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "").CallTool(context.Background(), "nope", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("应透传 JSON-RPC 错误：%v", err)
	}
}

func TestNotConfigured(t *testing.T) {
	_, err := New("", "t").CallTool(context.Background(), "kb_list_spaces", nil)
	if err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置时应有明确提示：%v", err)
	}
}
