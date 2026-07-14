<script setup lang="ts">
import type {
  ReportResult, ReportFilters,
  AssetReportRow, DeprReportRow, UtilReportRow, MaintReportRow,
  TransferReportRow, DisposalReportRow, OpnameReportRow
} from '~/composables/api/useReports'
import { useReports } from '~/composables/api/useReports'
import { useCategories } from '~/composables/api/useCategories'
import type { ReportKey, PeriodValue, PeriodPreset } from '~/constants/reportMeta'
import { REPORT_KEYS, REPORT_ICON, formatMoneyShort } from '~/constants/reportMeta'
import { formatInt } from '~/utils/format'

definePageMeta({ middleware: 'can', permission: 'report.view' })

// ---------------------------------------------------------------------------
// Cell / column view-model (shared by the hand-rolled result table)
// ---------------------------------------------------------------------------
type CellTone = 'default' | 'muted' | 'dimmed' | 'error' | 'success'
type CellWeight = 'normal' | 'medium' | 'semibold' | 'bold'
interface Cell { text: string, align: 'left' | 'right', tone: CellTone, mono: boolean, weight: CellWeight }
interface Col { label: string, align: 'left' | 'right' }
interface Kpi { label: string, value: string, sub: string }
interface Bar { label: string, display: string, w: string }

const TONE_CLASS: Record<CellTone, string> = { default: '', muted: 'text-muted', dimmed: 'text-dimmed', error: 'text-error', success: 'text-success' }
const WEIGHT_CLASS: Record<CellWeight, string> = { normal: '', medium: 'font-medium', semibold: 'font-semibold', bold: 'font-bold' }

function cell(text: string, align: 'left' | 'right' = 'left', opts: Partial<Pick<Cell, 'tone' | 'mono' | 'weight'>> = {}): Cell {
  return { text, align, tone: opts.tone ?? 'default', mono: opts.mono ?? false, weight: opts.weight ?? 'normal' }
}
function colDefs(labels: string[], rightFrom: number): Col[] {
  return labels.map((label, i): Col => ({ label, align: i >= rightFrom ? 'right' : 'left' }))
}

const { t, te } = useI18n()
const can = useCan()
const api = useReports()
const office = useOfficePicker()
const categoriesApi = useCategories()

const canExport = computed(() => can('report.export'))

// ---------------------------------------------------------------------------
// State + filters
// ---------------------------------------------------------------------------
// 'all' sentinel (not '') because Reka UI's SelectItem forbids an empty-string value.
const ALL = 'all'
const report = ref<ReportKey>('assets')
const period = ref<PeriodValue>({ preset: 'this_quarter' })
const officeId = ref<string | null>(null)
const categoryId = ref<string>(ALL)
const status = ref<string>(ALL)
const basis = ref<'commercial' | 'fiscal'>('commercial')

// Office: async search picker (no more eager `{ limit: 100 }` list) — the
// result-meta office label resolves on demand via the same adapter's
// resolveFn, memoized (useResolveCache).
const officeCache = useResolveCache(office.resolveFn)
const categoryOptions = ref<{ value: string, label: string }[]>([])

const applied = ref(false)
const loading = ref(false)
const loadError = ref(false)
const exporting = ref(false)
const result = ref<ReportResult | null>(null)

// Real asset-status enum keys (labels via dashboard.status.*, renamed in Task 12).
const STATUS_KEYS = ['available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost'] as const
const BASIS_OPTIONS: Array<{ key: 'commercial' | 'fiscal', icon: string }> = [
  { key: 'commercial', icon: 'i-lucide-scale' },
  { key: 'fiscal', icon: 'i-lucide-gavel' }
]

const catOptions = computed(() => [
  { value: ALL, label: t('reports.allCategories') },
  ...categoryOptions.value
])
const statusOptions = computed(() => [
  { value: ALL, label: t('reports.allStatus') },
  ...STATUS_KEYS.map(k => ({ value: k, label: t(`dashboard.status.${k}`) }))
])

const showStatus = computed(() => report.value === 'assets')
const showBasis = computed(() => report.value === 'depreciation')

const reportCards = computed(() => REPORT_KEYS.map(k => ({
  key: k,
  icon: REPORT_ICON[k],
  label: t(`reports.card.${k}.label`),
  desc: t(`reports.card.${k}.desc`),
  active: report.value === k
})))

