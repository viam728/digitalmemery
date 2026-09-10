import { useEffect, useState } from 'react'
import { Folder, FileText, FileCode2, Bot, Plus, Play, Loader2 } from 'lucide-react'
import { useApp } from '../../store'
import type { WorkspaceFile } from '../../types'

const PROMPT_HINT = '描述你要让 Agent 做的事，例如：总结引用文件的要点并输出计划…'

export default function WorkspaceView() {
  const loadWorkspace = useApp((s) => s.loadWorkspace)
  const activeId = useApp((s) => s.activeConversationId)
  const createWorkspace = useApp((s) => s.createWorkspace)
  const runWorkspace = useApp((s) => s.runWorkspace)
  const ws = useApp((s) => s.activeWorkspace)
  // 运行输入框默认留空，不再预填 AGENT 模式话术
  const [prompt, setPrompt] = useState('')

  useEffect(() => {
    if (activeId) loadWorkspace(activeId)
  }, [activeId, loadWorkspace])

  const handleNew = () => {
    createWorkspace('新任务-' + new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }))
  }

  const handleRun = () => {
    if (!ws || !prompt.trim()) return
    runWorkspace(ws.id, prompt.trim())
  }

  const wsFiles = ws?.files ?? []
  const running = ws?.status === 'running'

  return (
    <div className="flex-1 min-h-0 flex">
      {/* 工作区会话列表 */}
      <div className="w-56 shrink-0 border-r border-white/5 bg-ink-900/50 flex flex-col">
        <div className="p-2.5">
          <button
            onClick={handleNew}
            className="w-full flex items-center gap-2 px-2.5 py-2 rounded-lg bg-white/[0.05] hover:bg-white/[0.09] border border-white/10 text-[13px] text-neutral-200"
          >
            <Plus size={14} className="text-accent" />
            新任务
          </button>
        </div>
        <div className="px-2 text-[10px] uppercase tracking-wide text-neutral-500 pb-1">现有</div>
        <div className="px-2 space-y-0.5">
          <button className="w-full flex items-center gap-2 px-2 py-1.5 rounded-lg bg-white/10 text-neutral-100 text-[12.5px]">
            <Bot size={14} className="text-accent" />
            {ws?.title || 'greeting'}
          </button>
        </div>
      </div>

      {/* 任务主区 */}
      <div className="flex-1 min-w-0 flex flex-col">
        <div className="px-4 pt-3 pb-2 border-b border-white/5 flex items-center gap-2">
          <span className="text-[12px] text-neutral-500">{ws?.title || 'greeting'}</span>
          {ws?.status && (
            <span className={`px-1.5 py-0.5 rounded text-[10px] border ${ws.status === 'done' ? 'text-accent border-accent/30' : ws.status === 'running' ? 'text-amber-300 border-amber-300/30' : 'text-neutral-400 border-white/10'}`}>
              {ws.status === 'running' ? '运行中' : ws.status === 'done' ? '已完成' : ws.status === 'error' ? '出错' : '待命'}
            </span>
          )}
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto thin-scroll px-5 py-4">
          {wsFiles.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center gap-3 text-center">
              <div className="w-12 h-12 logo-glyph rounded-full border border-white/10 flex items-center justify-center">
                <Bot size={22} className="text-accent" />
              </div>
              <p className="text-[13px] text-neutral-400">创建一个新任务，或运行当前工作区以生成产出文件。</p>
            </div>
          ) : (
            <WorkspaceTree files={wsFiles} />
          )}

          {ws?.usage && (
            <div className="mt-4 rounded-lg border border-white/10 bg-white/[0.02] p-3">
              <div className="text-[11px] text-neutral-300 mb-2">本次用量</div>
              <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11.5px]">
                <Row label="输入 token" value={String(ws.usage.promptTokens)} />
                <Row label="输出 token" value={String(ws.usage.completionTokens)} />
              </div>
            </div>
          )}
        </div>

        {/* 运行区 */}
        <div className="px-4 pb-4 shrink-0">
          <div className="rounded-xl border border-white/10 bg-ink-850 focus-within:border-white/20 transition-colors p-3">
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={3}
              placeholder={PROMPT_HINT}
              className="w-full bg-transparent resize-none text-[13px] text-neutral-100 focus:outline-none placeholder:text-neutral-500"
            />
            <div className="flex items-center gap-2 mt-1">
              <span className="text-[11px] text-neutral-500">模型 {ws?.model || '当前模型'}</span>
              <div className="flex-1" />
              <button
                onClick={handleRun}
                disabled={running}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-accent text-ink-950 text-[12px] font-medium disabled:opacity-40 disabled:cursor-not-allowed hover:bg-accent-dim transition-colors"
              >
                {running ? <Loader2 size={13} className="animate-spin" /> : <Play size={13} />}
                {running ? '执行中...' : '运行任务'}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-neutral-500">{label}</span>
      <span className="text-neutral-200 font-mono">{value}</span>
    </div>
  )
}

// WorkspaceTree 工作区文件树
function WorkspaceTree({ files }: { files: WorkspaceFile[] }) {
  return (
    <div className="space-y-1">
      <div className="text-[11px] text-neutral-500 mb-2">工作区文件（引用标黄）</div>
      {files.map((f) => (
        <Node key={f.id} node={f} depth={0} />
      ))}
    </div>
  )
}

export function WorkspaceFileTree({ files }: { files: WorkspaceFile[] }) {
  return (
    <div className="px-2 py-2 space-y-0.5">
      {files.map((f) => (
        <Node key={f.id} node={f} depth={0} />
      ))}
    </div>
  )
}

function Node({ node, depth }: { node: WorkspaceFile; depth: number }) {
  const Icon = node.kind === 'folder' ? Folder : node.kind === 'code' ? FileCode2 : FileText
  const indent = { paddingLeft: `${depth * 14 + 6}px` }
  return (
    <div>
      <div
        className={`flex items-center gap-1.5 py-[3px] rounded pr-2 cursor-pointer group text-[12px] ${node.referenced ? 'text-accent' : 'text-neutral-300 hover:text-neutral-100'}`}
        style={indent}
        title={node.content ? node.content.slice(0, 120) : node.path}
      >
        <span className="w-4" />
        <Icon size={14} className="text-neutral-500 shrink-0" />
        <span className="truncate font-mono">{node.name}</span>
        {node.referenced && <span className="ml-auto text-[9px] text-accent/80">已引用</span>}
      </div>
      {node.children?.map((c) => <Node key={c.id} node={c} depth={depth + 1} />)}
    </div>
  )
}