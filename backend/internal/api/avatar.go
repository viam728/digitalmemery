package api

import (
	"net/http"

	"jasperlee/backend/internal/models"
)

// avatarData 个人数字分身的主页数据（基于李俊锋最新简历：Agent 工程师）
var avatarData = models.Avatar{
	Name:     "李俊锋",
	Role:     "Agent 工程师 · Go 全栈开发者（后端工程 + AI 工程化）",
	Age:      24,
	Location: "中国",
	MBTI:     "INTJ",
	About: "你好，我是李俊锋，2024 年毕业于浙大宁波理工学院计算机科学与技术专业（工科学士），2 年工作经验，求职意向是 Agent 工程师。" +
		"我具备后端工程底蕴与 AI 工程化实践经验：熟练落地业务需求完备的 Agent 项目，熟悉 Go 全栈开发、Agent 工程、大模型原理和训练；" +
		"熟练使用 Python、Go，熟悉 Linux、Docker、设计模式、分布式架构和微服务设计；深度实践 Agent（ClaudeCode、Codex、OpenCode、Cherry、Qoder），" +
		"熟悉 AI 编程范式和 AI 能力边界，能用 AI 快速验证想法和迭代。欢迎向我提问。",
	Skills: []string{"Agent 工程", "Go 全栈", "Python", "RAG / LangChain", "Linux / Docker", "分布式 / 微服务", "Next.js / React", "PostgreSQL / MySQL", "AI 编程范式", "跨境电商运营"},
	Projects: []models.Project{
		{
			Name:  "InkBloom 高性能 AIGC 创作者工作台",
			Desc:  "Electron + Go 的 AI 协同创作平台：自研 Agent Harness（多步推理 + 15+ 工具调用，SSE 步级事件流）；Go(Gin)+Python(FastAPI) 异构微服务；NATS 解耦长任务；九段状态机；Docker Compose 五组件部署。",
			Stack: []string{"Go", "Agent", "SSE", "NATS", "Docker"},
			Year:  "2026",
		},
		{
			Name:  "BeYoung B2B 海外独立站",
			Desc:  "Go 模块化单体 + Next.js：双车并存询价、三态价格体系、多语言 CMS；priceScope 价格中间件；Quote 版本化 + 四态状态机；OpenAPI 3 驱动；自托管 PG/Redis/Meilisearch。",
			Stack: []string{"Go", "Next.js", "PostgreSQL", "OpenAPI"},
			Year:  "2026",
		},
		{
			Name:  "LLM 检索增强问答系统",
			Desc:  "SpringBoot + Vue 企业级 RAG 问答：LangChain-Java 检索链路，PostgreSQL Vector 存储，会话状态管理（上传/多轮/标题生成/流式/记录管理）。",
			Stack: []string{"SpringBoot", "Vue", "LangChain", "pgvector"},
			Year:  "2024",
		},
		{
			Name:  "D-S 证据理论订货决策工具",
			Desc:  "多专家意见决策数学模型的轻量 Web 工具：D-S 证据理论 + 报童模型，主导工程化落地，与商学院协作获比赛优秀奖。",
			Stack: []string{"JavaScript", "D-S 证据理论", "运筹学"},
			Year:  "2021",
		},
	},
	Timeline: []models.TimelineItem{
		{Year: "2020-2024", Text: "浙大宁波理工学院 · 计算机科学与技术（工科学士，总成绩专业 12%）"},
		{Year: "2021", Text: "D-S 证据理论订货决策工具（技术负责，比赛优秀奖）"},
		{Year: "2023.10-2024.04", Text: "宁波宽易天地 · IT 技术支持（住建局官网运维独立负责）"},
		{Year: "2024.03-2024.06", Text: "LLM 检索增强问答系统（SpringBoot + Vue）+ 毕业设计《基于 LLM 的智能问答助手》"},
		{Year: "2024.12-至今", Text: "广东必晟康 · IT 技术支持（企业数字化 + 跨境电商运营，总询盘 +14%）"},
		{Year: "2026.09-至今", Text: "InkBloom AIGC 创作者工作台（Go 全栈 Agent 工程）"},
		{Year: "2026.09-至今", Text: "BeYoung B2B 海外独立站（Go 全栈 + Next.js）"},
	},
	Resume: []string{
		"求职意向：Agent 工程师",
		"学历：浙大宁波理工学院 计算机科学与技术 工科学士（2020-2024，专业前 12%）",
		"经验：2 年（企业数字化 + 跨境电商运营 + IT 运维）",
		"核心能力：Agent 工程（Harness/工具编排/降级链）、Go 全栈、RAG、高并发与微服务",
		"代表作：InkBloom（AIGC 工作台）、BeYoung（B2B 独立站）、LLM 检索增强问答系统",
	},
	Contact: []string{"📞 15767210739", "✉️ 1657203672@qq.com"},
}

// GetAvatar GET /api/avatar —— 个人数字分身主页数据
func (h *Handler) GetAvatar(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, avatarData)
}
