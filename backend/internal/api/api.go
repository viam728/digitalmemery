package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jasperlee/backend/internal/agent"
	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/llm"
	"jasperlee/backend/internal/models"
	"jasperlee/backend/internal/rag"
	"jasperlee/backend/internal/storage"
)

// Handler 聚合各模块
type Handler struct {
	cfg         *config.Config
	store       *storage.Store
	rag         *rag.Service
	agent       *agent.WorkspaceSvc
	llm         *llm.Client
	startTime   time.Time
	adminMu     sync.Mutex
	adminTokens map[string]time.Time // admin token -> 过期时间
}

func New(cfg *config.Config) *Handler {
	// 共享模型客户端：会话流式与 Agent 任务复用同一实例，仅构造一次
	llmClient := llm.NewWithOptions(cfg.ModelProvider, cfg.ModelAPIKey, cfg.ModelBaseURL, cfg.ModelHostIP, cfg.ModelName)
	ragSvc := rag.New(cfg.RAGProvider, cfg.DataDir, cfg.ModelAPIKey, cfg.ModelBaseURL)
	h := &Handler{
		cfg:         cfg,
		store:       storage.New(cfg),
		rag:         ragSvc,
		agent:       agent.NewWorkspaceSvc(llmClient),
		llm:         llmClient,
		startTime:   time.Now(),
		adminTokens: make(map[string]time.Time),
	}
	// GLM 默认模型：key 内的模型全配进去（默认 MODEL_NAME）
	h.initModelDefaults()
	// 后台异步向量化：不阻塞服务启动（网络慢时也能立即响应）
	go h.bootstrapIndex()
	// 后台刷新 GLM 模型列表（失败仅打日志）
	go llm.RefreshModels(cfg.ModelBaseURL, cfg.ModelAPIKey, cfg.ModelHostIP)
	return h
}

// initModelDefaults 初始化模型默认值：MODEL_LIST 全配进去，默认 MODEL_NAME
func (h *Handler) initModelDefaults() {
	if h.cfg.ModelList != "" {
		return // 已显式配置，不覆盖
	}
	// 默认把 key 内已知的 GLM 模型全配进去（启动后台刷新会更新缓存）
	h.cfg.ModelList = "glm-4.5,glm-4.5-air,glm-4.6,glm-4.7,glm-5,glm-5-turbo,glm-5.1,glm-5.2,glm-5.3,glm-5.3-flash"
	if h.cfg.ModelName == "" || h.cfg.ModelName == "deepseek-chat" {
		h.cfg.ModelName = "glm-4-flash"
	}
	h.llm.SetModel(h.cfg.ModelName)
}

// bootstrapIndex 启动时自动把「关于我」种子文档向量化入库，
// 保证招聘者第一次提问就能被分身基于知识库回答（无需手动点击入库）。
func (h *Handler) bootstrapIndex() {
	for _, f := range h.store.ListFiles() {
		if f.Ingested {
			continue
		}
		// 二进制类型（PDF/Office/图片）不做向量化，其全文由对应 md 承载
		switch f.Kind {
		case "pdf", "docx", "image":
			continue
		}
		content, ok := h.store.ReadContent(f)
		if !ok || len(content) < 20 {
			continue
		}
		if err := h.rag.Ingest(context.Background(), f.ID, f.Name, content); err != nil {
			log.Printf("[bootstrap] ingest %s failed: %v", f.Name, err)
			continue
		}
		h.store.MarkIngested(f.ID)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func bearerKey(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// requireKey 校验 Key 并写入上下文。无效或额度用尽返回 false。
func (h *Handler) requireKey(r *http.Request) (models.ApiKey, bool) {
	key := bearerKey(r)
	if key == "" {
		return models.ApiKey{}, false
	}
	k, ok := h.store.GetKey(key)
	return k, ok && k.Active
}

// Health GET /api/health
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
		"rag":    h.cfg.RAGProvider,
		"llm":    h.cfg.ModelProvider,
	})
}

