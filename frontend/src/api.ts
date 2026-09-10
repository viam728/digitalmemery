// 与 Go 后端交互的 API 客户端（含 Key 鉴权与 SSE 流式）

import type {
  Conversation,
  Message,
  FileMeta,
  RagHit,
  Workspace,
  InboxItem,
  Avatar,
} from './types'

const BASE = '/api'
const KEY_STORAGE = 'jl_api_key'

export function getKey(): string {
  try {
    return localStorage.getItem(KEY_STORAGE) || ''
  } catch {
    return ''
  }
}

export function setKey(k: string) {
  try {
    if (k) localStorage.setItem(KEY_STORAGE, k)
    else localStorage.removeItem(KEY_STORAGE)
  } catch {
    /* ignore */
  }
}

/** 仅带 Authorization 鉴权头；FormData 上传时不能强制 Content-Type */
function authHeaders(): Record<string, string> {
  const h: Record<string, string> = {}
  const k = getKey()
  if (k) h['Authorization'] = `Bearer ${k}`
  return h
}

/** JSON 请求头（streamChat 使用，强制 Content-Type json） */
function jsonHeaders(): Record<string, string> {
  return { 'Content-Type': 'application/json', ...authHeaders() }
}

async function http<T>(path: string, init?: RequestInit): Promise<T> {
  const isForm = typeof FormData !== 'undefined' && init?.body instanceof FormData
  const h = authHeaders()
  const custom = init?.headers as Record<string, string> | undefined
  if (custom) Object.assign(h, custom)
  else if (init?.body && !isForm) h['Content-Type'] = 'application/json'
  const res = await fetch(`${BASE}${path}`, { ...init, headers: h })
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${path}`)
  return res.json() as Promise<T>
}

/** 拉取文件原始内容（文本或二进制，Content-Type 由后端返回） */
async function fetchRaw(path: string): Promise<Response> {
  const res = await fetch(`${BASE}${path}`, { headers: authHeaders() })
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${path}`)
  return res
}

