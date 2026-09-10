import { useEffect, useState } from 'react'
import { LayoutDashboard, KeyRound, Inbox, LogOut, RefreshCw, Power, Trash2 } from 'lucide-react'
import { useApp } from '../../store'

// 管理员视图：看板 + Key 申请管理 + 收件箱消息
export default function AdminView() {
  const stats = useApp((s) => s.adminStats)
  const keys = useApp((s) => s.adminKeys)
  const inbox = useApp((s) => s.inbox)
  const refreshAdmin = useApp((s) => s.refreshAdmin)
  const refreshInbox = useApp((s) => s.refreshInbox)
  const toggleKey = useApp((s) => s.adminToggleKey)
  const renewKey = useApp((s) => s.adminRenewKey)
  const deleteKey = useApp((s) => s.adminDeleteKey)
  const logout = useApp((s) => s.adminLogout)

  const [tab, setTab] = useState<'board' | 'keys' | 'inbox'>('board')

  useEffect(() => {
    refreshAdmin()
    refreshInbox()
  }, [refreshAdmin, refreshInbox])

  const activeKeys = keys.filter((k) => k.active).length
  const uptime = stats ? fmtUptime(stats.uptimeSeconds) : '--'

  return (
    <div className="flex-1 min-h-0 flex">
      {/* 管理员导航 */}
      <div className="w-44 shrink-0 border-r border-white/5 bg-ink-900/50 flex flex-col">
        <div className="p-3 text-[12px] text-accent font-medium">管理员系统</div>
        <div className="px-2 space-y-0.5">
          <TabBtn active={tab === 'board'} onClick={() => setTab('board')}><LayoutDashboard size={15} /> 系统看板</TabBtn>
          <TabBtn active={tab === 'keys'} onClick={() => setTab('keys')}><KeyRound size={15} /> Key 申请</TabBtn>
          <TabBtn active={tab === 'inbox'} onClick={() => setTab('inbox')}><Inbox size={15} /> 收件箱消息</TabBtn>
        </div>
        <div className="flex-1" />
        <div className="p-2">
          <button onClick={logout} className="w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-[12px] text-neutral-400 hover:text-red-400 hover:bg-white/5">
            <LogOut size={14} /> 退出管理
          </button>
        </div>
      </div>

      {/* 主区 */}
      <div className="flex-1 min-w-0 overflow-y-auto thin-scroll">
        <div className="max-w-3xl mx-auto px-6 py-6">
          {tab === 'board' && (
            <section>
              <h1 className="text-[16px] font-medium text-neutral-100 flex items-center gap-2">
                <LayoutDashboard size={17} className="text-accent" /> 系统状态看板
              </h1>
              <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-3">
                <StatCard label="运行时长" value={uptime} />
                <StatCard label="对话模型" value={stats?.llm ?? '--'} mono />
                <StatCard label="RAG 向量" value={stats?.rag ?? '--'} mono />
                <StatCard label="活跃 Key" value={`${activeKeys}/${keys.length}`} />
                <StatCard label="文件数" value={String(stats?.counts.files ?? 0)} />
                <StatCard label="会话/消息" value={`${stats?.counts.conversations ?? 0} / ${stats?.counts.messages ?? 0}`} />
                <StatCard label="收件箱" value={String(inbox.length)} />
                <StatCard label="Token 消耗" value={fmtTokens(stats?.tokens.totalUsed)} />
              </div>
              <div className="mt-5 rounded-xl border border-white/10 bg-white/[0.02] p-4">
                <div className="flex justify-between text-[12px] text-neutral-300 mb-2">
                  <span>总额度 {fmtTokens(stats?.tokens.totalQuota)}</span>
                  <span>已用 {fmtTokens(stats?.tokens.totalUsed)} · {(stats?.tokens.usedRate ?? 0).toFixed(2)}%</span>
                </div>
                <div className="h-2 rounded-full bg-white/10 overflow-hidden">
                  <div className="h-full bg-accent transition-all" style={{ width: `${Math.min(100, stats?.tokens.usedRate ?? 0)}%` }} />
                </div>
              </div>
            </section>
          )}

          {tab === 'keys' && (
            <section>
              <div className="flex items-center justify-between">
                <h1 className="text-[16px] font-medium text-neutral-100 flex items-center gap-2">
                  <KeyRound size={17} className="text-accent" /> Key 申请管理
                </h1>
                <button onClick={refreshAdmin} className="flex items-center gap-1 px-2 py-1 rounded-lg text-[11px] text-neutral-400 hover:bg-white/5">
                  <RefreshCw size={13} /> 刷新
                </button>
              </div>
              <p className="mt-1 text-[12px] text-neutral-500">访客申请后在此查看/启用/停用/重置额度/删除。</p>
              <div className="mt-4 rounded-lg border border-white/5 overflow-hidden">
                <div className="grid grid-cols-[1fr_90px_80px_90px_64px] gap-2 px-3 py-2 bg-white/[0.03] text-[11px] text-neutral-500">
                  <span>标签 / Key</span><span>额度</span><span>已用</span><span>状态</span><span className="text-right">操作</span>
                </div>
                {keys.map((k) => (
                  <div key={k.key} className="grid grid-cols-[1fr_90px_80px_90px_64px] gap-2 px-3 py-2 border-t border-white/5 items-center text-[12px]">
                    <div className="min-w-0">
                      <div className="text-neutral-100 truncate">{k.label || '访客'}</div>
                      <div className="text-[10px] text-neutral-500 font-mono truncate">{k.key}</div>
                    </div>
                    <span className="text-neutral-300">{k.quota > 0 ? fmtTokens(k.quota) : '不限'}</span>
                    <span className="text-neutral-300">{fmtTokens(k.used)}</span>
                    <span className={k.active ? 'text-accent' : 'text-red-400'}>{k.active ? '启用' : '停用'}</span>
                    <span className="flex justify-end gap-1">
                      <button onClick={() => toggleKey(k.key, !k.active)} title={k.active ? '停用' : '启用'} className="p-1 text-neutral-400 hover:text-accent rounded">
                        <Power size={13} />
                      </button>
                      <button onClick={() => renewKey(k.key)} title="重置额度" className="p-1 text-neutral-400 hover:text-accent rounded">
                        <RefreshCw size={13} />
                      </button>
                      <button onClick={() => deleteKey(k.key)} title="删除" className="p-1 text-neutral-400 hover:text-red-400 rounded">
                        <Trash2 size={13} />
                      </button>
                    </span>
                  </div>
                ))}
              </div>
            </section>
          )}

          {tab === 'inbox' && (
            <section>
              <h1 className="text-[16px] font-medium text-neutral-100 flex items-center gap-2">
                <Inbox size={17} className="text-accent" /> 收件箱消息
              </h1>
              <p className="mt-1 text-[12px] text-neutral-500">招聘者上传的 JD / 资料 / 问题清单。</p>
              <div className="mt-4 space-y-2">
                {inbox.length === 0 ? (
                  <div className="text-[12.5px] text-neutral-500">暂无投递。</div>
                ) : (
                  inbox.map((it) => (
                    <div key={it.id} className="rounded-lg border border-white/5 bg-white/[0.02] px-3 py-2.5">
                      <div className="flex items-center justify-between">
                        <span className="text-[13px] text-neutral-100">{it.fileName}</span>
                        <span className="text-[11px] text-neutral-500">{fmtTime(it.createdAt)}</span>
                      </div>
                      <div className="mt-1 text-[12px] text-neutral-400">署名：{it.name || '招聘者'} · {fmtBytes(it.size)}</div>
                      {it.note && <div className="mt-1 text-[12px] text-neutral-300">留言：{it.note}</div>}
                    </div>
                  ))
                )}
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  )
}

function TabBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-[12.5px] ${active ? 'bg-white/10 text-neutral-100' : 'text-neutral-400 hover:bg-white/5 hover:text-neutral-100'}`}>
      {children}
    </button>
  )
}

function StatCard({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-white/10 bg-white/[0.02] px-4 py-3">
      <div className="text-[11px] text-neutral-500">{label}</div>
      <div className={`mt-1 text-[15px] text-neutral-100 ${mono ? 'font-mono' : ''}`}>{value}</div>
    </div>
  )
}

function fmtUptime(s: number) {
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.floor(s / 60)}m${s % 60}s`
  return `${(s / 3600).toFixed(1)}h`
}

function fmtTokens(n?: number) {
  if (n === undefined) return '--'
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}k`
  return String(n)
}

function fmtBytes(n: number) {
  if (n > 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  if (n > 1024) return (n / 1024).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtTime(t: string) {
  try {
    return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return t
  }
}