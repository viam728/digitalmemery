package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jasperlee/backend/internal/agent"
	"jasperlee/backend/internal/core"
	"jasperlee/backend/internal/llm"
	"jasperlee/backend/internal/models"
)

// Tool MCP 工具定义（JSON Schema 入参）。
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func schema(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// toolDefs 全部工具：覆盖「问答 / 检索 / 资料 / 人设 / 投递 / Agent 任务」，
// 以及 JasperKB 知识库联动（kb_*，数字分身改博客）。
func toolDefs() []Tool {
	tools := []Tool{
		{
			Name:        "ask_jasper",
			Description: "向李俊锋的数字分身提问：基于个人知识库（RAG）+ 人设作答，返回完整回答。",
			InputSchema: schema(map[string]any{
				"question": map[string]any{"type": "string", "description": "要问的问题"},
				"topK":     map[string]any{"type": "integer", "description": "检索片段数（默认 4）"},
			}, "question"),
		},
		{
			Name:        "search_knowledge",
			Description: "在个人知识库中做语义检索，返回命中片段与来源文件。",
			InputSchema: schema(map[string]any{
				"query": map[string]any{"type": "string", "description": "检索关键词/问题"},
				"topK":  map[string]any{"type": "integer", "description": "返回条数（默认 4）"},
			}, "query"),
		},
		{
			Name:        "list_materials",
			Description: "列出资料库文件（id / 名称 / 类型 / 大小 / 是否已入库）。",
			InputSchema: schema(map[string]any{}),
		},
		{
			Name:        "read_material",
			Description: "读取资料库文件正文（文本/markdown/code/docx 抽取；按 id 或名称定位）。",
			InputSchema: schema(map[string]any{
				"id":   map[string]any{"type": "string", "description": "文件 id（与 name 二选一）"},
				"name": map[string]any{"type": "string", "description": "文件名称（与 id 二选一）"},
			}),
		},
		{
			Name:        "get_profile",
			Description: "获取数字分身主页数据（姓名/人设/技能/项目/时间线/简历/联系方式）。",
			InputSchema: schema(map[string]any{}),
		},
		{
			Name:        "submit_inbox",
			Description: "向收件箱投递材料（外部插件把 JD/问题/资料送进来），返回条目 id。",
			InputSchema: schema(map[string]any{
				"name":     map[string]any{"type": "string", "description": "署名"},
				"note":     map[string]any{"type": "string", "description": "留言"},
				"fileName": map[string]any{"type": "string", "description": "文件名（默认 inbox.md）"},
				"content":  map[string]any{"type": "string", "description": "文件正文"},
			}, "content"),
		},
		{
			Name:        "run_task",
			Description: "让分身以 Agent 方式执行任务（生成 plan/result），可引用资料库文件，返回产出文本。",
			InputSchema: schema(map[string]any{
				"prompt": map[string]any{"type": "string", "description": "任务描述"},
				"refs":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "引用的文件 id 列表"},
			}, "prompt"),
		},
	}
	return append(tools, kbToolDefs()...)
}

