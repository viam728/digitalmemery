package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"jasperlee/backend/internal/models"
)

// adminTokenTTL 管理员会话有效期
const adminTokenTTL = 24 * time.Hour

// AdminLogin POST /api/admin/login —— 管理员入口：密码校验（默认 feng）
// body: {"password": "..."} → {token}
func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Password == "" || req.Password != h.cfg.AdminPassword {
		writeErr(w, http.StatusUnauthorized, "wrong password")
		return
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	h.adminMu.Lock()
	h.adminTokens[token] = time.Now().Add(adminTokenTTL)
	h.adminMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expiresIn": adminTokenTTL / 1e9})
}

// requireAdmin 校验管理员 Bearer Token
func (h *Handler) requireAdmin(r *http.Request) bool {
	token := bearerKey(r)
	if token == "" {
		return false
	}
	h.adminMu.Lock()
	defer h.adminMu.Unlock()
	exp, ok := h.adminTokens[token]
	return ok && exp.After(time.Now())
}

// AdminStats GET /api/admin/stats —— 系统状态看板
func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	keys := h.store.ListKeys()
	var totalQuota, totalUsed int64
	for _, k := range keys {
		totalQuota += k.Quota
		totalUsed += k.Used
	}
	msgs := 0
	for _, c := range h.store.ListConversations() {
		msgs += len(h.store.ListMessages(c.ID))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"uptimeSeconds": int64(time.Since(h.startTime).Seconds()),
		"llm":           h.cfg.ModelProvider,
		"rag":           h.cfg.RAGProvider,
		"counts": map[string]any{
			"files":         len(h.store.ListFiles()),
			"conversations": len(h.store.ListConversations()),
			"messages":      msgs,
			"inbox":         len(h.store.ListInbox()),
			"keys":          len(keys),
		},
		"tokens": map[string]any{
			"totalQuota": totalQuota,
			"totalUsed":  totalUsed,
			"usedRate":   float64(percent(totalUsed, totalQuota)),
		},
	})
}

// AdminListKeys GET /api/admin/keys —— 全部 Key（申请审核与用量）
func (h *Handler) AdminListKeys(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	keys := h.store.ListKeys()
	if keys == nil {
		keys = []models.ApiKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

// AdminUpdateKey PATCH /api/admin/keys/{key} —— 启用/停用/调整额度
// body: {"active": bool, "quota": int64}（active/quota 任一提供即生效）
func (h *Handler) AdminUpdateKey(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	key := r.PathValue("key")
	var req struct {
		Active *bool  `json:"active"`
		Quota  *int64 `json:"quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	k, ok := h.store.GetKey(key)
	if !ok {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	if req.Active != nil {
		k.Active = *req.Active
	}
	if req.Quota != nil {
		k.Quota = *req.Quota
	}
	h.store.UpdateKey(k)
	writeJSON(w, http.StatusOK, k)
}

// AdminRenewKey POST /api/admin/keys/{key}/renew —— 重置用量（重新放发额度）
func (h *Handler) AdminRenewKey(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	key := r.PathValue("key")
	k, ok := h.store.GetKey(key)
	if !ok {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	k.Used = 0
	k.LastUsedAt = time.Time{}
	k.Active = true
	h.store.UpdateKey(k)
	writeJSON(w, http.StatusOK, k)
}

// AdminDeleteKey DELETE /api/admin/keys/{key} —— 删除 Key
func (h *Handler) AdminDeleteKey(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	if !h.store.DeleteKey(r.PathValue("key")) {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}