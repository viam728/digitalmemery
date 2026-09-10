package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"jasperlee/backend/internal/models"
)

// randomKey 生成 16 字节随机 hex（memStore 与 pgStore 共用）。
func randomKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// memStore 内存存储 + JSON 持久化（无 PostgreSQL 时的降级实现，数据量小 <20MB，足够）
type memStore struct {
	mu            sync.RWMutex
	dataDir       string
	conversations []models.Conversation
	messages      map[string][]models.Message // conversationId -> messages
	files         []models.FileMeta
	workspaces    map[string]models.Workspace
	keys          map[string]models.ApiKey // key -> ApiKey
	inbox         []models.InboxItem      // 招聘者投递的收件箱条目
}

// newMemStore 创建内存+JSON降级存储，加载持久化数据；无数据时写入种子数据与演示 Key
func newMemStore(dataDir string) *memStore {
	s := &memStore{
		dataDir:       dataDir,
		messages:      make(map[string][]models.Message),
		workspaces:    make(map[string]models.Workspace),
		conversations: seedConversations(),
		files:         seedFiles(),
		keys:          make(map[string]models.ApiKey),
	}
	s.load()
	if len(s.keys) == 0 {
		k := s.NewKey("演示访客", 1_000_000, true) // 100 万 token
		s.keys[k.Key] = k
		s.persist()
	}
	return s
}

func (s *memStore) file() string { return filepath.Join(s.dataDir, "store.json") }

func (s *memStore) load() {
	b, err := os.ReadFile(s.file())
	if err != nil {
		return
	}
	var data struct {
		Conversations []models.Conversation    `json:"conversations"`
		Files         []models.FileMeta        `json:"files"`
		Keys          map[string]models.ApiKey `json:"keys"`
		Inbox         []models.InboxItem       `json:"inbox"`
	}
	if json.Unmarshal(b, &data) == nil {
		if len(data.Conversations) > 0 {
			s.conversations = data.Conversations
		}
		// 文件：合并持久化数据与种子文件，保证「关于我」的知识库种子始终存在
		seen := make(map[string]bool, len(data.Files))
		for _, f := range data.Files {
			seen[f.ID] = true
		}
		files := data.Files
		for _, sf := range seedFiles() {
			if !seen[sf.ID] {
				files = append(files, sf)
			}
		}
		s.files = files
		if len(data.Keys) > 0 {
			s.keys = data.Keys
		}
		if len(data.Inbox) > 0 {
			s.inbox = data.Inbox
		}
	}
}

func (s *memStore) persist() {
	_ = os.MkdirAll(s.dataDir, 0o755)
	b, _ := json.Marshal(map[string]any{
		"conversations": s.conversations,
		"files":         s.files,
		"keys":          s.keys,
		"inbox":         s.inbox,
	})
	_ = os.WriteFile(s.file(), b, 0o644)
}

// ListConversations 返回会话
func (s *memStore) ListConversations() []models.Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conversations
}

// AddConversation 新增会话
func (s *memStore) AddConversation(c models.Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations = append([]models.Conversation{c}, s.conversations...)
	s.persist()
}

// RenameConversation 会话改名
func (s *memStore) RenameConversation(id, title string) (models.Conversation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.conversations {
		if s.conversations[i].ID == id {
			s.conversations[i].Title = title
			s.conversations[i].UpdatedAt = time.Now()
			s.persist()
			return s.conversations[i], true
		}
	}
	return models.Conversation{}, false
}

// UpdateConversationPinned 置顶/取消置顶
func (s *memStore) UpdateConversationPinned(id string, pinned bool) (models.Conversation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.conversations {
		if s.conversations[i].ID == id {
			s.conversations[i].Pinned = pinned
			s.conversations[i].UpdatedAt = time.Now()
			s.persist()
			return s.conversations[i], true
		}
	}
	return models.Conversation{}, false
}

