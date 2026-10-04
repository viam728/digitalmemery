import { useEffect, useMemo, useState } from 'react'
import {
  ExternalLink,
  Plus,
  X,
  Check,
  Pencil,
  Trash2,
  Code2,
  MessagesSquare,
  PenLine,
  Target,
  Share2,
  Link2,
} from 'lucide-react'
import { useApp } from '../../store'
import type { SocialLink } from '../../types'

// 分类元数据：顺序 / 标题 / 图标 / 主色
const CATEGORIES: { key: string; label: string; icon: typeof Code2; color: string }[] = [
  { key: 'code', label: '代码 & 开源', icon: Code2, color: '#38bdf8' },
  { key: 'community', label: '技术社区', icon: MessagesSquare, color: '#a78bfa' },
  { key: 'blog', label: '博客 & 内容', icon: PenLine, color: '#f472b6' },
  { key: 'practice', label: '练习 & 竞赛', icon: Target, color: '#fbbf24' },
  { key: 'social', label: '社交 & 职业', icon: Share2, color: '#34d399' },
]

// 品牌色（卡片徽标），未命中时用所属分类色
const BRAND: Record<string, string> = {
  github: '#24292f', gitee: '#c71d23', stackoverflow: '#f48024', dockerhub: '#2496ed',
  juejin: '#1e80ff', csdn: '#fc5531', segmentfault: '#009a61', v2ex: '#333333', cnblogs: '#2b6cb0', zhihu: '#0084ff',
  blog: '#0ea5e9', 'wechat-mp': '#07c160', yuque: '#25b864', medium: '#111111',
  leetcode: '#ffa116', nowcoder: '#00c0a3', codeforces: '#1f8acb', luogu: '#3498db', kaggle: '#20beff',
  linkedin: '#0a66c2', x: '#111111', bilibili: '#fb7299', xiaohongshu: '#ff2442',
}

const INPUT =
  'bg-transparent border border-white/10 rounded-md px-2 py-1 text-[12.5px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none focus:border-accent/50'

export default function BoardView() {
  const social = useApp((s) => s.social)
  const refreshSocial = useApp((s) => s.refreshSocial)
  const isAdmin = useApp((s) => s.isAdmin)
  const [adding, setAdding] = useState(false)

  useEffect(() => {
    refreshSocial()
  }, [refreshSocial])

  const grouped = useMemo(() => {
    const byCat: Record<string, SocialLink[]> = {}
    for (const l of social) {
      if (!byCat[l.category]) byCat[l.category] = []
      byCat[l.category].push(l)
    }
    return byCat
  }, [social])

  return (
    <div className="flex-1 min-h-0 overflow-y-auto thin-scroll">
      <div className="max-w-5xl mx-auto px-6 py-8">
        <header className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-lg font-medium text-neutral-100 flex items-center gap-2">
              <Link2 size={18} className="text-accent" /> 平台看板
            </h1>
            <p className="mt-1 text-[12.5px] text-neutral-500">
              我的技术足迹：代码 · 社区 · 博客 · 练习 · 社交。
              {isAdmin ? ' 双击卡片即可就地编辑，保存后对访客生效。' : ' 点击卡片打开主页。'}
            </p>
          </div>
          {isAdmin && (
            <button
              onClick={() => setAdding(true)}
              className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent/15 text-accent text-[12.5px] hover:bg-accent/25"
            >
              <Plus size={14} /> 添加平台
            </button>
          )}
        </header>

        {adding && isAdmin && <AddCard onClose={() => setAdding(false)} />}

        {social.length === 0 && <div className="mt-8 text-[12.5px] text-neutral-500">暂无平台条目。</div>}

        {CATEGORIES.map((cat) => {
          const items = grouped[cat.key] ?? []
          if (items.length === 0) return null
          const Icon = cat.icon
          return (
            <section key={cat.key} className="mt-8">
              <div className="flex items-center gap-2 mb-3">
                <span
                  className="w-6 h-6 rounded-md flex items-center justify-center"
                  style={{ backgroundColor: cat.color + '22', color: cat.color }}
                >
                  <Icon size={13} />
                </span>
                <h2 className="text-[13.5px] font-medium text-neutral-100">{cat.label}</h2>
                <span className="text-[11px] text-neutral-600">{items.length}</span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {items.map((l) => (
                  <LinkCard key={l.id} link={l} fallbackColor={cat.color} />
                ))}
              </div>
            </section>
          )
        })}
      </div>
    </div>
  )
}

