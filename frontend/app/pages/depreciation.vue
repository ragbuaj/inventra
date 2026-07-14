<script setup lang="ts">
import type { Category } from '~/types'
import type { DepreciationPeriod, JournalResponse, ScheduleResponse, ScheduleRow } from '~/composables/api/useDepreciation'
import { BASIS_META, PERIOD_STATUS_TONE, type DepreciationBasis, type PeriodStatus } from '~/constants/depreciationMeta'
import { formatRupiah } from '~/utils/format'

definePageMeta({ middleware: 'can', permission: 'depreciation.view' })

const BASIS_OPTIONS: Array<{ key: DepreciationBasis, icon: string }> = [
  { key: 'commercial', icon: 'i-lucide-scale' },
  { key: 'fiscal', icon: 'i-lucide-gavel' }
]

const TONE_CLASS: Record<string, string> = {
  info: 'bg-info/15 text-info',
  neutral: 'bg-muted text-muted',
  primary: 'bg-primary/15 text-primary',
  warning: 'bg-warning/15 text-warning'
}

const { t, locale } = useI18n()
const can = useCan()
const toast = useToast()

const depApi = useDepreciation()
const categoriesApi = useCategories()
const office = useOfficePicker()

const canManage = computed(() => can('depreciation.manage'))

function periodLabel(p: string): string {
  const [y, m] = p.split('-').map(Number)
  if (!y || !m) return p
  const d = new Date(y, m - 1, 1)
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : 'id-ID', { month: 'long', year: 'numeric' }).format(d)
}

// ---------------------------------------------------------------------------
// Basis toggle — drives schedule + journal + KPIs.
// ---------------------------------------------------------------------------
const basis = ref<DepreciationBasis>('commercial')

// ---------------------------------------------------------------------------
// Periods + Jalankan Periode panel
// ---------------------------------------------------------------------------
const periods = ref<DepreciationPeriod[]>([])
const periodsLoading = ref(true)
const periodsError = ref(false)
const period = ref('')
const computing = ref(false)
const closing = ref(false)

const selectedPeriodObj = computed(() => periods.value.find(p => p.period === period.value) ?? null)
const periodStatus = computed<PeriodStatus>(() => selectedPeriodObj.value?.status ?? 'open')
const periodOptions = computed(() => periods.value.map(p => ({ value: p.period, label: periodLabel(p.period) })))
const canHitung = computed(() => periodStatus.value === 'open')
const canTutup = computed(() => periodStatus.value === 'computed')
const isClosed = computed(() => periodStatus.value === 'closed')
const isComputed = computed(() => periodStatus.value === 'computed')
// Mirrors the mockup's periodLocked behaviour: once the selected period has
// moved past "open", the period picker itself is locked.
const periodLocked = computed(() => periodStatus.value !== 'open')

// "Current period" = the newest period the backend knows about (not wall-clock
// "today"), so the reminder is deterministic and doesn't depend on the client
// clock drifting from the seeded/mocked period data.
const latestPeriodEntry = computed<DepreciationPeriod | null>(() => {
  if (periods.value.length === 0) return null
  return [...periods.value].sort((a, b) => a.period.localeCompare(b.period)).at(-1) ?? null
})
const showReminder = computed(() => latestPeriodEntry.value?.status === 'open')

function upsertPeriod(p: { period: string, status: PeriodStatus, asset_count?: number, total_amount?: string, skipped_count?: number }) {
  const idx = periods.value.findIndex(x => x.period === p.period)
  if (idx >= 0) {
    periods.value[idx] = { ...periods.value[idx]!, ...p }
  } else {
    periods.value.push({ period: p.period, status: p.status, asset_count: p.asset_count ?? 0, total_amount: p.total_amount ?? '0', skipped_count: p.skipped_count ?? 0 })
  }
}

async function loadPeriods() {
  periodsLoading.value = true
  periodsError.value = false
  try {
    periods.value = await depApi.periods()
    if (periods.value.length > 0 && !period.value) {
      period.value = [...periods.value].sort((a, b) => a.period.localeCompare(b.period)).at(-1)!.period
    }
  } catch {
    periodsError.value = true
  } finally {
    periodsLoading.value = false
  }
}