// DeleteConversation 删除会话及其全部消息
func (s *memStore) DeleteConversation(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	out := s.conversations[:0]
	for _, c := range s.conversations {
		if c.ID == id {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		return false
	}
	s.conversations = out
	delete(s.messages, id)
	s.persist()
	return true
}

// AddMessage 追加一条消息
func (s *memStore) AddMessage(m models.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[m.ConversationID] = append(s.messages[m.ConversationID], m)
	s.persist()
}

// ListMessages 取会话消息
func (s *memStore) ListMessages(id string) []models.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages[id]
}

// ListFiles 返回文件列表
func (s *memStore) ListFiles() []models.FileMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files
}

// MarkIngested 标记文件已向量化入库
func (s *memStore) MarkIngested(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.files {
		if s.files[i].ID == id {
			s.files[i].Ingested = true
			s.persist()
			return true
		}
	}
	return false
}

// MarkArtifact 标记文件为 Agent 产物（自动暴露到 Jasper 的空间）
func (s *memStore) MarkArtifact(id string, workspaceID string) (models.FileMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.files {
		if s.files[i].ID == id {
			s.files[i].IsArtifact = true
			s.files[i].WorkspaceID = workspaceID
			s.files[i].UpdatedAt = time.Now()
			s.persist()
			return s.files[i], true
		}
	}
	return models.FileMeta{}, false
}

// ListArtifacts 列出全部 Agent 产物（Jasper 的空间对外暴露的部分）
func (s *memStore) ListArtifacts() []models.FileMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.FileMeta
	for _, f := range s.files {
		if f.IsArtifact {
			out = append(out, f)
		}
	}
	return out
}

// AddFile 新增文件元数据（写入到最前）
func (s *memStore) AddFile(f models.FileMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = append([]models.FileMeta{f}, s.files...)
	s.persist()
}

// GetFile 按 ID 取文件元数据
func (s *memStore) GetFile(id string) (models.FileMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.files {
		if f.ID == id {
			return f, true
		}
	}
	return models.FileMeta{}, false
}

// ReadBytes 读取文件原始字节（磁盘优先；种子数据回退到示例内容）
func (s *memStore) ReadBytes(f models.FileMeta) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(s.dataDir, "uploads", f.ID+filepath.Ext(f.Name)))
	if err == nil {
		return b, nil
	}
	if c, ok := demoContents[f.ID]; ok {
		return []byte(c), nil
	}
	return nil, err
}

// ReadContent 读取文件文本内容用于 ingest / 上下文注入
func (s *memStore) ReadContent(f models.FileMeta) (string, bool) {
	b, err := s.ReadBytes(f)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// TouchFile 更新文件最近访问时间
func (s *memStore) TouchFile(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.files {
		if s.files[i].ID == id {
			s.files[i].AccessedAt = time.Now()
			s.persist()
			return
		}
	}
}

// SetWorkspace 保存 Agent 工作区（新建或覆盖）
func (s *memStore) SetWorkspace(w models.Workspace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaces[w.ID] = w
	s.persist()
}

// GetWorkspace 取 Agent 工作区
func (s *memStore) GetWorkspace(id string) (models.Workspace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.workspaces[id]
	return w, ok
}

// ---- 文件管理 ----

// RenameFile 重命名文件元数据
func (s *memStore) RenameFile(id, name string) (models.FileMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.files {
		if s.files[i].ID == id {
			s.files[i].Name = name
			s.files[i].UpdatedAt = time.Now()
			s.persist()
			return s.files[i], true
		}
	}
	return models.FileMeta{}, false
}

// DeleteFile 删除文件元数据（磁盘文件由调用方负责清理）
func (s *memStore) DeleteFile(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.files {
		if s.files[i].ID == id {
			s.files = append(s.files[:i], s.files[i+1:]...)
			s.persist()
			return true
		}
	}
	return false
}

// DeleteFileOnDisk 删除资料库磁盘文件
func (s *memStore) DeleteFileOnDisk(f models.FileMeta) error {
	path := filepath.Join(s.dataDir, "uploads", f.ID+filepath.Ext(f.Name))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // 种子文件无磁盘文件，视为已删除
	}
	return err
}

// ---- 收件箱 ----

// ListInbox 收件箱条目（新投递在前）
func (s *memStore) ListInbox() []models.InboxItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inbox
}

