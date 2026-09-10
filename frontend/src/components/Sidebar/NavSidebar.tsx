import { useState } from 'react'
import { Plus, Hash, Bot, Folder, FileText, Pencil, Trash2, Pin, PinOff, Package } from 'lucide-react'
import { useApp } from '../../store'
import type { Conversation, ConversationKind } from '../../types'

const KIND_ICON: Record<ConversationKind, typeof Hash> = {
  chat: Hash,
  workspace: Bot,
  library: Folder,
  avatar: FileText,
}

export default function NavSidebar() {
  const conversations = useApp((s) => s.conversations)
  const activeId = useApp((s) => s.activeConversationId)
  const selectConversation = useApp((s) => s.selectConversation)
  const newConversation = useApp((s) => s.newConversation)
  const setView = useApp((s) => s.setView)
  const sidebarCollapsed = useApp((s) => s.sidebarCollapsed)

  if (sidebarCollapsed) {
    return (
      <div className="w-[2px] shrink-0 bg-white/5" />
    )
  }

  return (
    <aside className="w-64 shrink-0 flex flex-col border-r border-white/5 bg-ink-900/60">
      {/* 置顶：新对话（完全按 Codex） */}
      <div className="p-2.5 pb-1.5">
        <button
          onClick={() => void newConversation()}
          className="w-full flex items-center gap-2 px-2.5 py-2 rounded-lg bg-white/[0.05] hover:bg-white/[0.09] border border-white/10 text-[13px] text-neutral-200 transition-colors"
        >
          <span className="w-6 h-6 rounded-md bg-accent/20 text-accent flex items-center justify-center">
            <Plus size={14} />
          </span>
          新对话
        </button>
      </div>

      {/* 分类标题 */}
      <div className="px-3 pb-1">
        <button className="flex items-center gap-1.5 text-[11px] text-neutral-500 hover:text-neutral-300">
          <span className="w-728">会话</span>
        </button>
      </div>

      {/* 会话列表（可管理：改名 / 删除 / 置顶） */}
      <div className="flex-1 overflow-y-auto thin-scroll px-2 pb-2 space-y-0.5">
        {conversations.map((c) => (
          <ConversationItem
            key={c.id}
            conv={c}
            active={c.id === activeId}
            onClick={() => selectConversation(c.id)}
          />
        ))}
        {conversations.length === 0 && (
          <div className="px-3 py-4 text-[12px] text-neutral-600">暂无会话，点击「新对话」开始</div>
        )}
      </div>

      <div className="px-2.5 py-2.5 border-t border-white/5">
        <button
          onClick={() => setView('avatar')}
          className="w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-[12px] text-neutral-400 hover:text-neutral-100 hover:bg-white/5"
        >
          <Package size={14} />
          JasperLee 数字分身
        </button>
      </div>
    </aside>
  )
}

function ConversationItem({ conv, active, onClick }: { conv: Conversation; active: boolean; onClick: () => void }) {
  const Icon = KIND_ICON[conv.kind] ?? Hash
  const renameConversation = useApp((s) => s.renameConversation)
  const deleteConversation = useApp((s) => s.deleteConversation)
  const togglePinned = useApp((s) => s.toggleConversationPinned)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(conv.title)
  const [confirming, setConfirming] = useState(false)

  const commitRename = () => {
    setEditing(false)
    if (draft.trim() && draft.trim() !== conv.title) void renameConversation(conv.id, draft.trim())
    else setDraft(conv.title)
  }

  if (editing) {
    return (
      <input
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commitRename}
        onKeyDown={(e) => {
          if (e.key === 'Enter') commitRename()
          if (e.key === 'Escape') {
            setDraft(conv.title)
            setEditing(false)
          }
        }}
        onClick={(e) => e.stopPropagation()}
        className="w-full px-2 py-1.5 rounded-lg text-[13px] bg-white/10 text-neutral-100 border border-accent/40 focus:outline-none"
      />
    )
  }

  if (confirming) {
    return (
      <div className="w-full flex items-center gap-1 px-2 py-1.5 rounded-lg bg-red-500/10 border border-red-500/30 text-[12px]">
        <span className="flex-1 text-red-300 truncate">删除「{conv.title}」？</span>
        <button
          onClick={(e) => {
            e.stopPropagation()
            setConfirming(false)
            void deleteConversation(conv.id)
          }}
          className="px-1.5 py-0.5 rounded bg-red-500/20 text-red-200 hover:bg-red-500/40"
        >
          确认
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation()
            setConfirming(false)
          }}
          className="px-1.5 py-0.5 rounded text-neutral-400 hover:bg-white/10"
        >
          取消
        </button>
      </div>
    )
  }

  return (
    <div
      onClick={onClick}
      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-left transition-colors cursor-pointer group ${
        active ? 'bg-white/10 text-neutral-100' : 'text-neutral-300 hover:bg-white/5'
      }`}
    >
      <span className="w-8 h-6 rounded-md bg-white/5 border border-white/5 flex items-center justify-center shrink-0">
        <Icon size={13} className={active ? 'text-accent' : 'text-neutral-400'} />
      </span>
      <span className="flex-1 text-[13px] truncate" title={conv.title}>
        {conv.pinned && <span className="text-accent mr-1">📌</span>}
        {conv.title}
      </span>
      <span className="hidden group-hover:flex items-center gap-0.5 shrink-0">
        <button
          title={conv.pinned ? '取消置顶' : '置顶'}
          onClick={(e) => {
            e.stopPropagation()
            void togglePinned(conv.id)
          }}
          className="p-1 rounded text-neutral-500 hover:text-accent hover:bg-white/10"
        >
          {conv.pinned ? <PinOff size={12} /> : <Pin size={12} />}
        </button>
        <button
          title="改名"
          onClick={(e) => {
            e.stopPropagation()
            setDraft(conv.title)
            setEditing(true)
          }}
          className="p-1 rounded text-neutral-500 hover:text-neutral-100 hover:bg-white/10"
        >
          <Pencil size={12} />
        </button>
        <button
          title="删除"
          onClick={(e) => {
            e.stopPropagation()
            setConfirming(true)
          }}
          className="p-1 rounded text-neutral-500 hover:text-red-400 hover:bg-white/10"
        >
          <Trash2 size={12} />
        </button>
      </span>
    </div>
  )
}
