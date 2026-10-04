// JasperLee 数字分身后端入口
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"jasperlee/backend/internal/api"
	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/core"
	"jasperlee/backend/internal/mcp"
)

func main() {
	mcpMode := flag.Bool("mcp", false, "run as MCP server over stdio")
	flag.Parse()

	cfg := config.Load()

	// MCP stdio 模式：与 HTTP 模式共享同一套 core，仅传输层不同（便于被宿主进程托管）
	if *mcpMode {
		srv := mcp.New(core.New(cfg))
		if err := srv.ServeStdio(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("mcp server error: %v", err)
		}
		return
	}

	mux := http.NewServeMux()
	h := api.New(cfg)

	// 健康检查
	mux.HandleFunc("GET /api/health", h.Health)

	// MCP（Model Context Protocol）HTTP 入口（Streamable HTTP，JSON-RPC 2.0）
	mux.HandleFunc("POST /mcp", h.MCP)

	// Key 机制（免登录，访客申请临时 Key 获得额度与资料库权限）
	mux.HandleFunc("POST /api/keys/apply", h.ApplyKey)
	mux.HandleFunc("GET /api/keys/me", h.VerifyKey)

	// 会话
	mux.HandleFunc("GET /api/conversations", h.ListConversations)
	mux.HandleFunc("POST /api/conversations", h.CreateConversation)
	mux.HandleFunc("PATCH /api/conversations/{id}", h.UpdateConversation)
	mux.HandleFunc("DELETE /api/conversations/{id}", h.DeleteConversation)
	mux.HandleFunc("GET /api/conversations/{id}/messages", h.ListMessages)
	mux.HandleFunc("POST /api/conversations/{id}/messages/stream", h.StreamChat)

	// 资料库
	mux.HandleFunc("GET /api/files", h.ListFiles)
	mux.HandleFunc("POST /api/files/upload", h.UploadFile)
	mux.HandleFunc("GET /api/files/{id}/content", h.GetFileContent)
	mux.HandleFunc("GET /api/files/{id}/download", h.DownloadFile)
	mux.HandleFunc("PATCH /api/files/{id}", h.RenameFile)
	mux.HandleFunc("DELETE /api/files/{id}", h.DeleteFile)
	mux.HandleFunc("POST /api/files/{id}/ingest", h.IngestFile)

	// 收件箱（招聘者上传给我）
	mux.HandleFunc("POST /api/inbox/upload", h.UploadInbox)
	mux.HandleFunc("GET /api/inbox", h.ListInbox)
	mux.HandleFunc("GET /api/inbox/{id}/download", h.DownloadInbox)
	mux.HandleFunc("DELETE /api/inbox/{id}", h.DeleteInbox)

	// RAG
	mux.HandleFunc("POST /api/rag/query", h.RAGQuery)

	// 模型：key 内全部可用模型（切换默认）
	mux.HandleFunc("GET /api/models", h.ListModels)
	mux.HandleFunc("POST /api/models/switch", h.SwitchModel)

	// Agent 产物：Jasper 的空间对外暴露的部分
	mux.HandleFunc("GET /api/artifacts", h.ListArtifacts)

	// Agent 工作区
	mux.HandleFunc("GET /api/workspaces/{id}", h.GetWorkspace)
	mux.HandleFunc("POST /api/workspaces", h.CreateWorkspace)
	mux.HandleFunc("POST /api/workspaces/{id}/run", h.RunWorkspace)

	// 数字分身简介
	mux.HandleFunc("GET /api/avatar", h.GetAvatar)

	// 管理员入口（密码 feng，可通过 .env 的 ADMIN_PASSWORD 修改）
	mux.HandleFunc("POST /api/admin/login", h.AdminLogin)
	mux.HandleFunc("GET /api/admin/stats", h.AdminStats)
	mux.HandleFunc("GET /api/admin/keys", h.AdminListKeys)
	mux.HandleFunc("PATCH /api/admin/keys/{key}", h.AdminUpdateKey)
	mux.HandleFunc("POST /api/admin/keys/{key}/renew", h.AdminRenewKey)
	mux.HandleFunc("DELETE /api/admin/keys/{key}", h.AdminDeleteKey)

	// 前端静态资源（A. 局域网部署：一个端口托管整个应用）
	// 相对可执行文件所在目录解析，保证从任意 cwd 启动都能找到 dist
	exeDir, _ := os.Executable()
	staticDir := filepath.Join(filepath.Dir(exeDir), "..", "frontend", "dist")
	mux.Handle("/", serveStatic(staticDir))

	addr := ":" + cfg.Port
	log.Printf("JasperLee backend listening on %s", addr)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// serveStatic 托管前端构建产物（frontend/dist），非 /api 的请求回退到 index.html（SPA 路由）
func serveStatic(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
