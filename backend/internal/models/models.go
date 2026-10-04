package models

import "time"

// ConversationKind 会话来源类型
type ConversationKind string

const (
	KindChat      ConversationKind = "chat"
	KindWorkspace ConversationKind = "workspace"
	KindLibrary   ConversationKind = "library"
	KindAvatar    ConversationKind = "avatar"
)

// Conversation 会话
//
// OwnerKey 归属访客 Key：会话按访客隔离，列表/读写仅限归属者（对他人一律 404，不暴露存在性）。
type Conversation struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Kind      ConversationKind `json:"kind"`
	Pinned    bool             `json:"pinned"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	OwnerKey  string           `json:"ownerKey,omitempty"`
}

// Message 聊天消息
type Message struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversationId"`
	Role           string           `json:"role"`
	Content        string           `json:"content"`
	CreatedAt      time.Time        `json:"createdAt"`
	Model          string           `json:"model,omitempty"`
	Details        *ResponseDetails `json:"details,omitempty"`
	Refs           []string         `json:"refs,omitempty"`
}

// ResponseDetails Agent 回复详情（对应截图三）
type ResponseDetails struct {
	Model        string `json:"model"`
	Status       string `json:"status"`
	ElapsedMs    int64  `json:"elapsedMs"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	AgentType    string `json:"agentType,omitempty"`
}

// FileMeta 文件元数据
//
// IsArtifact 产物标记：Agent 工作区产出的文件与资料库是同一份存储（同一行记录），
// 标记为产物的文件会自动暴露到「Jasper 的空间」，对外可见；非产物的工作区中间文件不暴露。
type FileMeta struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Path       string    `json:"path"`
	Owner      string    `json:"owner"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	AccessedAt time.Time `json:"accessedAt"`
	Tags       []string  `json:"tags"`
	Ingested   bool      `json:"ingested"`
	IsArtifact bool      `json:"isArtifact,omitempty"`
	// OwnerKey 上传者 Key（访客上传件归属；空表示 Jasper 公开资料/产物/历史文件）
	OwnerKey    string `json:"ownerKey,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

// RagHit 检索命中片段
type RagHit struct {
	FileID   string  `json:"fileId"`
	FileName string  `json:"fileName"`
	Chunk    string  `json:"chunk"`
	Score    float64 `json:"score"`
	Page     int     `json:"page,omitempty"`
	Line     int     `json:"line,omitempty"`
}

// WorkspaceFile 工作区文件树节点
type WorkspaceFile struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Path       string          `json:"path"`
	Children   []WorkspaceFile `json:"children,omitempty"`
	Referenced bool            `json:"referenced,omitempty"`
	// Content 文件内容（产出文件预览用）
	Content string `json:"content,omitempty"`
}

// Usage Agent 任务 token 用量（对应工作区右侧 Response details）
type Usage struct {
	PromptTokens     int64 `json:"promptTokens"`
	CompletionTokens int64 `json:"completionTokens"`
}

// RefContent 引用文件内容（Agent 任务运行时注入上下文的资料）
type RefContent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Workspace Agent 工作区
//
// OwnerKey 归属访客 Key：工作区同样按访客隔离（非归属者视为不存在）。
type Workspace struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Status   string          `json:"status"`
	Model    string          `json:"model"`
	Files    []WorkspaceFile `json:"files"`
	Usage    *Usage          `json:"usage,omitempty"`
	OwnerKey string          `json:"ownerKey,omitempty"`
}

// Avatar 数字分身主页数据（给招聘者看的个人主页）
type Avatar struct {
	Name     string         `json:"name"`     // 昵称
	Role     string         `json:"role"`     // 一句话人设
	Age      int            `json:"age"`      // 年龄
	Location string         `json:"location"` // 城市
	MBTI     string         `json:"mbti"`
	About    string         `json:"about"`    // 自我介绍段落
	Skills   []string       `json:"skills"`   // 技能标签
	Projects []Project      `json:"projects"` // 项目作品
	Timeline []TimelineItem `json:"timeline"` // 成长时间线
	Resume   []string       `json:"resume"`   // 简历要点
	Contact  []string       `json:"contact"`  // 联系方式（电话/邮箱/微信等）
}

// Project 项目作品卡片
type Project struct {
	Name  string   `json:"name"`  // 项目名
	Desc  string   `json:"desc"`  // 一句话介绍
	Stack []string `json:"stack"` // 技术栈
	Year  string   `json:"year"`  // 时间
}

// TimelineItem 成长时间线
type TimelineItem struct {
	Year string `json:"year"`
	Text string `json:"text"`
}

// InboxItem 访客投递到收件箱的条目（招聘者上传 JD / 资料 / 问题清单）。
// 数字分身会异步阅读投递内容并生成回复（Status/Reply/RepliedAt），实现「完全自动响应」。
type InboxItem struct {
	ID        string    `json:"id"`
	FileName  string    `json:"fileName"`
	Name      string    `json:"name"` // 署名
	Note      string    `json:"note"` // 留言
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
	// Status: pending（待处理）| replying（自动回复中）| replied（已回复）| failed（回复失败）
	Status    string    `json:"status,omitempty"`
	Reply     string    `json:"reply,omitempty"`     // 数字分身自动回复正文
	RepliedAt time.Time `json:"repliedAt,omitempty"` // 回复时间
	OwnerKey  string    `json:"ownerKey,omitempty"`  // 投递者 Key（收件箱按访客隔离）
}

// ApiKey 访客临时 Key：申请后获得 token 额度与资料库访问权限
//
//	Key: 生成的访问凭证
//	Quota: 总 token 额度（0 表示不限）
//	Used: 已消耗 token
//	Library: 是否解锁资料库
type ApiKey struct {
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	Quota      int64     `json:"quota"`
	Used       int64     `json:"used"`
	Library    bool      `json:"library"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}
