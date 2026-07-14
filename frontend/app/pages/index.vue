<script setup lang="ts">
import type { DashboardSummary, MaintenanceItem, ApprovalItem } from '~/composables/api/useDashboard'
import { useDashboard } from '~/composables/api/useDashboard'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'
import { useApproval } from '~/composables/api/useApproval'
import type { PeriodValue } from '~/constants/reportMeta'
import { formatMoneyShort, formatTrendPct } from '~/constants/reportMeta'
import { TYPE_META } from '~/constants/approvalMeta'
import { buildDonut, barWidths, formatCount, dueLabel, dueDiffDays } from '~/utils/dashboard'

interface KpiVM {
  label: string
  value: string
  icon: string
  iconTone: 'primary' | 'info' | 'neutral' | 'error' | 'warning'
  trendIcon: string
  trendText: string
  trendTone: 'success' | 'muted' | 'error' | 'warning'
}

const { t } = useI18n()
const toast = useToast()
const can = useCan()

const { summary, exportSummary } = useDashboard()
const approvalApi = useApproval()
const officesApi = useOffices()
const office = useOfficePicker()

const canDecide = computed(() => can('request.decide'))
const canExport = computed(() => can('report.export'))

// ---------------------------------------------------------------------------
// Filters + state
// ---------------------------------------------------------------------------
const officeId = ref<string | null>(null)
const period = ref<PeriodValue>({ preset: 'last30' })
const data = ref<DashboardSummary | null>(null)
const inboxItems = ref<ApprovalRequestRow[]>([])
// Only the caller's in-scope office *count* is needed (to decide whether to
// show the switcher at all) — no more eager `{ limit: 100 }` office list; the
// switcher itself is an async search picker.
const officeCount = ref(0)
const loading = ref(true)
const loadError = ref(false)
const busy = ref<Set<string>>(new Set())
const exporting = ref<'pdf' | 'xlsx' | null>(null)

// Hide the whole control when the caller's scope holds a single office (nothing to switch).
const showOfficeSelect = computed(() => officeCount.value > 1)

const scopeName = computed(() => data.value?.office_name ?? t('dashboard.scopeAll'))

function currentQuery() {
  return { officeId: officeId.value ?? undefined, period: period.value }
}

function onOfficeChange(id: string | null) {
  officeId.value = id
  load()
}

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const [summ, inbox] = await Promise.all([
      summary(currentQuery()),
      canDecide.value ? approvalApi.inbox() : Promise.resolve([] as ApprovalRequestRow[])
    ])
    data.value = summ
    inboxItems.value = inbox
  } catch {
    loadError.value = true
    data.value = null
    inboxItems.value = []
  } finally {
    loading.value = false
  }
}

async function loadOfficeCount() {
  try {
    const res = await officesApi.list({ limit: 1 })
    officeCount.value = res.total
  } catch {
    officeCount.value = 0
  }
}

