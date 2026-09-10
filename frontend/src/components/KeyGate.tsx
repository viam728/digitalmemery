import { useState } from 'react'
import { KeyRound, Sparkles, ShieldCheck } from 'lucide-react'
import { useApp } from '../store'

// 免登录 Key 门控：访客申请临时 Key 或粘贴已有 Key，解锁 AI 与资料库。
export default function KeyGate() {
  const applyKey = useApp((s) => s.applyKey)
  const setKey = useApp((s) => s.setKey)
  const [inputKey, setInputKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const adminLogin = useApp((s) => s.adminLogin)
  const [adminOpen, setAdminOpen] = useState(false)
  const [adminPwd, setAdminPwd] = useState('')
  const [adminErr, setAdminErr] = useState('')

  const doAdminLogin = async () => {
    const ok = await adminLogin(adminPwd)
    if (ok) setAdminOpen(false)
    else setAdminErr('密码错误')
  }

  const doApply = async () => {
    setBusy(true)
    setError('')
    const ok = await applyKey('访客')
    setBusy(false)
    if (!ok) setError('申请失败，请确认后端已启动')
  }

  return (
    <div className="flex-1 min-h-0 flex items-center justify-center bg-ink-950">
      <div className="w-full max-w-md px-6">
        <div className="flex flex-col items-center gap-3 mb-7">
          <div className="w-12 h-12 logo-glyph rounded-xl border border-white/10 flex items-center justify-center">
            <KeyRound size={22} className="text-accent" />
          </div>
          <div className="text-[16px] text-neutral-200 font-medium">JasperLee · 李俊锋的数字分身</div>
          <div className="text-[12px] text-neutral-500">招聘者免登录 · 一键申请临时 Key · 即可提问、看资料、上传文件给我</div>
        </div>

        <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-white/[0.05] border border-white/10">
            <span className="w-7 h-7 rounded-lg bg-accent/15 flex items-center justify-center"><Sparkles size={15} className="text-accent" /></span>
            <span className="text-[12.5px] text-neutral-100">申请新 Key（100 万 token 额度）</span>
          </div>

          <button
            onClick={doApply}
            disabled={busy}
            className="mt-3 w-full px-4 py-2.5 rounded-lg bg-accent text-ink-950 text-[13px] font-medium disabled:opacity-40 disabled:cursor-not-allowed hover:bg-accent-dim transition-colors"
          >
            {busy ? '申请中...' : '立即申请'}
          </button>

          <div className="mt-3 flex items-center gap-2">
            <div className="h-px flex-1 bg-white/10" />
            <span className="text-[11px] text-neutral-600">或粘贴已有 Key</span>
            <div className="h-px flex-1 bg-white/10" />
          </div>

          <input
            value={inputKey}
            onChange={(e) => setInputKey(e.target.value)}
            placeholder="粘贴已有 Key（可选）"
            className="w-full bg-transparent border border-white/10 rounded-lg px-3 py-2 mt-2 text-[12.5px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none focus:border-accent/50"
          />
          <button
            onClick={() => inputKey.trim() && setKey(inputKey.trim())}
            disabled={!inputKey.trim()}
            className="mt-3 w-full px-4 py-2.5 rounded-lg bg-white/[0.06] hover:bg-white/[0.1] border border-white/10 text-[12px] text-neutral-200 disabled:opacity-30 transition-colors"
          >
            进入 JasperLee
          </button>

          {error && <div className="mt-2 text-[12px] text-red-400">{error}</div>}
        </div>

        <p className="mt-5 text-center text-[11px] text-neutral-600">
          Key 仅用于计量 token 用量与控制资料库访问权限，无需注册登录。
        </p>

        <button
          onClick={() => { setAdminOpen(true); setAdminErr('') }}
          className="mt-6 mx-auto flex items-center gap-1.5 text-[11px] text-neutral-500 hover:text-accent transition-colors"
        >
          <ShieldCheck size={12} /> 管理员入口
        </button>
      </div>

      {/* 管理员密码弹窗 */}
      {adminOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setAdminOpen(false)}>
          <div className="w-80 rounded-xl border border-white/10 bg-ink-800 p-5 fade-in" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-2 text-[14px] font-medium text-neutral-100">
              <ShieldCheck size={16} className="text-accent" /> 管理员登录
            </div>
            <input
              type="password"
              value={adminPwd}
              onChange={(e) => setAdminPwd(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && doAdminLogin()}
              placeholder="请输入管理员密码"
              autoFocus
              className="mt-3 w-full bg-transparent border border-white/10 rounded-lg px-3 py-2 text-[13px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none focus:border-accent/50"
            />
            {adminErr && <div className="mt-2 text-[12px] text-red-400">{adminErr}</div>}
            <div className="mt-3 flex gap-2">
              <button onClick={() => setAdminOpen(false)} className="flex-1 px-3 py-2 rounded-lg bg-white/[0.06] text-[12.5px] text-neutral-300 hover:bg-white/10">取消</button>
              <button onClick={doAdminLogin} className="flex-1 px-3 py-2 rounded-lg bg-accent text-ink-950 text-[12.5px] font-medium hover:bg-accent-dim">进入</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
