package storage

import (
	"log"

	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/models"
)

// Store 统一存储门面：优先 PostgreSQL，连接失败时降级为内存 + JSON 持久化。
// 方法签名与旧 Store 完全一致，保证 api 层无需改动。
type Store struct {
	impl storeImpl
	pg   *pgStore // 非 nil 表示走 Postgres（暴露 RAG 块能力）
}

type storeImpl interface {
	ListConversations() []models.Conversation
	AddConversation(c models.Conversation)
	RenameConversation(id, title string) (models.Conversation, bool)
	UpdateConversationPinned(id string, pinned bool) (models.Conversation, bool)
	DeleteConversation(id string) bool
	ListMessages(id string) []models.Message
	AddMessage(m models.Message)
	ListFiles() []models.FileMeta
	MarkIngested(id string) bool
	MarkArtifact(id string, workspaceID string) (models.FileMeta, bool)
	ListArtifacts() []models.FileMeta
	AddFile(f models.FileMeta)
	GetFile(id string) (models.FileMeta, bool)
	ReadBytes(f models.FileMeta) ([]byte, error)
	ReadContent(f models.FileMeta) (string, bool)
	TouchFile(id string)
	SetWorkspace(w models.Workspace)
	GetWorkspace(id string) (models.Workspace, bool)
	RenameFile(id, name string) (models.FileMeta, bool)
	DeleteFile(id string) bool
	DeleteFileOnDisk(f models.FileMeta) error
	ListInbox() []models.InboxItem
	AddInbox(it models.InboxItem)
	GetInbox(id string) (models.InboxItem, bool)
	DeleteInbox(id string) bool
	DeleteInboxOnDisk(it models.InboxItem) error
	GetKey(key string) (models.ApiKey, bool)
	NewKey(label string, quota int64, lib bool) models.ApiKey
	ListKeys() []models.ApiKey
	UpdateKey(k models.ApiKey)
	DeleteKey(key string) bool
	ConsumeTokens(key string, tokens int64) (models.ApiKey, bool)
}

// New 创建存储：Postgres 可用则用 Postgres，否则降级内存版。
func New(cfg *config.Config) *Store {
	if pg, ok := newPostgres(cfg); ok {
		log.Printf("[storage] using PostgreSQL (%s:%s/%s)", cfg.PGHost, cfg.PGPort, cfg.PGDatabase)
		return &Store{impl: pg, pg: pg}
	}
	log.Printf("[storage] using memory store (dataDir=%s)", cfg.DataDir)
	return &Store{impl: newMemStore(cfg.DataDir)}
}

// UsingPostgres 是否走真实 Postgres（供 health / 日志）。
func (s *Store) UsingPostgres() bool { return s.pg != nil }

func (s *Store) ListConversations() []models.Conversation        { return s.impl.ListConversations() }
func (s *Store) AddConversation(c models.Conversation)           { s.impl.AddConversation(c) }
func (s *Store) RenameConversation(id, t string) (models.Conversation, bool) {
	return s.impl.RenameConversation(id, t)
}
func (s *Store) UpdateConversationPinned(id string, p bool) (models.Conversation, bool) {
	return s.impl.UpdateConversationPinned(id, p)
}
func (s *Store) DeleteConversation(id string) bool                { return s.impl.DeleteConversation(id) }
func (s *Store) ListMessages(id string) []models.Message          { return s.impl.ListMessages(id) }
func (s *Store) AddMessage(m models.Message)                     { s.impl.AddMessage(m) }
func (s *Store) ListFiles() []models.FileMeta                    { return s.impl.ListFiles() }
func (s *Store) MarkIngested(id string) bool                     { return s.impl.MarkIngested(id) }
func (s *Store) MarkArtifact(id, wid string) (models.FileMeta, bool) {
	return s.impl.MarkArtifact(id, wid)
}
func (s *Store) ListArtifacts() []models.FileMeta { return s.impl.ListArtifacts() }
func (s *Store) AddFile(f models.FileMeta)                       { s.impl.AddFile(f) }
func (s *Store) GetFile(id string) (models.FileMeta, bool)       { return s.impl.GetFile(id) }
func (s *Store) ReadBytes(f models.FileMeta) ([]byte, error)     { return s.impl.ReadBytes(f) }
func (s *Store) ReadContent(f models.FileMeta) (string, bool)    { return s.impl.ReadContent(f) }
func (s *Store) TouchFile(id string)                             { s.impl.TouchFile(id) }
func (s *Store) SetWorkspace(w models.Workspace)                 { s.impl.SetWorkspace(w) }
func (s *Store) GetWorkspace(id string) (models.Workspace, bool) { return s.impl.GetWorkspace(id) }
func (s *Store) RenameFile(id, n string) (models.FileMeta, bool) { return s.impl.RenameFile(id, n) }
func (s *Store) DeleteFile(id string) bool                       { return s.impl.DeleteFile(id) }
func (s *Store) DeleteFileOnDisk(f models.FileMeta) error        { return s.impl.DeleteFileOnDisk(f) }
func (s *Store) ListInbox() []models.InboxItem                   { return s.impl.ListInbox() }
func (s *Store) AddInbox(it models.InboxItem)                    { s.impl.AddInbox(it) }
func (s *Store) GetInbox(id string) (models.InboxItem, bool)     { return s.impl.GetInbox(id) }
func (s *Store) DeleteInbox(id string) bool                      { return s.impl.DeleteInbox(id) }
func (s *Store) DeleteInboxOnDisk(it models.InboxItem) error     { return s.impl.DeleteInboxOnDisk(it) }
func (s *Store) GetKey(key string) (models.ApiKey, bool)         { return s.impl.GetKey(key) }
func (s *Store) NewKey(label string, q int64, lib bool) models.ApiKey {
	return s.impl.NewKey(label, q, lib)
}
func (s *Store) ListKeys() []models.ApiKey               { return s.impl.ListKeys() }
func (s *Store) UpdateKey(k models.ApiKey)               { s.impl.UpdateKey(k) }
func (s *Store) DeleteKey(key string) bool               { return s.impl.DeleteKey(key) }
func (s *Store) ConsumeTokens(key string, t int64) (models.ApiKey, bool) {
	return s.impl.ConsumeTokens(key, t)
}

// PGStore 返回底层 pgStore（Postgres 不可用时返回 nil）。
// RAG 服务可用它做向量块持久化。
func (s *Store) PGStore() *pgStore { return s.pg }
