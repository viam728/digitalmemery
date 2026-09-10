import { useEffect, useRef, useState } from 'react'
import { Paperclip, Search, Hash, ArrowUp, FileText, MessageSquare } from 'lucide-react'
import { useApp } from '../../store'
import MessageBubble from './MessageBubble'
import type { FileMeta } from '../../types'

const PLACEHOLDER = '问我关于李俊锋的任何问题，按 Enter 发送；输入 / 用快捷指令，输入 @ 引用资料库文件'

// ---- 快捷指令（/ 命令）：全部接真实功能，无假按钮 ----
interface SlashCmd {
  name: string
  desc: string
  run: (ctx: SlashCtx) => void
}

interface SlashCtx {
  sendMessage: (text: string, refs?: string[]) => Promise<void>
  notice: (content: string) => void
  setView: (v: 'chat' | 'library' | 'workspace' | 'avatar' | 'inbox' | 'admin') => void
  ragQuery: (q: string) => void
  newConversation: () => Promise<void>
  clearInput: () => void
}

const SLASH_CMDS: SlashCmd[] = [
  {
    name: '/help',
    desc: '查看全部快捷指令',
    run: ({ notice }) =>
      notice(
        '快捷指令：\n' +
          '/help —— 查看本列表\n' +
          '/new —— 新建对话\n' +
          '/intro —— 自我介绍\n' +
          '/skills —— 技能与擅长方向\n' +
          '/projects —— 代表项目\n' +
          '/contact —— 联系方式\n' +
          '/resume —— 下载简历 PDF\n' +
          '/search 关键词 —— RAG 检索资料库\n' +
          '/library —— 打开资料库\n' +
          '/inbox —— 打开上传给我\n' +
          '/home —— 回个人主页',
      ),
  },
  { name: '/new', desc: '新建对话', run: ({ newConversation, clearInput }) => { clearInput(); void newConversation() } },
  { name: '/intro', desc: '自我介绍', run: ({ sendMessage, clearInput }) => { clearInput(); void sendMessage('介绍一下你自己和代表作') } },
  { name: '/skills', desc: '技能与擅长方向', run: ({ sendMessage, clearInput }) => { clearInput(); void sendMessage('你的技能与擅长方向是什么？') } },
  { name: '/projects', desc: '代表项目', run: ({ sendMessage, clearInput }) => { clearInput(); void sendMessage('聊聊你做过最有意思的项目') } },
  { name: '/contact', desc: '联系方式', run: ({ sendMessage, clearInput }) => { clearInput(); void sendMessage('你的联系方式是什么？') } },
  {
    name: '/resume',
    desc: '下载简历 PDF（李俊锋.pdf）',
    run: ({ notice, clearInput }) => {
      clearInput()
      void import('../../api').then(async ({ api }) => {
        try {
          const blob = await api.downloadFile('f6')
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = '李俊锋.pdf'
          a.click()
          URL.revokeObjectURL(url)
          notice('简历 PDF 已开始下载（李俊锋.pdf）。')
        } catch {
          notice('简历下载失败，请确认后端已启动后重试。')
        }
      })
    },
  },
  {
    name: '/search',
    desc: 'RAG 检索资料库，如 /search RAG',
    // /search 带参数，由 runSlashCommand 单独解析，此 run 仅为占位（不会被调用）
    run: () => {},
  },
  { name: '/library', desc: '打开资料库', run: ({ setView, clearInput }) => { clearInput(); setView('library') } },
  { name: '/inbox', desc: '打开上传给我', run: ({ setView, clearInput }) => { clearInput(); setView('inbox') } },
  { name: '/home', desc: '回个人主页', run: ({ setView, clearInput }) => { clearInput(); setView('avatar') } },
]