// ---------------------------------------------------------------------------
// Labels
// ---------------------------------------------------------------------------
const PRESET_SUFFIX: Record<PeriodPreset, string> = {
  last30: 'last30', this_month: 'thisMonth', this_quarter: 'thisQuarter', ytd: 'ytd'
}
const periodLabel = computed(() => {
  const p = period.value
  if (p.preset === 'custom' && p.from && p.to) return `${p.from} – ${p.to}`
  return t(`reports.period.${PRESET_SUFFIX[p.preset as PeriodPreset] ?? 'thisQuarter'}`)
})
const officeLabel = computed(() => {
  if (!officeId.value) return t('reports.allOffices')
  return officeCache.get(officeId.value)
})
const periodSlug = computed(() => {
  const p = period.value
  if (p.preset === 'custom' && p.from && p.to) return `${p.from}_${p.to}`
  return p.preset
})

const resultTitle = computed(() => t(`reports.card.${report.value}.label`))
const resultMeta = computed(() => t('reports.resultMeta', { period: periodLabel.value, office: officeLabel.value }))
const hasData = computed(() => !!result.value && result.value.rows.length > 0)

// ---------------------------------------------------------------------------
// Filter → query mapping
// ---------------------------------------------------------------------------
function currentFilters(): ReportFilters {
  return {
    period: period.value,
    officeId: officeId.value ?? undefined,
    categoryId: categoryId.value === ALL ? undefined : categoryId.value,
    status: report.value === 'assets' && status.value !== ALL ? status.value : undefined,
    basis: report.value === 'depreciation' ? basis.value : undefined
  }
}

// ---------------------------------------------------------------------------
// Value formatters
// ---------------------------------------------------------------------------
const MONEY_KPI = new Set([
  'total_acquisition', 'total_book', 'period_expense', 'accumulated', 'remaining_book',
  'total_cost', 'preventive', 'corrective', 'total_proceeds', 'total_gain_loss'
])
const MONEY_CHART = new Set<ReportKey>(['assets', 'depreciation', 'maintenance', 'disposals'])

function kpiValue(key: string, value: string): string {
  if (MONEY_KPI.has(key)) return formatMoneyShort(value)
  if (key === 'avg_utilization') return `${value}%`
  if (key === 'total_days') return `${formatInt(value)} ${t('reports.unit.days')}`
  return /^-?\d+$/.test(value.trim()) ? formatInt(value) : value
}
function toChartBars(chart: { label: string, value: string }[], money: boolean): Bar[] {
  const nums = chart.map(c => Number(c.value) || 0)
  const max = Math.max(1, ...nums)
  return chart.map((c, i) => ({
    label: c.label,
    display: money ? formatMoneyShort(c.value) : formatInt(nums[i]),
    w: `${Math.round((nums[i]! / max) * 100)}%`
  }))
}
function money(v: string | undefined): string {
  return formatMoneyShort(v ?? '0')
}
function statusLabel(base: string, key: string): string {
  const full = `${base}.${key}`
  return te(full) ? t(full) : key
}
function gainLossTone(v: string): CellTone {
  const n = Number(v)
  return n < 0 ? 'error' : n > 0 ? 'success' : 'default'
}

// ---------------------------------------------------------------------------
// Result view-model
// ---------------------------------------------------------------------------
interface TableView { mode: 'table', kpis: Kpi[], chartTitle: string, chartBars: Bar[], cols: Col[], rows: Cell[][], footer: Cell[] | null }
interface OpnameView { mode: 'opname', kpis: Kpi[], chartTitle: string, chartBars: Bar[], cols: Col[], rows: { sessionId: string, cells: Cell[] }[] }
type View = TableView | OpnameView