export const api = {
  // Key
  applyKey: (label: string): Promise<{ key: string }> =>
    http('/keys/apply', { method: 'POST', body: JSON.stringify({ label }) }),
  verifyKey: (): Promise<{ valid: boolean; remaining: number; library: boolean; label: string }> =>
    http('/keys/me'),
  // 模型（key 内全部可用模型，默认第一个）
  listModels: (): Promise<{ default: string; models: { id: string; ownedBy?: string }[] }> =>
    http('/models'),
  switchModel: (model: string): Promise<{ default: string }> =>
    http('/models/switch', { method: 'POST', body: JSON.stringify({ model }) }),
  // Agent 产物（Jasper 的空间对外暴露的部分）
  listArtifacts: (): Promise<FileMeta[]> => http('/artifacts'),
  // 会话
  listConversations: (): Promise<Conversation[]> => http('/conversations'),
  createConversation: (title: string): Promise<Conversation> =>
    http('/conversations', { method: 'POST', body: JSON.stringify({ title }) }),
  listMessages: (cid: string): Promise<Message[]> => http(`/conversations/${cid}/messages`),
  renameConversation: (id: string, title: string): Promise<Conversation> =>
    http(`/conversations/${id}`, { method: 'PATCH', body: JSON.stringify({ title }) }),
  deleteConversation: (id: string): Promise<{ deleted: boolean }> =>
    http(`/conversations/${id}`, { method: 'DELETE' }),
  toggleConversationPinned: (id: string, pinned: boolean): Promise<Conversation> =>
    http(`/conversations/${id}`, { method: 'PATCH', body: JSON.stringify({ pinned }) }),
  // 资料库
  listFiles: (): Promise<FileMeta[]> => http('/files'),
  uploadFile: (file: File): Promise<FileMeta> => {
    const form = new FormData()
    form.append('file', file)
    return http('/files/upload', { method: 'POST', body: form })
  },
  ingestFile: (id: string): Promise<{ ingested: boolean }> =>
    http(`/files/${id}/ingest`, { method: 'POST' }),
  getFileContent: async (id: string): Promise<string> => (await fetchRaw(`/files/${id}/content`)).text(),
  getFileBlob: async (id: string): Promise<Blob> => (await fetchRaw(`/files/${id}/content`)).blob(),
  renameFile: (id: string, name: string): Promise<FileMeta> =>
    http(`/files/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) }),
  deleteFile: (id: string): Promise<{ deleted: boolean }> =>
    http(`/files/${id}`, { method: 'DELETE' }),
  downloadFile: async (id: string): Promise<Blob> => (await fetchRaw(`/files/${id}/download`)).blob(),
  // 收件箱（招聘者上传给我）
  listInbox: (): Promise<InboxItem[]> => http('/inbox'),
  uploadInbox: (file: File, name: string, note: string): Promise<InboxItem> => {
    const form = new FormData()
    form.append('file', file)
    form.append('name', name)
    form.append('note', note)
    return http('/inbox/upload', { method: 'POST', body: form })
  },
  deleteInbox: (id: string): Promise<{ deleted: boolean }> =>
    http(`/inbox/${id}`, { method: 'DELETE' }),
  downloadInbox: async (id: string): Promise<Blob> => (await fetchRaw(`/inbox/${id}/download`)).blob(),
  // 个人主页
  getAvatar: (): Promise<Avatar> => http('/avatar'),
  // RAG
  ragQuery: (q: string): Promise<RagHit[]> =>
    http('/rag/query', { method: 'POST', body: JSON.stringify({ query: q }) }),
  // 工作区
  getWorkspace: (id: string): Promise<Workspace> => http(`/workspaces/${id}`),
  createWorkspace: (title: string, model: string, refs: string[]): Promise<Workspace> =>
    http('/workspaces', { method: 'POST', body: JSON.stringify({ title, model, refs }) }),
  runWorkspace: (id: string, prompt: string): Promise<Workspace> =>
    http(`/workspaces/${id}/run`, { method: 'POST', body: JSON.stringify({ prompt }) }),
  // 管理员（用管理员 token 鉴权，非访客 Key）
  adminLogin: (password: string): Promise<{ token: string; expiresIn: number }> =>
    http('/admin/login', { method: 'POST', body: JSON.stringify({ password }) }),
  adminStats: (): Promise<AdminStats> => adminHttp('/admin/stats'),
  adminListKeys: (): Promise<ApiKeyInfo[]> => adminHttp('/admin/keys'),
  adminUpdateKey: (key: string, patch: { active?: boolean; quota?: number }): Promise<ApiKeyInfo> =>
    adminHttp(`/admin/keys/${key}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  adminRenewKey: (key: string): Promise<ApiKeyInfo> =>
    adminHttp(`/admin/keys/${key}/renew`, { method: 'POST' }),
  adminDeleteKey: (key: string): Promise<{ deleted: boolean }> =>
    adminHttp(`/admin/keys/${key}`, { method: 'DELETE' }),
}

const ADMIN_TOKEN_KEY = 'jl_admin_token'

export function getAdminToken(): string {
  try {
    return localStorage.getItem(ADMIN_TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function setAdminToken(t: string) {
  try {
    if (t) localStorage.setItem(ADMIN_TOKEN_KEY, t)
    else localStorage.removeItem(ADMIN_TOKEN_KEY)
  } catch {
    /* ignore */
  }
}

/** 管理员请求：Authorization 携带管理员 token */
async function adminHttp<T>(path: string, init?: RequestInit): Promise<T> {
  const t = getAdminToken()
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (t) h['Authorization'] = `Bearer ${t}`
  const res = await fetch(`${BASE}${path}`, { ...init, headers: h })
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${path}`)
  return res.json() as Promise<T>
}

/** 管理员看板数据 */
export interface AdminStats {
  uptimeSeconds: number
  llm: string
  rag: string
  counts: { files: number; conversations: number; messages: number; inbox: number; keys: number }
  tokens: { totalQuota: number; totalUsed: number; usedRate: number }
}

/** 管理员视图下的 Key 信息 */
export interface ApiKeyInfo {
  key: string
  label: string
  quota: number
  used: number
  library: boolean
  active: boolean
  createdAt: string
  lastUsedAt: string
}

// streamChat 流式发送消息，解析 SSE 并回调 delta；返回本次用量
// mode: 'chat'（面试应答）| 'agent'（Agent 任务）；onStep 接收可观测的工作流步骤
export async function streamChat(
  cid: string,
  content: string,
  refs: string[],
  onDelta: (delta: string) => void,
  opts?: { mode?: 'chat' | 'agent'; onStep?: (steps: { name: string; detail: string }[], model: string) => void },
): Promise<{ usage: number; remaining: number }> {
  const res = await fetch(`${BASE}/conversations/${cid}/messages/stream`, {
    method: 'POST',
    headers: jsonHeaders(),
    body: JSON.stringify({ content, refs, mode: opts?.mode ?? 'chat' }),
  })
  if (!res.ok || !res.body) throw new Error(`stream error ${res.status}`)

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  let usage = 0
  let remaining = -1

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const lines = buf.split('\n')
    buf = lines.pop() ?? ''
    for (const line of lines) {
      const t = line.trim()
      if (!t.startsWith('data:')) continue
      const payload = t.slice(5).trim()
      try {
        const evt = JSON.parse(payload)
        if (evt.delta) onDelta(evt.delta)
        if (evt.steps && opts?.onStep) opts.onStep(evt.steps, evt.model ?? '')
        if (evt.usage) usage = evt.usage
        if (evt.remaining !== undefined) remaining = evt.remaining
        if (evt.error) throw new Error(evt.error)
        if (evt.done) return { usage, remaining }
      } catch {
        /* ignore malformed */
      }
    }
  }
  return { usage, remaining }
}

