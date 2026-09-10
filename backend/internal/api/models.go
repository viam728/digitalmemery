package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"jasperlee/backend/internal/llm"
)

func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ListModels GET /api/models —— 返回 key 内全部可用模型（供前端切换，默认第一个）
//
// 来源优先级：启动时从 GLM /models 拉取的缓存 > MODEL_LIST（逗号分隔）> MODEL_NAME 单个。
// 无需鉴权（只暴露模型 id，不含密钥）。
func (h *Handler) ListModels(w http.ResponseWriter, _ *http.Request) {
	models := llm.CachedModels()
	if len(models) == 0 && h.cfg.ModelList != "" {
		for _, m := range strings.Split(h.cfg.ModelList, ",") {
			if m = strings.TrimSpace(m); m != "" {
				models = append(models, llm.ModelInfo{ID: m})
			}
		}
	}
	if len(models) == 0 && h.cfg.ModelName != "" {
		models = append(models, llm.ModelInfo{ID: h.cfg.ModelName})
	}
	def := h.currentModel()
	writeJSON(w, http.StatusOK, map[string]any{"default": def, "models": models})
}

// currentModel 当前生效的默认模型
func (h *Handler) currentModel() string {
	if h.llm.Model() != "" {
		return h.llm.Model()
	}
	if h.cfg.ModelName != "" {
		return h.cfg.ModelName
	}
	return "glm-4-flash"
}

// SwitchModel POST /api/models/switch —— 切换默认模型（访客 Key 即可，切换作用于全局）
// body: {"model": "glm-4.5"}，model 必须在可用列表中
func (h *Handler) SwitchModel(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireKey(r); !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	var req struct {
		Model string `json:"model"`
	}
	if err := jsonDecode(r, &req); err != nil || req.Model == "" {
		writeErr(w, http.StatusBadRequest, "model required")
		return
	}
	allowed := map[string]bool{}
	for _, m := range llm.CachedModels() {
		allowed[m.ID] = true
	}
	if h.cfg.ModelList != "" {
		for _, m := range strings.Split(h.cfg.ModelList, ",") {
			if m = strings.TrimSpace(m); m != "" {
				allowed[m] = true
			}
		}
	}
	if h.cfg.ModelName != "" {
		allowed[h.cfg.ModelName] = true
	}
	if len(allowed) > 0 && !allowed[req.Model] {
		writeErr(w, http.StatusBadRequest, "unknown model: "+req.Model)
		return
	}
	h.llm.SetModel(req.Model)
	writeJSON(w, http.StatusOK, map[string]any{"default": req.Model})
}
