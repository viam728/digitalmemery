import {
  Plus,
  MessageSquare,
  FolderOpen,
  Settings,
  BookOpen,
  LifeBuoy,
  Upload,
  User,
  LayoutGrid,
} from 'lucide-react'
import { useApp } from '../../store'
import type { View } from '../../types'

interface RailItem {
  key: View
  icon: typeof MessageSquare
  label: string
}

// 导航项：Agent 工作区已合并进「对话」入口（统一在对话内切换 Ask/Agent 模式），不再单列
const ITEMS: RailItem[] = [
  { key: 'avatar', icon: User, label: '个人主页' },
  { key: 'board', icon: LayoutGrid, label: '平台看板' },
  { key: 'chat', icon: MessageSquare, label: '对话' },
  { key: 'library', icon: FolderOpen, label: '资料库' },
  { key: 'inbox', icon: Upload, label: '上传给我' },
]

export default function IconRail() {
  const view = useApp((s) => s.view)
  const setView = useApp((s) => s.setView)

  return (
    <div className="w-11 shrink-0 flex flex-col items-center py-2 gap-1.5 border-r border-white/5 bg-ink-900">
      <button
        onClick={() => setView('chat')}
        className="w-8 h-8 flex items-center justify-center rounded-lg text-neutral-300 hover:bg-white/10 hover:text-white transition-colors"
        title="新建对话"
      >
        <Plus size={17} />
      </button>

      <div className="w-[22px] h-px bg-white/10 my-0.5" />

      {ITEMS.map((it) => {
        const Icon = it.icon
        const active = view === it.key
        return (
          <button
            key={it.key}
            onClick={() => setView(it.key)}
            className={`w-8 h-8 flex items-center justify-center rounded-lg transition-colors ${
              active ? 'bg-accent/15 text-accent' : 'text-neutral-400 hover:bg-white/5 hover:text-neutral-100'
            }`}
            title={it.label}
          >
            <Icon size={17} />
          </button>
        )
      })}

      <div className="flex-1" />

      <button className="w-8 h-8 flex items-center justify-center rounded-lg text-neutral-500 hover:bg-white/5 hover:text-neutral-100"><BookOpen size={16} /></button>
      <button className="w-8 h-8 flex items-center justify-center rounded-lg text-neutral-500 hover:bg-white/5 hover:text-neutral-100"><LifeBuoy size={16} /></button>
      <button className="w-8 h-8 flex items-center justify-center rounded-lg text-neutral-500 hover:bg-white/5 hover:text-neutral-100"><Settings size={16} /></button>
    </div>
  )
}
