package api

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"jasperlee/backend/internal/models"
)

// publishArtifacts 把 Agent 工作区产出文件落到资料库（同一份存储，同一行记录）。
//
// 约定：工作区文件树中 output/ 目录下的节点视为产物。每个产物：
//  1. 正文写入 DATA_DIR/uploads/<fileID><ext>（与资料库文件同一磁盘位置）
//  2. 在 files 表中创建/更新同一 ID 的 FileMeta，IsArtifact=true、WorkspaceID=工作区 ID
//  3. 产物自动出现在「Jasper 的空间」（ListArtifacts / 前端分组）
//
// expose=false 时仅更新工作区树，不落资料库。
func (h *Handler) publishArtifacts(ws models.Workspace, workspaceID string, expose bool) []models.FileMeta {
	if !expose {
		return []models.FileMeta{}
	}
	var out []models.FileMeta
	var walk func(nodes []models.WorkspaceFile)
	walk = func(nodes []models.WorkspaceFile) {
		for _, n := range nodes {
			if len(n.Children) > 0 {
				walk(n.Children)
			}
			if n.Kind == "folder" {
				continue
			}
			// 只收 output/ 下的产物节点
			if filepath.Dir(n.Path) != "output" && !isOutputPath(n.Path) {
				continue
			}
			if n.Content == "" {
				continue
			}
			f := h.persistArtifact(n, workspaceID)
			if f != nil {
				out = append(out, *f)
			}
		}
	}
	walk(ws.Files)
	return out
}

func isOutputPath(p string) bool {
	// 兼容 output/plan.md 与 agents/user-task/output/... 两种形式
	for _, seg := range filepath.SplitList(filepath.Dir(p)) {
		if seg == "output" {
			return true
		}
	}
	parts := splitPath(p)
	for _, part := range parts {
		if part == "output" {
			return true
		}
	}
	return false
}

func splitPath(p string) []string {
	var parts []string
	for _, s := range filepath.SplitList(filepath.Dir(p)) {
		parts = append(parts, s)
	}
	// filepath.SplitList 按 PATH 分隔符切，不适合切路径；改用手动切
	parts = nil
	cur := ""
	for _, r := range p {
		if r == '/' || r == '\\' {
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

// persistArtifact 产物正文写盘 + files 表同一 ID 建记录（IsArtifact=true）
func (h *Handler) persistArtifact(n models.WorkspaceFile, workspaceID string) *models.FileMeta {
	id := "art-" + workspaceID + "-" + n.ID
	if existing, ok := h.store.GetFile(id); ok {
		// 已存在：更新正文与时间，补产物标记
		_ = h.writeArtifactDisk(existing, n.Content)
		marked, ok := h.store.MarkArtifact(id, workspaceID)
		if !ok {
			return nil
		}
		return &marked
	}
	now := time.Now()
	f := models.FileMeta{
		ID:          id,
		Name:        n.Name,
		Kind:        artifactKind(n.Kind),
		Path:        "Jasper 的空间",
		Owner:       "李俊锋",
		Size:        int64(len(n.Content)),
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessedAt:  now,
		Tags:        []string{"agent-artifact"},
		Ingested:    false,
		IsArtifact:  true,
		WorkspaceID: workspaceID,
	}
	_ = h.writeArtifactDisk(f, n.Content)
	h.store.AddFile(f)
	// AddFile 后补标记（AddFile 不保留标记语义，显式 MarkArtifact 确保列与 jsonb 一致）
	marked, ok := h.store.MarkArtifact(id, workspaceID)
	if !ok {
		return &f
	}
	return &marked
}

func (h *Handler) writeArtifactDisk(f models.FileMeta, content string) error {
	uploads := filepath.Join(h.cfg.DataDir, "uploads")
	_ = os.MkdirAll(uploads, 0o755)
	ext := filepath.Ext(f.Name)
	if ext == "" {
		ext = ".md"
	}
	return os.WriteFile(filepath.Join(uploads, f.ID+ext), []byte(content), 0o644)
}

func artifactKind(k string) string {
	switch k {
	case "markdown", "code", "json", "csv", "text":
		return k
	}
	return "markdown"
}

// ListArtifacts GET /api/artifacts —— Jasper 的空间对外暴露的 Agent 产物
func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	k, ok := h.requireKey(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "a valid key is required")
		return
	}
	if !k.Library {
		writeErr(w, http.StatusForbidden, "library access not granted on this key")
		return
	}
	writeJSON(w, http.StatusOK, h.store.ListArtifacts())
}
