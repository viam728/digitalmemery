import { useEffect, useState } from 'react'
import { X, FileText, Search } from 'lucide-react'
import { useApp } from '../../store'
import { WorkspaceFileTree } from '../Workspace/WorkspaceView'
import { api } from '../../api'
import type { FileMeta } from '../../types'

export default function RightPanel() {
  const open = useApp((s) => s.rightPanelOpen)
  const mode = useApp((s) => s.rightPanelMode)
  const setRightPanel = useApp((s) => s.setRightPanel)
  const ws = useApp((s) => s.activeWorkspace)
  const ragHits = useApp((s) => s.ragHits)
  const previewFile = useApp((s) => s.previewFile)
  const previewContent = useApp((s) => s.previewContent)

  if (!open) return null

  return (
    <aside className="w-72 shrink-0 border-l border-white/5 bg-ink-900/70 flex flex-col fade-in">
      <div className="flex items-center gap-2 px-3 h-9 border-b border-white/5 shrink-0">
        <FileText size={14} className="text-neutral-400" />
        <span className="text-[12px] text-neutral-300 font-medium">{mode === 'preview' ? '预览' : '文件'}</span>
        <div className="flex-1" />
        <button onClick={() => setRightPanel(false)} className="p-1 text-neutral-500 hover:text-neutral-100 rounded">
          <X size={13} />
        </button>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto thin-scroll">
        {mode === 'files' && ws && <WorkspaceFileTree files={ws.files} />}

        {mode === 'files' && !ws && (
          <div className="p-4 text-[12px] text-neutral-500">选择一个 Agent 工作区以查看其引用的文件。</div>
        )}

        {mode === 'preview' && <PreviewPanel file={previewFile} content={previewContent} />}

        {mode === 'rag' && (
          <div className="p-3">
            <div className="flex items-center gap-1.5 mb-2 text-[12px] text-neutral-300">
              <Search size={13} />
              RAG 检索结果
            </div>
            {ragHits.length === 0 ? (
              <div className="text-[12px] text-neutral-500">暂无命中，先在资料库中入库文档。</div>
            ) : (
              ragHits.map((h, i) => (
                <div key={i} className="mb-2 rounded-lg border border-white/5 p-2.5">
                  <div className="text-[11px] text-accent mb-1">📄 {h.fileName}</div>
                  <p className="text-[12px] text-neutral-300 leading-relaxed line-clamp-3">{h.chunk}</p>
                  <div className="mt-1 text-[10px] text-neutral-500">相关性 {h.score.toFixed(2)}</div>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </aside>
  )
}

// PreviewPanel 按文件 kind 渲染预览：文本类用 <pre>，图片用 <img>，PDF 用 iframe，Office 占位
function PreviewPanel({ file, content }: { file: FileMeta | null; content: string }) {
  const [blobUrl, setBlobUrl] = useState('')

  useEffect(() => {
    setBlobUrl('')
    if (!file) return
    if (file.kind === 'image' || file.kind === 'pdf') {
      let alive = true
      api.getFileBlob(file.id).then((b) => { if (alive) setBlobUrl(URL.createObjectURL(b)) })
      return () => { alive = false; if (blobUrl) URL.revokeObjectURL(blobUrl) }
    }
  }, [file?.id])

  if (!file) {
    return <div className="p-4 text-[12px] text-neutral-500">点击资料库中的文件以预览。</div>
  }

  if (file.kind === 'image') {
    return blobUrl ? <img src={blobUrl} alt={file.name} className="w-full rounded-lg border border-white/5" /> : <Placeholder text="加载图片中..." />
  }

  if (file.kind === 'pdf' && blobUrl) {
    return <iframe src={blobUrl} title={file.name} className="w-full h-72 rounded-lg border border-white/10 bg-white" />
  }
  if (file.kind === 'pdf') {
    return <Placeholder text="PDF 预览（使用浏览器内嵌查看器）" />
  }
  // docx：后端已抽取正文（content）；解析失败或为空时提示下载
  if (file.kind === 'docx' && !content) {
    return <Placeholder text="Word 文档暂无文本内容（解析失败或为空），可下载后查看。" />
  }

  if (!content) return <Placeholder text="该文件暂无文本内容可预览（未入库或为二进制）。" />

  const isJson = file.kind === 'json'
  const pretty = isJson ? safePretty(content) : content
  return (
    <pre className={`p-3 text-[12px] leading-relaxed whitespace-pre-wrap break-words ${isJson ? 'text-accent' : 'text-neutral-200'} font-mono`}>
      {pretty}
    </pre>
  )
}

function Placeholder({ text }: { text: string }) {
  return <div className="p-4 text-[12px] text-neutral-500">{text}</div>
}

function safePretty(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}