// ---------------------------------------------------------------------------
// KPI view-model
// ---------------------------------------------------------------------------
const kpis = computed<KpiVM[]>(() => {
  const d = data.value
  if (!d) return []
  const k = d.kpi
  const acqPct = formatTrendPct(k.trends.acquisition_pct)
  const bookPct = formatTrendPct(k.trends.book_value_pct)
  const costPct = formatTrendPct(k.trends.maintenance_cost_pct)
  const upDown = (pct: number | null) => (pct !== null && pct < 0 ? 'i-lucide-trending-down' : 'i-lucide-trending-up')
  return [
    {
      label: t('dashboard.kpi.total'), value: formatCount(k.total_assets),
      icon: 'i-lucide-package', iconTone: 'primary',
      trendIcon: 'i-lucide-trending-up', trendText: t('dashboard.kpiTrend.growing'), trendTone: 'success'
    },
    {
      label: t('dashboard.kpi.acquisition'), value: formatMoneyShort(k.acquisition_value),
      icon: 'i-lucide-wallet', iconTone: 'info',
      trendIcon: upDown(k.trends.acquisition_pct), trendText: acqPct ?? t('dashboard.kpiTrend.stable'), trendTone: 'success'
    },
    {
      label: t('dashboard.kpi.bookValue'), value: formatMoneyShort(k.book_value),
      icon: 'i-lucide-trending-down', iconTone: 'neutral',
      trendIcon: upDown(k.trends.book_value_pct), trendText: bookPct ?? t('dashboard.kpiTrend.stable'), trendTone: 'muted'
    },
    {
      label: t('dashboard.kpi.overdue'), value: formatCount(k.overdue_assets),
      icon: 'i-lucide-clock-alert', iconTone: 'error',
      trendIcon: 'i-lucide-triangle-alert', trendText: t('dashboard.kpiTrend.needsAction'), trendTone: 'error'
    },
    {
      label: t('dashboard.kpi.maintenanceDue'), value: formatCount(k.maintenance_due),
      icon: 'i-lucide-wrench', iconTone: 'warning',
      trendIcon: 'i-lucide-triangle-alert', trendText: t('dashboard.kpiTrend.within7Days'), trendTone: 'warning'
    },
    {
      label: t('dashboard.kpi.maintenanceCost'), value: formatMoneyShort(k.maintenance_cost),
      icon: 'i-lucide-receipt', iconTone: 'warning',
      trendIcon: upDown(k.trends.maintenance_cost_pct), trendText: costPct ?? t('dashboard.kpiTrend.stable'), trendTone: 'warning'
    }
  ]
})

// ---------------------------------------------------------------------------
// Charts
// ---------------------------------------------------------------------------
const donut = computed(() => buildDonut((data.value?.by_status ?? []).map(s => s.count)))
const kategoriBars = computed(() =>
  barWidths((data.value?.by_category ?? []).map(i => [i.name ?? t('dashboard.noRoom'), i.count]))
)
const lokasiBars = computed(() =>
  barWidths((data.value?.by_location ?? []).map(i => [i.name ?? t('dashboard.noRoom'), i.count]))
)
const locationTitle = computed(() =>
  data.value?.location_kind === 'room' ? t('dashboard.chart.locationRooms') : t('dashboard.chart.locationOffices')
)

// ---------------------------------------------------------------------------
// Panels
// ---------------------------------------------------------------------------
const maintItems = computed<MaintenanceItem[]>(() =>
  (data.value?.maintenance_due_list ?? []).map((m) => {
    const dl = dueLabel(m.next_due_date)
    return {
      asset: `${m.asset_name} · ${m.asset_tag}`,
      task: m.category_name ?? t('dashboard.panel.maintenanceGeneric'),
      icon: 'i-lucide-wrench',
      urg: dueDiffDays(m.next_due_date) <= 1 ? 1 : 0,
      due: t(dl.key, dl.n !== undefined ? { n: dl.n } : {})
    }
  })
)

function apprTone(tone?: string): ApprovalItem['tone'] {
  if (tone === 'info') return 'info'
  if (tone === 'primary') return 'primary'
  return 'neutral'
}

const apprItems = computed<ApprovalItem[]>(() =>
  inboxItems.value.slice(0, 5).map((row) => {
    const meta = TYPE_META[row.type]
    return {
      id: row.id,
      title: t(`approval.type.${row.type}`) + (row.office_name ? ` — ${row.office_name}` : ''),
      meta: `${row.requested_by_name ?? '—'} · ${row.requested_by_role ?? '—'}`,
      icon: meta?.icon ?? 'i-lucide-file',
      tone: apprTone(meta?.tone)
    }
  })
)

// ---------------------------------------------------------------------------
// Approve / reject
// ---------------------------------------------------------------------------
async function approve(id: string) {
  if (busy.value.has(id)) return
  busy.value = new Set([...busy.value, id])
  try {
    await approvalApi.approve(id)
    toast.add({ title: t('dashboard.panel.approvedToast'), color: 'success', icon: 'i-lucide-check' })
    await load()
  } catch {
    // useApiClient surfaces the error toast — just release the guard below.
  } finally {
    const next = new Set(busy.value)
    next.delete(id)
    busy.value = next
  }
}

