// 资料库：Jasper 的空间 = 只读文件浏览器（简历/个人文档 + 工作区暴露的 Agent 产物）；
// 我分享的 = 招聘者可上传/管理。参照 Cherry 文件页范式：侧栏类型过滤 + 排序表头 +
// 行内改名 + 悬浮操作 + 已选计数条（只读区无上传/改名/删除）。
import { useEffect, useMemo, useState } from 'react'
import {
  Search, Sparkles, FileText, FileCode2, Folder, UploadCloud, Pencil, Download,
  Trash2, Image as ImageIcon, Music, Video, FileQuestion, Package, ChevronUp, ChevronDown, ChevronsUpDown,
} from 'lucide-react'
import { useApp } from '../../store'
import type { FileKind, FileMeta } from '../../types'
import { formatFileSize, getBrowserFileType, getFormatLabel, type BrowserFileType } from './fileDisplay'

/** Jasper 的空间归属：Jasper 所有的只读资料（简历/个人文档/Agent 产物都在此）。 */
function isJasperSpace(f: FileMeta): boolean {
  return f.owner === '李俊锋' || f.path === '我的资料' || f.path === 'Jasper 的空间' || !!f.isArtifact
}

type SidebarFilter =
  | { kind: 'space'; value: 'jasper' | 'shared' }
  | { kind: 'type'; value: BrowserFileType }

type SortKey = 'name' | 'size' | 'updatedAt' | 'type'
type SortDir = 'asc' | 'desc'

const TYPE_ENTRIES: { value: BrowserFileType; label: string; icon: typeof ImageIcon }[] = [
  { value: 'image', label: '图片', icon: ImageIcon },
  { value: 'video', label: '视频', icon: Video },
  { value: 'audio', label: '音频', icon: Music },
  { value: 'text', label: '文本代码', icon: FileCode2 },
  { value: 'document', label: '文档', icon: FileText },
  { value: 'other', label: '其他', icon: FileQuestion },
]