// kbToolDefs JasperKB 工具定义（数字分身改博客：经 MCP 代理到知识库服务端，与知识库工具同名同参）。
func kbToolDefs() []Tool {
	return []Tool{
		{
			Name:        "kb_list_spaces",
			Description: "列出 JasperKB 知识空间（博客内容源）。",
			InputSchema: schema(map[string]any{}),
		},
		{
			Name:        "kb_get_tree",
			Description: "查看 JasperKB 某空间下的文档树（文件夹 / 文档，含 id、状态与 slug）。",
			InputSchema: schema(map[string]any{
				"space": map[string]any{"type": "string", "description": "空间 id 或 key"},
			}, "space"),
		},
		{
			Name:        "kb_search",
			Description: "在 JasperKB 知识库中做关键词检索（标题 + 正文），返回命中文档与片段。",
			InputSchema: schema(map[string]any{
				"query": map[string]any{"type": "string", "description": "关键词"},
				"space": map[string]any{"type": "string", "description": "限定空间（可选，id 或 key）"},
			}, "query"),
		},
		{
			Name:        "kb_get_doc",
			Description: "读取 JasperKB 文档全文（含状态 / slug / 标签 / 修订数）。",
			InputSchema: schema(map[string]any{
				"id": map[string]any{"type": "string", "description": "文档节点 id"},
			}, "id"),
		},
		{
			Name:        "kb_create_doc",
			Description: "在 JasperKB 新建文档（草稿）。",
			InputSchema: schema(map[string]any{
				"space":    map[string]any{"type": "string", "description": "空间 id 或 key"},
				"parentId": map[string]any{"type": "string", "description": "父文件夹节点 id（可选）"},
				"title":    map[string]any{"type": "string", "description": "文档标题"},
				"content":  map[string]any{"type": "string", "description": "Markdown 正文（可选）"},
				"tags":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "标签（可选）"},
			}, "space", "title"),
		},
		{
			Name:        "kb_update_doc",
			Description: "修改 JasperKB 文档（标题 / 正文 / 标签）——「数字分身改博客」的正文修改走这里。",
			InputSchema: schema(map[string]any{
				"id":      map[string]any{"type": "string", "description": "文档节点 id"},
				"title":   map[string]any{"type": "string", "description": "新标题（可选）"},
				"content": map[string]any{"type": "string", "description": "新正文（可选）"},
				"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "新标签（可选）"},
				"note":    map[string]any{"type": "string", "description": "修改说明（可选）"},
			}, "id"),
		},
		{
			Name:        "kb_publish_doc",
			Description: "发布 JasperKB 文档到公开接口（博客可见）——「数字分身改博客」的发布走这里。",
			InputSchema: schema(map[string]any{
				"id":   map[string]any{"type": "string", "description": "文档节点 id"},
				"slug": map[string]any{"type": "string", "description": "对外 URL 标识（可选）"},
			}, "id"),
		},
		{
			Name:        "kb_unpublish_doc",
			Description: "撤稿（从公开接口下线，保留 slug 占用）。",
			InputSchema: schema(map[string]any{
				"id": map[string]any{"type": "string", "description": "文档节点 id"},
			}, "id"),
		},
	}
}

// kbProxy 通用代理：把工具调用转发给 JasperKB 的 MCP（HTTP），返回其文本结果。
func kbProxy(name string) toolHandler {
	return func(s *Server, raw json.RawMessage) (string, error) {
		args := map[string]any{}
		if strings.TrimSpace(string(raw)) != "" {
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return s.c.KB.CallTool(ctx, name, args)
	}
}

type toolHandler func(*Server, json.RawMessage) (string, error)

var toolHandlers = map[string]toolHandler{
	"ask_jasper":       (*Server).toolAskJasper,
	"search_knowledge": (*Server).toolSearch,
	"list_materials":   (*Server).toolListMaterials,
	"read_material":    (*Server).toolReadMaterial,
	"get_profile":      (*Server).toolGetProfile,
	"submit_inbox":     (*Server).toolSubmitInbox,
	"run_task":         (*Server).toolRunTask,
	// JasperKB 知识库（数字分身改博客）：经 MCP 代理到知识库服务
	"kb_list_spaces":   kbProxy("kb_list_spaces"),
	"kb_get_tree":      kbProxy("kb_get_tree"),
	"kb_search":        kbProxy("kb_search"),
	"kb_get_doc":       kbProxy("kb_get_doc"),
	"kb_create_doc":    kbProxy("kb_create_doc"),
	"kb_update_doc":    kbProxy("kb_update_doc"),
	"kb_publish_doc":   kbProxy("kb_publish_doc"),
	"kb_unpublish_doc": kbProxy("kb_unpublish_doc"),
}

// callTool 执行 tools/call；执行错误按 MCP 约定放进 result.isError。
func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{codeInvalidParams, "invalid tool call params"}
	}
	h, ok := toolHandlers[p.Name]
	if !ok {
		return nil, &rpcError{codeMethodNotFound, "unknown tool: " + p.Name}
	}
	text, err := h(s, p.Arguments)
	if err != nil {
		return toolResult(err.Error(), true), nil
	}
	return toolResult(text, false), nil
}

