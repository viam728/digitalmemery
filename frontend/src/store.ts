// 全局应用状态（zustand）——只用真实后端数据，无 mock。
import { create } from 'zustand'
import type { Conversation, ConversationKind, Message, FileMeta, RagHit, Workspace, View, InboxItem, Avatar } from './types'
import type { AdminStats, ApiKeyInfo } from './api'
import { api, streamChat, getKey, setKey, getAdminToken, setAdminToken } from './api'

interface AppState {
  // Key / 鉴权
  key: string
  keyVerified: boolean
  keyRemaining: number
  // 布局
  view: View
  sidebarCollapsed: boolean
  rightPanelOpen: boolean
  rightPanelMode: 'files' | 'preview' | 'rag'
  // 数据
  conversations: Conversation[]
  activeConversationId: string | null
  messages: Message[]
  // 当前模型（key 内全部可用模型，默认第一个）
  currentModel: string
  availableModels: { id: string; ownedBy?: string }[]
  chatMode: 'chat' | 'agent'
  files: FileMeta[]
  artifacts: FileMeta[]
  ragHits: RagHit[]
  activeWorkspace: Workspace | null
  previewFile: FileMeta | null
  previewContent: string
  inbox: InboxItem[]
  avatar: Avatar | null
  // 管理员
  adminToken: string
  isAdmin: boolean
  adminStats: AdminStats | null
  adminKeys: ApiKeyInfo[]
  // 动作
  setKey: (k: string) => void
  applyKey: (label: string) => Promise<boolean>
  verifyKey: () => void
  setView: (v: View) => void
  toggleSidebar: () => void
  setRightPanel: (open: boolean, mode?: 'files' | 'preview' | 'rag') => void
  refreshConversations: () => void
  refreshFiles: () => void
  selectConversation: (id: string) => void
  refreshModels: () => void
  switchModel: (model: string) => Promise<void>
  setChatMode: (mode: 'chat' | 'agent') => void
  refreshArtifacts: () => void
  notice: (content: string) => void
  newConversation: () => Promise<void>
  renameConversation: (id: string, title: string) => Promise<void>
  deleteConversation: (id: string) => Promise<void>
  toggleConversationPinned: (id: string) => Promise<void>
  sendMessage: (text: string, refs?: string[]) => Promise<void>
  ingestFile: (id: string) => void
  uploadFile: (file: File) => void
  openFile: (id: string) => void
  ragQuery: (q: string) => void
  loadWorkspace: (id: string) => void
  createWorkspace: (title: string, refs?: string[]) => void
  runWorkspace: (id: string, prompt: string) => void
  // 收件箱 / 主页 / 文件管理
  refreshInbox: () => void
  uploadInbox: (file: File, name: string, note: string) => Promise<boolean>
  deleteInboxItem: (id: string) => void
  refreshAvatar: () => void
  renameFile: (id: string, name: string) => void
  deleteFile: (id: string) => void
  adminLogin: (password: string) => Promise<boolean>
  adminLogout: () => void
  refreshAdmin: () => void
  adminToggleKey: (key: string, active: boolean) => void
  adminRenewKey: (key: string) => void
  adminDeleteKey: (key: string) => void
}

// 本地临时消息 id（仅用于渲染；入库 id 由后端生成）
let idSeq = 100
const uid = (p: string) => `${p}${idSeq++}_${Date.now().toString(36)}`

// 会话排序：置顶优先，其次按 updatedAt 倒序
function sortConvs(list: Conversation[]): Conversation[] {
  return [...list].sort((a, b) => {
    const pa = a.pinned ? 1 : 0
    const pb = b.pinned ? 1 : 0
    if (pa !== pb) return pb - pa
    return String(b.updatedAt ?? '').localeCompare(String(a.updatedAt ?? ''))
  })
}