// LinkCard 单个平台卡片：展示 + 双击进入编辑态（管理员）
function LinkCard({ link, fallbackColor }: { link: SocialLink; fallbackColor: string }) {
  const isAdmin = useApp((s) => s.isAdmin)
  const saveSocialLink = useApp((s) => s.saveSocialLink)
  const deleteSocialLink = useApp((s) => s.deleteSocialLink)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<SocialLink>(link)
  const [hint, setHint] = useState('')

  const color = BRAND[link.id] || fallbackColor
  const initial = (link.platform || '?').trim().charAt(0).toUpperCase()
  const openable = /^https?:\/\//i.test(link.url)

  const startEdit = () => {
    if (!isAdmin) {
      setHint('双击编辑需先登录管理员')
      setTimeout(() => setHint(''), 1800)
      return
    }
    setDraft(link)
    setEditing(true)
  }

  const commit = async () => {
    const ok = await saveSocialLink(link.id, {
      platform: draft.platform,
      category: draft.category,
      account: draft.account,
      url: draft.url,
      note: draft.note,
    })
    if (ok) setEditing(false)
  }

  if (editing) {
    return (
      <div className="rounded-xl border border-accent/30 bg-white/[0.04] p-3.5">
        <input value={draft.platform} onChange={(e) => setDraft({ ...draft, platform: e.target.value })} placeholder="平台名" className={`w-full mb-2 ${INPUT}`} />
        <div className="grid grid-cols-2 gap-2 mb-2">
          <input value={draft.account} onChange={(e) => setDraft({ ...draft, account: e.target.value })} placeholder="账号 / 昵称" className={INPUT} />
          <select value={draft.category} onChange={(e) => setDraft({ ...draft, category: e.target.value })} className={`${INPUT} bg-ink-900`}>
            {CATEGORIES.map((c) => (
              <option key={c.key} value={c.key}>
                {c.label}
              </option>
            ))}
          </select>
        </div>
        <input value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} placeholder="主页链接 https://…" className={`w-full mb-2 ${INPUT}`} />
        <input
          value={draft.note}
          onChange={(e) => setDraft({ ...draft, note: e.target.value })}
          placeholder="一句话说明"
          className={`w-full mb-2.5 ${INPUT}`}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void commit()
            if (e.key === 'Escape') setEditing(false)
          }}
        />
        <div className="flex items-center gap-2">
          <button onClick={() => void commit()} className="flex items-center gap-1 px-2.5 py-1 rounded-md bg-accent text-ink-950 text-[12px] font-medium hover:bg-accent-dim">
            <Check size={13} /> 保存
          </button>
          <button onClick={() => setEditing(false)} className="flex items-center gap-1 px-2 py-1 rounded-md text-neutral-400 hover:bg-white/5 text-[12px]">
            <X size={13} /> 取消
          </button>
          <span className="flex-1" />
          <button onClick={() => deleteSocialLink(link.id)} title="删除" className="p-1 rounded-md text-neutral-500 hover:text-red-400 hover:bg-white/5">
            <Trash2 size={13} />
          </button>
        </div>
      </div>
    )
  }

  return (
    <div
      onDoubleClick={startEdit}
      title={isAdmin ? '双击编辑' : '双击可编辑（管理员）'}
      className="group rounded-xl border border-white/5 bg-white/[0.02] hover:bg-white/[0.045] hover:border-white/10 p-3.5 transition-colors cursor-default"
    >
      <div className="flex items-start gap-3">
        <span
          className="w-9 h-9 rounded-lg flex items-center justify-center text-[15px] font-semibold text-white shrink-0"
          style={{ backgroundColor: color }}
        >
          {initial}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <span className="text-[13px] text-neutral-100 truncate">{link.platform}</span>
            {openable && (
              <a href={link.url} target="_blank" rel="noreferrer" className="text-neutral-500 hover:text-accent" title="打开链接">
                <ExternalLink size={12} />
              </a>
            )}
            {isAdmin && <Pencil size={11} className="text-neutral-600 opacity-0 group-hover:opacity-100" />}
          </div>
          <div className="text-[12px] text-neutral-400 truncate">
            {link.account || <span className="text-neutral-600">未设置账号</span>}
          </div>
        </div>
      </div>
      {link.note && <div className="mt-2 text-[11px] text-neutral-500 line-clamp-1">{link.note}</div>}
      {link.url && !openable && <div className="mt-1 text-[11px] text-neutral-600 truncate">{link.url}</div>}
      {hint && <div className="mt-2 text-[11px] text-accent">{hint}</div>}
    </div>
  )
}

// AddCard 新增平台（管理员）
function AddCard({ onClose }: { onClose: () => void }) {
  const addSocialLink = useApp((s) => s.addSocialLink)
  const [draft, setDraft] = useState({ platform: '', category: 'community', account: '', url: '', note: '' })
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!draft.platform.trim()) return
    setBusy(true)
    const ok = await addSocialLink(draft)
    setBusy(false)
    if (ok) onClose()
  }

  return (
    <div className="mt-5 rounded-xl border border-dashed border-accent/30 bg-accent/[0.04] p-3.5">
      <div className="text-[12.5px] text-accent mb-2 flex items-center gap-1.5">
        <Plus size={13} /> 新增平台
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 mb-2">
        <input autoFocus value={draft.platform} onChange={(e) => setDraft({ ...draft, platform: e.target.value })} placeholder="平台名（如 掘金）" className={INPUT} />
        <input value={draft.account} onChange={(e) => setDraft({ ...draft, account: e.target.value })} placeholder="账号 / 昵称" className={INPUT} />
        <select value={draft.category} onChange={(e) => setDraft({ ...draft, category: e.target.value })} className={`${INPUT} bg-ink-900`}>
          {CATEGORIES.map((c) => (
            <option key={c.key} value={c.key}>
              {c.label}
            </option>
          ))}
        </select>
      </div>
      <input value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} placeholder="主页链接 https://…" className={`w-full mb-2 ${INPUT}`} />
      <input
        value={draft.note}
        onChange={(e) => setDraft({ ...draft, note: e.target.value })}
        placeholder="一句话说明"
        className={`w-full mb-2.5 ${INPUT}`}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void submit()
          if (e.key === 'Escape') onClose()
        }}
      />
      <div className="flex items-center gap-2">
        <button
          onClick={() => void submit()}
          disabled={busy || !draft.platform.trim()}
          className="px-3 py-1 rounded-md bg-accent text-ink-950 text-[12px] font-medium hover:bg-accent-dim disabled:opacity-40"
        >
          {busy ? '保存中…' : '添加'}
        </button>
        <button onClick={onClose} className="px-2 py-1 rounded-md text-neutral-400 hover:bg-white/5 text-[12px]">
          取消
        </button>
      </div>
    </div>
  )
}
