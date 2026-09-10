// 文件展示工具（参照 Cherry src/renderer/pages/files/fileDisplay.ts 的思想，
// 按本项目 FileMeta 裁剪：类型图标/格式标签/大小格式化）。

/** 浏览器内的文件类型分组（对应 Cherry 侧栏的类型过滤） */
export type BrowserFileType = 'image' | 'video' | 'audio' | 'text' | 'document' | 'other'

/** 由 kind + 扩展名判定展示类型 */
export function getBrowserFileType(kind: string, name: string): BrowserFileType {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico'].includes(ext)) return 'image'
  if (['mp4', 'webm', 'mov', 'avi', 'mkv'].includes(ext)) return 'video'
  if (['mp3', 'wav', 'ogg', 'flac', 'm4a'].includes(ext)) return 'audio'
  if (['md', 'markdown', 'txt', 'log', 'json', 'csv', 'ts', 'tsx', 'js', 'jsx', 'py', 'go', 'java', 'rs', 'c', 'cpp', 'h', 'sql', 'sh', 'yml', 'yaml', 'xml'].includes(ext)) return 'text'
  if (['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'].includes(ext)) return 'document'
  if (kind === 'image') return 'image'
  if (kind === 'pdf' || kind === 'docx') return 'document'
  if (kind === 'code' || kind === 'markdown' || kind === 'json' || kind === 'csv' || kind === 'text') return 'text'
  return 'other'
}

/** 格式标签（如 PDF / Markdown / TSX），参照 Cherry getFormatLabel */
export function getFormatLabel(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  if (!ext || ext === name.toLowerCase()) return '—'
  const map: Record<string, string> = {
    png: 'PNG', jpg: 'JPG', jpeg: 'JPEG', gif: 'GIF', webp: 'WEBP',
    pdf: 'PDF', doc: 'DOC', docx: 'DOCX', xls: 'Excel', xlsx: 'Excel', ppt: 'PPT', pptx: 'PPT',
    py: 'Python', ts: 'TypeScript', tsx: 'TSX', js: 'JavaScript', jsx: 'JSX', go: 'Go', java: 'Java',
    json: 'JSON', yaml: 'YAML', yml: 'YAML', md: 'Markdown', txt: 'Text', csv: 'CSV',
    mp3: 'MP3', mp4: 'MP4',
  }
  return map[ext] || ext.toUpperCase()
}

/** 人性化文件大小，参照 Cherry formatFileSize */
export function formatFileSize(bytes: number | null | undefined): string {
  if (bytes === null || bytes === undefined || !Number.isFinite(bytes)) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB'] as const
  let value = bytes / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`
}
