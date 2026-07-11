/**
 * Report catalog metadata + period/value formatters shared by the dashboard and
 * reports screens. Pure functions only — no Vue/Nuxt runtime deps so this file is
 * unit-testable in a plain (node) vitest environment.
 */

/** The seven report kinds. Order: the 4 mockup reports first, then the 3 new ones. */
export type ReportKey = 'assets' | 'depreciation' | 'utilization' | 'maintenance' | 'transfers' | 'disposals' | 'opname'

export const REPORT_KEYS: ReportKey[] = [
  'assets',
  'depreciation',
  'utilization',
  'maintenance',
  'transfers',
  'disposals',
  'opname'
]

/** Lucide icon name per report key. */
export const REPORT_ICON: Record<ReportKey, string> = {
  assets: 'i-lucide-package',
  depreciation: 'i-lucide-trending-down',
  utilization: 'i-lucide-gauge',
  maintenance: 'i-lucide-receipt',
  transfers: 'i-lucide-arrow-left-right',
  disposals: 'i-lucide-trash-2',
  opname: 'i-lucide-clipboard-check'
}

/**
 * Period presets. These use the BACKEND query names (snake_case for month/quarter)
 * so `periodToQuery` can pass them straight through as `period=<preset>`.
 */
export type PeriodPreset = 'last30' | 'this_month' | 'this_quarter' | 'ytd'

/**
 * The value emitted by `PeriodFilter`. `from`/`to` (ISO `YYYY-MM-DD`) are set iff
 * `preset === 'custom'`.
 */
export interface PeriodValue {
  preset: PeriodPreset | 'custom'
  from?: string
  to?: string
}

/**
 * Convert a `PeriodValue` into the backend query params. Preset → `{ period }`;
 * custom → `{ date_from, date_to }` (mutually exclusive, per the report contract).
 */
export function periodToQuery(p: PeriodValue): Record<string, string> {
  if (p.preset === 'custom') {
    return { date_from: p.from!, date_to: p.to! }
  }
  return { period: p.preset }
}

/** Group an integer with Indonesian thousands separators (`.`). */
function groupThousands(n: number): string {
  return Math.round(n).toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

/**
 * Abbreviate a scaled number to at most 2 decimals, trimming trailing zeros and
 * using the Indonesian decimal comma. e.g. 3.82 → "3,82", 42.5 → "42,5", 1 → "1".
 */
function abbreviate(n: number): string {
  return n
    .toFixed(2)
    .replace(/\.?0+$/, '')
    .replace('.', ',')
}

/**
 * Format a decimal string amount to a short Rupiah label:
 * ≥1e9 → "Rp 3,82 M", ≥1e6 → "Rp 42,5 Jt", else full "Rp 950.000".
 * Unparseable input is returned verbatim.
 */
export function formatMoneyShort(v: string): string {
  const n = Number(v)
  if (!Number.isFinite(n) || v.trim() === '') return v
  if (n >= 1e9) return `Rp ${abbreviate(n / 1e9)} M`
  if (n >= 1e6) return `Rp ${abbreviate(n / 1e6)} Jt`
  return `Rp ${groupThousands(n)}`
}

/**
 * Format a signed trend percentage: 8.3 → "+8,3%", -6.4 → "−6,4%" (real minus,
 * U+2212). Returns null when there is no trend value.
 */
export function formatTrendPct(p: number | null | undefined): string | null {
  if (p === null || p === undefined) return null
  const sign = p < 0 ? '−' : '+'
  const abs = Math.abs(p).toString().replace('.', ',')
  return `${sign}${abs}%`
}
