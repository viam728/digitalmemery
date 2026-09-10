import { useEffect } from 'react'
import { MapPin, Sparkles, MessageCircle, Upload, ArrowUpRight, FileDown, Copy } from 'lucide-react'
import { useApp } from '../../store'

// 个人主页：给招聘者看的数字分身资料
export default function AvatarView() {
  const avatar = useApp((s) => s.avatar)
  const refreshAvatar = useApp((s) => s.refreshAvatar)
  const setView = useApp((s) => s.setView)

  useEffect(() => {
    refreshAvatar()
  }, [refreshAvatar])

  const a = avatar

  // 真实后端未返回主页数据时显示加载态（不再用写死 mock 数据冒充）
  if (!a) {
    return (
      <div className="flex-1 min-h-0 flex flex-col items-center justify-center gap-3 text-neutral-500">
        <div className="w-12 h-12 rounded-2xl logo-glyph border border-white/10 flex items-center justify-center text-lg font-bold text-neutral-400">JL</div>
        <p className="text-[13px]">正在加载李俊锋的个人主页…</p>
      </div>
    )
  }

  return (
    <div className="flex-1 min-h-0 overflow-y-auto thin-scroll">
      <div className="max-w-3xl mx-auto px-6 py-8">
        {/* 头部 */}
        <div className="flex items-center gap-5">
          <div className="w-20 h-20 rounded-2xl logo-glyph border border-white/10 flex items-center justify-center text-2xl font-bold text-neutral-300">
            JL
          </div>
          <div>
            <h1 className="text-2xl font-semibold text-neutral-100">{a.name}</h1>
            <p className="text-[13px] text-accent mt-1">{a.role}</p>
            <p className="text-[12px] text-neutral-500 mt-1 flex items-center gap-1">
              <MapPin size={12} /> {a.location} · {a.mbti} · {a.age} 岁
            </p>
          </div>
        </div>

        {/* 自我介绍 */}
        <p className="mt-6 text-[13.5px] leading-relaxed text-neutral-300">{a.about}</p>

        {/* 技能 */}
        <div className="mt-6 flex flex-wrap gap-2">
          {a.skills.map((s) => (
            <span key={s} className="px-3 py-1 rounded-full border border-white/10 text-[12px] text-neutral-300">
              {s}
            </span>
          ))}
        </div>

        {/* 招聘者行动 */}
        <div className="mt-7 grid grid-cols-2 gap-3">
          <button
            onClick={() => setView('chat')}
            className="flex items-center gap-3 rounded-xl border border-accent/30 bg-accent/10 px-4 py-3 text-left hover:bg-accent/20 transition-colors"
          >
            <MessageCircle size={18} className="text-accent" />
            <div>
              <div className="text-[13px] text-neutral-100">向 JasperLee 提问</div>
              <div className="text-[11px] text-neutral-500">了解我的经历、技能与项目</div>
            </div>
          </button>
          <button
            onClick={() => setView('inbox')}
            className="flex items-center gap-3 rounded-xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left hover:bg-white/[0.06] transition-colors"
          >
            <Upload size={18} className="text-neutral-300" />
            <div>
              <div className="text-[13px] text-neutral-100">上传资料给我</div>
              <div className="text-[11px] text-neutral-500">JD / 招聘资料 / 问题清单</div>
            </div>
          </button>
          <button
            onClick={() => downloadResume()}
            className="flex items-center gap-3 rounded-xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left hover:bg-white/[0.06] transition-colors"
          >
            <FileDown size={18} className="text-neutral-300" />
            <div>
              <div className="text-[13px] text-neutral-100">下载简历 PDF</div>
              <div className="text-[11px] text-neutral-500">完整版简历（李俊锋.pdf）</div>
            </div>
          </button>
        </div>

        {/* 联系方式 */}
        {a.contact && a.contact.length > 0 && (
          <div className="mt-4 flex flex-wrap items-center gap-3 rounded-xl border border-white/10 bg-white/[0.02] px-4 py-3">
            {a.contact.map((c) => (
              <button
                key={c}
                onClick={() => { navigator.clipboard?.writeText(c.replace(/^[^\s]*\s/, '')); }}
                title="点击复制"
                className="flex items-center gap-1.5 text-[12.5px] text-neutral-200 hover:text-accent"
              >
                <Copy size={12} className="text-neutral-500" /> {c}
              </button>
            ))}
          </div>
        )}

        {/* 项目作品 */}
        <h2 className="mt-8 mb-3 text-[15px] font-medium text-neutral-100 flex items-center gap-2">
          <Sparkles size={15} className="text-accent" /> 项目作品
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {a.projects.map((p) => (
            <div key={p.name} className="rounded-xl border border-white/10 bg-white/[0.02] p-4 hover:bg-white/[0.05] transition-colors">
              <div className="flex items-center justify-between">
                <span className="text-[13px] text-neutral-100">{p.name}</span>
                <ArrowUpRight size={13} className="text-neutral-600" />
              </div>
              <p className="mt-1.5 text-[12px] text-neutral-400 leading-relaxed">{p.desc}</p>
              <div className="mt-2.5 flex flex-wrap gap-1">
                {p.stack.map((t) => (
                  <span key={t} className="px-1.5 py-0.5 rounded bg-white/5 text-[10px] text-neutral-400">{t}</span>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* 时间线 + 简历要点 */}
        <div className="mt-8 grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <h3 className="text-[14px] font-medium text-neutral-100 mb-3">成长时间线</h3>
            <div className="space-y-2.5">
              {a.timeline.map((t) => (
                <div key={t.year} className="flex gap-3">
                  <span className="w-12 shrink-0 text-[12px] text-accent">{t.year}</span>
                  <span className="text-[12.5px] text-neutral-300">{t.text}</span>
                </div>
              ))}
            </div>
          </div>
          <div>
            <h3 className="text-[14px] font-medium text-neutral-100 mb-3">简历要点</h3>
            <ul className="space-y-2">
              {a.resume.map((r) => (
                <li key={r} className="text-[12.5px] text-neutral-300 flex gap-2">
                  <span className="text-accent">·</span> {r}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}

// 下载资料库中的简历 PDF（f6 = 李俊锋.pdf）
async function downloadResume() {
  try {
    const { api } = await import('../../api')
    const blob = await api.downloadFile('f6')
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = '李俊锋.pdf'
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    /* ignore */
  }
}

