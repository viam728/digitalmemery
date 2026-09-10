package api

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jasperlee/backend/internal/models"
)

// UploadInbox POST /api/inbox/upload —— 招聘者上传文件给我（JD / 资料 / 问题清单）
// multipart 字段：file（必填）、name（署名）、note（留言）
func (h *Handler) UploadInbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
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
	}
	h.store.AddInbox(it)
	writeJSON(w, http.StatusCreated, it)
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