const view = computed<View | null>(() => {
  const r = result.value
  if (!r) return null

  const kpis: Kpi[] = r.kpis.map(k => ({
    label: t(`reports.kpi.${k.key}`),
    value: kpiValue(k.key, k.value),
    sub: t(`reports.kpiSub.${k.key}`)
  }))
  const shared = { kpis, chartTitle: t(`reports.chart.${r.type}`), chartBars: toChartBars(r.chart, MONEY_CHART.has(r.type)) }

  const T = r.totals
  const TOTAL = t('reports.total')
  const daysShort = t('reports.unit.daysShort')
  const dash = '—'

  if (r.type === 'assets') {
    const rows = r.rows as AssetReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.code'), t('reports.col.name'), t('reports.col.category'), t('reports.col.buyPrice'), t('reports.col.accumDeprec'), t('reports.col.bookValue')], 3),
      rows: rows.map(a => [cell(a.asset_tag, 'left', { mono: true, tone: 'muted' }), cell(a.name, 'left', { weight: 'medium' }), cell(a.category_name, 'left', { tone: 'muted' }), cell(money(a.purchase_cost), 'right'), cell(money(a.accum_deprec), 'right', { tone: 'muted' }), cell(money(a.book_value), 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell('', 'left'), cell(money(T.purchase_cost), 'right', { weight: 'bold' }), cell(money(T.accum_deprec), 'right', { weight: 'bold' }), cell(money(T.book_value), 'right', { weight: 'bold' })]
    }
  }
  if (r.type === 'depreciation') {
    const rows = r.rows as DeprReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.period'), t('reports.col.opening'), t('reports.col.deprec'), t('reports.col.closing')], 1),
      rows: rows.map(d => [cell(d.period, 'left', { mono: true, weight: 'semibold' }), cell(money(d.opening), 'right', { tone: 'muted' }), cell(money(d.amount), 'right', { tone: 'error' }), cell(money(d.closing), 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell(money(T.opening), 'right', { weight: 'bold' }), cell(money(T.amount), 'right', { weight: 'bold' }), cell(money(T.closing), 'right', { weight: 'bold' })]
    }
  }
  if (r.type === 'utilization') {
    const rows = r.rows as UtilReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.assetName'), t('reports.col.category'), t('reports.col.daysLoaned'), t('reports.col.loanCount'), t('reports.col.utilization')], 2),
      rows: rows.map(u => [cell(u.name, 'left', { weight: 'medium' }), cell(u.category_name, 'left', { tone: 'muted' }), cell(`${formatInt(u.days_loaned)} ${daysShort}`, 'right', { tone: 'muted' }), cell(`${formatInt(u.loan_count)}×`, 'right', { tone: 'muted' }), cell(`${u.utilization_pct}%`, 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell(`${formatInt(T.days_loaned ?? 0)} ${daysShort}`, 'right', { weight: 'bold' }), cell(`${formatInt(T.loan_count ?? 0)}×`, 'right', { weight: 'bold' }), cell('', 'right')]
    }
  }
  if (r.type === 'maintenance') {
    const rows = r.rows as MaintReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.asset'), t('reports.col.category'), t('reports.col.type'), t('reports.col.actions'), t('reports.col.totalCost')], 3),
      rows: rows.map(b => [cell(b.asset_name, 'left', { weight: 'medium' }), cell(b.category_name, 'left', { tone: 'muted' }), cell(b.type, 'left', { tone: 'muted' }), cell(formatInt(b.actions), 'right', { tone: 'muted' }), cell(money(b.total_cost), 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell('', 'left'), cell(formatInt(T.actions ?? 0), 'right', { weight: 'bold' }), cell(money(T.total_cost), 'right', { weight: 'bold' })]
    }
  }
  if (r.type === 'transfers') {
    const rows = r.rows as TransferReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.name'), t('reports.col.fromOffice'), t('reports.col.toOffice'), t('reports.filter.status'), t('reports.col.shipped'), t('reports.col.received'), t('reports.col.bast')], 99),
      rows: rows.map(x => [cell(x.asset_name, 'left', { weight: 'medium' }), cell(x.from_office, 'left', { tone: 'muted' }), cell(x.to_office, 'left', { tone: 'muted' }), cell(statusLabel('transfer.status', x.status), 'left'), cell(x.shipped_date || dash, 'left', { tone: 'muted', mono: true }), cell(x.received_date || dash, 'left', { tone: 'muted', mono: true }), cell(x.bast_no || dash, 'left', { mono: true })]),
      footer: null
    }
  }
  if (r.type === 'disposals') {
    const rows = r.rows as DisposalReportRow[]
    return {
      mode: 'table', ...shared,
      cols: colDefs([t('reports.col.name'), t('reports.col.method'), t('reports.col.date'), t('reports.col.bookValue'), t('reports.col.proceeds'), t('reports.col.gainLoss')], 3),
      rows: rows.map(x => [cell(x.asset_name, 'left', { weight: 'medium' }), cell(statusLabel('disposal.method', x.method), 'left', { tone: 'muted' }), cell(x.disposal_date || dash, 'left', { tone: 'muted', mono: true }), cell(money(x.book_value), 'right'), cell(money(x.proceeds), 'right'), cell(money(x.gain_loss), 'right', { weight: 'semibold', tone: gainLossTone(x.gain_loss) })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell('', 'left'), cell(money(T.book_value), 'right', { weight: 'bold' }), cell(money(T.proceeds), 'right', { weight: 'bold' }), cell(money(T.gain_loss), 'right', { weight: 'bold', tone: gainLossTone(T.gain_loss ?? '0') })]
    }
  }
  // opname
  const rows = r.rows as OpnameReportRow[]
  return {
    mode: 'opname', ...shared,
    cols: [
      { label: t('reports.col.session'), align: 'left' },
      { label: t('reports.col.office'), align: 'left' },
      { label: t('reports.col.period'), align: 'left' },
      { label: t('reports.col.totalItems'), align: 'right' },
      { label: t('reports.col.variance'), align: 'right' },
      { label: t('reports.col.sessionStatus'), align: 'left' },
      { label: t('reports.col.downloadBa'), align: 'right' }
    ],
    rows: rows.map(o => ({
      sessionId: o.session_id,
      cells: [
        cell(o.name, 'left', { weight: 'medium' }),
        cell(o.office_name, 'left', { tone: 'muted' }),
        cell(o.period, 'left', { tone: 'muted', mono: true }),
        cell(formatInt(o.total_items), 'right', { tone: 'muted' }),
        cell(formatInt(o.variance), 'right', { weight: o.variance !== 0 ? 'semibold' : 'normal', tone: o.variance !== 0 ? 'error' : 'default' }),
        cell(statusLabel('stockOpname.status', o.status), 'left')
      ]
    }))
  }
})

// ---------------------------------------------------------------------------
// Data loading
// ---------------------------------------------------------------------------
async function apply() {
  loading.value = true
  loadError.value = false
  applied.value = true
  try {
    result.value = await api.run(report.value, currentFilters())
  } catch {
    loadError.value = true
    result.value = null
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  officeId.value = null
  categoryId.value = ALL
  status.value = ALL
  basis.value = 'commercial'
}

// Switching the report type invalidates the current (typed) result — force a
// re-apply so `view` never renders rows of a different shape.
function selectReport(k: ReportKey) {
  if (report.value === k) return
  report.value = k
  applied.value = false
  loadError.value = false
  result.value = null
}

async function loadCategories() {
  try {
    const cats = await categoriesApi.tree()
    categoryOptions.value = cats.map(c => ({ value: c.id, label: c.name }))
  } catch {
    categoryOptions.value = []
  }
}

// ---------------------------------------------------------------------------
// Export (blob-anchor download)
// ---------------------------------------------------------------------------
function download(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = name
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(url)
}

async function doExport(format: 'pdf' | 'xlsx') {
  if (exporting.value) return
  exporting.value = true
  try {
    const blob = await api.exportReport(report.value, currentFilters(), format)
    download(blob, `laporan-${report.value}-${periodSlug.value}.${format}`)
  } catch {
    // useApiClient surfaces the error toast.
  } finally {
    exporting.value = false
  }
}
async function doExportGl(format: 'pdf' | 'xlsx') {
  if (exporting.value) return
  exporting.value = true
  try {
    const blob = await api.exportReport('disposals', currentFilters(), format, 'gl_recap')
    download(blob, `laporan-disposals-gl-${periodSlug.value}.${format}`)
  } catch {
    // useApiClient surfaces the error toast.
  } finally {
    exporting.value = false
  }
}
async function doOpnameBa(sessionId: string, format: 'pdf' | 'xlsx') {
  if (exporting.value) return
  exporting.value = true
  try {
    const blob = await api.opnameBa(sessionId, format)
    download(blob, `berita-acara-opname-${sessionId}.${format}`)
  } catch {
    // useApiClient surfaces the error toast.
  } finally {
    exporting.value = false
  }
}

const glItems = computed(() => [[
  { 'label': t('reports.pdf'), 'icon': 'i-lucide-file-text', 'data-testid': 'reports-export-gl-pdf', 'onSelect': () => doExportGl('pdf') },
  { 'label': t('reports.excel'), 'icon': 'i-lucide-file-spreadsheet', 'data-testid': 'reports-export-gl-xlsx', 'onSelect': () => doExportGl('xlsx') }
]])

onMounted(() => {
  loadCategories()
})

// Driving teleported menus / Nuxt UI selects via DOM is brittle — expose the
// flows and filter state so tests can drive them deterministically.
defineExpose({ apply, doExport, doExportGl, doOpnameBa, resetFilters, selectReport, report, period, officeId, categoryId, status, basis })
</script>

<template>
  <div>
    <!-- Header -->
    <div class="mb-4">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('reports.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('reports.subtitle') }}
      </p>
    </div>

    <!-- Report type cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
      <button
        v-for="c in reportCards"
        :key="c.key"
        type="button"
        :data-testid="`reports-card-${c.key}`"
        class="flex flex-col gap-2 p-[15px] rounded-[13px] border text-left transition-colors hover:border-primary"
        :class="c.active ? 'border-primary bg-primary/5 shadow-none' : 'border-default bg-default shadow-sm'"
        @click="selectReport(c.key)"
      >
        <span
          class="size-[34px] rounded-[9px] flex items-center justify-center"
          :class="c.active ? 'bg-primary/20 text-primary' : 'bg-muted text-muted'"
        >
          <UIcon
            :name="c.icon"
            class="size-[17px]"
          />
        </span>
        <span
          class="text-[13.5px] font-semibold leading-tight"
          :class="c.active ? 'text-primary' : ''"
        >{{ c.label }}</span>
        <span class="text-[11.5px] leading-snug text-muted">{{ c.desc }}</span>
      </button>
    </div>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] p-3.5 shadow-sm mb-4 flex items-end gap-2.5 flex-wrap">
      <div class="flex flex-col gap-1">
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.period') }}</span>
        <PeriodFilter v-model="period" />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.office') }}</span>
        <AsyncSearchPicker
          :model-value="officeId"
          :search-fn="office.searchFn"
          :resolve-fn="office.resolveFn"
          :placeholder="t('common.searchOffice')"
          testid="reports-office-filter"
          clearable
          class="min-w-[190px]"
          @update:model-value="officeId = $event"
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.category') }}</span>
        <USelect
          v-model="categoryId"
          value-key="value"
          :items="catOptions"
          data-testid="reports-category-filter"
          class="min-w-[150px]"
        />
      </div>
      <div
        v-if="showStatus"
        class="flex flex-col gap-1"
      >
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.status') }}</span>
        <USelect
          v-model="status"
          value-key="value"
          :items="statusOptions"
          data-testid="reports-status-filter"
          class="min-w-[150px]"
        />
      </div>
      <div
        v-if="showBasis"
        class="flex flex-col gap-1"
      >
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.basis') }}</span>
        <div
          class="flex gap-0.5 p-1 bg-muted rounded-[11px]"
          data-testid="reports-basis-toggle"
        >
          <button
            v-for="opt in BASIS_OPTIONS"
            :key="opt.key"
            type="button"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[13px] font-semibold transition-colors"
            :class="basis === opt.key ? 'bg-default shadow-sm text-default' : 'text-muted hover:text-default'"
            :data-testid="`reports-basis-${opt.key}`"
            @click="basis = opt.key"
          >
            <UIcon
              :name="opt.icon"
              class="size-3.5"
            />
            {{ t(`reports.basis.${opt.key}`) }}
          </button>
        </div>
      </div>
      <div class="flex-1" />
      <UButton
        icon="i-lucide-rotate-ccw"
        color="neutral"
        variant="outline"
        :label="t('reports.reset')"
        data-testid="reports-reset"
        @click="resetFilters"
      />
      <UButton
        icon="i-lucide-filter"
        :label="t('reports.apply')"
        :loading="loading"
        data-testid="reports-apply"
        @click="apply"
      />
    </div>

    <!-- Placeholder (before apply) -->
    <div
      v-if="!applied"
      class="bg-default border-[1.5px] border-dashed border-default rounded-2xl py-16 px-6 text-center"
    >
      <div class="size-[58px] mx-auto mb-4 rounded-[15px] bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-line-chart"
          class="size-7"
        />
      </div>
      <div class="text-base font-semibold mb-1.5">
        {{ t('reports.phTitle') }}
      </div>
      <div class="text-sm leading-relaxed text-muted max-w-[360px] mx-auto">
        {{ t('reports.phSub') }}
      </div>
    </div>

    <!-- Load error -->
    <div
      v-else-if="loadError"
      class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
    >
      <p class="text-sm text-muted mb-3">
        {{ t('reports.error') }}
      </p>
      <UButton
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-rotate-cw"
        data-testid="reports-retry"
        :label="t('common.retry')"
        @click="apply"
      />
    </div>

    <!-- Loading -->
    <div
      v-else-if="loading"
      class="bg-default border border-default rounded-2xl shadow-sm py-[60px] px-6 text-center"
      data-testid="reports-loading"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-7 animate-spin text-dimmed mx-auto mb-3"
      />
      <div class="text-sm text-muted">
        {{ t('reports.loading') }}
      </div>
    </div>

    <!-- Empty -->
    <div
      v-else-if="!hasData"
      class="bg-default border border-default rounded-2xl shadow-sm py-[60px] px-6 text-center"
    >
      <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-chart-no-axes-column"
          class="size-[26px]"
        />
      </div>
      <div class="text-base font-semibold mb-1.5">
        {{ t('reports.emptyTitle') }}
      </div>
      <div class="text-sm text-muted max-w-[340px] mx-auto mb-[18px]">
        {{ t('reports.emptySub') }}
      </div>
      <UButton
        color="neutral"
        variant="outline"
        :label="t('reports.reset')"
        data-testid="reports-empty-reset"
        @click="resetFilters"
      />
    </div>

    <!-- Result -->
    <div v-else-if="view">
      <div class="flex items-center justify-between gap-3.5 flex-wrap mb-3.5">
        <div>
          <div class="text-[17px] font-bold">
            {{ resultTitle }}
          </div>
          <div class="text-[12.5px] text-muted mt-px">
            {{ resultMeta }}
          </div>
        </div>
        <Can permission="report.export">
          <div class="flex gap-2.5">
            <UButton
              icon="i-lucide-file-text"
              color="neutral"
              variant="outline"
              size="sm"
              :label="t('reports.pdf')"
              :loading="exporting"
              data-testid="reports-export-pdf"
              @click="doExport('pdf')"
            />
            <UButton
              icon="i-lucide-file-spreadsheet"
              color="neutral"
              variant="outline"
              size="sm"
              :label="t('reports.excel')"
              :loading="exporting"
              data-testid="reports-export-xlsx"
              @click="doExport('xlsx')"
            />
            <UDropdownMenu
              v-if="report === 'disposals'"
              :items="glItems"
              :content="{ align: 'end' }"
            >
              <UButton
                icon="i-lucide-book-open"
                color="neutral"
                variant="outline"
                size="sm"
                :label="t('reports.exportGl')"
                :loading="exporting"
                data-testid="reports-export-gl"
              />
            </UDropdownMenu>
          </div>
        </Can>
      </div>

      <!-- KPI strip -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5 mb-4">
        <div
          v-for="(k, i) in view.kpis"
          :key="i"
          class="bg-default border border-default rounded-[13px] p-4 px-[18px] shadow-sm"
        >
          <div class="text-[12.5px] font-medium text-muted">
            {{ k.label }}
          </div>
          <div class="text-2xl font-bold tracking-tight my-1.5">
            {{ k.value }}
          </div>
          <div class="text-xs text-dimmed">
            {{ k.sub }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-4 items-start">
        <!-- Chart -->
        <div class="bg-default border border-default rounded-[13px] p-[18px] shadow-sm">
          <div class="text-sm font-semibold mb-4">
            {{ view.chartTitle }}
          </div>
          <div class="flex flex-col gap-3.5">
            <div
              v-for="(b, i) in view.chartBars"
              :key="i"
            >
              <div class="flex justify-between text-[12.5px] font-medium mb-1.5">
                <span class="text-muted">{{ b.label }}</span>
                <span class="font-semibold">{{ b.display }}</span>
              </div>
              <div class="h-2 rounded-full bg-muted overflow-hidden">
                <div
                  class="h-full rounded-full bg-primary"
                  :style="{ width: b.w }"
                />
              </div>
            </div>
          </div>
        </div>

        <!-- Table -->
        <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
          <div class="overflow-x-auto">
            <!-- Opname: rows carry a Berita Acara download action -->
            <table
              v-if="view.mode === 'opname'"
              class="w-full border-collapse text-[13px] whitespace-nowrap"
            >
              <thead>
                <tr class="bg-muted">
                  <th
                    v-for="(c, i) in view.cols"
                    :key="i"
                    class="px-4 py-[11px] text-[11.5px] font-semibold uppercase text-muted"
                    :class="c.align === 'right' ? 'text-right' : 'text-left'"
                  >
                    {{ c.label }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, ri) in view.rows"
                  :key="ri"
                  class="border-t border-default hover:bg-muted transition-colors"
                >
                  <td
                    v-for="(c, ci) in row.cells"
                    :key="ci"
                    class="px-4 py-[11px] tabular-nums"
                    :class="[c.align === 'right' ? 'text-right' : 'text-left', TONE_CLASS[c.tone], WEIGHT_CLASS[c.weight], c.mono ? 'font-mono' : '']"
                  >
                    {{ c.text }}
                  </td>
                  <td class="px-4 py-[11px] text-right">
                    <div
                      v-if="canExport"
                      class="flex gap-1.5 justify-end"
                    >
                      <UButton
                        icon="i-lucide-file-text"
                        color="neutral"
                        variant="outline"
                        size="xs"
                        :aria-label="t('reports.pdf')"
                        :data-testid="`reports-opname-ba-pdf-${row.sessionId}`"
                        @click="doOpnameBa(row.sessionId, 'pdf')"
                      />
                      <UButton
                        icon="i-lucide-file-spreadsheet"
                        color="neutral"
                        variant="outline"
                        size="xs"
                        :aria-label="t('reports.excel')"
                        :data-testid="`reports-opname-ba-xlsx-${row.sessionId}`"
                        @click="doOpnameBa(row.sessionId, 'xlsx')"
                      />
                    </div>
                    <span
                      v-else
                      class="text-dimmed"
                    >—</span>
                  </td>
                </tr>
              </tbody>
            </table>

            <!-- Generic report table -->
            <table
              v-else
              class="w-full border-collapse text-[13px] whitespace-nowrap"
            >
              <thead>
                <tr class="bg-muted">
                  <th
                    v-for="(c, i) in view.cols"
                    :key="i"
                    class="px-4 py-[11px] text-[11.5px] font-semibold uppercase text-muted"
                    :class="c.align === 'right' ? 'text-right' : 'text-left'"
                  >
                    {{ c.label }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, ri) in view.rows"
                  :key="ri"
                  class="border-t border-default hover:bg-muted transition-colors"
                >
                  <td
                    v-for="(c, ci) in row"
                    :key="ci"
                    class="px-4 py-[11px] tabular-nums"
                    :class="[c.align === 'right' ? 'text-right' : 'text-left', TONE_CLASS[c.tone], WEIGHT_CLASS[c.weight], c.mono ? 'font-mono' : '']"
                  >
                    {{ c.text }}
                  </td>
                </tr>
              </tbody>
              <tfoot v-if="view.footer">
                <tr class="border-t-2 border-default bg-muted">
                  <td
                    v-for="(c, ci) in view.footer"
                    :key="ci"
                    class="px-4 py-3 text-[13px] font-bold tabular-nums"
                    :class="[c.align === 'right' ? 'text-right' : 'text-left', TONE_CLASS[c.tone]]"
                  >
                    {{ c.text }}
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        </div>
      </div>

      <!-- Truncation notice -->
      <p
        v-if="result?.truncated"
        class="text-[12.5px] text-muted mt-3"
        data-testid="reports-truncated"
      >
        {{ t('reports.truncated', { n: result.row_count }) }}
      </p>
    </div>
  </div>
</template>
