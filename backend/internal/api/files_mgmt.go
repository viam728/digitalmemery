package api

import (
	"encoding/json"
	"net/http"
	"net/url"

	"jasperlee/backend/internal/models"
)

// isJasperReadOnly Jasper 的空间只读：Jasper 所有的资料与 Agent 产物不允许改名/删除。
func isJasperReadOnly(f fileMetaLike) bool {
	return f.getOwner() == "李俊锋" || f.getPath() == "我的资料" || f.getPath() == "Jasper 的空间" || f.getIsArtifact()
}

// fileVisibleTo 文件对该访客 Key 是否可见：Jasper 的公开资料/产物、本人上传、历史无归属文件。
func fileVisibleTo(f models.FileMeta, key string) bool {
	if isJasperReadOnly(fileMetaWrap{f}) {
		return true
	}
	return f.OwnerKey == "" || f.OwnerKey == key
}

// fileWritableBy 文件是否可由该访客 Key 修改（改名/删除）：仅本人上传的分享文件。
func fileWritableBy(f models.FileMeta, key string) bool {
	if isJasperReadOnly(fileMetaWrap{f}) {
		return false
	}
	return f.OwnerKey != "" && f.OwnerKey == key
}

// RenameFile PATCH /api/files/{id} —— 重命名资料库文件（仅「我分享的」，Jasper 的空间只读）
func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	id := r.PathValue("id")
	existing, ok := h.store.GetFile(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	if isJasperReadOnly(fileMetaWrap{existing}) {
		writeErr(w, http.StatusForbidden, "Jasper 的空间只读，不可重命名")
		return
	}
	if !fileWritableBy(existing, k.Key) {
		writeErr(w, http.StatusForbidden, "仅可操作本人上传的文件")
		return
	}
	f, ok := h.store.RenameFile(id, req.Name)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// DeleteFile DELETE /api/files/{id} —— 删除资料库文件（仅「我分享的」，Jasper 的空间只读）
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	f, ok := h.store.GetFile(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	if isJasperReadOnly(fileMetaWrap{f}) {
		writeErr(w, http.StatusForbidden, "Jasper 的空间只读，不可删除")
		return
	}
	if !fileWritableBy(f, k.Key) {
		writeErr(w, http.StatusForbidden, "仅可操作本人上传的文件")
		return
	}
	_ = h.store.DeleteFileOnDisk(f)
	if !h.store.DeleteFile(id) {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// DownloadFile GET /api/files/{id}/download —— 附件下载
func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	if !k.Library {
		writeErr(w, http.StatusForbidden, "library access not granted on this key")
		return
	}
	id := r.PathValue("id")
	f, ok := h.store.GetFile(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	if !fileVisibleTo(f, k.Key) {
		writeErr(w, http.StatusForbidden, "无权访问该文件")
		return
	}
	data, err := h.store.ReadBytes(f)
	if err != nil {
		writeErr(w, http.StatusNotFound, "file content not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(f.Name))
	_, _ = w.Write(data)
}