/** 解析并执行 / 快捷指令；返回 true 表示已消费（调用方不再发送）。 */
function runSlashCommand(raw: string, ctx: SlashCtx): boolean {
  const text = raw.trim()
  if (!text.startsWith('/')) return false
  const [cmd, ...rest] = text.split(/\s+/)
  const arg = rest.join(' ').trim()
  if (cmd === '/search') {
    ctx.clearInput()
    if (!arg) {
      ctx.notice('用法：/search 关键词，例如 /search RAG')
      return true
    }
    ctx.ragQuery(arg)
    return true
  }
  const hit = SLASH_CMDS.find((c) => c.name === cmd)
  if (hit && hit.name !== '/search') {
    hit.run(ctx)
    return true
  }
  ctx.clearInput()
  ctx.notice(`未知指令 ${cmd}，输入 /help 查看全部快捷指令。`)
  return true
}

export default function ChatView() {
  const messages = useApp((s) => s.messages)
  const hasConversation = useApp((s) => s.activeConversationId != null)

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      {messages.length === 0 ? (
        <EmptyState hasConversation={hasConversation} />
      ) : (
        <div className="flex-1 min-h-0 overflow-y-auto px-4 py-6 space-y-5 thin-scroll">
          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} />
          ))}
        </div>
      )}
      <Composer />
    </div>
  )
}

