package storage

import (
	"testing"
	"time"

	"jasperlee/backend/internal/models"
)

func TestMemStoreKeyQuota(t *testing.T) {
	s := newMemStore(t.TempDir())
	k := s.NewKey("tester", 100, true)
	if _, ok := s.GetKey(k.Key); !ok {
		t.Fatal("新建 Key 应可读取")
	}
	if _, ok := s.ConsumeTokens(k.Key, 60); !ok {
		t.Fatal("额度内扣减应成功")
	}
	if _, ok := s.ConsumeTokens(k.Key, 60); ok {
		t.Fatal("超出额度应失败")
	}
	got, _ := s.GetKey(k.Key)
	if got.Used != 60 {
		t.Fatalf("used = %d, want 60", got.Used)
	}
}

func TestMemStoreFileAndArtifact(t *testing.T) {
	s := newMemStore(t.TempDir())
	now := time.Now()
	s.AddFile(models.FileMeta{ID: "x1", Name: "a.md", Kind: "markdown", Path: "我的资料", CreatedAt: now, UpdatedAt: now})
	if _, ok := s.GetFile("x1"); !ok {
		t.Fatal("新增文件应可读取")
	}
	if _, ok := s.MarkArtifact("x1", "ws1"); !ok {
		t.Fatal("标记产物应成功")
	}
	if got := s.ListArtifacts(); len(got) != 1 || got[0].ID != "x1" {
		t.Fatalf("产物列表异常：%+v", got)
	}
}

func TestMemStoreConversationIsolationFields(t *testing.T) {
	s := newMemStore(t.TempDir())
	now := time.Now()
	conv := models.Conversation{ID: "c1", Title: "t", Kind: models.KindChat, CreatedAt: now, UpdatedAt: now, OwnerKey: "keyA"}
	s.AddConversation(conv)
	got, ok := s.GetConversation("c1")
	if !ok || got.OwnerKey != "keyA" {
		t.Fatalf("会话归属丢失：%+v", got)
	}
}

func TestMemStoreSocialLinks(t *testing.T) {
	s := newMemStore(t.TempDir())
	if len(s.ListSocialLinks()) == 0 {
		t.Fatal("应有预置平台看板条目")
	}
	if !s.UpsertSocialLink(models.SocialLink{ID: "github", Platform: "GitHub", Category: "code", Account: "changed"}) {
		t.Fatal("Upsert 应成功")
	}
	found := false
	for _, l := range s.ListSocialLinks() {
		if l.ID == "github" && l.Account == "changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("更新未生效")
	}
	if !s.DeleteSocialLink("github") {
		t.Fatal("删除应成功")
	}
	for _, l := range s.ListSocialLinks() {
		if l.ID == "github" {
			t.Fatal("删除未生效")
		}
	}
}