export default function LibraryView() {
  const [filter, setFilter] = useState<SidebarFilter>({ kind: 'space', value: 'jasper' })
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState<SortKey>('updatedAt')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const files = useApp((s) => s.files)
  const artifacts = useApp((s) => s.artifacts)
  const refreshArtifacts = useApp((s) => s.refreshArtifacts)
  const refreshFiles = useApp((s) => s.refreshFiles)
  const ingestFile = useApp((s) => s.ingestFile)
  const openFile = useApp((s) => s.openFile)
  const uploadFile = useApp((s) => s.uploadFile)
  const renameFile = useApp((s) => s.renameFile)
  const deleteFile = useApp((s) => s.deleteFile)
  const setView = useApp((s) => s.setView)

  useEffect(() => {
    refreshArtifacts()
    refreshFiles()
  }, [refreshArtifacts, refreshFiles])

  // Jasper 的空间 = 只读资料 + Agent 产物（同一份存储，产物带标记）；其余归我分享的
  const artifactIds = useMemo(() => new Set(artifacts.map((a) => a.id)), [artifacts])
  const jasperFiles = useMemo(() => {
    const base = files.filter(isJasperSpace)
    const out = [...base]
    for (const a of artifacts) {
      if (!out.some((f) => f.id === a.id)) out.push(a)
    }
    return out
  }, [files, artifacts])
  const sharedFiles = useMemo(
    () => files.filter((f) => !isJasperSpace(f) && !artifactIds.has(f.id)),
    [files, artifactIds],
  )

  const counts = useMemo(() => {
    const c: Record<string, number> = { jasper: jasperFiles.length, shared: sharedFiles.length }
    for (const t of TYPE_ENTRIES) {
      c[`type_${t.value}`] =
        jasperFiles.filter((f) => getBrowserFileType(f.kind, f.name) === t.value).length +
        sharedFiles.filter((f) => getBrowserFileType(f.kind, f.name) === t.value).length
    }
    return c
  }, [jasperFiles, sharedFiles])

  const inJasper = filter.kind === 'space' ? filter.value === 'jasper' : null
  const pool = filter.kind === 'space'
    ? (filter.value === 'jasper' ? jasperFiles : sharedFiles)
    : [...jasperFiles, ...sharedFiles].filter((f) => getBrowserFileType(f.kind, f.name) === filter.value)

  const matched = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = q === '' ? pool : pool.filter((f) => f.name.toLowerCase().includes(q))
    const sorted = [...list]
    const dir = sortDir === 'asc' ? 1 : -1
    sorted.sort((a, b) => {
      switch (sortKey) {
        case 'name': return a.name.localeCompare(b.name, 'zh-CN') * dir
        case 'size': return (a.size - b.size) * dir
        case 'type': return getFormatLabel(a.name).localeCompare(getFormatLabel(b.name)) * dir
        case 'updatedAt':
        default: return String(a.updatedAt ?? '').localeCompare(String(b.updatedAt ?? '')) * dir
      }
    })
    return sorted
  }, [pool, query, sortKey, sortDir])

  // 只读判定：Jasper 的空间不可上传/改名/删除（浏览 + 预览 + 下载 + RAG 入库入口）
  const readOnly = inJasper !== false && (filter.kind === 'type' ? false : inJasper === true)
  const effectiveReadOnly = filter.kind === 'space' ? filter.value === 'jasper' : false

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }
  const selectAll = (on: boolean) => {
    setSelected(on ? new Set(matched.map((f) => f.id)) : new Set())
  }

  const onSort = (k: SortKey) => {
    if (k === sortKey) setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    else {
      setSortKey(k)
      setSortDir(k === 'name' ? 'asc' : 'desc')
    }
  }

  const confirmRename = (id: string, name: string) => {
    setRenamingId(null)
    const cur = files.find((f) => f.id === id)
    if (cur && name && name !== cur.name) renameFile(id, name)
  }

  const batchDelete = () => {
    if (selected.size === 0) return
    const names = matched.filter((f) => selected.has(f.id)).map((f) => f.name)
    if (window.confirm(`删除选中的 ${names.length} 个文件？\n${names.slice(0, 5).join('\n')}${names.length > 5 ? '\n…' : ''}`)) {
      for (const id of selected) deleteFile(id)
      setSelected(new Set())
    }
  }

  return (
    <div className="flex-1 min-h-0 flex">
      {/* 左：浏览器侧栏（Cherry FileSidebar 范式：空间 + 类型过滤 + 计数） */}
      <aside className="w-52 shrink-0 border-r border-white/5 bg-ink-900/50 flex flex-col select-none">
        <div className="px-3 pt-3 pb-1 text-[12px] font-medium text-neutral-200">资料库</div>
        <div className="flex-1 overflow-y-auto thin-scroll px-2 pb-2">
          <div className="px-2 pt-1 pb-0.5 text-[10px] uppercase tracking-wide text-neutral-500">空间</div>
          <SidebarRow
            icon={<Package size={15} className="text-neutral-500" />}
            label="Jasper 的空间"
            hint="只读"
            count={counts.jasper}
            active={filter.kind === 'space' && filter.value === 'jasper'}
            onClick={() => { setFilter({ kind: 'space', value: 'jasper' }); setSelected(new Set()) }}
          />
          <SidebarRow
            icon={<Folder size={15} className="text-neutral-500" />}
            label="我分享的"
            hint="可上传"
            count={counts.shared}
            active={filter.kind === 'space' && filter.value === 'shared'}
            onClick={() => { setFilter({ kind: 'space', value: 'shared' }); setSelected(new Set()) }}
          />
          <div className="px-2 pt-2 pb-0.5 text-[10px] uppercase tracking-wide text-neutral-500">类型</div>
          {TYPE_ENTRIES.map((t) => {
            const Icon = t.icon
            return (
              <SidebarRow
                key={t.value}
                icon={<Icon size={15} className="text-neutral-500" />}
                label={t.label}
                count={counts[`type_${t.value}`] ?? 0}
                active={filter.kind === 'type' && filter.value === t.value}
                onClick={() => { setFilter({ kind: 'type', value: t.value }); setSelected(new Set()) }}
              />
            )
          })}
        </div>
      </aside>

      {/* 右：文件浏览器主区 */}
      <div className="flex-1 min-w-0 flex flex-col">
        <div className="px-5 pt-4 pb-3">
          <div className="flex items-center justify-between">
            <div className="text-[13px] text-neutral-200 font-medium">
              {filter.kind === 'space'
                ? (filter.value === 'jasper' ? 'Jasper 的空间' : '我分享的')
                : TYPE_ENTRIES.find((t) => t.value === filter.value)?.label}
              <span className="ml-2 text-[11px] text-neutral-500 font-normal">
                {matched.length} 项
                {effectiveReadOnly && ' · 只读（浏览 / 预览 / 下载）'}
              </span>
            </div>
            <button onClick={() => setView('workspace')} className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-accent/15 text-accent text-[12px] hover:bg-accent/25">
              <Sparkles size={13} />
              RAG 检索
            </button>
          </div>

          <div className="mt-3 relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-neutral-500" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜索文件名..."
              className="w-full bg-white/[0.04] border border-white/10 rounded-lg pl-8 pr-3 py-2 text-[13px] text-neutral-100 placeholder:text-neutral-500 focus:outline-none focus:border-accent/50"
            />
          </div>
        </div>

        <div className="flex-1 min-h-0 overflow-y-auto thin-scroll px-5 pb-3">
          {/* 上传区只出现在「我分享的」——Jasper 的空间只读，不可上传 */}
          {!effectiveReadOnly && filter.kind === 'space' && <UploadZone onUpload={uploadFile} />}

          {/* 已选计数条（Cherry 范式：批量操作） */}
          {selected.size > 0 && !readOnly && (
            <div className="mb-2 flex items-center gap-2 rounded-lg border border-accent/30 bg-accent/10 px-3 py-1.5 text-[12px] text-neutral-200">
              <span>已选 {selected.size} 项</span>
              <div className="flex-1" />
              <button onClick={batchDelete} className="flex items-center gap-1 text-red-300 hover:text-red-200">
                <Trash2 size={12} /> 批量删除
              </button>
              <button onClick={() => setSelected(new Set())} className="text-neutral-400 hover:text-neutral-200">
                清除选择
              </button>
            </div>
          )}

          <FileTable
            files={matched}
            readOnly={effectiveReadOnly || readOnly}
            selected={selected}
            renamingId={renamingId}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={onSort}
            onToggleSelect={toggleSelect}
            onSelectAll={selectAll}
            onOpen={(f) => openFile(f.id)}
            onIngest={(f) => ingestFile(f.id)}
            onRename={(f) => setRenamingId(f.id)}
            onRenameConfirm={confirmRename}
            onRenameCancel={() => setRenamingId(null)}
            onDelete={(f) => {
              if (window.confirm(`删除「${f.name}」？`)) deleteFile(f.id)
            }}
            onDownload={(f) => void downloadFile(f.id)}
          />
          {matched.length === 0 && (
            <div className="mt-6 text-center text-[12px] text-neutral-600">
              {effectiveReadOnly ? 'Jasper 的空间暂无文件' : '暂无文件，可通过上方上传区分享'}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function SidebarRow({ icon, label, hint, count, active, onClick }: {
  icon: React.ReactNode; label: string; hint?: string; count: number; active: boolean; onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-[13px] transition-colors ${active ? 'bg-white/10 text-neutral-100' : 'text-neutral-300 hover:bg-white/5'}`}
    >
      {icon}
      <span className="flex-1 text-left truncate">{label}</span>
      {hint && <span className="text-[10px] text-neutral-600">{hint}</span>}
      {count > 0 && <span className="text-[11px] text-neutral-500">{count}</span>}
    </button>
  )
}

// UploadZone 拖拽/点击上传区（仅「我分享的」可见）
function UploadZone({ onUpload }: { onUpload: (file: File) => void }) {
  const [drag, setDrag] = useState(false)

  const handleFiles = (list: FileList | null) => {
    if (!list) return
    for (const f of Array.from(list)) onUpload(f)
  }

  return (
    <label
      onDragOver={(e) => { e.preventDefault(); setDrag(true) }}
      onDragLeave={() => setDrag(false)}
      onDrop={(e) => { e.preventDefault(); setDrag(false); handleFiles(e.dataTransfer?.files) }}
      className={`mb-3 flex items-center justify-center gap-2 rounded-lg border border-dashed px-4 py-5 text-[12.5px] transition-colors cursor-pointer ${drag ? 'border-accent/60 bg-accent/10 text-accent' : 'border-white/10 bg-white/[0.02] text-neutral-400 hover:border-accent/40 hover:text-neutral-200'}`}
    >
      <input type="file" multiple className="hidden" onChange={(e) => handleFiles(e.target.files)} />
      <UploadCloud size={16} />
      拖拽文件到此处，或点击上传到「我分享的」
    </label>
  )
}

function SortHeader({ label, field, sortKey, sortDir, onSort }: {
  label: string; field: SortKey; sortKey: SortKey; sortDir: SortDir; onSort: (k: SortKey) => void
}) {
  const active = sortKey === field
  const Icon = active ? (sortDir === 'asc' ? ChevronUp : ChevronDown) : ChevronsUpDown
  return (
    <button onClick={() => onSort(field)} className="flex items-center gap-1 text-[11px] text-neutral-500 hover:text-neutral-300">
      <span>{label}</span>
      <Icon size={11} className={active ? 'opacity-80' : 'opacity-40'} />
    </button>
  )
}

function FileTable({
  files,
  readOnly,
  selected,
  renamingId,
  sortKey,
  sortDir,
  onSort,
  onToggleSelect,
  onSelectAll,
  onOpen,
  onIngest,
  onRename,
  onRenameConfirm,
  onRenameCancel,
  onDelete,
  onDownload,
}: {
  files: FileMeta[]
  readOnly: boolean
  selected: Set<string>
  renamingId: string | null
  sortKey: SortKey
  sortDir: SortDir
  onSort: (k: SortKey) => void
  onToggleSelect: (id: string) => void
  onSelectAll: (on: boolean) => void
  onOpen: (f: FileMeta) => void
  onIngest: (f: FileMeta) => void
  onRename: (f: FileMeta) => void
  onRenameConfirm: (id: string, name: string) => void
  onRenameCancel: () => void
  onDelete: (f: FileMeta) => void
  onDownload: (f: FileMeta) => void
}) {
  const allChecked = files.length > 0 && files.every((f) => selected.has(f.id))
  return (
    <div className="rounded-lg border border-white/5 overflow-hidden">
      <div className="grid grid-cols-[28px_1fr_80px_90px_150px_64px_96px] gap-2 px-3 py-2 bg-white/[0.03] items-center">
        {!readOnly ? (
          <input
            type="checkbox"
            checked={allChecked}
            onChange={(e) => onSelectAll(e.target.checked)}
            className="w-3.5 h-3.5 accent-emerald-500"
            title="全选"
          />
        ) : <span />}
        <SortHeader label="名称" field="name" sortKey={sortKey} sortDir={sortDir} onSort={onSort} />
        <SortHeader label="大小" field="size" sortKey={sortKey} sortDir={sortDir} onSort={onSort} />
        <SortHeader label="类型" field="type" sortKey={sortKey} sortDir={sortDir} onSort={onSort} />
        <SortHeader label="更新时间" field="updatedAt" sortKey={sortKey} sortDir={sortDir} onSort={onSort} />
        <span className="text-[11px] text-neutral-500">RAG</span>
        <span className="text-[11px] text-neutral-500 text-right">操作</span>
      </div>
      {files.map((f) => {
        const checked = selected.has(f.id)
        const renaming = renamingId === f.id
        return (
          <div
            key={f.id}
            className={`grid grid-cols-[28px_1fr_80px_90px_150px_64px_96px] gap-2 px-3 py-2 border-t border-white/5 items-center text-[12.5px] cursor-pointer group ${checked ? 'bg-accent/10' : 'hover:bg-white/[0.03]'}`}
            onClick={() => { if (!renaming) onOpen(f) }}
          >
            {!readOnly ? (
              <input
                type="checkbox"
                checked={checked}
                onChange={() => onToggleSelect(f.id)}
                onClick={(e) => e.stopPropagation()}
                className="w-3.5 h-3.5 accent-emerald-500"
              />
            ) : <span />}
            <div className="flex items-center gap-2 min-w-0">
              <FileIcon kind={f.kind} name={f.name} />
              {renaming && !readOnly ? (
                <InlineRename
                  value={f.name}
                  onConfirm={(v) => onRenameConfirm(f.id, v)}
                  onCancel={onRenameCancel}
                />
              ) : (
                <span className="text-neutral-100 truncate">{f.name}</span>
              )}
              {f.isArtifact && (
                <span className="shrink-0 px-1 py-px rounded text-[10px] bg-accent/15 text-accent border border-accent/30" title={`Agent 产物（工作区 ${f.workspaceId || ''}）`}>
                  产物
                </span>
              )}
            </div>
            <span className="text-neutral-500">{formatFileSize(f.size)}</span>
            <span className="text-neutral-500">{getFormatLabel(f.name)}</span>
            <span className="text-neutral-500 truncate">{fmtTime(f.updatedAt)}</span>
            <span>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  onIngest(f)
                }}
                title={f.ingested ? '已入库' : '向量化入库'}
                className={`px-1.5 py-0.5 rounded text-[10px] border ${f.ingested ? 'text-accent border-accent/30' : 'text-neutral-400 border-white/10 group-hover:border-accent/40'}`}
              >
                {f.ingested ? '已入库' : '入库'}
              </button>
            </span>
            <span className="flex justify-end gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                onClick={(e) => { e.stopPropagation(); onDownload(f) }}
                title="下载"
                className="p-1 text-neutral-500 hover:text-neutral-100 hover:bg-white/5 rounded"
              >
                <Download size={13} />
              </button>
              {!readOnly && (
                <button
                  onClick={(e) => { e.stopPropagation(); onRename(f) }}
                  title="重命名"
                  className="p-1 text-neutral-500 hover:text-neutral-100 hover:bg-white/5 rounded"
                >
                  <Pencil size={13} />
                </button>
              )}
              {!readOnly && (
                <button
                  onClick={(e) => { e.stopPropagation(); onDelete(f) }}
                  title="删除"
                  className="p-1 text-neutral-500 hover:text-red-400 hover:bg-white/5 rounded"
                >
                  <Trash2 size={13} />
                </button>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// InlineRename 行内改名（参照 Cherry InlineRename：自动聚焦到文件名主体，不含扩展名）
function InlineRename({ value, onConfirm, onCancel }: {
  value: string; onConfirm: (v: string) => void; onCancel: () => void
}) {
  const [text, setText] = useState(value)
  return (
    <input
      autoFocus
      value={text}
      onChange={(e) => setText(e.target.value)}
      onFocus={(e) => {
        const dot = value.lastIndexOf('.')
        e.target.setSelectionRange(0, dot > 0 ? dot : value.length)
      }}
      onBlur={() => { if (text.trim()) onConfirm(text.trim()); else onCancel() }}
      onKeyDown={(e) => {
        if (e.key === 'Enter' && text.trim()) onConfirm(text.trim())
        if (e.key === 'Escape') onCancel()
      }}
      onClick={(e) => e.stopPropagation()}
      className="flex-1 min-w-0 px-1.5 py-0.5 rounded border border-accent/40 bg-white/10 text-[12.5px] text-neutral-100 focus:outline-none"
    />
  )
}

function fmtTime(t: string): string {
  return (t && t.length > 16 ? t.slice(0, 16).replace('T', ' ') : (t || '-'))
}

async function downloadFile(id: string) {
  try {
    const { api } = await import('../../api')
    const blob = await api.downloadFile(id)
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

function FileIcon({ kind, name }: { kind: FileKind; name: string }) {
  const t = getBrowserFileType(kind, name)
  const cls = 'text-neutral-500 shrink-0'
  switch (t) {
    case 'image': return <ImageIcon size={14} className={cls} />
    case 'video': return <Video size={14} className={cls} />
    case 'audio': return <Music size={14} className={cls} />
    case 'text': return <FileCode2 size={14} className={cls} />
    case 'document': return <FileText size={14} className={cls} />
    default: return <FileQuestion size={14} className={cls} />
  }
}
