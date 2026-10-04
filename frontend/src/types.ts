// 全局共享类型定义

// 会话
/** 会话来源类型 */
export type ConversationKind = 'chat' | 'workspace' | 'library' | 'avatar'

/** 会话 */
export interface Conversation {
  id: string
  title: string
  kind: ConversationKind
  pinned?: boolean
  createdAt: string
  updatedAt: string
  /** 归属访客 Key（会话按访客隔离） */
  ownerKey?: string
}

/** 聊天消息角色 */
export type Role = 'user' | 'assistant' | 'system' | 'tool'

/** 消息 */
export interface Message {
  id: string
  conversationId: string
  role: Role
  content: string
  createdAt: string
  model?: string
  /** Agent 回复详情（对应截图三 Response details） */
  details?: {
    model: string
    status: string
    elapsedMs: number
    inputTokens: number
    outputTokens: number
    agentType?: string
  }
  /** 消息引用的文件 */
  refs?: string[]
}

// 文件 / 资料库
/** 文件类型 */
export type FileKind = 'text' | 'markdown' | 'image' | 'pdf' | 'docx' | 'code' | 'json' | 'csv' | 'folder' | 'other'

/** 文件元数据 */
export interface FileMeta {
  id: string
  name: string
  kind: FileKind
  path: string
  owner: string
  size: number
  createdAt: string
  updatedAt: string
  accessedAt: string
  tags: string[]
  /** 是否已向量化入库 */
  ingested: boolean
  /** 解析出的文本/预览内容 */
  content?: string
  /** 是否为 Agent 产物（产物与资料库同一份存储，自动暴露到 Jasper 的空间） */
  isArtifact?: boolean
  /** 产出来源工作区 */
  workspaceId?: string
}

// RAG
/** 检索命中的片段 */
export interface RagHit {
  fileId: string
  fileName: string
  chunk: string
  score: number
  page?: number
  line?: number
}

// Agent 工作区
/** Agent 工作区 */
export interface Workspace {
  id: string
  title: string
  status: 'idle' | 'running' | 'done' | 'error'
  model: string
  /** 工作区文件树（右侧栏「文件」面板） */
  files: WorkspaceFile[]
  /** token 用量（运行任务后填充） */
  usage?: { promptTokens: number; completionTokens: number }
}

/** 工作区文件树节点 */
export interface WorkspaceFile {
  id: string
  name: string
  kind: FileKind
  path: string
  children?: WorkspaceFile[]
  /** 被会话引用 */
  referenced?: boolean
  /** 文件内容（产出文件预览用） */
  content?: string
}

// 收件箱（招聘者上传给我）
/** 收件箱条目 */
export interface InboxItem {
  id: string
  fileName: string
  name: string
  note: string
  size: number
  createdAt: string
  /** 自动应答状态：pending | replying | replied | failed */
  status?: 'pending' | 'replying' | 'replied' | 'failed'
  /** 数字分身自动回复正文 */
  reply?: string
  repliedAt?: string
}

// 个人主页（数字分身）
/** 项目作品卡片 */
export interface Project {
  name: string
  desc: string
  stack: string[]
  year: string
}

/** 个人主页数据（给招聘者看） */
export interface Avatar {
  name: string
  role: string
  age: number
  location: string
  mbti: string
  about: string
  skills: string[]
  projects: Project[]
  timeline: { year: string; text: string }[]
  resume: string[]
  contact?: string[]
}

// 视图
/** 主视图类型 */
export type View = 'chat' | 'library' | 'workspace' | 'avatar' | 'inbox' | 'admin'