export const useApp = create<AppState>((set, get) => ({
  key: getKey(),
  keyVerified: false,
  keyRemaining: -1,
  view: 'chat',
  sidebarCollapsed: false,
  rightPanelOpen: false,
  rightPanelMode: 'files',
  conversations: [],
  activeConversationId: null,
  messages: [],
  currentModel: '',
  availableModels: [],
  chatMode: 'chat',
  files: [],
  artifacts: [],
  ragHits: [],
  activeWorkspace: null,
  previewFile: null,
  previewContent: '',
  inbox: [],
  avatar: null,
  adminToken: getAdminToken(),
  isAdmin: false,
  adminStats: null,
  adminKeys: [],

  setKey: (k) => {
    setKey(k)
    set({ key: k })
    get().verifyKey()
  },

  applyKey: async (label) => {
    try {
      const r = await api.applyKey(label)
      if (r.key) {
        setKey(r.key)
        set({ key: r.key })
        await get().verifyKey()
        return true
      }
      return false
    } catch {
      return false
    }
  },

  verifyKey: () => {
    api.verifyKey().then(
      (r) => set({ keyVerified: r.valid === true, keyRemaining: r.remaining ?? -1 }),
      () => set({ keyVerified: false, keyRemaining: -1 }),
    )
  },

  setView: (v) => set({ view: v, rightPanelOpen: false }),
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  setRightPanel: (open, mode) => set((s) => ({ rightPanelOpen: open, rightPanelMode: mode ?? s.rightPanelMode })),

  refreshConversations: () => {
    api.listConversations().then(
      (list) => set({ conversations: sortConvs(list ?? []) }),
      () => set({ conversations: [] }),
    )
  },

  refreshFiles: () => {
    api.listFiles().then(
      (list) => set({ files: list ?? [] }),
      () => set({ files: [] }),
    )
  },

  selectConversation: (id) => {
    const conv = get().conversations.find((c) => c.id === id)
    if (!conv) return
    set({ activeConversationId: id, messages: [], rightPanelOpen: false })
    // 左栏点会话必须切换主视图
    const viewOf: Record<ConversationKind, View> = {
      chat: 'chat',
      workspace: 'workspace',
      library: 'library',
      avatar: 'avatar',
    }
    set({ view: viewOf[conv.kind] ?? 'chat' })
    if (conv.kind === 'workspace') {
      get().loadWorkspace(id)
      setTimeout(() => set({ rightPanelOpen: true, rightPanelMode: 'files' }), 60)
    } else {
      api.listMessages(id).then(
        (m) => {
          if (get().activeConversationId === id) set({ messages: m ?? [] })
        },
        () => {
          if (get().activeConversationId === id) set({ messages: [] })
        },
      )
    }
  },

  refreshModels: () => {
    api.listModels().then(
      (r) => set({ currentModel: r.default ?? '', availableModels: r.models ?? [] }),
      () => {},
    )
  },

  switchModel: async (model) => {
    try {
      const r = await api.switchModel(model)
      set({ currentModel: r.default ?? model })
    } catch {
      /* ignore */
    }
  },

  setChatMode: (mode) => set({ chatMode: mode }),

  refreshArtifacts: () => {
    api.listArtifacts().then(
      (list) => set({ artifacts: list ?? [] }),
      () => set({ artifacts: [] }),
    )
  },

  // notice 在当前会话追加一条助手提示消息（快捷指令的本地回执，不经过后端）。
  notice: (content) => {
    let cid = get().activeConversationId
    if (!cid) {
      const now = new Date().toISOString()
      const c: Conversation = { id: `local-${Date.now()}`, title: '新对话', kind: 'chat', pinned: false, createdAt: now, updatedAt: now }
      cid = c.id
      set((s) => ({
        conversations: sortConvs([c, ...s.conversations]),
        activeConversationId: cid,
        messages: [],
        view: 'chat',
      }))
    }
    const nowT = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    const note: Message = { id: uid('m-'), conversationId: cid as string, role: 'assistant', content, createdAt: nowT }
    set((s) => ({ messages: [...s.messages, note] }))
  },

  newConversation: async () => {
    try {
      const c = await api.createConversation('新对话')
      set((s) => ({
        conversations: sortConvs([c, ...s.conversations]),
        activeConversationId: c.id,
        messages: [],
        rightPanelOpen: false,
        view: 'chat',
      }))
    } catch {
      /* 后端不可用时不创建本地假会话 */
    }
  },

  renameConversation: async (id, title) => {
    const name = title.trim()
    if (!name) return
    try {
      const updated = await api.renameConversation(id, name)
      set((s) => ({ conversations: sortConvs(s.conversations.map((c) => (c.id === id ? updated : c))) }))
    } catch {
      /* ignore */
    }
  },

  deleteConversation: async (id) => {
    try {
      const r = await api.deleteConversation(id)
      if (!r.deleted) return
    } catch {
      return
    }
    const wasActive = get().activeConversationId === id
    const rest = sortConvs(get().conversations.filter((c) => c.id !== id))
    if (wasActive) {
      set({ conversations: rest, activeConversationId: null, messages: [], view: 'chat' })
    } else {
      set({ conversations: rest })
    }
  },

  toggleConversationPinned: async (id) => {
    const conv = get().conversations.find((c) => c.id === id)
    if (!conv) return
    try {
      const updated = await api.toggleConversationPinned(id, !conv.pinned)
      set((s) => ({ conversations: sortConvs(s.conversations.map((c) => (c.id === id ? updated : c))) }))
    } catch {
      /* ignore */
    }
  },

  sendMessage: async (text, refs = []) => {
    // 无活动会话时先在后端建一个真实会话
    let cid = get().activeConversationId
    if (!cid) {
      try {
        const c = await api.createConversation(text.slice(0, 20) || '新对话')
        set((s) => ({
          conversations: sortConvs([c, ...s.conversations]),
          activeConversationId: c.id,
          messages: [],
          view: 'chat',
        }))
        cid = c.id
      } catch {
        return
      }
    }
    const nowT = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    const userMsg: Message = { id: uid('m-'), conversationId: cid, role: 'user', content: text, createdAt: nowT }
    const asstMsg: Message = { id: uid('m-'), conversationId: cid, role: 'assistant', content: '', createdAt: '' }
    set((s) => ({ messages: [...s.messages, userMsg, asstMsg] }))

    const start = Date.now()
    const mode = get().chatMode
    const model = get().currentModel
    try {
      let steps: { name: string; detail: string }[] = []
      const { usage, remaining } = await streamChat(
        cid,
        text,
        refs,
        (delta) => {
          set((s) => ({
            messages: s.messages.map((m) => (m.id === asstMsg.id ? { ...m, content: m.content + delta } : m)),
          }))
        },
        {
          mode,
          onStep: (s) => {
            steps = s
          },
        },
      )
      const stepText = steps.length > 0 ? `\n\n---\n工作流：${steps.map((s) => `${s.name}（${s.detail}）`).join(' → ')}` : ''
      set((s) => ({
        keyRemaining: remaining,
        messages: s.messages.map((m) =>
          m.id === asstMsg.id
            ? {
                ...m,
                createdAt: m.createdAt || nowT,
                content: m.content + stepText,
                model: model || undefined,
                details: { model: model || 'chat', status: 'Completed', elapsedMs: Date.now() - start, inputTokens: 0, outputTokens: usage, agentType: mode },
              }
            : m,
        ),
      }))
    } catch (e) {
      const msg = e instanceof Error ? e.message : '请求失败'
      set((s) => ({
        messages: s.messages.map((m) =>
          m.id === asstMsg.id ? { ...m, content: `发送失败：${msg}，请确认后端已启动后重试。`, createdAt: m.createdAt || nowT } : m,
        ),
      }))
    }
  },

  ingestFile: (id) => {
    api.ingestFile(id).then(
      (r) => {
        if (r?.ingested) set((s) => ({ files: s.files.map((f) => (f.id === id ? { ...f, ingested: true } : f)) }))
      },
      () => {},
    )
  },

  uploadFile: (file) => {
    api.uploadFile(file).then(
      (f) => {
        if (f) set((s) => ({ files: [f, ...s.files] }))
      },
      () => {},
    )
  },

  openFile: async (id) => {
    const f = get().files.find((x) => x.id === id)
    set({ previewFile: f ?? null, previewContent: '', rightPanelOpen: true, rightPanelMode: 'preview' })
    try {
      const c = await api.getFileContent(id)
      set({ previewContent: c })
    } catch {
      set({ previewContent: '' })
    }
  },

  ragQuery: (q) => {
    api.ragQuery(q).then(
      (hits) => set({ ragHits: hits ?? [], rightPanelOpen: true, rightPanelMode: 'rag' }),
      () => set({ ragHits: [], rightPanelOpen: true, rightPanelMode: 'rag' }),
    )
  },

  loadWorkspace: (id) => {
    api.getWorkspace(id).then(
      (w) => set({ activeWorkspace: w }),
      () => set({ activeWorkspace: null }),
    )
  },

  createWorkspace: (title, refs = []) => {
    // model 传空，后端自动使用当前配置模型
    api.createWorkspace(title, '', refs).then(
      (w) => set({ activeWorkspace: w, rightPanelOpen: true, rightPanelMode: 'files' }),
      () => {},
    )
  },

  runWorkspace: (id, prompt) => {
    api.runWorkspace(id, prompt).then(
      (w) => {
        if (w) set({ activeWorkspace: w, rightPanelOpen: true, rightPanelMode: 'files' })
      },
      () => {},
    )
  },

  refreshInbox: () => {
    api.listInbox().then(
      (items) => set({ inbox: items ?? [] }),
      () => set({ inbox: [] }),
    )
  },

  uploadInbox: async (file, name, note) => {
    try {
      await api.uploadInbox(file, name, note)
      get().refreshInbox()
      return true
    } catch {
      return false
    }
  },

  deleteInboxItem: (id) => {
    api.deleteInbox(id).then(
      () => get().refreshInbox(),
      () => {},
    )
  },

  refreshAvatar: () => {
    api.getAvatar().then(
      (a) => {
        if (a) set({ avatar: a })
      },
      () => {},
    )
  },

  renameFile: (id, name) => {
    api.renameFile(id, name).then(
      (f) => {
        if (f) set((s) => ({ files: s.files.map((x) => (x.id === id ? f : x)) }))
      },
      () => {},
    )
  },

  deleteFile: (id) => {
    api.deleteFile(id).then(
      (r) => {
        if (r?.deleted) set((s) => ({ files: s.files.filter((x) => x.id !== id) }))
      },
      () => {},
    )
  },

  adminLogin: async (password) => {
    try {
      const r = await api.adminLogin(password)
      if (r.token) {
        setAdminToken(r.token)
        set({ adminToken: r.token, isAdmin: true, view: 'admin' })
        get().refreshAdmin()
        return true
      }
      return false
    } catch {
      return false
    }
  },

  adminLogout: () => {
    setAdminToken('')
    set({ adminToken: '', isAdmin: false, view: 'avatar' })
  },

  refreshAdmin: () => {
    api.adminStats().then(
      (s) => {
        if (s) set({ adminStats: s })
      },
      () => {},
    )
    api.adminListKeys().then(
      (k) => set({ adminKeys: k ?? [] }),
      () => set({ adminKeys: [] }),
    )
  },

  adminToggleKey: (key, active) => {
    api.adminUpdateKey(key, { active }).then(
      () => get().refreshAdmin(),
      () => {},
    )
  },

  adminRenewKey: (key) => {
    api.adminRenewKey(key).then(
      () => get().refreshAdmin(),
      () => {},
    )
  },

  adminDeleteKey: (key) => {
    api.adminDeleteKey(key).then(
      () => get().refreshAdmin(),
      () => {},
    )
  },
}))
