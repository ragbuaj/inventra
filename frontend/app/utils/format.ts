const idrFormatter = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 })

/** Formats a backend decimal string / number as IDR; '—' when absent or invalid. */
export function formatRupiah(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  return idrFormatter.format(n)
}

export function formatDate(iso: string | null, opts: { withTime?: boolean } = {}): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    dateStyle: 'medium',
    ...(opts.withTime ? { timeStyle: 'short' } : {})
  }).format(d)
}

// Display an integer count with id-ID thousands grouping ('1500' → '1.500',
// -1500 → '-1.500'); '—' when absent or non-numeric. For plain counts in reports.
export function formatInt(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  return Math.trunc(n).toLocaleString('id-ID')
}

// Display a numeric string with id-ID thousands grouping ('1000000' → '1.000.000').
export function formatThousands(v: string | number | null | undefined): string {
  const s = String(v ?? '').replace(/\D/g, '')
  if (!s) return ''
  return Number(s).toLocaleString('id-ID')
}

// Strip grouping back to a bare digit string ('1.000.000' → '1000000').
export function parseThousands(v: string | null | undefined): string {
  return String(v ?? '').replace(/\D/g, '')
}

// Localized relative time ('2 jam lalu' / '2 hours ago', 'kemarin' / 'yesterday')
// via the built-in Intl.RelativeTimeFormat — no hardcoded per-locale strings.
// `now` is injectable so tests are deterministic. Returns '' for a bad date.
export function formatRelativeTime(iso: string | null | undefined, locale: string, now: number = Date.now()): string {
  if (!iso) return ''
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const diffSec = Math.round((now - then) / 1000)
  const abs = Math.abs(diffSec)
  const units: Array<{ limit: number, div: number, unit: Intl.RelativeTimeFormatUnit }> = [
    { limit: 60, div: 1, unit: 'second' },
    { limit: 3600, div: 60, unit: 'minute' },
    { limit: 86400, div: 3600, unit: 'hour' },
    { limit: 604800, div: 86400, unit: 'day' },
    { limit: 2592000, div: 604800, unit: 'week' },
    { limit: 31536000, div: 2592000, unit: 'month' }
  ]
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  for (const { limit, div, unit } of units) {
    if (abs < limit) {
      // Past → negative value ('… ago'); clamp sub-minute to 0 → 'just now'/'baru saja'.
      const value = unit === 'second' ? 0 : -Math.round(diffSec / div)
      return rtf.format(value, unit)
    }
  }
  return rtf.format(-Math.round(diffSec / 31536000), 'year')
}

// Compact IDR for tight KPI tiles: 'Rp 1,23 M', 'Rp 3,4 T'. Full precision
// belongs in tables — pair this with a title tooltip carrying formatRupiah().
export function formatRupiahCompact(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return '—'
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  const scales: Array<{ v: number, s: string }> = [
    { v: 1e12, s: 'T' }, { v: 1e9, s: 'M' }, { v: 1e6, s: 'jt' }, { v: 1e3, s: 'rb' }
  ]
  for (const { v, s } of scales) {
    if (abs >= v) {
      const scaled = abs / v
      const digits = scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2
      const num = scaled.toLocaleString('id-ID', { maximumFractionDigits: digits })
      return `${sign}Rp ${num} ${s}`
    }
  }
  return `${sign}Rp ${abs.toLocaleString('id-ID')}`
}
