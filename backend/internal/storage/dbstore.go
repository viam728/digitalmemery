// Package storage —— PostgreSQL 实现。
//
// pgStore 用项目自研的 pgpool（纯标准库 Postgres v3 线协议客户端）读写数据库，
// 零第三方依赖。newPostgres 失败时返回 ok=false，调用方回退到内存实现。
package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/models"
	"jasperlee/backend/internal/pgpool"
)

// pgStore PostgreSQL 持久化实现。单连接 + 内部互斥（pgpool 已串行化），低并发场景足够。
type pgStore struct {
	db      *pgpool.Conn
	dataDir string
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS conversations(
  id text PRIMARY KEY,
  title text NOT NULL DEFAULT '',
  kind text NOT NULL DEFAULT 'chat',
  pinned boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
-- 会话归属访客 Key（会话按访客隔离）；存量库幂等补列
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS owner_key text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS messages(
  id text PRIMARY KEY,
  conversation_id text NOT NULL,
  role text NOT NULL,
  content text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  model text NOT NULL DEFAULT '',
  details jsonb,
  refs jsonb
);
CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, created_at);
CREATE TABLE IF NOT EXISTS files(id text PRIMARY KEY, data jsonb NOT NULL);
-- 产物标记：Agent 工作区产出与资料库同一份存储，标记为产物的文件暴露到 Jasper 的空间。
-- 存量库幂等补列（新字段同时写进 data jsonb，列仅作快速过滤）。
ALTER TABLE files ADD COLUMN IF NOT EXISTS is_artifact boolean NOT NULL DEFAULT false;
CREATE TABLE IF NOT EXISTS workspaces(id text PRIMARY KEY, data jsonb NOT NULL);
CREATE TABLE IF NOT EXISTS api_keys(
  key text PRIMARY KEY,
  label text NOT NULL DEFAULT '',
  quota bigint NOT NULL DEFAULT 0,
  used bigint NOT NULL DEFAULT 0,
  library boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz
);
CREATE TABLE IF NOT EXISTS inbox(id text PRIMARY KEY, data jsonb NOT NULL);
CREATE TABLE IF NOT EXISTS rag_chunks(
  seq bigserial PRIMARY KEY,
  file_id text NOT NULL,
  file_name text NOT NULL,
  chunk text NOT NULL,
  vec jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chunks_file ON rag_chunks(file_id);
`

// newPostgres 连接 Postgres 并建表。ok=false 表示不可用（调用方应降级）。
func newPostgres(cfg *config.Config) (st *pgStore, ok bool) {
	port, _ := strconv.Atoi(cfg.PGPort)
	db, err := pgpool.Open(pgpool.Config{
		Host: cfg.PGHost, Port: port,
		User: cfg.PGUser, Password: cfg.PGPassword, Database: cfg.PGDatabase,
	})
	if err != nil {
		log.Printf("[storage] postgres unavailable (%s:%s/%s): %v — fallback to memory",
			cfg.PGHost, cfg.PGPort, cfg.PGDatabase, err)
		return nil, false
	}
	if err := db.Exec(schemaSQL); err != nil {
		log.Printf("[storage] schema init failed: %v — fallback to memory", err)
		_ = db.Close()
		return nil, false
	}
	st = &pgStore{db: db, dataDir: cfg.DataDir}
	st.seedIfEmpty()
	return st, true
}

// count 返回表行数（表为空即需种入种子）。
func (s *pgStore) count(table string) int64 {
	rows, err := s.db.Query("SELECT COUNT(*) FROM " + table)
	if err != nil || len(rows) == 0 || len(rows[0]) == 0 {
		return 0
	}
	return pgpool.ParseInt64(pgpool.Str(rows[0][0]))
}

// seedIfEmpty 空表时种入李俊锋真实简历种子（与 memStore 同源）。
func (s *pgStore) seedIfEmpty() {
	if s.count("conversations") == 0 {
		for _, c := range seedConversations() {
			s.AddConversation(c)
		}
	}
	if s.count("files") == 0 {
		for _, f := range seedFiles() {
			s.AddFile(f)
		}
	}
	if s.count("api_keys") == 0 {
		k := s.NewKey("演示访客", 1_000_000, true)
		log.Printf("[storage] seeded demo key %s", k.Key)
	}
}

// ---- 会话 CRUD ----

func scanConversation(r []any) models.Conversation {
	return models.Conversation{
		ID:        pgpool.Str(r[0]),
		Title:     pgpool.Str(r[1]),
		Kind:      models.ConversationKind(pgpool.Str(r[2])),
		Pinned:    pgpool.ParseBool(pgpool.Str(r[3])),
		CreatedAt: pgpool.ParseTime(pgpool.Str(r[4])),
		UpdatedAt: pgpool.ParseTime(pgpool.Str(r[5])),
		OwnerKey:  pgpool.Str(r[6]),
	}
}

const convCols = "id,title,kind,pinned,created_at,updated_at,owner_key"

// ListConversations 返回会话（pinned 优先，其次 updated_at 倒序）
func (s *pgStore) ListConversations() []models.Conversation {
	rows, err := s.db.Query("SELECT " + convCols + " FROM conversations ORDER BY pinned DESC, updated_at DESC")
	if err != nil || len(rows) == 0 {
		return []models.Conversation{}
	}
	out := make([]models.Conversation, 0, len(rows))
	for _, r := range rows {
		out = append(out, scanConversation(r))
	}
	return out
}

// AddConversation 新增会话
func (s *pgStore) AddConversation(c models.Conversation) {
	_ = s.db.Exec(fmt.Sprintf(
		"INSERT INTO conversations(id,title,kind,pinned,created_at,updated_at,owner_key) VALUES(%s,%s,%s,%v,%s,%s,%s) ON CONFLICT(id) DO NOTHING",
		pgpool.Quote(c.ID), pgpool.Quote(c.Title), pgpool.Quote(string(c.Kind)),
		c.Pinned, pgpool.QuoteTime(c.CreatedAt), pgpool.QuoteTime(c.UpdatedAt), pgpool.Quote(c.OwnerKey)))
}

// RenameConversation 改名；未命中返回 false
func (s *pgStore) RenameConversation(id, title string) (models.Conversation, bool) {
	n, err := s.db.ExecCount(fmt.Sprintf("UPDATE conversations SET title=%s, updated_at=now() WHERE id=%s",
		pgpool.Quote(title), pgpool.Quote(id)))
	if err != nil || n == 0 {
		return models.Conversation{}, false
	}
	return s.getConversation(id)
}

// UpdateConversationPinned 置顶/取消置顶
func (s *pgStore) UpdateConversationPinned(id string, pinned bool) (models.Conversation, bool) {
	n, err := s.db.ExecCount(fmt.Sprintf("UPDATE conversations SET pinned=%v, updated_at=now() WHERE id=%s",
		pinned, pgpool.Quote(id)))
	if err != nil || n == 0 {
		return models.Conversation{}, false
	}
	return s.getConversation(id)
}

// DeleteConversation 删除会话及其全部消息
func (s *pgStore) DeleteConversation(id string) bool {
	_ = s.db.Exec(fmt.Sprintf("DELETE FROM messages WHERE conversation_id=%s", pgpool.Quote(id)))
	n, err := s.db.ExecCount(fmt.Sprintf("DELETE FROM conversations WHERE id=%s", pgpool.Quote(id)))
	return err == nil && n > 0
}

func (s *pgStore) getConversation(id string) (models.Conversation, bool) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT "+convCols+" FROM conversations WHERE id=%s", pgpool.Quote(id)))
	if err != nil || len(rows) == 0 {
		return models.Conversation{}, false
	}
	return scanConversation(rows[0]), true
}

// GetConversation 按 ID 取会话（供 api 层做归属校验）
func (s *pgStore) GetConversation(id string) (models.Conversation, bool) {
	return s.getConversation(id)
}

// ---- 消息 ----

func scanMessage(r []any) models.Message {
	m := models.Message{
		ID:             pgpool.Str(r[0]),
		ConversationID: pgpool.Str(r[1]),
		Role:           pgpool.Str(r[2]),
		Content:        pgpool.Str(r[3]),
		CreatedAt:      pgpool.ParseTime(pgpool.Str(r[4])),
		Model:          pgpool.Str(r[5]),
	}
	if d := pgpool.Str(r[6]); d != "" && d != "null" {
		var det models.ResponseDetails
		if json.Unmarshal([]byte(d), &det) == nil {
			m.Details = &det
		}
	}
	if rf := pgpool.Str(r[7]); rf != "" && rf != "null" {
		var refs []string
		if json.Unmarshal([]byte(rf), &refs) == nil {
			m.Refs = refs
		}
	}
	return m
}

const msgCols = "id,conversation_id,role,content,created_at,model,details,refs"

// ListMessages 取会话消息（创建时间升序）
func (s *pgStore) ListMessages(id string) []models.Message {
	rows, err := s.db.Query(fmt.Sprintf("SELECT "+msgCols+" FROM messages WHERE conversation_id=%s ORDER BY created_at ASC",
		pgpool.Quote(id)))
	if err != nil || len(rows) == 0 {
		return []models.Message{}
	}
	out := make([]models.Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, scanMessage(r))
	}
	return out
}

// AddMessage 追加一条消息
func (s *pgStore) AddMessage(m models.Message) {
	det := "NULL"
	if m.Details != nil {
		if b, err := json.Marshal(m.Details); err == nil {
			det = pgpool.Quote(string(b))
		}
	}
	refs := "NULL"
	if m.Refs != nil {
		if b, err := json.Marshal(m.Refs); err == nil {
			refs = pgpool.Quote(string(b))
		}
	}
	_ = s.db.Exec(fmt.Sprintf(
		"INSERT INTO messages(id,conversation_id,role,content,created_at,model,details,refs) VALUES(%s,%s,%s,%s,%s,%s,%s::jsonb,%s::jsonb) ON CONFLICT(id) DO NOTHING",
		pgpool.Quote(m.ID), pgpool.Quote(m.ConversationID), pgpool.Quote(m.Role),
		pgpool.Quote(m.Content), pgpool.QuoteTime(m.CreatedAt), pgpool.Quote(m.Model), det, refs))
}

// ---- 文件（元数据整对象存 jsonb，磁盘正文不变） ----

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

// ListFiles 返回文件列表
func (s *pgStore) ListFiles() []models.FileMeta {
	rows, err := s.db.Query("SELECT data FROM files")
	if err != nil || len(rows) == 0 {
		return []models.FileMeta{}
	}
	out := make([]models.FileMeta, 0, len(rows))
	for _, r := range rows {
		var f models.FileMeta
		if json.Unmarshal([]byte(pgpool.Str(r[0])), &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

// MarkIngested 标记文件已向量化入库
func (s *pgStore) MarkIngested(id string) bool {
	f, ok := s.GetFile(id)
	if !ok {
		return false
	}
	f.Ingested = true
	return s.upsertFile(f)
}

// AddFile 新增文件元数据
func (s *pgStore) AddFile(f models.FileMeta) {
	_ = s.upsertFile(f)
}

func (s *pgStore) upsertFile(f models.FileMeta) bool {
	err := s.db.Exec(fmt.Sprintf("INSERT INTO files(id,data,is_artifact) VALUES(%s,%s::jsonb,%v) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data, is_artifact=EXCLUDED.is_artifact",
		pgpool.Quote(f.ID), pgpool.Quote(mustJSON(f)), f.IsArtifact))
	return err == nil
}

// MarkArtifact 标记文件为 Agent 产物（自动暴露到 Jasper 的空间）
func (s *pgStore) MarkArtifact(id string, workspaceID string) (models.FileMeta, bool) {
	f, ok := s.GetFile(id)
	if !ok {
		return models.FileMeta{}, false
	}
	f.IsArtifact = true
	f.WorkspaceID = workspaceID
	f.UpdatedAt = time.Now()
	if !s.upsertFile(f) {
		return models.FileMeta{}, false
	}
	return f, true
}

// ListArtifacts 列出全部 Agent 产物（Jasper 的空间对外暴露的部分）
func (s *pgStore) ListArtifacts() []models.FileMeta {
	rows, err := s.db.Query("SELECT data FROM files WHERE is_artifact=true")
	if err != nil || len(rows) == 0 {
		return []models.FileMeta{}
	}
	out := make([]models.FileMeta, 0, len(rows))
	for _, r := range rows {
		var f models.FileMeta
		if json.Unmarshal([]byte(pgpool.Str(r[0])), &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

// GetFile 按 ID 取文件元数据
func (s *pgStore) GetFile(id string) (models.FileMeta, bool) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT data FROM files WHERE id=%s", pgpool.Quote(id)))
	if err != nil || len(rows) == 0 {
		return models.FileMeta{}, false
	}
	var f models.FileMeta
	if json.Unmarshal([]byte(pgpool.Str(rows[0][0])), &f) != nil {
		return models.FileMeta{}, false
	}
	return f, true
}

// ReadBytes 读取文件原始字节（磁盘优先；种子数据回退到示例内容）
func (s *pgStore) ReadBytes(f models.FileMeta) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(s.dataDir, "uploads", f.ID+filepath.Ext(f.Name)))
	if err == nil {
		return b, nil
	}
	if c, ok := demoContents[f.ID]; ok {
		return []byte(c), nil
	}
	return nil, err
}

// ReadContent 读取文件文本内容用于 ingest / 上下文注入
func (s *pgStore) ReadContent(f models.FileMeta) (string, bool) {
	b, err := s.ReadBytes(f)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// TouchFile 更新文件最近访问时间
func (s *pgStore) TouchFile(id string) {
	if f, ok := s.GetFile(id); ok {
		f.AccessedAt = time.Now()
		s.upsertFile(f)
	}
}

// RenameFile 重命名文件元数据
func (s *pgStore) RenameFile(id, name string) (models.FileMeta, bool) {
	f, ok := s.GetFile(id)
	if !ok {
		return models.FileMeta{}, false
	}
	f.Name = name
	f.UpdatedAt = time.Now()
	if !s.upsertFile(f) {
		return models.FileMeta{}, false
	}
	return f, true
}

// DeleteFile 删除文件元数据（磁盘文件由调用方负责清理）
func (s *pgStore) DeleteFile(id string) bool {
	n, err := s.db.ExecCount(fmt.Sprintf("DELETE FROM files WHERE id=%s", pgpool.Quote(id)))
	return err == nil && n > 0
}

// DeleteFileOnDisk 删除资料库磁盘文件
func (s *pgStore) DeleteFileOnDisk(f models.FileMeta) error {
	path := filepath.Join(s.dataDir, "uploads", f.ID+filepath.Ext(f.Name))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ---- 工作区 ----

// SetWorkspace 保存 Agent 工作区（新建或覆盖）
func (s *pgStore) SetWorkspace(w models.Workspace) {
	_ = s.db.Exec(fmt.Sprintf("INSERT INTO workspaces(id,data) VALUES(%s,%s::jsonb) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data",
		pgpool.Quote(w.ID), pgpool.Quote(mustJSON(w))))
}

// GetWorkspace 取 Agent 工作区
func (s *pgStore) GetWorkspace(id string) (models.Workspace, bool) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT data FROM workspaces WHERE id=%s", pgpool.Quote(id)))
	if err != nil || len(rows) == 0 {
		return models.Workspace{}, false
	}
	var w models.Workspace
	if json.Unmarshal([]byte(pgpool.Str(rows[0][0])), &w) != nil {
		return models.Workspace{}, false
	}
	return w, true
}

// ---- 收件箱 ----

// ListInbox 收件箱条目（新投递在前，按 data.createdAt 倒序即插入倒序；简化用 seq 倒序的近似：id 创建时间前缀
func (s *pgStore) ListInbox() []models.InboxItem {
	rows, err := s.db.Query("SELECT data FROM inbox")
	if err != nil || len(rows) == 0 {
		return []models.InboxItem{}
	}
	out := make([]models.InboxItem, 0, len(rows))
	for _, r := range rows {
		var it models.InboxItem
		if json.Unmarshal([]byte(pgpool.Str(r[0])), &it) == nil {
			out = append(out, it)
		}
	}
	// 插入时写最前：此处按 CreatedAt 倒序
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// AddInbox 新增收件箱条目
func (s *pgStore) AddInbox(it models.InboxItem) {
	_ = s.db.Exec(fmt.Sprintf("INSERT INTO inbox(id,data) VALUES(%s,%s::jsonb) ON CONFLICT(id) DO UPDATE SET data=EXCLUDED.data",
		pgpool.Quote(it.ID), pgpool.Quote(mustJSON(it))))
}

// GetInbox 按 ID 取收件箱条目
func (s *pgStore) GetInbox(id string) (models.InboxItem, bool) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT data FROM inbox WHERE id=%s", pgpool.Quote(id)))
	if err != nil || len(rows) == 0 {
		return models.InboxItem{}, false
	}
	var it models.InboxItem
	if json.Unmarshal([]byte(pgpool.Str(rows[0][0])), &it) != nil {
		return models.InboxItem{}, false
	}
	return it, true
}

// DeleteInbox 删除收件箱条目元数据
func (s *pgStore) DeleteInbox(id string) bool {
	n, err := s.db.ExecCount(fmt.Sprintf("DELETE FROM inbox WHERE id=%s", pgpool.Quote(id)))
	return err == nil && n > 0
}

// DeleteInboxOnDisk 删除收件箱磁盘文件
func (s *pgStore) DeleteInboxOnDisk(it models.InboxItem) error {
	path := filepath.Join(s.dataDir, "inbox", it.ID+filepath.Ext(it.FileName))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SetInboxReply 更新收件箱条目的自动回复与状态（数字分身自动应答）
func (s *pgStore) SetInboxReply(id, reply, status string) (models.InboxItem, bool) {
	it, ok := s.GetInbox(id)
	if !ok {
		return models.InboxItem{}, false
	}
	it.Status = status
	if reply != "" {
		it.Reply = reply
		it.RepliedAt = time.Now()
	}
	s.AddInbox(it)
	return it, true
}

// ---- Key ----

func scanKey(r []any) models.ApiKey {
	k := models.ApiKey{
		Key:     pgpool.Str(r[0]),
		Label:   pgpool.Str(r[1]),
		Quota:   pgpool.ParseInt64(pgpool.Str(r[2])),
		Used:    pgpool.ParseInt64(pgpool.Str(r[3])),
		Library: pgpool.ParseBool(pgpool.Str(r[4])),
		Active:  pgpool.ParseBool(pgpool.Str(r[5])),
	}
	k.CreatedAt = pgpool.ParseTime(pgpool.Str(r[6]))
	k.LastUsedAt = pgpool.ParseTime(pgpool.Str(r[7]))
	return k
}

const keyCols = "key,label,quota,used,library,active,created_at,last_used_at"

// GetKey 按凭证取 Key
func (s *pgStore) GetKey(key string) (models.ApiKey, bool) {
	rows, err := s.db.Query(fmt.Sprintf("SELECT "+keyCols+" FROM api_keys WHERE key=%s", pgpool.Quote(key)))
	if err != nil || len(rows) == 0 {
		return models.ApiKey{}, false
	}
	return scanKey(rows[0]), true
}

// NewKey 生成一个新的临时访问 Key（随机 16 字节 hex，PG 冲突概率可忽略）
func (s *pgStore) NewKey(label string, quota int64, lib bool) models.ApiKey {
	k := models.ApiKey{
		Key: randomKey(), Label: label, Quota: quota, Library: lib,
		Active: true, CreatedAt: time.Now(),
	}
	_ = s.db.Exec(fmt.Sprintf(
		"INSERT INTO api_keys(key,label,quota,used,library,active,created_at) VALUES(%s,%s,%d,0,%v,true,%s)",
		pgpool.Quote(k.Key), pgpool.Quote(k.Label), quota, lib, pgpool.QuoteTime(k.CreatedAt)))
	return k
}

// ListKeys 返回全部 Key（管理员查看申请与用量）
func (s *pgStore) ListKeys() []models.ApiKey {
	rows, err := s.db.Query("SELECT " + keyCols + " FROM api_keys ORDER BY created_at DESC")
	if err != nil || len(rows) == 0 {
		return []models.ApiKey{}
	}
	out := make([]models.ApiKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, scanKey(r))
	}
	return out
}

// UpdateKey 更新 Key（管理员启用/停用/调额度/重置用量）
func (s *pgStore) UpdateKey(k models.ApiKey) {
	lu := "NULL"
	if !k.LastUsedAt.IsZero() {
		lu = pgpool.QuoteTime(k.LastUsedAt)
	}
	_ = s.db.Exec(fmt.Sprintf(
		"UPDATE api_keys SET label=%s, quota=%d, used=%d, library=%v, active=%v, last_used_at=%s WHERE key=%s",
		pgpool.Quote(k.Label), k.Quota, k.Used, k.Library, k.Active, lu, pgpool.Quote(k.Key)))
}

// DeleteKey 删除 Key
func (s *pgStore) DeleteKey(key string) bool {
	n, err := s.db.ExecCount(fmt.Sprintf("DELETE FROM api_keys WHERE key=%s", pgpool.Quote(key)))
	return err == nil && n > 0
}

// ConsumeTokens 从 Key 额度中扣除 tokens，额度用尽返回 false（单条原子 UPDATE 避免竞态）
func (s *pgStore) ConsumeTokens(key string, tokens int64) (models.ApiKey, bool) {
	n, err := s.db.ExecCount(fmt.Sprintf(
		"UPDATE api_keys SET used=used+%d, last_used_at=now() WHERE key=%s AND active AND (quota<=0 OR used+%d<=quota)",
		tokens, pgpool.Quote(key), tokens))
	if err != nil || n == 0 {
		k, _ := s.GetKey(key)
		return k, false
	}
	k, _ := s.GetKey(key)
	return k, true
}

// ---- RAG chunks（文件入库的文本块；向量存 jsonb 供 SQL 检索/导出） ----

// SaveRagChunks 替换某文件的全部文本块（先删后插，保证幂等）。
func (s *pgStore) SaveRagChunks(fileID, fileName string, chunks []string, vecs [][]float64) error {
	if err := s.db.Exec(fmt.Sprintf("DELETE FROM rag_chunks WHERE file_id=%s", pgpool.Quote(fileID))); err != nil {
		return err
	}
	for i, ch := range chunks {
		var v string = "[]"
		if i < len(vecs) && len(vecs[i]) > 0 {
			b, _ := json.Marshal(vecs[i])
			v = string(b)
		}
		if err := s.db.Exec(fmt.Sprintf("INSERT INTO rag_chunks(file_id,file_name,chunk,vec) VALUES(%s,%s,%s,%s::jsonb)",
			pgpool.Quote(fileID), pgpool.Quote(fileName), pgpool.Quote(ch), pgpool.Quote(v))); err != nil {
			return err
		}
	}
	return nil
}

// ListRagChunks 返回某文件的文本块与向量（vec 为 nil 表示尚未向量化）。
func (s *pgStore) ListRagChunks(fileID string) (chunks []string, vecs [][]float64, fileName string, err error) {
	rows, err := s.db.Query(fmt.Sprintf(
		"SELECT file_name,chunk,vec FROM rag_chunks WHERE file_id=%s ORDER BY seq ASC", pgpool.Quote(fileID)))
	if err != nil {
		return nil, nil, "", err
	}
	for _, r := range rows {
		fileName = pgpool.Str(r[0])
		chunks = append(chunks, pgpool.Str(r[1]))
		if vs := pgpool.Str(r[2]); vs != "" && vs != "[]" && vs != "null" {
			var v []float64
			if json.Unmarshal([]byte(vs), &v) == nil && len(v) > 0 {
				vecs = append(vecs, v)
				continue
			}
		}
		vecs = append(vecs, nil)
	}
	return chunks, vecs, fileName, nil
}

// ListRagFiles 列出已有文本块的文件 ID。
func (s *pgStore) ListRagFiles() []string {
	rows, err := s.db.Query("SELECT DISTINCT file_id FROM rag_chunks")
	if err != nil || len(rows) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, pgpool.Str(r[0]))
	}
	return out
}

// DeleteRagChunks 删除某文件的全部文本块。
func (s *pgStore) DeleteRagChunks(fileID string) {
	_ = s.db.Exec(fmt.Sprintf("DELETE FROM rag_chunks WHERE file_id=%s", pgpool.Quote(fileID)))
}

var _ = sync.Mutex{}