// AddInbox 新增收件箱条目（写入到最前）
func (s *memStore) AddInbox(it models.InboxItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inbox = append([]models.InboxItem{it}, s.inbox...)
	s.persist()
}

// GetInbox 按 ID 取收件箱条目
func (s *memStore) GetInbox(id string) (models.InboxItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, it := range s.inbox {
		if it.ID == id {
			return it, true
		}
	}
	return models.InboxItem{}, false
}

// DeleteInbox 删除收件箱条目元数据
func (s *memStore) DeleteInbox(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.inbox {
		if s.inbox[i].ID == id {
			s.inbox = append(s.inbox[:i], s.inbox[i+1:]...)
			s.persist()
			return true
		}
	}
	return false
}

// DeleteInboxOnDisk 删除收件箱磁盘文件
func (s *memStore) DeleteInboxOnDisk(it models.InboxItem) error {
	path := filepath.Join(s.dataDir, "inbox", it.ID+filepath.Ext(it.FileName))
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ---- Key 相关 ----

// GetKey 按凭证取 Key
func (s *memStore) GetKey(key string) (models.ApiKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.keys[key]
	return k, ok
}

// NewKey 生成一个新的临时访问 Key
func (s *memStore) NewKey(label string, quota int64, lib bool) models.ApiKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	k := models.ApiKey{
		Key:       hex.EncodeToString(b),
		Label:     label,
		Quota:     quota,
		Library:   lib,
		Active:    true,
		CreatedAt: time.Now(),
	}
	s.keys[k.Key] = k
	s.persist()
	return k
}

// ListKeys 返回全部 Key（管理员查看申请与用量）
func (s *memStore) ListKeys() []models.ApiKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.ApiKey, 0, len(s.keys))
	for _, k := range s.keys {
		out = append(out, k)
	}
	return out
}

// UpdateKey 更新 Key（管理员启用/停用/调额度/重置用量）
func (s *memStore) UpdateKey(k models.ApiKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[k.Key] = k
	s.persist()
}

// DeleteKey 删除 Key
func (s *memStore) DeleteKey(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[key]; !ok {
		return false
	}
	delete(s.keys, key)
	s.persist()
	return true
}

// ConsumeTokens 从 Key 额度中扣除 tokens，额度用尽返回 false
func (s *memStore) ConsumeTokens(key string, tokens int64) (models.ApiKey, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[key]
	if !ok || !k.Active {
		return models.ApiKey{}, false
	}
	if k.Quota > 0 && k.Used+tokens > k.Quota {
		return k, false // 超出额度
	}
	k.Used += tokens
	k.LastUsedAt = time.Now()
	s.keys[key] = k
	s.persist()
	return k, true
}

// seedConversations 初始会话为空：招聘者进来自己新建对话，不再预置假会话。
func seedConversations() []models.Conversation {
	return []models.Conversation{}
}

