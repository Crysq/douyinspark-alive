export function formatCountdown(seconds: number): string {
  if (seconds <= 0) return '已断火'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h} 小时 ${String(m).padStart(2, '0')} 分`
  return `${m} 分 ${String(s).padStart(2, '0')} 秒`
}

export function compactCountdown(seconds: number): string {
  if (seconds <= 0) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m`
  return `${String(m).padStart(2, '0')}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
}

export function timeAgo(value?: string | null): string {
  if (!value) return '从未'
  const diff = Date.now() - new Date(value).getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  if (diff < 2_592_000_000) return `${Math.floor(diff / 86_400_000)} 天前`
  return formatDate(value)
}

export function formatDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  return `${date.getMonth() + 1} 月 ${date.getDate()} 日`
}

export function formatDay(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  const now = new Date()
  const month = date.getMonth() + 1
  const day = date.getDate()
  if (date.getFullYear() === now.getFullYear()) return `${month} 月 ${day} 日`
  return `${date.getFullYear()}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

export function formatDateTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export function formatClock(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function percent(value: number, digits = 0): string {
  return `${(value * 100).toFixed(digits)}%`
}

export function greeting(): string {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 11) return '早上好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
}
