package agent

import (
	"context"
	"fmt"
	"strings"

	"jasperlee/backend/internal/llm"
	"jasperlee/backend/internal/models"
)

// WorkspaceSvc Agent 工作区服务：维护任务状态与「引用文件集合」。
// 对应截图三：Agent 执行时把工作产物/引用文件挂到右侧「文件」面板。
type WorkspaceSvc struct {
	llm *llm.Client
}

func NewWorkspaceSvc(llmClient *llm.Client) *WorkspaceSvc {
	return &WorkspaceSvc{llm: llmClient}
}

// CreateWorkspace 新建一个 Agent 任务工作区
func (s *WorkspaceSvc) CreateWorkspace(id, title, model string) models.Workspace {
	return models.Workspace{
		ID:     id,
		Title:  title,
		Status: "idle",
		Model:  model,
		Files:  buildTree([]models.WorkspaceFile{}),
	}
}

// WithRefs 把引用文件挂到工作区文件树（新建任务时预置引用）
func (s *WorkspaceSvc) WithRefs(ws models.Workspace, refs []models.WorkspaceFile) models.Workspace {
	ws.Files = buildTree(refs)
	return ws
}

// Run 执行 Agent 任务（兼容旧调用，内部走 RunWithSoul）。
func (s *WorkspaceSvc) Run(ctx context.Context, ws models.Workspace, prompt string, refs []models.RefContent, hits []models.RagHit) models.Workspace {
	return s.RunWithSoul(ctx, ws, prompt, refs, hits, ws.Model)
}

// RunWithSoul 执行 Agent 任务：灵魂提示词 → 真实模型规划 → 产出结果文件并扩展到文件树。
// 返回更新后的工作区。
//
// 任务编排流程（参考 Cherry 的 Agent 会话：chat + 工作产物）：
//
//  1. 引用文件内容 + RAG 命中组装知识上下文
//  2. SoulSystemPrompt 组装灵魂 + SKILL + 任务提示词
//  3. 调用 llm 生成执行计划与输出
//  4. 生成若干输出文件（plan.md, result.md 等）挂到引用集合
//  5. 更新状态与耗时
func (s *WorkspaceSvc) RunWithSoul(ctx context.Context, ws models.Workspace, prompt string, refs []models.RefContent, hits []models.RagHit, model string) models.Workspace {
	ws.Status = "running"

	// 1. 知识上下文：引用文件内容 + RAG 命中
	var knowledge strings.Builder
	for _, r := range refs {
		knowledge.WriteString(fmt.Sprintf("# 引用文件 %s:\n%s\n", r.Name, r.Content))
	}
	for _, h := range hits {
		knowledge.WriteString(fmt.Sprintf("# 资料库片段(%s): %s\n", h.FileName, h.Chunk))
	}

	// 2. 灵魂提示词：身份 + 准则 + SKILL + 任务
	sys := SoulSystemPrompt(knowledge.String(), "", "Agent 任务："+prompt)

	// 3. 调用真实模型
	var reply strings.Builder
	usage, err := s.llm.Stream(ctx, llm.StreamRequest{
		Model:    model,
		Messages: []llm.ChatMessage{{Role: "system", Content: sys}, {Role: "user", Content: prompt}},
		Hits:     hits,
	}, func(delta string) { reply.WriteString(delta) })

	if err != nil {
		ws.Status = "error"
		return ws
	}

	// 4. 产出文件挂到文件树（引用集 + 输出）
	ws.Files = buildOutputTree(ws.Files, refs, reply.String())

	// 5. 状态与用量
	ws.Status = "done"
	ws.Usage = &models.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
	}
	return ws
}

// buildOutputTree 基于工作区文件树追加输出文件，产出结果写入 result 文件
func buildOutputTree(existing []models.WorkspaceFile, refs []models.RefContent, reply string) []models.WorkspaceFile {
	// 保留已有引用节点，仅替换工作目录下的输出节点
	var kids []models.WorkspaceFile
	for _, r := range refs {
		kids = append(kids, models.WorkspaceFile{
			ID:         r.ID,
			Name:       r.Name,
			Kind:       "text",
			Path:       "refs/" + r.Name,
			Referenced: true,
		})
	}
	kids = append(kids,
		models.WorkspaceFile{ID: "out-plan", Name: "plan.md", Kind: "markdown", Path: "output/plan.md", Referenced: true},
		models.WorkspaceFile{ID: "out-result", Name: "result.md", Kind: "markdown", Path: "output/result.md", Referenced: true, Content: reply},
	)

	root := models.WorkspaceFile{ID: "root", Name: "workspace", Kind: "folder", Path: "workspace"}
	root.Children = []models.WorkspaceFile{{ID: "ws", Name: "agents/user-task", Kind: "folder", Path: "agents/user-task", Children: kids}}
	return []models.WorkspaceFile{root}
}

// buildTree 把一组扁平文件按 / 路径组装成树，根节点为工作区目录
func buildTree(files []models.WorkspaceFile) []models.WorkspaceFile {
	root := models.WorkspaceFile{
		ID:   "root",
		Name: "workspace",
		Kind: "folder",
		Path: "workspace",
	}
	children := []models.WorkspaceFile{{ID: "ws", Name: "agents/user-task", Kind: "folder", Path: "agents/user-task", Children: files}}
	root.Children = children
	return []models.WorkspaceFile{root}
}

// GreetingWorkspace 返回示例 greeting 工作区（Git 集成），用于演示右栏文件树
func (s *WorkspaceSvc) GreetingWorkspace() models.Workspace {
	ws := models.Workspace{
		ID:     "ws-greeting",
		Title:  "greeting",
		Status: "done",
		Model:  "JasperLee-Chat",
	}
	ws.Files = buildTree([]models.WorkspaceFile{
		{ID: "d1", Name: "smoke_gallery", Kind: "folder", Path: "agents/greeting-git-integration/smoke_gallery"},
		{ID: "d2", Name: "packages", Kind: "folder", Path: "agents/greeting-git-integration/packages"},
		{ID: "d3", Name: "docs", Kind: "folder", Path: "agents/greeting-git-integration/docs"},
		{ID: "d4", Name: ".env", Kind: "text", Path: "agents/greeting-git-integration/.env"},
		{ID: "d5", Name: "Dockerfile", Kind: "text", Path: "agents/greeting-git-integration/Dockerfile", Referenced: true},
		{ID: "d6", Name: "go.mod", Kind: "code", Path: "agents/greeting-git-integration/go.mod"},
		{ID: "d7", Name: "main.go", Kind: "code", Path: "agents/greeting-git-integration/main.go", Referenced: true},
		{ID: "d8", Name: "Makefile", Kind: "text", Path: "agents/greeting-git-integration/Makefile"},
		{ID: "d9", Name: "README.md", Kind: "markdown", Path: "agents/greeting-git-integration/README.md"},
	})
	return ws
}
