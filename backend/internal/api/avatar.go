package api

import (
	"net/http"

	"jasperlee/backend/internal/core"
)

// GetAvatar GET /api/avatar —— 个人数字分身主页数据（数据定义在 core 领域层）
func (h *Handler) GetAvatar(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, core.AvatarData())
}
