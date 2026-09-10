import { FileText } from 'lucide-react'
import type { Message } from '../../types'

export default function MessageBubble({ message: m }: { message: Message }) {
  const isUser = m.role === 'user'

  if (isUser) {
    return (
      <div className="fade-in flex justify-end">
        <div className="max-w-[80%] rounded-2xl rounded-tr-sm bg-white/10 px-4 py-2.5 text-[13px] text-neutral-100 whitespace-pre-wrap">
          {m.content}
        </div>
      </div>
    )
  }

  return (
    <div className="fade-in flex flex-col gap-2">
      <div className="flex items-center gap-1.5 pb-1">
        <div className="w-8 h-8 logo-glyph rounded-full border border-white/10 flex items-center justify-center text-[10px] font-bold text-neutral-300">
          JL
        </div>
        <div className="text-[12px] text-neutral-400">JasperLee</div>
      </div>

      <div className="max-w-[88%] text-[13.5px] leading-relaxed text-neutral-100 whitespace-pre-wrap">
        {m.content || <span className="cursor-blink">▊</span>}
      </div>

      {/* 消息引用文件 */}
      {m.refs && m.refs.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {m.refs.map((r) => (
            <span key={r} className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-white/5 border border-white/10 text-[11px] text-accent">
              <FileText size={11} />
              {r}
            </span>
          ))}
        </div>
      )}

      {/* Response details（对应截图三） */}
      {m.details && <ResponseDetails d={m.details} />}
    </div>
  )
}

function ResponseDetails({
  d,
}: {
  d: NonNullable<Message['details']>
}) {
  return (
    <div className="mt-1 max-w-[420px] rounded-lg border border-white/10 bg-ink-900/80 p-2.5">
      <div className="flex items-center gap-1.5 mb-2">
        <span className="text-[11px] font-medium text-neutral-300">Response details</span>
        <div className="flex-1" />
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[11px]">
        <Row label="状态" value={d.status} />
        <Row label="模型" value={d.model} mono />
        <Row label="Agent 类型" value={d.agentType ?? '-'} mono />
        <Row label="耗时" value={`${(d.elapsedMs / 1000).toFixed(1)}s`} />
        <Row label="输入 token" value={String(d.inputTokens)} mono />
        <Row label="输出 token" value={String(d.outputTokens)} mono />
      </div>
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-neutral-500">{label}</span>
      <span className={`text-neutral-200 ${mono ? 'font-mono' : ''}`}>{value}</span>
    </div>
  )
}
