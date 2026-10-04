package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jasperlee/backend/internal/agent"
	"jasperlee/backend/internal/llm"
	"jasperlee/backend/internal/models"
)

// UploadInbox POST /api/inbox/upload —— 招聘者上传文件给我（JD / 资料 / 问题清单）
// multipart 字段：file（必填）、name（署名）、note（留言）
func (h *Handler) UploadInbox(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	inbox := filepath.Join(h.cfg.DataDir, "inbox")
	_ = os.MkdirAll(inbox, 0o755)
	id := newID()
	name := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	diskPath := filepath.Join(inbox, id+ext)
	out, err := os.Create(diskPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "write file failed")
		return
	}
	size, err := io.Copy(out, file)
	out.Close()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save file failed")
		return
	}

	it := models.InboxItem{
		ID:        id,
		FileName:  name,
		Name:      r.FormValue("name"),
		Note:      r.FormValue("note"),
		Size:      size,
		CreatedAt: time.Now(),
		Status:    "pending",
	}
	h.store.AddInbox(it)
	// 完全自动响应：后台自动阅读投递并生成回复，不阻塞上传返回
	go h.autoReplyInbox(it.ID, k.Key)
	writeJSON(w, http.StatusCreated, it)
}

// autoReplyInbox 数字分身自动应答收件箱投递（异步后台执行）。
//
// 流程：标记 replying → 提取文本（文本类文件正文 + 留言）→ RAG 检索 →
// 灵魂提示词（人设 + SKILL + 资料）→ 生成回复 → 落库并标记 replied。
// 失败时标记 failed；回复生成后按用量扣减上传者 Key 额度（失败忽略）。
func (h *Handler) autoReplyInbox(id, key string) {
	it, ok := h.store.GetInbox(id)
	if !ok {
		return
	}
	h.store.SetInboxReply(id, "", "replying")

	// 1) 提取投递正文（文本类文件读正文；PDF/Office/图片等仅凭留言与文件名）
	var parts []string
	if text, ok := h.readInboxText(it); ok && strings.TrimSpace(text) != "" {
		if runes := []rune(text); len(runes) > 6000 {
			text = string(runes[:6000]) + "…（已截断）"
		}
		parts = append(parts, "文件《"+it.FileName+"》正文：\n"+text)
	}
	if strings.TrimSpace(it.Note) != "" {
		parts = append(parts, "留言："+strings.TrimSpace(it.Note))
	}
	body := strings.Join(parts, "\n\n")

	// 2) RAG 检索（以留言 + 文件名作查询）
	query := strings.TrimSpace(it.Note + " " + it.FileName)
	if query == "" {
		query = "招聘投递"
	}
	hits := h.rag.Query(context.Background(), query, 4)
	var knowledge strings.Builder
	for _, hit := range hits {
		knowledge.WriteString(fmt.Sprintf("- 《%s》：%s\n", hit.FileName, hit.Chunk))
	}

	// 3) 灵魂提示词 + 生成自动回复
	task := fmt.Sprintf("招聘者通过收件箱投递了文件《%s》。请以第一人称（李俊锋的语气）写一段 2-6 句的中文回复：1) 确认收到投递；2) 基于投递内容给出初步看法（若为 JD，给出岗位匹配度初步判断；若为问题清单，逐条简答或说明后续答复安排）；3) 给出下一步建议。简洁、真诚、不编造。", it.FileName)
	sys := agent.SoulSystemPrompt(knowledge.String(), body, task)

	var reply strings.Builder
	usage, err := h.llm.Stream(context.Background(), llm.StreamRequest{
		Model:    h.currentModel(),
		Messages: []llm.ChatMessage{{Role: "system", Content: sys}, {Role: "user", Content: task + "\n\n" + body}},
		Hits:     hits,
	}, func(delta string) { reply.WriteString(delta) })

	// 4) 落库
	if err != nil || strings.TrimSpace(reply.String()) == "" {
		h.store.SetInboxReply(id, "", "failed")
		return
	}
	h.store.SetInboxReply(id, strings.TrimSpace(reply.String()), "replied")

	// 5) 扣减上传者 Key 额度（失败忽略：回复已生成）
	tokens := usage.PromptTokens + usage.CompletionTokens
	if tokens == 0 {
		tokens = int64(len([]rune(body+task))/4) + 10
	}
	_, _ = h.store.ConsumeTokens(key, tokens)
}

// readInboxText 读取文本类投递文件的正文（二进制/富文本格式返回 false，交由留言兜底）
func (h *Handler) readInboxText(it models.InboxItem) (string, bool) {
	switch inferKind(strings.ToLower(filepath.Ext(it.FileName))) {
	case "markdown", "json", "csv", "code", "text":
	default:
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(h.cfg.DataDir, "inbox", it.ID+filepath.Ext(it.FileName)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

// ListInbox GET /api/inbox —— 收件箱列表
func (h *Handler) ListInbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	items := h.store.ListInbox()
	if items == nil {
		items = []models.InboxItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

// DownloadInbox GET /api/inbox/{id}/download —— 下载招聘者投递的文件
func (h *Handler) DownloadInbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	it, ok := h.store.GetInbox(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "inbox item not found")
		return
	}
	path := filepath.Join(h.cfg.DataDir, "inbox", it.ID+filepath.Ext(it.FileName))
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(it.FileName))
	_, _ = w.Write(data)
}

// DeleteInbox DELETE /api/inbox/{id} —— 删除收件箱条目
func (h *Handler) DeleteInbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	it, ok := h.store.GetInbox(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "inbox item not found")
		return
	}
	_ = h.store.DeleteInboxOnDisk(it)
	if !h.store.DeleteInbox(id) {
		writeErr(w, http.StatusNotFound, "inbox item not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
