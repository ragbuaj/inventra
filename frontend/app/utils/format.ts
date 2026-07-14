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