// ApplyKey POST /api/keys/apply  —— 访客申请临时 Key
// body: {label?, library?} ，默认发放 100 万 token 额度 + 资料库访问权限
// （招聘者一键申请即看简历资料；显式传 library:false 可关闭）
func (h *Handler) ApplyKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label   string `json:"label"`
		Library *bool  `json:"library"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Label == "" {
		req.Label = "访客"
	}
	lib := true
	if req.Library != nil {
		lib = *req.Library
	}
	k := h.store.NewKey(req.Label, 1_000_000, lib)
	writeJSON(w, http.StatusCreated, k)
}

// VerifyKey GET /api/keys/me  —— 校验当前 Bearer Key，返回状态与剩余额度
func (h *Handler) VerifyKey(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "invalid or inactive key")
		return
	}
	remaining := int64(-1)
	if k.Quota > 0 {
		remaining = k.Quota - k.Used
		if remaining < 0 {
			remaining = 0
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":     true,
		"label":     k.Label,
		"library":   k.Library,
		"quota":     k.Quota,
		"used":      k.Used,
		"remaining": remaining,
	})
}

// ListConversations GET /api/conversations
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	// 会话按访客隔离：只返回归属当前 Key 的会话
	out := []models.Conversation{}
	for _, c := range h.store.ListConversations() {
		if c.OwnerKey == k.Key {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateConversation POST /api/conversations
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	now := time.Now()
	c := models.Conversation{
		ID:        newID(),
		Title:     req.Title,
		Kind:      models.KindChat,
		CreatedAt: now,
		UpdatedAt: now,
		Pinned:    true,
		OwnerKey:  k.Key,
	}
	h.store.AddConversation(c)
	writeJSON(w, http.StatusCreated, c)
}

// UpdateConversation PATCH /api/conversations/{id} —— 改名 / 置顶
// body: {"title"?: "...", "pinned"?: bool}，任选其一（同时给则都生效）
func (h *Handler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	// 会话按访客隔离：仅归属者可见/可改（对他人一律 404，不暴露存在性）
	if conv, ok := h.store.GetConversation(id); !ok || conv.OwnerKey != k.Key {
		writeErr(w, http.StatusNotFound, "conversation not found")
		return
	}
	var req struct {
		Title  *string `json:"title"`
		Pinned *bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Title == nil && req.Pinned == nil {
		writeErr(w, http.StatusBadRequest, "title or pinned required")
		return
	}
	var c models.Conversation
	ok = true
	if req.Title != nil {
		c, ok = h.store.RenameConversation(id, *req.Title)
		if !ok {
			writeErr(w, http.StatusNotFound, "conversation not found")
			return
		}
	}
	if req.Pinned != nil {
		c, ok = h.store.UpdateConversationPinned(id, *req.Pinned)
		if !ok {
			writeErr(w, http.StatusNotFound, "conversation not found")
			return
		}
	}
	writeJSON(w, http.StatusOK, c)
}

// DeleteConversation DELETE /api/conversations/{id} —— 删除会话及其消息
func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	// 会话按访客隔离：仅归属者可删
	if conv, ok := h.store.GetConversation(r.PathValue("id")); !ok || conv.OwnerKey != k.Key {
		writeErr(w, http.StatusNotFound, "conversation not found")
		return
	}
	if !h.store.DeleteConversation(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ListMessages GET /api/conversations/{id}/messages
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	// 会话按访客隔离：仅归属者可读消息
	if conv, ok := h.store.GetConversation(id); !ok || conv.OwnerKey != k.Key {
		writeErr(w, http.StatusNotFound, "conversation not found")
		return
	}
	msgs := h.store.ListMessages(id)
	if msgs == nil {
		msgs = []models.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ListFiles GET /api/files
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	if !k.Library {
		writeErr(w, http.StatusForbidden, "library access not granted on this key")
		return
	}
	writeJSON(w, http.StatusOK, h.store.ListFiles())
}

// UploadFile POST /api/files/upload —— multipart 上传文件到资料库
// 保存到 DATA_DIR/uploads，按扩展名推断 kind 并新建 FileMeta
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok || !k.Library {
		writeErr(w, http.StatusUnauthorized, "library access required")
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

	uploads := filepath.Join(h.cfg.DataDir, "uploads")
	_ = os.MkdirAll(uploads, 0o755)
	id := newID()
	name := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	diskPath := filepath.Join(uploads, id+ext)
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

	// 上传落到「我分享的」：Jasper 的空间只读，访客上传只能进分享区
	f := models.FileMeta{
		ID:         id,
		Name:       name,
		Kind:       inferKind(ext),
		Path:       "我分享的",
		Owner:      k.Label,
		Size:       size,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		AccessedAt: time.Now(),
	}
	h.store.AddFile(f)
	writeJSON(w, http.StatusCreated, f)
}

// GetFileContent GET /api/files/{id}/content —— 返回文件内容用于预览
// 文本类（markdown/json/csv/code/text）返回 UTF-8 文本；图片/PDF/Office 返回原始字节
func (h *Handler) GetFileContent(w http.ResponseWriter, r *http.Request) {
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
	h.store.TouchFile(id)
	w.Header().Set("Content-Type", contentTypeOf(f))
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(f.Name))
	_, _ = w.Write(data)
}

// IngestFile POST /api/files/{id}/ingest —— 读取文件内容 → 分块 → 索引 → 标记
func (h *Handler) IngestFile(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok || !k.Library {
		writeErr(w, http.StatusUnauthorized, "library access required")
		return
	}
	id := r.PathValue("id")
	f, ok := h.store.GetFile(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	content, ok := h.store.ReadContent(f)
	if !ok {
		writeErr(w, http.StatusBadRequest, "no readable text content")
		return
	}
	if err := h.rag.Ingest(r.Context(), f.ID, f.Name, content); err != nil {
		writeErr(w, http.StatusInternalServerError, "ingest failed: "+err.Error())
		return
	}
	h.store.MarkIngested(id)
	writeJSON(w, http.StatusOK, map[string]any{"ingested": true})
}

// RAGQuery POST /api/rag/query
func (h *Handler) RAGQuery(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok || !k.Library {
		writeErr(w, http.StatusUnauthorized, "library access required")
		return
	}
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		writeErr(w, http.StatusBadRequest, "query required")
		return
	}
	hits := h.rag.Query(r.Context(), req.Query, 3)
	if hits == nil {
		hits = []models.RagHit{}
	}
	writeJSON(w, http.StatusOK, hits)
}

// GetWorkspace GET /api/workspaces/{id}
func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	id := r.PathValue("id")
	// 工作区按访客隔离：非归属者视为不存在（返回示例树）
	if ws, ok := h.store.GetWorkspace(id); ok && ws.OwnerKey == k.Key {
		writeJSON(w, http.StatusOK, ws)
		return
	}
	writeJSON(w, http.StatusOK, h.agent.GreetingWorkspace())
}

// CreateWorkspace POST /api/workspaces —— 新建 Agent 任务工作区
// body: {title?, model?, refs?} ；refs 为要引用的资料库文件 id
func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	var req struct {
		Title string   `json:"title"`
		Model string   `json:"model"`
		Refs  []string `json:"refs"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		req.Title = "新任务"
	}
	model := req.Model
	if model == "" {
		model = cfgModel(h.cfg)
	}
	ws := h.agent.CreateWorkspace(newWSID(), req.Title, model)
	ws.OwnerKey = k.Key

	// 把引用的资料库文件挂到工作区文件树
	var refFiles []models.WorkspaceFile
	for _, rid := range req.Refs {
		if f, ok := h.store.GetFile(rid); ok {
			refFiles = append(refFiles, models.WorkspaceFile{
				ID:         f.ID,
				Name:       f.Name,
				Kind:       f.Kind,
				Path:       f.Path,
				Referenced: true,
			})
		}
	}
	if len(refFiles) > 0 {
		ws = h.agent.WithRefs(ws, refFiles)
	}
	h.store.SetWorkspace(ws)
	writeJSON(w, http.StatusCreated, ws)
}