function EmptyState({ hasConversation }: { hasConversation: boolean }) {
  const sendMessage = useApp((s) => s.sendMessage)
  const suggestions = ['介绍一下你自己和代表作', '你的技能与擅长方向是什么？', '聊聊你做过最有意思的项目']
  return (
    <div className="flex-1 min-h-0 flex flex-col items-center justify-center gap-4 px-6">
      <div className="w-14 h-14 logo-glyph rounded-full border border-white/10 flex items-center justify-center">
        <span className="text-2xl font-bold text-neutral-300">JL</span>
      </div>
      <p className="text-[17px] text-neutral-200">想了解李俊锋？问 JasperLee</p>
      <p className="text-[12.5px] text-neutral-500">经历 · 技能 · 项目 · 岗位匹配度，都可以问我</p>
      {!hasConversation && (
        <div className="mt-2 flex flex-wrap justify-center gap-2">
          {suggestions.map((s) => (
            <button
              key={s}
              onClick={() => void sendMessage(s)}
              className="px-3 py-1.5 rounded-full border border-white/10 text-[12px] text-neutral-400 hover:text-neutral-100 hover:border-accent/40 transition-colors"
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function Composer() {
  const sendMessage = useApp((s) => s.sendMessage)
  const notice = useApp((s) => s.notice)
  const setView = useApp((s) => s.setView)
  const ragQuery = useApp((s) => s.ragQuery)
  const newConversation = useApp((s) => s.newConversation)
  const files = useApp((s) => s.files)
  const conversations = useApp((s) => s.conversations)
  const refreshFiles = useApp((s) => s.refreshFiles)
  const refreshConversations = useApp((s) => s.refreshConversations)
  const currentModel = useApp((s) => s.currentModel)
  const availableModels = useApp((s) => s.availableModels)
  const switchModel = useApp((s) => s.switchModel)
  const refreshModels = useApp((s) => s.refreshModels)
  const chatMode = useApp((s) => s.chatMode)
  const setChatMode = useApp((s) => s.setChatMode)
  const [value, setValue] = useState('')
  const [atOpen, setAtOpen] = useState(false)
  const [atQuery, setAtQuery] = useState('')
  const [slashOpen, setSlashOpen] = useState(false)
  const [slashQuery, setSlashQuery] = useState('')
  const [slashIndex, setSlashIndex] = useState(0)
  const [pickedRefs, setPickedRefs] = useState<string[]>([])
  const taRef = useRef<HTMLTextAreaElement>(null)

  // 面板打开时保证有最新数据
  useEffect(() => {
    if (atOpen && files.length === 0) refreshFiles()
    if (atOpen && conversations.length === 0) refreshConversations()
  }, [atOpen, files.length, conversations.length, refreshFiles, refreshConversations])

  const clearInput = () => {
    setValue('')
    setPickedRefs([])
    setAtOpen(false)
    setSlashOpen(false)
  }

  const ctx: SlashCtx = { sendMessage, notice, setView, ragQuery, newConversation, clearInput }

  const submit = () => {
    const text = value.trim()
    if (!text) return
    if (runSlashCommand(text, ctx)) return
    const { clean, refs } = extractAtRefs(text)
    void sendMessage(clean || text, [...pickedRefs, ...refs])
    clearInput()
  }

  // 输入变化：检测 @ 与 / 触发面板
  const handleChange = (v: string) => {
    setValue(v)
    const cursor = taRef.current?.selectionStart ?? v.length
    const before = v.slice(0, cursor)
    const atMatch = before.match(/@([^\s@]*)$/)
    if (atMatch) {
      setAtOpen(true)
      setAtQuery(atMatch[1] ?? '')
    } else {
      setAtOpen(false)
    }
    // / 仅在行首（或空输入）时触发指令面板
    const lineStart = before.slice(0, cursor).split('\n').pop() ?? ''
    if (/^\/[a-z]*$/.test(lineStart.trim()) && (before.trimStart().startsWith('/'))) {
      setSlashOpen(true)
      setSlashQuery(lineStart.trim().slice(1))
      setSlashIndex(0)
    } else {
      setSlashOpen(false)
    }
  }

  const pickRef = (id: string, label: string) => {
    if (!pickedRefs.includes(id)) setPickedRefs((p) => [...p, id])
    // 把 @xxx 替换为 @label（空格结尾，便于继续输入）
    const cursor = taRef.current?.selectionStart ?? value.length
    const before = value.slice(0, cursor)
    const after = value.slice(cursor)
    const nb = before.replace(/@[^\s@]*$/, `@${label} `)
    setValue(nb + after)
    setAtOpen(false)
    taRef.current?.focus()
  }

  const slashList = SLASH_CMDS.filter(
    (c) => c.name !== '/search' || true,
  ).filter((c) => c.name.slice(1).startsWith(slashQuery.toLowerCase()))

  const acceptSlash = (cmd: string) => {
    if (cmd === '/search') {
      // /search 需要参数：填入前缀让用户补关键词
      setValue('/search ')
      setSlashOpen(false)
      taRef.current?.focus()
      return
    }
    setValue(cmd)
    setSlashOpen(false)
    // 直接执行
    runSlashCommand(cmd, ctx)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (slashOpen && slashList.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSlashIndex((i) => (i + 1) % slashList.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSlashIndex((i) => (i - 1 + slashList.length) % slashList.length)
        return
      }
      if (e.key === 'Tab' || e.key === 'Enter') {
        const hit = slashList[slashIndex]
        if (hit) {
          e.preventDefault()
          acceptSlash(hit.name)
          return
        }
      }
      if (e.key === 'Escape') {
        setSlashOpen(false)
        return
      }
    }
    if (e.key === 'Enter' && !e.shiftKey && !atOpen) {
      e.preventDefault()
      submit()
    }
    if (e.key === 'Escape') {
      setAtOpen(false)
      setSlashOpen(false)
    }
  }

  const atFiles = files
    .filter((f) => f.name.toLowerCase().includes(atQuery.toLowerCase()))
    .slice(0, 6)
  const atConvs = conversations
    .filter((c) => c.title.toLowerCase().includes(atQuery.toLowerCase()))
    .slice(0, 4)

  return (
    <div className="px-4 pb-4 shrink-0">
      <div className="max-w-3xl mx-auto relative">
        {/* / 快捷指令面板 */}
        {slashOpen && slashList.length > 0 && (
          <div className="absolute bottom-full mb-2 left-0 right-0 rounded-xl border border-white/10 bg-ink-800 shadow-xl overflow-hidden z-20">
            {slashList.map((c, i) => (
              <button
                key={c.name}
                onMouseDown={(e) => {
                  e.preventDefault()
                  acceptSlash(c.name)
                }}
                onMouseEnter={() => setSlashIndex(i)}
                className={`w-full flex items-center gap-2.5 px-3 py-2 text-left ${i === slashIndex ? 'bg-accent/15' : ''}`}
              >
                <span className="font-mono text-[12px] text-accent">{c.name}</span>
                <span className="text-[12px] text-neutral-400 truncate">{c.desc}</span>
              </button>
            ))}
          </div>
        )}

        {/* @ 引用面板：资料库文件 + 历史会话 */}
        {atOpen && (
          <div className="absolute bottom-full mb-2 left-0 right-0 rounded-xl border border-white/10 bg-ink-800 shadow-xl overflow-hidden z-20 max-h-64 overflow-y-auto thin-scroll">
            {atFiles.length > 0 && (
              <>
                <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-wide text-neutral-500">资料库文件</div>
                {atFiles.map((f) => (
                  <AtRow
                    key={f.id}
                    icon={<FileText size={13} className="text-neutral-500" />}
                    title={f.name}
                    sub={`${f.owner} · ${f.kind}`}
                    badge={f.ingested ? '已入库' : undefined}
                    onPick={() => pickRef(f.id, f.name)}
                  />
                ))}
              </>
            )}
            {atConvs.length > 0 && (
              <>
                <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-wide text-neutral-500">历史会话</div>
                {atConvs.map((c) => (
                  <AtRow
                    key={c.id}
                    icon={<MessageSquare size={13} className="text-neutral-500" />}
                    title={c.title}
                    sub={c.kind}
                    onPick={() => pickRef(`conv:${c.id}`, c.title)}
                  />
                ))}
              </>
            )}
            {atFiles.length === 0 && atConvs.length === 0 && (
              <div className="px-3 py-3 text-[12px] text-neutral-500">无匹配：输入关键词过滤，或先去资料库上传文件</div>
            )}
          </div>
        )}

        {/* 已选引用 chips */}
        {pickedRefs.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mb-2">
            {pickedRefs.map((id) => (
              <button
                key={id}
                onClick={() => setPickedRefs((p) => p.filter((x) => x !== id))}
                title="点击移除"
                className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-accent/10 border border-accent/30 text-[11px] text-accent"
              >
                <FileText size={11} /> {refLabel(id, files)} ×
              </button>
            ))}
          </div>
        )}

        <div className="rounded-xl border border-white/10 bg-ink-850 focus-within:border-white/20 transition-colors">
          <textarea
            ref={taRef}
            value={value}
            onChange={(e) => handleChange(e.target.value)}
            onKeyDown={handleKeyDown}
            rows={2}
            placeholder={PLACEHOLDER}
            className="w-full bg-transparent px-3 pt-3 pb-1 resize-none text-[13px] text-neutral-100 focus:outline-none placeholder:text-neutral-500"
          />
          <div className="flex items-center gap-1 px-2 pb-2">
            <button className="p-1.5 text-neutral-400 hover:text-neutral-100 hover:bg-white/5 rounded-md" title="添加文件"><Paperclip size={15} /></button>
            <button
              onClick={() => setSlashOpen((o) => !o)}
              className="p-1.5 text-neutral-400 hover:text-neutral-100 hover:bg-white/5 rounded-md"
              title="快捷指令（/）"
            >
              <Search size={15} />
            </button>
            <button
              onClick={() => {
                setAtOpen((o) => !o)
                taRef.current?.focus()
              }}
              className="p-1.5 text-neutral-400 hover:text-neutral-100 hover:bg-white/5 rounded-md"
              title="引用会话/文件（@）"
            >
              <Hash size={15} />
            </button>
            <div className="flex-1" />
            {/* 对话 / Agent 模式切换（共用同一会话历史） */}
            <div className="flex items-center rounded-md bg-white/[0.04] p-0.5">
              <button
                onClick={() => setChatMode('chat')}
                title="对话模式：面试应答"
                className={`px-2 py-0.5 rounded text-[11px] transition-colors ${chatMode === 'chat' ? 'bg-accent/20 text-accent' : 'text-neutral-500 hover:text-neutral-300'}`}
              >
                对话
              </button>
              <button
                onClick={() => setChatMode('agent')}
                title="Agent 模式：灵魂工作流 + 产物"
                className={`px-2 py-0.5 rounded text-[11px] transition-colors ${chatMode === 'agent' ? 'bg-accent/20 text-accent' : 'text-neutral-500 hover:text-neutral-300'}`}
              >
                Agent
              </button>
            </div>
            {/* 模型切换（key 内全部可用模型，默认第一个） */}
            <ModelSelect
              current={currentModel}
              options={availableModels}
              onRefresh={refreshModels}
              onSwitch={(m) => void switchModel(m)}
            />
            <button
              onClick={submit}
              disabled={!value.trim()}
              className="w-7 h-7 flex items-center justify-center rounded-lg bg-neutral-100 text-ink-950 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-white transition-colors"
              title="发送"
            >
              <ArrowUp size={15} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function ModelSelect({ current, options, onRefresh, onSwitch }: {
  current: string
  options: { id: string; ownedBy?: string }[]
  onRefresh: () => void
  onSwitch: (m: string) => void
}) {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    if (open && options.length === 0) onRefresh()
  }, [open, options.length, onRefresh])
  return (
    <div className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        title="切换模型"
        className="flex items-center gap-1 px-2 py-1 rounded-md text-[11px] text-neutral-400 hover:text-neutral-100 hover:bg-white/5 max-w-40"
      >
        <span className="truncate">{current || '模型'}</span>
      </button>
      {open && (
        <div className="absolute bottom-full mb-2 right-0 w-52 rounded-xl border border-white/10 bg-ink-800 shadow-xl overflow-hidden z-20 max-h-64 overflow-y-auto thin-scroll">
          {options.length === 0 && (
            <div className="px-3 py-2.5 text-[12px] text-neutral-500">加载中…</div>
          )}
          {options.map((m) => (
            <button
              key={m.id}
              onClick={() => {
                onSwitch(m.id)
                setOpen(false)
              }}
              className={`w-full flex items-center gap-2 px-3 py-1.5 text-left text-[12px] hover:bg-white/5 ${m.id === current ? 'text-accent' : 'text-neutral-200'}`}
            >
              <span className="flex-1 truncate font-mono">{m.id}</span>
              {m.id === current && <span>✓</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function AtRow({ icon, title, sub, badge, onPick }: { icon: React.ReactNode; title: string; sub: string; badge?: string; onPick: () => void }) {
  return (
    <button
      onMouseDown={(e) => {
        e.preventDefault()
        onPick()
      }}
      className="w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-white/5"
    >
      {icon}
      <span className="flex-1 min-w-0 text-[12.5px] text-neutral-100 truncate">{title}</span>
      {badge && <span className="text-[10px] text-accent border border-accent/30 rounded px-1">{badge}</span>}
      <span className="text-[11px] text-neutral-500 shrink-0">{sub}</span>
    </button>
  )
}

/** 文本内 @name 反查文件 id（用于未走面板直接手打 @文件名 的情况）。 */
function extractAtRefs(text: string): { clean: string; refs: string[] } {
  // 面板已把引用收进 pickedRefs；此处仅保留文本原样，后端按 refs 注入上下文。
  // 为避免 @xxx 干扰模型阅读，把裸 @token 转为「引用：xxx」提示。
  const clean = text.replace(/@([^\s@]+)/g, '（引用：$1）')
  return { clean, refs: [] }
}

function refLabel(id: string, files: FileMeta[]): string {
  if (id.startsWith('conv:')) return `会话:${id.slice(5, 13)}…`
  const f = files.find((x) => x.id === id)
  return f ? f.name : id.slice(0, 8)
}
