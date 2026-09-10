import { useEffect, useState } from 'react'
import { Search, PanelLeft } from 'lucide-react'
import { useApp } from '../../store'

export default function TopBar() {
  const toggleSidebar = useApp((s) => s.toggleSidebar)
  const keyRemaining = useApp((s) => s.keyRemaining)
  const ragQuery = useApp((s) => s.ragQuery)
  const [q, setQ] = useState('')
  const [now, setNow] = useState(() => new Date())

  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 30000)
    return () => clearInterval(t)
  }, [])

  const submitSearch = () => {
    const text = q.trim()
    if (text) ragQuery(text)
  }

  return (
    <div className="flex items-center h-10 px-3 gap-3 border-b border-white/5 bg-ink-900 shrink-0 select-none">
      <button
        onClick={toggleSidebar}
        className="p-1.5 text-neutral-400 hover:text-neutral-100 hover:bg-white/5 rounded-md transition-colors"
        title="折叠/展开侧栏"
      >
        <PanelLeft size={15} />
      </button>

      <div className="flex items-center gap-2">
        <div className="w-5 h-5 logo-glyph rounded flex items-center justify-center text-[10px] font-bold text-neutral-400 border border-white/10">
          JL
        </div>
        <span className="text-[13px] text-neutral-200 font-medium">JasperLee</span>
      </div>

      <div className="flex-1" />

      {/* 实时 RAG 搜索：输入即向后端 /api/rag/query 查询，结果进右侧面板 */}
      <div className="hidden md:flex items-center gap-1.5 rounded-lg bg-white/[0.03] border border-white/10 px-2 py-1">
        <Search size={13} className="text-neutral-500" />
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submitSearch()
          }}
          placeholder="搜索资料库（RAG）..."
          className="w-44 bg-transparent text-[12px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none"
        />
      </div>

      {/* Key 剩余额度 */}
      <span
        className="text-[11px] text-neutral-500"
        title={keyRemaining >= 0 ? `剩余 ${keyRemaining} tokens` : '额度未知'}
      >
        {keyRemaining >= 0 ? `剩 ${formatTokens(keyRemaining)}` : '额度 --'}
      </span>

      {/* 当前时间 */}
      <span className="text-[11px] text-neutral-500 tabular-nums">
        {now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
      </span>
    </div>
  )
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return `${n}`
}