// RunWorkspace POST /api/workspaces/{id}/run —— 执行 Agent 任务并挂载产出文件
// 读取引用文件内容 + RAG 命中 → 灵魂提示词 → 真实模型 → 产物落资料库（带产物标记，暴露到 Jasper 的空间）
func (h *Handler) RunWorkspace(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	// 额度硬校验：与对话一致，用尽即拒绝
	if k.Quota > 0 && k.Used >= k.Quota {
		writeErr(w, http.StatusPaymentRequired, "token 额度已用尽：请联系管理员在后台重置额度后继续")
		return
	}
	id := r.PathValue("id")
	var req struct {
		Prompt string `json:"prompt"`
		// Expose 是否把产物暴露到 Jasper 的空间（默认 true）
		Expose *bool `json:"expose"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	ws, ok := h.store.GetWorkspace(id)
	if !ok || ws.OwnerKey != k.Key {
		writeErr(w, http.StatusNotFound, "workspace not found")
		return
	}

	// 收集引用文件内容作为上下文
	refs := collectRefs(ws.Files, func(fid string) models.RefContent {
		if f, ok := h.store.GetFile(fid); ok {
			if c, ok := h.store.ReadContent(f); ok {
				return models.RefContent{ID: f.ID, Name: f.Name, Content: c}
			}
		}
		return models.RefContent{ID: fid, Name: fid, Content: ""}
	})

	// RAG 检索命中
	hits := h.rag.Query(r.Context(), req.Prompt, 4)

	model := ws.Model
	if model == "" {
		model = h.currentModel()
		ws.Model = model
	}
	ws = h.agent.RunWithSoul(r.Context(), ws, req.Prompt, refs, hits, model)

	// 产物落资料库：同一份存储（同一行记录），带产物标记 → 自动暴露到 Jasper 的空间
	expose := true
	if req.Expose != nil {
		expose = *req.Expose
	}
	artifacts := h.publishArtifacts(ws, id, expose)

	// 扣减 token 用量
	if ws.Usage != nil {
		tokens := ws.Usage.PromptTokens + ws.Usage.CompletionTokens
		k, _ = h.store.ConsumeTokens(k.Key, tokens)
	}
	h.store.SetWorkspace(ws)
	writeJSON(w, http.StatusOK, map[string]any{"workspace": ws, "artifacts": artifacts})
}

// StreamChat POST /api/conversations/{id}/messages/stream —— SSE 流式对话
// body: {content, refs?, mode?} ；完成时扣减 token
//
// mode: "chat"（默认，面试应答）| "agent"（Agent 任务：走灵魂工作流并产出文件）。
// 对话与 Agent 共用同一个会话历史（同一 conversation id），Agent 产物同时落库为助手消息。
// 采用 Server-Sent Events：逐行输出 delta，末尾追加 usage + details + steps 事件。
func (h *Handler) StreamChat(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	// 额度硬校验：用尽即拒绝，给出明确提示（成熟 Agent 的配额边界）
	if k.Quota > 0 && k.Used >= k.Quota {
		writeErr(w, http.StatusPaymentRequired, "token 额度已用尽：请联系管理员在后台重置额度后继续对话")
		return
	}
	// 会话归属校验：只能向自己的会话发消息（防跨访客串话）
	if conv, ok := h.store.GetConversation(r.PathValue("id")); !ok || conv.OwnerKey != k.Key {
		writeErr(w, http.StatusNotFound, "conversation not found")
		return
	}

	var req struct {
		Content string   `json:"content"`
		Refs    []string `json:"refs"`
		Mode    string   `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		writeErr(w, http.StatusBadRequest, "content required")
		return
	}
	cid := r.PathValue("id")
	mode := req.Mode
	if mode != "agent" {
		mode = "chat"
	}
	// 自动命名：默认标题（空/新对话）时用首条消息命名
	h.maybeAutoTitle(cid, req.Content)

	// 构造对话上下文：历史 + 本轮 + RAG 命中
	msgs := h.store.ListMessages(cid)
	chatMsgs := make([]llm.ChatMessage, 0, len(msgs)+2)
	for _, m := range msgs {
		chatMsgs = append(chatMsgs, llm.ChatMessage{Role: m.Role, Content: m.Content})
	}
	chatMsgs = append(chatMsgs, llm.ChatMessage{Role: "user", Content: req.Content})

	// 始终做一次语义检索：把「关于我」知识库命中片段注入上下文，让分身基于个人资料回答
	hits := h.rag.Query(r.Context(), req.Content, 4)
	// 显式 @ 引用的资料库文件：读正文注入上下文（附件/引用真实生效）
	var refBlobs []string
	for _, rid := range req.Refs {
		fid := rid
		if strings.HasPrefix(fid, "conv:") {
			continue // 会话引用仅做标记，不注入正文
		}
		if f, ok := h.store.GetFile(fid); ok {
			if c, ok := h.store.ReadContent(f); ok && c != "" {
				if len(c) > 6000 {
					c = c[:6000] + "…（已截断）"
				}
				refBlobs = append(refBlobs, fmt.Sprintf("《%s》：\n%s", f.Name, c))
			}
		}
	}
	var knowledge strings.Builder
	for _, h := range hits {
		knowledge.WriteString(fmt.Sprintf("- 《%s》：%s\n", h.FileName, h.Chunk))
	}
	var refsText strings.Builder
	for _, b := range refBlobs {
		refsText.WriteString("- " + b + "\n")
	}

	// Agent 工作流：面试应答计划（可观测步骤），chat 模式同样记录
	plan := agent.PlanInterview(req.Content)
	model := h.currentModel()
	sys := agent.SoulSystemPrompt(knowledge.String(), refsText.String(),
		fmt.Sprintf("当前模式：%s。面试问题/用户输入：%s。应答计划：考察点[%s]，选用项目[%s]，使用 SKILL[%s]。",
			mode, req.Content, plan.Focus, strings.Join(plan.Projects, "、"), plan.Skill))
	chatMsgs = append([]llm.ChatMessage{{Role: "system", Content: sys}}, chatMsgs...)

	// 落库用户消息（对话与 Agent 共用历史）
	h.store.AddMessage(models.Message{
		ID:             newID(),
		ConversationID: cid,
		Role:           "user",
		Content:        req.Content,
		CreatedAt:      time.Now(),
		Refs:           req.Refs,
	})

	// SSE 响应头
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	emit := func(s string) {
		fmt.Fprintf(w, "data: %s\n\n", jsonMarshal(map[string]any{"delta": s}))
		if flusher != nil {
			flusher.Flush()
		}
	}
	// 先下发工作流步骤（可观测），再流式正文
	fmt.Fprintf(w, "data: %s\n\n", jsonMarshal(map[string]any{
		"steps": []map[string]any{
			{"name": "识别考察点", "detail": "考察点：" + plan.Focus + "；SKILL：" + plan.Skill},
			{"name": "定位简历项目", "detail": "选用真实项目：" + strings.Join(plan.Projects, "、")},
		},
		"model": model,
	}))
	if flusher != nil {
		flusher.Flush()
	}

	start := time.Now()
	var reply strings.Builder
	wrappedEmit := func(s string) {
		reply.WriteString(s)
		emit(s)
	}
	usage, err := h.llm.Stream(r.Context(), llm.StreamRequest{
		Model:    model,
		Messages: chatMsgs,
		Hits:     hits,
	}, wrappedEmit)

	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", jsonMarshal(map[string]any{"error": err.Error()}))
		return
	}

	tokens := usage.PromptTokens + usage.CompletionTokens
	if tokens == 0 {
		tokens = int64(len(req.Content)/4) + 10 // 兜底估算
	}
	k, _ = h.store.ConsumeTokens(k.Key, tokens)

	// 落库助手消息（保存全量文本，对话与 Agent 共用历史可回看）
	h.store.AddMessage(models.Message{
		ID:             newID(),
		ConversationID: cid,
		Role:           "assistant",
		Content:        reply.String(),
		CreatedAt:      time.Now(),
		Model:          model,
		Details: &models.ResponseDetails{
			Model:        model,
			Status:       "Completed",
			ElapsedMs:    time.Since(start).Milliseconds(),
			InputTokens:  int(usage.PromptTokens),
			OutputTokens: int(usage.CompletionTokens),
			AgentType:    mode,
		},
	})

	fmt.Fprintf(w, "data: %s\n\n", jsonMarshal(map[string]any{"usage": uint64(tokens), "remaining": uint64(remaining(k, tokens))}))
	fmt.Fprintf(w, "data: %s\n\n", jsonMarshal(map[string]any{"done": true}))
}

func jsonMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func remaining(k models.ApiKey, justConsumed int64) int64 {
	if k.Quota <= 0 {
		return -1
	}
	r := k.Quota - k.Used - justConsumed
	if r < 0 {
		return 0
	}
	return r
}

// cfgModel 返回当前模型名（后续可扩展为真正的模型选择逻辑）
func cfgModel(c *config.Config) string {
	if c.ModelProvider == "mock" {
		return "JasperLee-Chat"
	}
	if c.ModelName != "" {
		return c.ModelName
	}
	return c.ModelProvider
}

func newID() string {
	return "c-" + time.Now().Format("150405.000000000")
}

func newWSID() string {
	return "ws-" + time.Now().Format("150405.000000000")
}

// maybeAutoTitle 会话若仍为默认标题（空 / “新对话”），用当前消息自动命名。
// 成熟 Agent 体验：对话不再是一排“新对话”，而是自动整理成有意义的标题。
func (h *Handler) maybeAutoTitle(cid, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	for _, c := range h.store.ListConversations() {
		if c.ID != cid {
			continue
		}
		if c.Title == "" || c.Title == "新对话" {
			runes := []rune(content)
			if len(runes) > 18 {
				runes = runes[:18]
			}
			h.store.RenameConversation(cid, string(runes))
		}
		return
	}
}

