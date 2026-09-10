import { useEffect, useRef, useState } from 'react'
import { Upload, FileText, Download, Trash2, Inbox } from 'lucide-react'
import { useApp } from '../../store'

// 收件箱：招聘者上传文件给我（JD / 招聘资料 / 问题清单）
export default function InboxView() {
  const inbox = useApp((s) => s.inbox)
  const refreshInbox = useApp((s) => s.refreshInbox)
  const uploadInbox = useApp((s) => s.uploadInbox)
  const deleteInboxItem = useApp((s) => s.deleteInboxItem)

  const [name, setName] = useState('')
  const [note, setNote] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    refreshInbox()
  }, [refreshInbox])

  const doUpload = async () => {
    if (!file) {
      setMsg('请先选择文件')
      return
    }
    setBusy(true)
    const ok = await uploadInbox(file, name || '招聘者', note)
    setBusy(false)
    setMsg(ok ? '已送达收件箱 ✅' : '上传失败，请重试')
    if (ok) {
      setFile(null)
      setNote('')
      if (inputRef.current) inputRef.current.value = ''
    }
  }

  return (
    <div className="flex-1 min-h-0 overflow-y-auto thin-scroll">
      <div className="max-w-3xl mx-auto px-6 py-8">
        <h1 className="text-lg font-medium text-neutral-100 flex items-center gap-2">
          <Inbox size={18} className="text-accent" /> 上传给我
        </h1>
        <p className="mt-1 text-[12.5px] text-neutral-500">
          作为招聘者，你可以把 JD、公司介绍、招聘资料或问题清单上传给我，我会在收件箱看到并尽快回复。
        </p>

        {/* 上传区 */}
        <div className="mt-5 rounded-xl border border-dashed border-white/15 bg-white/[0.02] p-5">
          <label className="flex flex-col items-center gap-2 cursor-pointer">
            <input
              ref={inputRef}
              type="file"
              className="hidden"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
            <Upload size={20} className="text-neutral-400" />
            <span className="text-[13px] text-neutral-200">{file ? file.name : '点击选择文件（支持 PDF / Word / 图片 / 文本）'}</span>
          </label>

          <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-3">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="你的署名（公司 / 姓名）"
              className="bg-transparent border border-white/10 rounded-lg px-3 py-2 text-[12.5px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none focus:border-accent/50"
            />
            <input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="留言（如：想了解你对我们岗位的看法）"
              className="bg-transparent border border-white/10 rounded-lg px-3 py-2 text-[12.5px] text-neutral-100 placeholder:text-neutral-600 focus:outline-none focus:border-accent/50"
            />
          </div>

          <button
            onClick={doUpload}
            disabled={busy || !file}
            className="mt-4 px-5 py-2 rounded-lg bg-accent text-ink-950 text-[13px] font-medium disabled:opacity-40 hover:bg-accent-dim transition-colors"
          >
            {busy ? '上传中...' : '上传给我'}
          </button>
          {msg && <div className="mt-2 text-[12px] text-neutral-400">{msg}</div>}
        </div>

        {/* 收件箱列表 */}
        <h2 className="mt-8 mb-3 text-[14px] font-medium text-neutral-100">收件箱</h2>
        {inbox.length === 0 ? (
          <div className="text-[12.5px] text-neutral-500">暂无投递。</div>
        ) : (
          <div className="space-y-2">
            {inbox.map((it) => (
              <div key={it.id} className="flex items-center gap-3 rounded-lg border border-white/5 bg-white/[0.02] px-3 py-2.5">
                <FileText size={15} className="text-neutral-500 shrink-0" />
                <div className="min-w-0 flex-1">
                  <div className="text-[13px] text-neutral-100 truncate">{it.fileName}</div>
                  <div className="text-[11px] text-neutral-500">
                    {it.name || '招聘者'} · {fmtSize(it.size)} · {fmtTime(it.createdAt)}
                    {it.note && ` · ${it.note}`}
                  </div>
                </div>
                <button
                  onClick={() => download(it.id)}
                  className="p-1.5 text-neutral-400 hover:text-neutral-100 hover:bg-white/5 rounded-md"
                  title="下载"
                >
                  <Download size={14} />
                </button>
                <button
                  onClick={() => deleteInboxItem(it.id)}
                  className="p-1.5 text-neutral-400 hover:text-red-400 hover:bg-white/5 rounded-md"
                  title="删除"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

async function download(id: string) {
  const { api } = await import('../../api')
  try {
    const blob = await api.downloadInbox(id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'download'
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    /* ignore */
  }
}

function fmtSize(n: number) {
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
