package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"jasperlee/backend/internal/models"
)

// ListSocial GET /api/social —— 平台看板条目（公开，供访客浏览）
func (h *Handler) ListSocial(w http.ResponseWriter, _ *http.Request) {
	links := h.store.ListSocialLinks()
	if links == nil {
		links = []models.SocialLink{}
	}
	writeJSON(w, http.StatusOK, links)
}

// AddSocial POST /api/social —— 新增平台条目（管理员）
// body: {platform, category?, account?, url?, note?}
func (h *Handler) AddSocial(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	var req models.SocialLink
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Platform) == "" {
		writeErr(w, http.StatusBadRequest, "platform required")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = "s-" + time.Now().Format("20060102-150405")
	}
	if strings.TrimSpace(req.Category) == "" {
		req.Category = "community"
	}
	h.store.UpsertSocialLink(req)
	writeJSON(w, http.StatusCreated, req)
}

// UpdateSocial PATCH /api/social/{id} —— 编辑条目（管理员）
// body: {platform?, category?, account?, url?, note?}
func (h *Handler) UpdateSocial(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	id := r.PathValue("id")
	var cur *models.SocialLink
	for _, l := range h.store.ListSocialLinks() {
		if l.ID == id {
			cp := l
			cur = &cp
			break
		}
	}
	if cur == nil {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	var patch struct {
		Platform *string `json:"platform"`
		Category *string `json:"category"`
		Account  *string `json:"account"`
		URL      *string `json:"url"`
		Note     *string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if patch.Platform != nil {
		cur.Platform = *patch.Platform
	}
	if patch.Category != nil {
		cur.Category = *patch.Category
	}
	if patch.Account != nil {
		cur.Account = *patch.Account
	}
	if patch.URL != nil {
		cur.URL = *patch.URL
	}
	if patch.Note != nil {
		cur.Note = *patch.Note
	}
	h.store.UpsertSocialLink(*cur)
	writeJSON(w, http.StatusOK, *cur)
}

// DeleteSocial DELETE /api/social/{id} —— 删除条目（管理员）
func (h *Handler) DeleteSocial(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		writeErr(w, http.StatusUnauthorized, "admin token required")
		return
	}
	if !h.store.DeleteSocialLink(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "link not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