func seedFiles() []models.FileMeta {
	t := time.Now()
	return []models.FileMeta{
		{ID: "f1", Name: "自我介绍.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 3200, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f2", Name: "个人简历.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 7800, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f3", Name: "项目与作品.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 8600, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f4", Name: "技能图谱.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 3400, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f5", Name: "成长时间线.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 2600, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f6", Name: "李俊锋-Agent.pdf", Kind: "pdf", Path: "我的资料", Owner: "李俊锋", Size: pdfSize, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
		{ID: "f7", Name: "个人信息全文.md", Kind: "markdown", Path: "我的资料", Owner: "李俊锋", Size: 9200, CreatedAt: t, UpdatedAt: t, AccessedAt: t, Ingested: false},
	}
}

// pdfSize 李俊锋-Agent.pdf 的真实字节大小（data/uploads/f6.pdf）
var pdfSize int64 = 0

func init() {
	if fi, err := os.Stat(filepath.Join("..", "data", "uploads", "f6.pdf")); err == nil {
		pdfSize = fi.Size()
	}
}

// demoContents 种子文件内容（新简历 Agent 工程师版，供预览与 RAG 入库）
var demoContents = map[string]string{
	"f1": "# 自我介绍\n\n你好，我是李俊锋，男，24 岁，2024 年毕业于浙大宁波理工学院计算机科学与技术专业（工科学士），2 年工作经验，求职意向是 Agent 工程师。\n\n我具备后端工程底蕴与 AI 工程化实践经验：熟练落地业务需求完备的 Agent 项目，熟悉 Go 全栈开发、Agent 工程、大模型原理和训练；熟练使用 Python、Go，熟悉 Linux、Docker、设计模式、分布式架构和微服务设计；深度实践 Agent（ClaudeCode、Codex、OpenCode、Cherry、Qoder 等），熟悉 AI 编程范式和 AI 能力边界；能用 AI 快速验证想法和迭代，善于发现和改进业务逻辑与软件体验。欢迎招聘者向我提问。\n",
	"f2": "# 个人简历\n\n## 基本信息\n李俊锋，男，24 岁，2 年工作经验，求职意向：Agent 工程师。联系方式：15767210739 / 1657203672@qq.com。\n\n## 个人优势\n具备后端工程底蕴与 AI 工程化实践经验，善于将复杂业务需求转化为 Agent 系统方案，并利用 AI 技术生态实现系统开发。AI 应用工程化：熟练落地业务需求完备的 Agent 项目，熟悉 Go 全栈开发，Agent 工程，大模型原理和训练。高并发技术：熟练使用 Python、Go，熟悉 Linux、Docker、设计模式、分布式架构和微服务设计。新实践：深度实践 Agent，熟悉 AI 编程范式和 AI 能力边界。业务设计与验证：具备创意与独立验证的能力，用 AI 快速验证想法和迭代。\n\n## 教育经历\n浙大宁波理工学院 2020-2024 本科 计算机科学与技术 工科学士。品学兼优，总成绩保持专业 12%。毕业设计《基于 LLM 的智能问答助手开发》，探讨多角色扮演的 LLM 对话应用。社团：平面设计社团、计算机协会（PS/PR，贡献协会网站代码）。2023 大学生创新创业比赛：服装订货策略专家系统数学模型技术负责，获优秀奖。\n\n## 工作经历\n1. 广东必晟康智能科技有限公司（2024.12-至今）IT 技术支持：微小企业数字化；独立完成企业官网开发（AI 编程降本增效）；跨境电商 OEM/ODM 按摩器产品阿里国际站运营与投流，总询盘量提升 14%，主推品询盘量提升 22%；外贸业务账号英语商谈，月均创造 5 千销售额。\n2. 宁波宽易天地信息科技有限公司（2023.10-2024.04）IT 技术支持：独立负责宁波住建局官网及数据库技术支持；前端修改与高访问量渲染顺序重构，修复多处逻辑漏洞；MySQL 查询模板与 ECharts 数据可视化报表；数据仓库 SQL 优化；半年度服务商考核独立汇报。\n\n## 其他\n图书馆助理、兼职中小学教培；志愿活动策划组长；班文体委，校运会跳高第 7 名。\n",
	"f3": "# 项目与作品\n\n## InkBloom：高性能 AIGC 创作者工作台（Go 全栈，2026.09-至今）\n为自媒体创作者与小说作者打造 IDEA + AIGC 全流程平台，基于 Electron + Go：全本起稿工作流、异步插图、上帝模式世界模拟。Go + Python 异构微服务承载高并发业务流与多模态 AI 推理。\n- AI Agent 工程化：自研 Agent Harness 执行循环——LLM 多步推理 + 15+ 创作工具调用（建书/写章/知识库检索/世界模拟干预），SSE 步级事件流（start/step/tool_call/usage/final）与 token 级打字机响应，Agent 决策端到端透明可观测。\n- 异构微服务架构：Go(Gin) + Python(FastAPI) 以 Protobuf 强类型契约协作；多厂商 LLM Provider 注册表，DeepSeek/GLM 可插拔路由与 fail-closed 跨厂商降级链，Server Streaming 毫秒级流式响应与背压控制。\n- 高可靠系统设计：NATS 解耦 AIGC 长耗时任务；全本创作九段状态机（人在环暂停点、阶段回退锁、产物幂等入库）；LWW + 墓碑 + changeset 增量双向同步引擎，保障 PG/SQLite 双形态数据一致性。\n- 可观测性与生产部署：Zap 结构化日志 + Prometheus 指标（AIGC 失败链路埋点），golangci-lint 12 linter 质量门禁，鉴权链路单测覆盖率 93%+，Docker Compose 五组件一键集群部署。\n\n## BeYoung：B2B 海外独立站（Go 全栈，2026.09-至今）\n跨境 B2B 电商内容营销到询价交易全流程：Go 模块化单体 + Next.js 前后端分离，商品分类展示、双车并存询价、三态价格体系、多语言 CMS、卖家网站管理桌面端。\n- 全栈工程化：买家侧 Next.js（App Router）SSR + ISR 承载 SEO；卖家后台 React SPA 与 Electron 桌面端，渲染层 100% 复用，经 contextBridge 注入打印/Excel 能力。\n- 价格与交易核心：Go 侧 priceScope 中间件统一三态价格可见性（游客 MSRP / C 端零售 / B 端批发）；询价车与购物车双车并存、匿名可用、登录合并；SKU 级 RTS/MTO 库存双模式防超卖；统一订单模型 + 支付计划。\n- 高可靠：询盘报价 Quote 版本化 + 四态状态机；pgxpool 原生 SQL + internal 模块边界禁止跨模块读表；改价/报价/退款全量审计。\n- 生产交付：OpenAPI 3 契约驱动前端 api-client；多语言三级回退链；内容软删可回收；Docker Compose 自托管 PostgreSQL/Redis/Meilisearch 一键集群部署。\n\n## LLM 检索增强问答系统（全栈，2024.03-2024.06）\n企业级 LLM 检索增强问答系统，SpringBoot + Vue：RAG 增强检索、LLM 多模态对话。LangChain-Java 构建检索链路，PostgreSQL Vector 结构化存储与相似性检索；会话状态管理（文件上传、多轮上下文、标题自动生成、流式响应、会话记录管理）；MVC + Axios 三大模块。\n\n## D-S 证据理论的订货决策工具（算法程序，2021.11-2021.12）\n针对电商服装新品首次需求预测不准，应用多专家意见决策数学模型开发轻量 Web 程序。D-S 证据理论 + 报童模型，JavaScript 实现多专家预测信度融合到最优订货量；产品型号管理、专家意见管理、需求概率分布可视化、最优订货量弹窗；主导工程化落地，与商学院协作，获比赛优秀奖。\n",
	"f4": "# 技能图谱\n\n- Agent 工程：Agent Harness 执行循环、15+ 工具调用编排、SSE 步级事件流、多厂商 LLM Provider 注册表与降级链、NATS 异步任务、状态机设计（九段创作/四态报价）\n- 后端：Go（Gin、全栈、模块化单体、微服务）、Python（FastAPI）、pbxpool 原生 SQL、PostgreSQL（含 Vector）、MySQL、Redis、Meilisearch、NATS\n- 前端：Next.js（App Router SSR/ISR）、React SPA、Electron 桌面端、Vue、TypeScript、ECharts 数据可视化\n- AI 应用：RAG 检索增强（LangChain-Java）、大模型原理和训练、PyTorch、多模态推理、AI 编程范式（ClaudeCode/Codex/OpenCode/Cherry/Qoder）\n- 工程与部署：Linux、Docker、Docker Compose 集群部署、Protobuf 契约、OpenAPI 3、Zap 日志、Prometheus 指标、golangci-lint、设计模式、分布式架构、微服务设计\n- 业务：跨境电商运营（阿里国际站投流）、外贸英语商谈、企业数字化、平面设计（PS/PR）\n",
	"f5": "# 成长时间线\n\n- 2020-2024：浙大宁波理工学院 计算机科学与技术（工科学士，总成绩专业 12%）\n- 2021.11-2021.12：D-S 证据理论订货决策工具（技术负责，比赛优秀奖）\n- 2023.10-2024.04：宁波宽易天地 IT 技术支持（住建局官网运维独立负责）\n- 2024.03-2024.06：LLM 检索增强问答系统（SpringBoot + Vue 全栈）+ 毕业设计《基于 LLM 的智能问答助手》\n- 2024.12-至今：广东必晟康 IT 技术支持（企业数字化 + 跨境电商运营）\n- 2026.09-至今：InkBloom 高性能 AIGC 创作者工作台（Go 全栈 Agent 工程）\n- 2026.09-至今：BeYoung B2B 海外独立站（Go 全栈 + Next.js）\n",
	"f6": "李俊锋 Agent 工程师简历 PDF（李俊锋-Agent.pdf，含完整经历，可下载查看原文）。\n",
	"f7": "# 个人信息全文（来自李俊锋-Agent.pdf）\n\n李俊锋，男，24 岁，2 年工作经验，求职意向：Agent 工程师。联系方式：15767210739 / 1657203672@qq.com。\n\n## 个人优势\n具备后端工程底蕴与 AI 工程化实践经验，善于将复杂业务需求转化为 Agent 系统方案，并利用 AI 技术生态实现系统开发。AI 应用工程化：熟练落地业务需求完备的 Agent 项目，熟悉 Go 全栈开发，Agent 工程，大模型原理和训练。高并发技术：熟练使用 Python、Go，熟悉 Linux、Docker、设计模式、分布式架构和微服务设计。新实践：深度实践 Agent（ClaudeCode、Codex、OpenCode、Cherry、Qoder），熟悉 AI 编程范式和 AI 能力边界。业务设计与验证：具备创意与独立验证的能力，用 AI 快速验证想法和迭代，善于发现和改进业务逻辑和软件体验。\n\n## 教育经历\n浙大宁波理工学院 2020-2024 本科 计算机科学与技术 工科学士。品学兼优，总成绩保持专业 12%。毕业设计《基于 LLM 的智能问答助手开发》（多角色扮演 LLM 对话应用）。社团：平面设计社团、计算机协会（PS/PR，贡献协会网站代码）。2023 大学生创新创业比赛：服装订货策略专家系统数学模型技术负责，获优秀奖。\n\n## 项目经历\n1. InkBloom 高性能 AIGC 创作者工作台（Go 全栈，2026.09-至今）：Electron + Go AI 协同创作平台，全本起稿、异步插图、上帝模式世界模拟；自研 Agent Harness（LLM 多步推理 + 15+ 工具调用，SSE 步级事件流）；Go(Gin) + Python(FastAPI) Protobuf 协作，多厂商 LLM 路由与降级链；NATS 解耦长任务，九段状态机，LWW 增量同步；Zap + Prometheus 可观测，单测 93%+，Docker Compose 五组件部署。\n2. BeYoung B2B 海外独立站（Go 全栈，2026.09-至今）：Go 模块化单体 + Next.js，商品分类、双车并存询价、三态价格体系、多语言 CMS、卖家桌面端；priceScope 三态价格中间件；Quote 版本化 + 四态状态机；pgxpool + 模块边界；OpenAPI 3 驱动；Docker Compose 自托管 PG/Redis/Meilisearch。\n3. LLM 检索增强问答系统（全栈，2024.03-2024.06）：SpringBoot + Vue，LangChain-Java 检索链路，PostgreSQL Vector 存储，会话状态管理（上传/多轮/标题生成/流式/记录管理）。\n4. D-S 证据理论的订货决策工具（2021.11-2021.12）：多专家意见决策数学模型，JavaScript 实现信度融合到最优订货量，主导工程化，获优秀奖。\n\n## 实习/工作经历\n1. 广东必晟康智能科技有限公司（2024.12-至今）IT 技术支持：企业数字化与官网开发（AI 编程降本增效）；阿里国际站运营投流（总询盘 +14%，主推品 +22%）；外贸英语商谈，月均 5 千销售额。\n2. 宁波宽易天地信息科技有限公司（2023.10-2024.04）IT 技术支持：宁波住建局官网及数据库独立负责；前端重构与漏洞修复；MySQL + ECharts 可视化报表；数据仓库 SQL 优化；独立汇报考核。\n\n## 其他\n图书馆助理、教培兼职；志愿活动策划组长；班文体委，校运会跳高第 7 名。\n",
}
