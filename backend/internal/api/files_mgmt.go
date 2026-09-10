package api

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// isJasperReadOnly Jasper 的空间只读：Jasper 所有的资料与 Agent 产物不允许改名/删除。
func isJasperReadOnly(f fileMetaLike) bool {
	return f.getOwner() == "李俊锋" || f.getPath() == "我的资料" || f.getPath() == "Jasper 的空间" || f.getIsArtifact()
}

// RenameFile PATCH /api/files/{id} —— 重命名资料库文件（仅「我分享的」，Jasper 的空间只读）
func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
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
	f, ok := h.store.RenameFile(id, req.Name)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// DeleteFile DELETE /api/files/{id} —— 删除资料库文件（仅「我分享的」，Jasper 的空间只读）
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
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
	_ = h.store.DeleteFileOnDisk(f)
	if !h.store.DeleteFile(id) {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// DownloadFile GET /api/files/{id}/download —— 附件下载
func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	f, ok := h.store.GetFile(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
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
