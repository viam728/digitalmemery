package rag

import (
	"context"
	"strings"
	"testing"
)

func TestChunk(t *testing.T) {
	text := strings.Repeat("好", 1200)
	chunks := Chunk(text, 500, 50)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	if !strings.HasPrefix(chunks[1], strings.Repeat("好", 50)) {
		t.Fatal("相邻分块应保留重叠")
	}
	if len(Chunk("短文本", 0, -1)) != 1 {
		t.Fatal("短文本应只有 1 块（默认参数）")
	}
}

func TestMockIngestAndQuery(t *testing.T) {
	svc := New("mock", t.TempDir(), "", "")
	ctx := context.Background()
	if err := svc.Ingest(ctx, "f1", "about.md", "我是李俊锋，Agent 工程师，擅长 Go 与 RAG 检索增强。"); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	hits := svc.Query(ctx, "Agent 工程师", 3)
	if len(hits) == 0 {
		t.Fatal("关键词模式应有命中")
	}
	if hits[0].FileName != "about.md" {
		t.Fatalf("命中文件 = %s", hits[0].FileName)
	}
}

func TestQueryEmptyStore(t *testing.T) {
	svc := New("mock", t.TempDir(), "", "")
	if hits := svc.Query(context.Background(), "任意", 3); len(hits) != 0 {
		t.Fatalf("空库应无命中，got %d", len(hits))
	}
}