async function computePeriod() {
  if (!canManage.value || !period.value || computing.value) return
  computing.value = true
  try {
    const updated = await depApi.compute(period.value)
    upsertPeriod(updated)
    await Promise.all([loadSchedule(), loadJournal(), loadKpis()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    computing.value = false
  }
}

async function closePeriod() {
  if (!canManage.value || !period.value || closing.value) return
  closing.value = true
  try {
    const updated = await depApi.close(period.value)
    upsertPeriod(updated)
    await Promise.all([loadSchedule(), loadJournal(), loadKpis()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    closing.value = false
  }
}

// ---------------------------------------------------------------------------
// Lookups: categories for the schedule filter (best-effort). Office is an
// async search picker (no more eager `{ limit: 100 }` list) — see `office`.
// ---------------------------------------------------------------------------
const categories = ref<Category[]>([])

const categoryOptions = computed(() => [
  { value: 'all', label: t('depreciation.schedule.filterCategoryAll') },
  ...categories.value.map(c => ({ value: c.id, label: c.name }))
])

async function loadLookups() {
  try {
    categories.value = await categoriesApi.tree()
  } catch {
    // Best-effort — the filter just stays at "all".
  }
}

// ---------------------------------------------------------------------------
// Jadwal per Aset
// ---------------------------------------------------------------------------
const scheduleResp = ref<ScheduleResponse | null>(null)
const scheduleLoading = ref(true)
const scheduleError = ref(false)
const search = ref('')
const debouncedSearch = ref('')
const categoryId = ref('all')
const officeId = ref<string | null>(null)
let searchTimer: ReturnType<typeof setTimeout> | undefined

const scheduleRows = computed(() => scheduleResp.value?.rows ?? [])

watch(search, (v) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    debouncedSearch.value = v
  }, 300)
})

let scheduleSeq = 0
async function loadSchedule() {
  if (!period.value) {
    scheduleLoading.value = false
    return
  }
  const mine = ++scheduleSeq
  scheduleLoading.value = true
  scheduleError.value = false
  try {
    const res = await depApi.schedule({
      period: period.value,
      basis: basis.value,
      search: debouncedSearch.value.trim() || undefined,
      category_id: categoryId.value !== 'all' ? categoryId.value : undefined,
      office_id: officeId.value ?? undefined
    })
    if (mine !== scheduleSeq) return
    scheduleResp.value = res
  } catch {
    if (mine !== scheduleSeq) return
    scheduleError.value = true
    scheduleResp.value = null
  } finally {
    if (mine === scheduleSeq) scheduleLoading.value = false
  }
}

// ---------------------------------------------------------------------------
// KPI tiles — driven by an UNFILTERED schedule() call (period + basis only).
// The mockup computes KPIs across the full asset set unconditionally, so the
// table filters (search/category/office) must never shrink the tiles.
// ---------------------------------------------------------------------------
const kpiResp = ref<ScheduleResponse | null>(null)
const kpiLoading = ref(true)

let kpiSeq = 0
async function loadKpis() {
  if (!period.value) {
    kpiLoading.value = false
    return
  }
  const mine = ++kpiSeq
  kpiLoading.value = true
  try {
    const res = await depApi.schedule({ period: period.value, basis: basis.value })
    if (mine !== kpiSeq) return
    kpiResp.value = res
  } catch {
    if (mine !== kpiSeq) return
    kpiResp.value = null
  } finally {
    if (mine === kpiSeq) kpiLoading.value = false
  }
}

const kpiItems = computed(() => {
  const kpi = kpiResp.value?.kpi ?? null
  const kpiAssetCount = kpiResp.value?.rows.length ?? 0
  return [
    {
      key: 'acquisition', testid: 'depr-kpi-acquisition', icon: 'i-lucide-wallet', tone: 'info',
      label: t('depreciation.kpi.acquisition'), value: formatRupiah(kpi?.total_cost),
      sub: t('depreciation.kpi.acquisitionSub', { n: kpiAssetCount }), valueClass: ''
    },
    {
      key: 'accumulated', testid: 'depr-kpi-accumulated', icon: 'i-lucide-trending-down', tone: 'neutral',
      label: t('depreciation.kpi.accumulated'), value: formatRupiah(kpi?.total_accumulated),
      sub: t(BASIS_META[basis.value].refKey), valueClass: ''
    },
    {
      key: 'book-value', testid: 'depr-kpi-book-value', icon: 'i-lucide-book-open', tone: 'primary',
      label: t('depreciation.kpi.bookValue'), value: formatRupiah(kpi?.total_book_value),
      sub: period.value ? periodLabel(period.value) : '—', valueClass: ''
    },
    {
      key: 'period-expense', testid: 'depr-kpi-period-expense', icon: 'i-lucide-receipt', tone: 'warning',
      label: t('depreciation.kpi.periodExpense'), value: formatRupiah(kpi?.period_expense),
      sub: t('depreciation.kpi.periodExpenseSub'), valueClass: 'text-error'
    }
  ]
})

function methodTone(method: string): 'warning' | 'info' {
  return method === 'declining_balance' ? 'warning' : 'info'
}
function methodLabel(method: string): string {
  return method === 'declining_balance' ? t('depreciation.schedule.methodDecliningBalance') : t('depreciation.schedule.methodStraightLine')
}
function expenseFor(row: ScheduleRow): string {
  // Deviation (a): a fully-depreciated asset always shows a zero period
  // expense, regardless of any residual rounding the backend returns.
  return formatRupiah(row.fully_depreciated ? '0' : row.amount)
}
function impairTitleFor(): string {
  if (basis.value === 'fiscal') return t('depreciation.schedule.impairDisabledFiscalTooltip')
  if (!canManage.value) return t('depreciation.noManageNote')
  return t('depreciation.schedule.impairAction')
}
function impairDisabled(): boolean {
  return basis.value === 'fiscal' || !canManage.value
}

// ---------------------------------------------------------------------------
// Rekap Siap-Jurnal
// ---------------------------------------------------------------------------
const journalResp = ref<JournalResponse | null>(null)
const journalLoading = ref(true)
const journalError = ref(false)
const exportingPdf = ref(false)
const exportingXlsx = ref(false)

let journalSeq = 0
async function loadJournal() {
  if (!period.value) {
    journalLoading.value = false
    return
  }
  const mine = ++journalSeq
  journalLoading.value = true
  journalError.value = false
  try {
    const res = await depApi.journal(period.value, basis.value)
    if (mine !== journalSeq) return
    journalResp.value = res
  } catch {
    if (mine !== journalSeq) return
    journalError.value = true
    journalResp.value = null
  } finally {
    if (mine === journalSeq) journalLoading.value = false
  }
}

async function doExport(format: 'pdf' | 'xlsx') {
  if (!period.value) return
  const flag = format === 'pdf' ? exportingPdf : exportingXlsx
  flag.value = true
  try {
    const blob = await depApi.exportJournal(period.value, basis.value, format)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `jurnal-penyusutan-${period.value}-${basis.value}.${format}`
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    flag.value = false
  }
}

watch([period, basis], () => {
  loadSchedule()
  loadJournal()
  loadKpis()
})
// KPIs are intentionally NOT refetched here — the table filters must never
// shrink the tiles (they stay period+basis scoped).
watch([debouncedSearch, categoryId, officeId], () => loadSchedule())

// ---------------------------------------------------------------------------
// Impairment modal
// ---------------------------------------------------------------------------
const impairOpen = ref(false)
const impairTarget = ref<ScheduleRow | null>(null)
const impairRecoverRaw = ref('')
const impairReason = ref('')
const impairSubmitting = ref(false)

const impairLoss = computed<number | null>(() => {
  if (!impairTarget.value || impairRecoverRaw.value === '') return null
  const closingVal = Number(impairTarget.value.closing)
  const recoverVal = Number(impairRecoverRaw.value)
  return Math.max(0, closingVal - recoverVal)
})

function openImpair(row: ScheduleRow) {
  if (impairDisabled()) return
  impairTarget.value = row
  impairRecoverRaw.value = ''
  impairReason.value = ''
  impairOpen.value = true
}
function closeImpair() {
  impairOpen.value = false
}

async function saveImpair() {
  const row = impairTarget.value
  if (!row || impairSubmitting.value || impairRecoverRaw.value === '') return
  impairSubmitting.value = true
  try {
    await depApi.recordImpairment(row.asset_id, impairRecoverRaw.value, impairReason.value.trim())
    impairOpen.value = false
    toast.add({ title: t('depreciation.impairment.title'), description: row.asset_name, color: 'success' })
    await loadSchedule()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    impairSubmitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Section tabs
// ---------------------------------------------------------------------------
type SectionKey = 'schedule' | 'journal'
const section = ref<SectionKey>('schedule')
const sectionTabs = computed(() => [
  { key: 'schedule' as const, label: t('depreciation.section.schedule'), icon: 'i-lucide-book-text' },
  { key: 'journal' as const, label: t('depreciation.section.journal'), icon: 'i-lucide-book' }
])

onMounted(() => {
  loadPeriods()
  loadLookups()
})

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <div class="max-w-[1060px] mx-auto">
    <!-- Header + basis toggle -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-[18px]">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('depreciation.title') }}
        </h1>
        <p class="text-sm text-muted">
          {{ t('depreciation.subtitle') }}
        </p>
      </div>
      <div
        class="flex gap-0.5 p-1 bg-muted rounded-[11px]"
        data-testid="depr-basis-toggle"
      >
        <button
          v-for="opt in BASIS_OPTIONS"
          :key="opt.key"
          type="button"
          class="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-[13px] font-semibold transition-colors"
          :class="basis === opt.key ? 'bg-default shadow-sm text-default' : 'text-muted hover:text-default'"
          :data-testid="`depr-basis-${opt.key}`"
          @click="basis = opt.key"
        >
          <UIcon
            :name="opt.icon"
            class="size-3.5"
          />
          {{ t(`depreciation.basis.${opt.key}`) }}
          <span
            class="px-1.5 py-0 rounded text-[9px] font-semibold"
            :class="basis === opt.key ? 'bg-muted text-muted' : 'text-dimmed'"
          >
            {{ t(BASIS_META[opt.key].refKey) }}
          </span>
        </button>
      </div>
    </div>

    <!-- Reminder banner (deviation c): current period open & never computed -->
    <div
      v-if="showReminder"
      data-testid="depr-reminder"
      class="flex gap-2.5 items-start px-4 py-3 mb-4 rounded-xl bg-warning/10 border border-warning/30"
    >
      <UIcon
        name="i-lucide-alert-triangle"
        class="size-[17px] flex-none mt-0.5 text-warning"
      />
      <div>
        <div class="text-[13px] font-semibold text-warning">
          {{ t('depreciation.period.reminderTitle') }}
        </div>
        <div class="text-[12.5px] leading-relaxed text-muted mt-0.5">
          {{ t('depreciation.period.reminderBody') }}
        </div>
      </div>
    </div>

    <!-- KPI tiles -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5 mb-4">
      <div
        v-for="k in kpiItems"
        :key="k.key"
        :data-testid="k.testid"
        class="bg-default border border-default rounded-[13px] p-4 shadow-sm"
      >
        <div class="flex items-center gap-2">
          <span
            class="size-[30px] rounded-lg flex items-center justify-center flex-none"
            :class="TONE_CLASS[k.tone]"
          >
            <UIcon
              :name="k.icon"
              class="size-[15px]"
            />
          </span>
          <span class="text-xs font-medium text-muted">{{ k.label }}</span>
        </div>
        <USkeleton
          v-if="kpiLoading"
          class="h-6 w-24 rounded mt-2"
        />
        <div
          v-else
          class="text-[22px] font-bold tracking-tight mt-2"
          :class="k.valueClass"
        >
          {{ k.value }}
        </div>
        <div class="text-[11.5px] text-dimmed mt-0.5">
          {{ k.sub }}
        </div>
      </div>
    </div>

    <!-- Jalankan Periode panel -->
    <div
      v-if="periodsLoading"
      class="bg-default border border-default rounded-2xl shadow-sm p-5 mb-4"
    >
      <USkeleton class="h-10 w-full rounded-lg" />
    </div>
    <div
      v-else-if="periodsError"
      class="bg-default border border-default rounded-2xl shadow-sm py-8 px-6 text-center mb-4"
    >
      <p class="text-sm text-muted mb-3">
        {{ t('common.loadError') }}
      </p>
      <UButton
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-rotate-cw"
        data-testid="depr-periods-retry"
        @click="loadPeriods"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>
    <div
      v-else
      class="bg-default border border-default rounded-2xl shadow-sm p-5 mb-4"
    >
      <div class="flex items-center gap-4 flex-wrap">
        <div class="flex items-center gap-2.5 flex-1 min-w-[260px]">
          <span class="size-10 rounded-[11px] bg-primary/15 text-primary flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-calendar-check"
              class="size-5"
            />
          </span>
          <div>
            <div class="text-[15px] font-semibold">
              {{ t('depreciation.period.runTitle') }}
            </div>
            <div class="flex items-center gap-2 mt-0.5">
              <span class="text-[12.5px] text-muted">{{ t('depreciation.period.prefix') }}</span>
              <UBadge
                :color="PERIOD_STATUS_TONE[periodStatus]"
                variant="subtle"
                class="rounded-full gap-1.5"
              >
                <span class="size-1.5 rounded-full bg-current" />
                {{ t(`depreciation.period.status.${periodStatus}`) }}
              </UBadge>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2.5 flex-wrap">
          <USelect
            v-model="period"
            data-testid="depr-period-select"
            value-key="value"
            :items="periodOptions"
            :disabled="periodLocked"
            class="min-w-[150px]"
          />
          <UButton
            v-if="canHitung"
            data-testid="depr-compute"
            icon="i-lucide-calculator"
            :label="t('depreciation.period.compute')"
            :loading="computing"
            :disabled="!canManage"
            @click="computePeriod"
          />
          <UButton
            v-if="canTutup"
            data-testid="depr-close"
            icon="i-lucide-lock"
            :label="t('depreciation.period.close')"
            :loading="closing"
            :disabled="!canManage"
            @click="closePeriod"
          />
          <span
            v-if="isClosed"
            class="inline-flex items-center gap-1.5 px-3.5 py-2 text-[13px] font-medium rounded-lg bg-muted text-muted"
          >
            <UIcon
              name="i-lucide-lock"
              class="size-[15px]"
            />
            {{ t('depreciation.period.closed') }}
          </span>
        </div>
      </div>
      <div class="flex gap-6 flex-wrap items-center mt-3.5 pt-3.5 border-t border-default">
        <div class="flex items-center gap-2">
          <span class="text-[12.5px] text-muted">{{ t('depreciation.period.previewAssets') }}</span>
          <span class="text-[15px] font-bold">{{ selectedPeriodObj?.asset_count ?? 0 }}</span>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-[12.5px] text-muted">{{ t('depreciation.period.previewExpense') }}</span>
          <span class="text-[15px] font-bold text-info">{{ formatRupiah(selectedPeriodObj?.total_amount) }}</span>
        </div>
        <div
          v-if="isComputed"
          class="inline-flex items-center gap-1.5 text-[12.5px] font-medium text-success"
        >
          <UIcon
            name="i-lucide-circle-check"
            class="size-3.5"
          />
          {{ t('depreciation.period.computedNote') }}
        </div>
      </div>
      <div
        v-if="!canManage"
        data-testid="depr-no-manage"
        class="flex gap-2 items-center px-3 py-2.5 mt-3.5 rounded-[10px] bg-muted border border-default text-muted text-[12.5px] leading-snug font-medium"
      >
        <UIcon
          name="i-lucide-lock"
          class="size-4 flex-none"
        />
        {{ t('depreciation.noManageNote') }}
      </div>
    </div>

    <!-- Section tabs -->
    <div class="flex gap-1 border-b border-default mb-[18px]">
      <button
        v-for="tb in sectionTabs"
        :key="tb.key"
        class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-sm border-b-2 transition-colors"
        :class="section === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
        :data-testid="`depr-tab-${tb.key}`"
        @click="section = tb.key"
      >
        <UIcon
          :name="tb.icon"
          class="size-[15px]"
        />
        {{ tb.label }}
      </button>
    </div>

    <!-- ===== JADWAL PER ASET ===== -->
    <div v-if="section === 'schedule'">
      <div class="flex items-center gap-2.5 flex-wrap mb-3.5">
        <UInput
          v-model="search"
          data-testid="depr-search"
          icon="i-lucide-search"
          :placeholder="t('depreciation.schedule.searchPlaceholder')"
          class="flex-1 min-w-[200px] max-w-[300px]"
        />
        <USelect
          v-model="categoryId"
          data-testid="depr-filter-category"
          value-key="value"
          :items="categoryOptions"
          class="min-w-[170px]"
        />
        <AsyncSearchPicker
          :model-value="officeId"
          :search-fn="office.searchFn"
          :resolve-fn="office.resolveFn"
          :placeholder="t('common.searchOffice')"
          testid="depr-filter-office"
          clearable
          class="min-w-[190px]"
          @update:model-value="officeId = $event"
        />
      </div>

      <div
        v-if="scheduleLoading"
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <USkeleton class="h-[42px] w-full rounded-none" />
        <div
          v-for="n in 5"
          :key="n"
          class="flex items-center gap-4 px-4 py-3.5 border-t border-default"
        >
          <USkeleton class="h-3 w-[150px] rounded" />
          <USkeleton class="h-3 flex-1 rounded" />
        </div>
      </div>

      <div
        v-else-if="scheduleError"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          data-testid="depr-schedule-retry"
          @click="loadSchedule"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.asset') }}
                </th>
                <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.method') }}
                </th>
                <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.life') }}
                </th>
                <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.opening') }}
                </th>
                <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.expense') }}
                </th>
                <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.accumulated') }}
                </th>
                <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.closing') }}
                </th>
                <th class="text-right px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.schedule.column.actions') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in scheduleRows"
                :key="row.asset_id"
                data-testid="depr-schedule-row"
                class="border-t border-default hover:bg-muted/60 transition-colors"
              >
                <td class="px-4 py-3">
                  <div class="flex items-center gap-1.5">
                    <span class="font-medium">{{ row.asset_name }}</span>
                    <UIcon
                      v-if="row.impaired"
                      name="i-lucide-trending-down"
                      :title="t('depreciation.schedule.impairedTooltip')"
                      class="size-3.5 text-violet-600 dark:text-violet-400"
                    />
                  </div>
                  <div class="font-mono text-[11px] text-dimmed">
                    {{ row.asset_tag }}
                  </div>
                </td>
                <td class="px-3 py-3">
                  <UBadge
                    :color="methodTone(row.method)"
                    variant="subtle"
                    class="rounded-full"
                  >
                    {{ methodLabel(row.method) }}
                  </UBadge>
                </td>
                <td class="px-3 py-3 text-right text-muted tabular-nums">
                  {{ row.life_months }}
                </td>
                <td class="px-3 py-3 text-right tabular-nums text-muted">
                  {{ formatRupiah(row.opening) }}
                </td>
                <td class="px-3 py-3 text-right tabular-nums text-error font-medium">
                  {{ expenseFor(row) }}
                </td>
                <td class="px-3 py-3 text-right tabular-nums text-muted">
                  {{ formatRupiah(row.accumulated) }}
                </td>
                <td class="px-3 py-3 text-right tabular-nums font-semibold">
                  {{ formatRupiah(row.closing) }}
                </td>
                <td class="px-4 py-3 text-right">
                  <button
                    type="button"
                    data-testid="depr-impair"
                    :disabled="impairDisabled()"
                    :title="impairTitleFor()"
                    class="inline-flex items-center justify-center size-[30px] rounded-lg border border-strong text-muted transition-colors"
                    :class="impairDisabled() ? 'opacity-50 cursor-not-allowed' : 'hover:bg-violet-500/10 hover:text-violet-600 hover:border-transparent cursor-pointer'"
                    @click="openImpair(row)"
                  >
                    <UIcon
                      name="i-lucide-trending-down"
                      class="size-[15px]"
                    />
                  </button>
                </td>
              </tr>
            </tbody>
            <tfoot>
              <tr class="border-t-2 border-strong bg-muted">
                <td
                  class="px-4 py-3 font-bold text-[12.5px]"
                  colspan="3"
                >
                  {{ t('depreciation.total') }}
                </td>
                <td class="px-3 py-3 text-right font-bold text-[12.5px] tabular-nums">
                  {{ formatRupiah(scheduleResp?.totals?.opening) }}
                </td>
                <td class="px-3 py-3 text-right font-bold text-[12.5px] tabular-nums text-error">
                  {{ formatRupiah(scheduleResp?.totals?.amount) }}
                </td>
                <td class="px-3 py-3 text-right font-bold text-[12.5px] tabular-nums">
                  {{ formatRupiah(scheduleResp?.totals?.accumulated) }}
                </td>
                <td class="px-3 py-3 text-right font-bold text-[12.5px] tabular-nums">
                  {{ formatRupiah(scheduleResp?.totals?.closing) }}
                </td>
                <td />
              </tr>
            </tfoot>
          </table>
        </div>
        <div
          v-if="scheduleRows.length === 0"
          data-testid="depr-schedule-empty"
          class="py-[34px] px-4 text-center text-[13px] text-dimmed"
        >
          {{ t('depreciation.schedule.noMatch') }}
        </div>
      </div>
    </div>

    <!-- ===== REKAP SIAP-JURNAL ===== -->
    <div v-else>
      <div class="flex items-center justify-between gap-3 flex-wrap mb-3.5">
        <div class="text-[13px] text-muted">
          {{ t('depreciation.journal.subtitle') }}
        </div>
        <div class="flex gap-2.5">
          <UButton
            data-testid="depr-export-pdf"
            color="neutral"
            variant="outline"
            icon="i-lucide-file-text"
            :label="t('depreciation.journal.exportPdf')"
            :loading="exportingPdf"
            @click="doExport('pdf')"
          />
          <UButton
            data-testid="depr-export-xlsx"
            color="neutral"
            variant="outline"
            icon="i-lucide-file-spreadsheet"
            :label="t('depreciation.journal.exportExcel')"
            :loading="exportingXlsx"
            @click="doExport('xlsx')"
          />
        </div>
      </div>

      <div
        v-if="journalLoading"
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <USkeleton class="h-[42px] w-full rounded-none" />
        <div
          v-for="n in 4"
          :key="n"
          class="flex items-center gap-4 px-4 py-3.5 border-t border-default"
        >
          <USkeleton class="h-3 w-[150px] rounded" />
          <USkeleton class="h-3 flex-1 rounded" />
        </div>
      </div>

      <div
        v-else-if="journalError"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          data-testid="depr-journal-retry"
          @click="loadJournal"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <div class="px-[18px] py-3.5 border-b border-default">
          <div class="text-sm font-semibold">
            {{ t('depreciation.journal.title') }}
          </div>
          <div class="text-xs text-muted mt-0.5">
            {{ t(`depreciation.basis.${basis}`) }} · {{ period ? periodLabel(period) : '—' }}
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.journal.column.account') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.journal.column.name') }}
                </th>
                <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.journal.column.debit') }}
                </th>
                <th class="text-right px-[18px] py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('depreciation.journal.column.credit') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, i) in (journalResp?.rows ?? [])"
                :key="i"
                data-testid="depr-journal-row"
                class="border-t border-default hover:bg-muted/60 transition-colors"
              >
                <td class="px-[18px] py-3 font-mono text-[12.5px] text-muted">
                  {{ row.account_code }}
                </td>
                <td class="px-3.5 py-3">
                  {{ row.account_name }}
                </td>
                <td
                  class="px-3.5 py-3 text-right tabular-nums"
                  :class="Number(row.debit) > 0 ? 'text-default' : 'text-dimmed'"
                >
                  {{ Number(row.debit) > 0 ? formatRupiah(row.debit) : '—' }}
                </td>
                <td
                  class="px-[18px] py-3 text-right tabular-nums"
                  :class="Number(row.credit) > 0 ? 'text-default' : 'text-dimmed'"
                >
                  {{ Number(row.credit) > 0 ? formatRupiah(row.credit) : '—' }}
                </td>
              </tr>
            </tbody>
            <tfoot>
              <tr class="border-t-2 border-strong bg-muted">
                <td
                  class="px-[18px] py-3.5 font-bold text-[13px]"
                  colspan="2"
                >
                  {{ t('depreciation.total') }}
                </td>
                <td class="px-3.5 py-3.5 text-right font-bold text-[13px] tabular-nums">
                  {{ formatRupiah(journalResp?.total_debit) }}
                </td>
                <td class="px-[18px] py-3.5 text-right font-bold text-[13px] tabular-nums">
                  {{ formatRupiah(journalResp?.total_credit) }}
                </td>
              </tr>
            </tfoot>
          </table>
        </div>
        <div
          v-if="journalResp?.balanced"
          data-testid="depr-journal-balanced"
          class="flex items-center gap-2 px-[18px] py-3.5 border-t border-default bg-success/10"
        >
          <UIcon
            name="i-lucide-circle-check"
            class="size-[15px] text-success"
          />
          <span class="text-[12.5px] font-medium text-success">{{ t('depreciation.journal.balanced') }}</span>
        </div>
      </div>
    </div>

    <!-- Impairment modal -->
    <UModal
      v-model:open="impairOpen"
      :title="t('depreciation.impairment.title')"
      :description="t('depreciation.impairment.ref')"
    >
      <template #body>
        <div
          v-if="impairTarget"
          class="flex flex-col gap-4"
        >
          <div class="px-3.5 py-3 rounded-[10px] bg-muted">
            <div class="text-[13.5px] font-semibold">
              {{ impairTarget.asset_name }}
            </div>
            <div class="font-mono text-[11.5px] text-dimmed mt-px">
              {{ impairTarget.asset_tag }}
            </div>
            <div class="flex items-center justify-between mt-2.5">
              <span class="text-xs text-muted">{{ t('depreciation.impairment.currentBookValue') }}</span>
              <span
                data-testid="depr-impair-current-value"
                class="text-sm font-semibold"
              >{{ formatRupiah(impairTarget.closing) }}</span>
            </div>
          </div>
          <UFormField
            :label="t('depreciation.impairment.recoverableAmount')"
            required
          >
            <NumberInput
              v-model="impairRecoverRaw"
              money
              data-testid="depr-impair-recoverable"
              class="w-full"
            />
          </UFormField>
          <div
            v-if="impairLoss !== null"
            data-testid="depr-impair-loss"
            class="flex items-center justify-between px-3.5 py-2.5 rounded-[10px] bg-error/10"
          >
            <span class="text-[12.5px] font-medium text-error">{{ t('depreciation.impairment.loss') }}</span>
            <span class="text-[15px] font-bold text-error">− {{ formatRupiah(impairLoss) }}</span>
          </div>
          <UFormField :label="t('depreciation.impairment.reason')">
            <UTextarea
              v-model="impairReason"
              data-testid="depr-impair-reason"
              :rows="3"
              :placeholder="t('depreciation.impairment.reasonPlaceholder')"
              class="w-full"
            />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            color="neutral"
            variant="ghost"
            @click="closeImpair"
          >
            {{ t('depreciation.impairment.cancel') }}
          </UButton>
          <UButton
            data-testid="depr-impair-save"
            class="bg-violet-600 hover:bg-violet-700 text-white"
            :disabled="impairRecoverRaw === ''"
            :loading="impairSubmitting"
            @click="saveImpair"
          >
            {{ t('depreciation.impairment.save') }}
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>
