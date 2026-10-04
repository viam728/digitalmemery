// Package core 组装数字分身的领域服务（存储 / RAG / 模型 / Agent / 人设数据）。
//
// 架构意图（服务端「小体量 + 高成熟度」）：
//   - core 与传输层无关：只做服务装配与启动期副作用；
//   - 上层传输各自极薄：HTTP API（internal/api）与 MCP 服务（internal/mcp）共享同一套 core；
//   - 新增一种对外形态（MCP / CLI / gRPC）时，只需再写一个薄传输层。
package core

import (
	"context"
	"log"

	"jasperlee/backend/internal/agent"
	"jasperlee/backend/internal/config"
	"jasperlee/backend/internal/kb"
	"jasperlee/backend/internal/llm"
	"jasperlee/backend/internal/rag"
	"jasperlee/backend/internal/storage"
)

// Core 数字分身的共享领域服务集合。
type Core struct {
	Cfg   *config.Config
	Store *storage.Store
	RAG   *rag.Service
	LLM   *llm.Client
	Agent *agent.WorkspaceSvc
	// KB JasperKB 知识库客户端（数字分身改博客）
	KB *kb.Client
}

// New 装配全部服务并触发启动期副作用（后台 goroutine）。
func New(cfg *config.Config) *Core { return NewWithOptions(cfg, Options{Background: true}) }

// Options 装配选项。
type Options struct {
	// Background 是否以后台 goroutine 执行启动期任务（知识库引导 / 模型列表刷新）。
	// 生产为 true；测试可置 false，改为同步执行且不产生游离 goroutine（保证确定性）。
	Background bool
}

// NewWithOptions 装配全部服务并触发启动期副作用。
func NewWithOptions(cfg *config.Config, opt Options) *Core {
	llmClient := llm.NewWithOptions(cfg.ModelProvider, cfg.ModelAPIKey, cfg.ModelBaseURL, cfg.ModelHostIP, cfg.ModelName)
	c := &Core{
		Cfg:   cfg,
		Store: storage.New(cfg),
		RAG:   rag.New(cfg.RAGProvider, cfg.DataDir, cfg.ModelAPIKey, cfg.ModelBaseURL),
		LLM:   llmClient,
		Agent: agent.NewWorkspaceSvc(llmClient),
		KB:    kb.New(cfg.KBURL, cfg.KBToken),
	}
	c.initModelDefaults()
	if opt.Background {
		// 后台异步：不阻塞服务启动（网络慢时也能立即响应）
		go c.bootstrapIndex()
		go llm.RefreshModels(cfg.ModelBaseURL, cfg.ModelAPIKey, cfg.ModelHostIP)
	} else {
		c.bootstrapIndex()
	}
	return c
}

// CurrentModel 当前生效的默认模型。
func (c *Core) CurrentModel() string {
	if m := c.LLM.Model(); m != "" {
		return m
	}
	if c.Cfg.ModelName != "" {
		return c.Cfg.ModelName
	}
	return "glm-4-flash"
}

// initModelDefaults 初始化模型默认值：MODEL_LIST 全配进去，默认 MODEL_NAME。
func (c *Core) initModelDefaults() {
	if c.Cfg.ModelList != "" {
		return // 已显式配置，不覆盖
	}
	// 默认把已知的 GLM 模型全配进去（启动后台刷新会更新缓存）
	c.Cfg.ModelList = "glm-4.5,glm-4.5-air,glm-4.6,glm-4.7,glm-5,glm-5-turbo,glm-5.1,glm-5.2,glm-5.3,glm-5.3-flash,glm-5.3-flashx"
	if c.Cfg.ModelName == "" || c.Cfg.ModelName == "deepseek-chat" {
		c.Cfg.ModelName = "glm-4-flash"
	}
	c.LLM.SetModel(c.Cfg.ModelName)
}

// bootstrapIndex 启动时自动把未入库的文本类文档向量化，
// 保证外部调用方第一次提问就能被分身基于知识库回答（无需手动点击入库）。
func (c *Core) bootstrapIndex() {
	for _, f := range c.Store.ListFiles() {
		if f.Ingested {
			continue
		}
		// PDF/图片无文本抽取、跳过；docx 由 textract 抽取正文
		switch f.Kind {
		case "pdf", "image":
			continue
		}
		content, ok := c.Store.ReadContent(f)
		if !ok || len(content) < 20 {
			continue
		}
		if err := c.RAG.Ingest(context.Background(), f.ID, f.Name, content); err != nil {
			log.Printf("[bootstrap] ingest %s failed: %v", f.Name, err)
			continue
		}
		c.Store.MarkIngested(f.ID)
	}
}