// inferKind 按扩展名推断文件 kind
func inferKind(ext string) string {
	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".csv":
		return "csv"
	case ".txt", ".log", ".env":
		return "text"
	case ".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".rs", ".c", ".cpp", ".h", ".sql", ".sh":
		return "code"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".ico":
		return "image"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "docx"
	}
	return "other"
}

// contentTypeOf 按 kind 返回预览用 Content-Type
func contentTypeOf(f models.FileMeta) string {
	switch f.Kind {
	case "json":
		return "application/json; charset=utf-8"
	case "markdown", "csv", "code", "text":
		return "text/plain; charset=utf-8"
	case "image":
		return imageMIME(f.Name)
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return "application/octet-stream"
}

// imageMIME 按图片文件扩展名返回正确 MIME 类型
func imageMIME(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	}
	return "image/png"
}

// collectRefs 深度遍历工作区文件树，收集被引用的文件内容
func collectRefs(files []models.WorkspaceFile, lookup func(fid string) models.RefContent) []models.RefContent {
	var out []models.RefContent
	var walk func(list []models.WorkspaceFile)
	walk = func(list []models.WorkspaceFile) {
		for _, f := range list {
			if f.Referenced {
				out = append(out, lookup(f.ID))
			}
			if len(f.Children) > 0 {
				walk(f.Children)
			}
		}
	}
	walk(files)
	return out
}