const rejectOpen = ref(false)
const rejectTargetId = ref<string | null>(null)

function reject(id: string) {
  rejectTargetId.value = id
  rejectOpen.value = true
}

async function onRejectConfirm(note: string) {
  const id = rejectTargetId.value
  rejectTargetId.value = null
  if (!id) return
  try {
    await approvalApi.reject(id, note)
    toast.add({ title: t('dashboard.panel.rejectedToast'), color: 'neutral', icon: 'i-lucide-x' })
    await load()
  } catch {
    // useApiClient surfaces the error toast.
  }
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------
const periodSlug = computed(() => {
  const p = period.value
  if (p.preset === 'custom' && p.from && p.to) return `${p.from}_${p.to}`
  return p.preset
})

async function doExport(format: 'pdf' | 'xlsx') {
  if (exporting.value) return
  exporting.value = format
  try {
    const blob = await exportSummary(currentQuery(), format)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `dashboard-${periodSlug.value}.${format}`
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
  } catch {
    // useApiClient surfaces the error toast.
  } finally {
    exporting.value = null
  }
}

const exportItems = computed(() => [[
  { 'label': t('dashboard.export.pdf'), 'icon': 'i-lucide-file-text', 'data-testid': 'dashboard-export-pdf', 'onSelect': () => doExport('pdf') },
  { 'label': t('dashboard.export.xlsx'), 'icon': 'i-lucide-file-spreadsheet', 'data-testid': 'dashboard-export-xlsx', 'onSelect': () => doExport('xlsx') }
]])

const skeletonKpis = [0, 1, 2, 3, 4, 5]

onMounted(() => {
  load()
  loadOfficeCount()
})

// Driving the teleported dropdown menu via DOM is brittle (see PeriodFilter);
// expose doExport so the export flow is testable deterministically.
defineExpose({ doExport, load, officeId })
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-[22px]">
      <div>
        <h1 class="text-2xl font-bold tracking-tight mb-1.5">
          {{ t('dashboard.title') }}
        </h1>
        <div class="flex items-center gap-2 flex-wrap">
          <span class="inline-flex items-center gap-1.5 text-[13px] font-medium text-muted">
            <UIcon
              name="i-lucide-building-2"
              class="size-[15px]"
            />
            {{ scopeName }}
          </span>
          <span class="inline-flex items-center gap-1.5 px-[9px] py-0.5 text-[11.5px] font-medium rounded-full bg-info/10 text-info">
            <UIcon
              name="i-lucide-info"
              class="size-3"
            />
            {{ t('dashboard.scopeNote') }}
          </span>
        </div>
      </div>
      <div class="flex items-center gap-[10px] flex-wrap">
        <PeriodFilter
          v-model="period"
          label-base="dashboard.period"
          @update:model-value="load"
        />
        <AsyncSearchPicker
          v-if="showOfficeSelect"
          :model-value="officeId"
          :search-fn="office.searchFn"
          :resolve-fn="office.resolveFn"
          :placeholder="t('common.searchOffice')"
          testid="dashboard-office"
          clearable
          class="min-w-[200px]"
          @update:model-value="onOfficeChange"
        />
        <UButton
          color="neutral"
          variant="outline"
          square
          :aria-label="t('dashboard.reload')"
          :disabled="loading"
          @click="load"
        >
          <UIcon
            name="i-lucide-refresh-cw"
            class="size-5"
            :class="{ 'animate-spin': loading }"
          />
        </UButton>
        <UDropdownMenu
          v-if="canExport"
          :items="exportItems"
          :content="{ align: 'end' }"
        >
          <UButton
            icon="i-lucide-download"
            color="neutral"
            variant="outline"
            :label="t('dashboard.export.label')"
            :loading="!!exporting"
            data-testid="dashboard-export"
          />
        </UDropdownMenu>
      </div>
    </div>

    <!-- Load error -->
    <div
      v-if="loadError"
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
        data-testid="dashboard-retry"
        @click="load"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>

    <template v-else>
      <!-- KPI row -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-[18px]">
        <template v-if="!loading">
          <DashboardKpiCard
            v-for="(kpi, i) in kpis"
            :key="i"
            v-bind="kpi"
          />
        </template>
        <template v-else>
          <div
            v-for="n in skeletonKpis"
            :key="n"
            class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm"
          >
            <div class="flex justify-between">
              <USkeleton class="h-3 w-[46%] rounded" />
              <USkeleton class="size-8 rounded-[9px]" />
            </div>
            <USkeleton class="h-6 w-[62%] rounded mt-[14px] mb-[10px]" />
            <USkeleton class="h-2.5 w-[38%] rounded" />
          </div>
        </template>
      </div>

      <!-- Valuation-exclusion transparency note -->
      <p
        v-if="!loading && data && data.excluded_count > 0"
        data-testid="dashboard-excluded-note"
        class="text-[12.5px] text-muted -mt-2 mb-[18px]"
      >
        {{ t('dashboard.excludedNote', { n: data.excluded_count }) }}
      </p>

      <!-- Charts row -->
      <div class="grid grid-cols-1 lg:grid-cols-[1.05fr_1fr_1fr] gap-4 mb-[18px]">
        <template v-if="!loading">
          <DashboardDonut
            :title="t('dashboard.chart.statusTitle')"
            :total="donut.total"
            :total-label="t('dashboard.totalLabel')"
            :segments="donut.segments"
          />
          <DashboardBarList
            :title="t('dashboard.chart.categoryTitle')"
            :items="kategoriBars"
            color="primary"
          />
          <DashboardBarList
            :title="locationTitle"
            :items="lokasiBars"
            color="info"
          />
        </template>
        <template v-else>
          <div
            v-for="n in 3"
            :key="n"
            class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm"
          >
            <USkeleton class="h-3 w-[48%] rounded mb-[18px]" />
            <div class="flex flex-col gap-4">
              <USkeleton class="h-2 w-full rounded-full" />
              <USkeleton class="h-2 w-full rounded-full" />
              <USkeleton class="h-2 w-full rounded-full" />
              <USkeleton class="h-2 w-full rounded-full" />
            </div>
          </div>
        </template>
      </div>

      <!-- Panels row -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <template v-if="!loading">
          <DashboardMaintenancePanel
            :title="t('dashboard.panel.maintenanceTitle')"
            :see-all-label="t('dashboard.panel.seeAll')"
            :items="maintItems"
            @see-all="navigateTo('/maintenance')"
          />
          <DashboardApprovalPanel
            v-if="canDecide"
            :title="t('dashboard.panel.approvalTitle')"
            :items="apprItems"
            :count="inboxItems.length"
            :empty-title="t('dashboard.panel.allHandledTitle')"
            :empty-sub="t('dashboard.panel.allHandledSub')"
            @approve="approve"
            @reject="reject"
          />
        </template>
        <template v-else>
          <div
            v-for="n in 2"
            :key="n"
            class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm"
          >
            <USkeleton class="h-3 w-[42%] rounded mb-[18px]" />
            <div class="flex flex-col gap-4">
              <USkeleton class="h-[34px] w-full rounded-[9px]" />
              <USkeleton class="h-[34px] w-full rounded-[9px]" />
              <USkeleton class="h-[34px] w-full rounded-[9px]" />
            </div>
          </div>
        </template>
      </div>
    </template>

    <!-- Reject-confirmation modal -->
    <DashboardRejectModal
      v-model:open="rejectOpen"
      @confirm="onRejectConfirm"
    />
  </div>
</template>