func toolResult(text string, isErr bool) map[string]any {
	if text == "" {
		text = "(空)"
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

// ---- 工具实现 ----

// toolAskJasper 知识库问答（RAG + 灵魂人设），聚合流式输出为完整文本。
func (s *Server) toolAskJasper(raw json.RawMessage) (string, error) {
	var a struct {
		Question string `json:"question"`
		TopK     int    `json:"topK"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("参数解析失败：%v", err)
	}
	q := strings.TrimSpace(a.Question)
	if q == "" {
		return "", fmt.Errorf("question 不能为空")
	}
	if a.TopK <= 0 {
		a.TopK = 4
	}
	hits := s.c.RAG.Query(context.Background(), q, a.TopK)
	var kb strings.Builder
	for _, hit := range hits {
		kb.WriteString("- 《" + hit.FileName + "》：" + hit.Chunk + "\n")
	}
	sys := agent.SoulSystemPrompt(kb.String(), "", "外部调用方提问："+q)
	var sb strings.Builder
	if _, err := s.c.LLM.Stream(context.Background(), llm.StreamRequest{
		Model:    s.c.CurrentModel(),
		Messages: []llm.ChatMessage{{Role: "system", Content: sys}, {Role: "user", Content: q}},
		Hits:     hits,
	}, func(d string) { sb.WriteString(d) }); err != nil {
		return "", err
	}
	out := strings.TrimSpace(sb.String())
	if len(hits) > 0 {
		names := make([]string, 0, len(hits))
		for _, hit := range hits {
			names = append(names, hit.FileName)
		}
		out += "\n\n（参考：" + strings.Join(names, "、") + "）"
	}
	return out, nil
}

// toolSearch 语义检索。
func (s *Server) toolSearch(raw json.RawMessage) (string, error) {
	var a struct {
		Query string `json:"query"`
		TopK  int    `json:"topK"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	q := strings.TrimSpace(a.Query)
	if q == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	if a.TopK <= 0 {
		a.TopK = 4
	}
	hits := s.c.RAG.Query(context.Background(), q, a.TopK)
	if len(hits) == 0 {
		return "未检索到相关片段。", nil
	}
	var sb strings.Builder
	for i, hit := range hits {
		sb.WriteString(fmt.Sprintf("%d. 《%s》 score=%.3f\n%s\n\n", i+1, hit.FileName, hit.Score, strings.TrimSpace(hit.Chunk)))
	}
	return strings.TrimSpace(sb.String()), nil
}

// toolListMaterials 资料库文件列表。
func (s *Server) toolListMaterials(_ json.RawMessage) (string, error) {
	files := s.c.Store.ListFiles()
	if len(files) == 0 {
		return "资料库为空。", nil
	}
	var sb strings.Builder
	for _, f := range files {
		ing := "未入库"
		if f.Ingested {
			ing = "已入库"
		}
		sb.WriteString(fmt.Sprintf("- %s | %s | %s | %d bytes | %s\n", f.ID, f.Name, f.Kind, f.Size, ing))
	}
	return strings.TrimSpace(sb.String()), nil
}

// toolReadMaterial 读取文件正文（文本 / docx 抽取）。
func (s *Server) toolReadMaterial(raw json.RawMessage) (string, error) {
	var a struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	var target models.FileMeta
	found := false
	switch {
	case a.ID != "":
		if f, ok := s.c.Store.GetFile(a.ID); ok {
			target, found = f, true
		}
	case a.Name != "":
		for _, f := range s.c.Store.ListFiles() {
			if f.Name == a.Name {
				target, found = f, true
				break
			}
		}
	}
	if !found {
		return "", fmt.Errorf("未找到文件：请提供正确的 id 或 name")
	}
	content, ok := s.c.Store.ReadContent(target)
	if !ok {
		return "", fmt.Errorf("文件《%s》无可读文本（二进制或为空）", target.Name)
	}
	if runes := []rune(content); len(runes) > 8000 {
		content = string(runes[:8000]) + "…（已截断）"
	}
	return "《" + target.Name + "》\n\n" + content, nil
}

// toolGetProfile 个人主页数据（JSON 文本）。
func (s *Server) toolGetProfile(_ json.RawMessage) (string, error) {
	b, err := json.MarshalIndent(core.AvatarData(), "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// toolSubmitInbox 投递材料到收件箱（外部插件入口）。
func (s *Server) toolSubmitInbox(raw json.RawMessage) (string, error) {
	var a struct {
		Name     string `json:"name"`
		Note     string `json:"note"`
		FileName string `json:"fileName"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	if strings.TrimSpace(a.Content) == "" {
		return "", fmt.Errorf("content 不能为空")
	}
	if strings.TrimSpace(a.FileName) == "" {
		a.FileName = "inbox.md"
	}
	id := "mcp-" + time.Now().Format("20060102-150405.000")
	dir := filepath.Join(s.c.Cfg.DataDir, "inbox")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, id+filepath.Ext(a.FileName)), []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	it := models.InboxItem{
		ID:        id,
		FileName:  a.FileName,
		Name:      a.Name,
		Note:      a.Note,
		Size:      int64(len(a.Content)),
		CreatedAt: time.Now(),
		Status:    "pending",
		OwnerKey:  "mcp",
	}
	s.c.Store.AddInbox(it)
	return fmt.Sprintf("已投递到收件箱：id=%s 文件名=%s（%d bytes）", it.ID, it.FileName, it.Size), nil
}

// toolRunTask 运行 Agent 任务并汇总产出（引用文件 + RAG + 灵魂提示）。
func (s *Server) toolRunTask(raw json.RawMessage) (string, error) {
	var a struct {
		Prompt string   `json:"prompt"`
		Refs   []string `json:"refs"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	prompt := strings.TrimSpace(a.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt 不能为空")
	}
	wsID := "ws-mcp-" + time.Now().Format("20060102-150405.000")
	ws := s.c.Agent.CreateWorkspace(wsID, "MCP 任务", s.c.CurrentModel())

	var refFiles []models.WorkspaceFile
	var refContents []models.RefContent
	for _, rid := range a.Refs {
		f, ok := s.c.Store.GetFile(rid)
		if !ok {
			continue
		}
		refFiles = append(refFiles, models.WorkspaceFile{ID: f.ID, Name: f.Name, Kind: f.Kind, Path: f.Path, Referenced: true})
		if c, ok := s.c.Store.ReadContent(f); ok {
			refContents = append(refContents, models.RefContent{ID: f.ID, Name: f.Name, Content: c})
		}
	}
	if len(refFiles) > 0 {
		ws = s.c.Agent.WithRefs(ws, refFiles)
	}
	hits := s.c.RAG.Query(context.Background(), prompt, 4)
	ws = s.c.Agent.RunWithSoul(context.Background(), ws, prompt, refContents, hits, s.c.CurrentModel())

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务状态：%s（workspace=%s，模型=%s）\n\n", ws.Status, ws.ID, ws.Model))
	var walk func([]models.WorkspaceFile)
	walk = func(nodes []models.WorkspaceFile) {
		for _, n := range nodes {
			if len(n.Children) > 0 {
				walk(n.Children)
			}
			if strings.TrimSpace(n.Content) != "" {
				sb.WriteString("## " + n.Name + "\n" + n.Content + "\n\n")
			}
		}
	}
	walk(ws.Files)
	return strings.TrimSpace(sb.String()), nil
}

// ---- 资源：资料库文件 ----

const resourcePrefix = "jasperlee://materials/"

// listResources 把资料库文件暴露为 MCP 资源。
func (s *Server) listResources() map[string]any {
	files := s.c.Store.ListFiles()
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		out = append(out, map[string]any{
			"uri":         resourcePrefix + f.ID,
			"name":        f.Name,
			"description": fmt.Sprintf("%s · %s", f.Kind, humanSize(f.Size)),
			"mimeType":    mimeOf(f.Kind),
		})
	}
	return map[string]any{"resources": out}
}

// readResource 读取资源正文（文本 / docx 抽取）。
func (s *Server) readResource(params json.RawMessage) (any, *rpcError) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &p); err != nil || !strings.HasPrefix(p.URI, resourcePrefix) {
		return nil, &rpcError{codeInvalidParams, "invalid or unsupported uri"}
	}
	id := strings.TrimPrefix(p.URI, resourcePrefix)
	f, ok := s.c.Store.GetFile(id)
	if !ok {
		return nil, &rpcError{codeInvalidParams, "file not found: " + id}
	}
	content, ok := s.c.Store.ReadContent(f)
	if !ok {
		return nil, &rpcError{codeInternalError, "file has no readable text content"}
	}
	return map[string]any{
		"contents": []map[string]any{{
			"uri":      p.URI,
			"mimeType": mimeOf(f.Kind),
			"text":     content,
		}},
	}, nil
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func mimeOf(kind string) string {
	switch kind {
	case "markdown":
		return "text/markdown"
	case "json":
		return "application/json"
	case "csv", "code", "text":
		return "text/plain"
	case "image":
		return "image/png"
	case "pdf":
		return "application/pdf"
	default:
		return "text/plain"
	}
}
