<script setup lang="ts">
import type { DashboardSummary, Locale } from '~/composables/api/useDashboard'
import { useDashboard } from '~/composables/api/useDashboard'
import type { Scope } from '~/mock/dashboard'
import { dashboardData, scopeOrder } from '~/mock/dashboard'
import { buildDonut, barWidths, formatCount } from '~/utils/dashboard'

interface KpiVM {
  label: string
  value: string
  icon: string
  iconTone: 'primary' | 'info' | 'neutral' | 'error' | 'warning'
  trendIcon: string
  trendText: string
  trendTone: 'success' | 'muted' | 'error' | 'warning'
}

const { t, locale } = useI18n()
const toast = useToast()
const { summary } = useDashboard()

const scope = ref<Scope>('jaksel')
const period = ref('0')
const loading = ref(true)
const data = ref<DashboardSummary | null>(null)
const handled = ref<Set<string>>(new Set())

const scopeOptions = computed(() =>
  scopeOrder.map(s => ({ value: s, label: dashboardData[s].name[locale.value as Locale] ?? dashboardData[s].name.id }))
)
const periodOptions = computed(() => [
  { value: '0', label: t('dashboard.period.last30') },
  { value: '1', label: t('dashboard.period.thisMonth') },
  { value: '2', label: t('dashboard.period.thisQuarter') },
  { value: '3', label: t('dashboard.period.ytd') }
])
const scopeName = computed(() => scopeOptions.value.find(o => o.value === scope.value)?.label ?? '')

async function load() {
  loading.value = true
  data.value = await summary(scope.value, period.value, locale.value as Locale)
  loading.value = false
}

function onScopeChange() {
  handled.value = new Set()
  load()
}

function approve(id: string) {
  handled.value = new Set([...handled.value, id])
  toast.add({ title: t('dashboard.panel.approvedToast'), color: 'success', icon: 'i-lucide-check' })
}

function reject(id: string) {
  handled.value = new Set([...handled.value, id])
  toast.add({ title: t('dashboard.panel.rejectedToast'), color: 'neutral', icon: 'i-lucide-x' })
}

function comingSoon() {
  toast.add({ title: t('dashboard.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}

const donut = computed(() => buildDonut(data.value?.status ?? []))
const kategoriBars = computed(() => barWidths(data.value?.kategori ?? []))
const lokasiBars = computed(() => barWidths(data.value?.lokasi ?? []))
const visibleAppr = computed(() => (data.value?.appr ?? []).filter(a => !handled.value.has(a.id)))

const kpis = computed<KpiVM[]>(() => {
  const d = data.value
  if (!d) return []
  return [
    { label: t('dashboard.kpi.total'), value: formatCount(d.total), icon: 'i-lucide-package', iconTone: 'primary', trendIcon: 'i-lucide-trending-up', trendText: t('dashboard.kpiTrend.growing'), trendTone: 'success' },
    { label: t('dashboard.kpi.acquisition'), value: d.perolehan, icon: 'i-lucide-wallet', iconTone: 'info', trendIcon: 'i-lucide-trending-up', trendText: t('dashboard.kpiTrend.acqUp'), trendTone: 'success' },
    { label: t('dashboard.kpi.bookValue'), value: d.buku, icon: 'i-lucide-trending-down', iconTone: 'neutral', trendIcon: 'i-lucide-trending-down', trendText: t('dashboard.kpiTrend.depreciation'), trendTone: 'muted' },
    { label: t('dashboard.kpi.overdue'), value: formatCount(d.overdue), icon: 'i-lucide-clock-alert', iconTone: 'error', trendIcon: 'i-lucide-triangle-alert', trendText: t('dashboard.kpiTrend.needsAction'), trendTone: 'error' },
    { label: t('dashboard.kpi.maintenanceDue'), value: formatCount(d.due), icon: 'i-lucide-wrench', iconTone: 'warning', trendIcon: 'i-lucide-triangle-alert', trendText: t('dashboard.kpiTrend.within7Days'), trendTone: 'warning' },
    { label: t('dashboard.kpi.maintenanceCost'), value: d.biaya, icon: 'i-lucide-receipt', iconTone: 'warning', trendIcon: 'i-lucide-trending-up', trendText: t('dashboard.kpiTrend.costUp'), trendTone: 'warning' }
  ]
})

const skeletonKpis = [0, 1, 2, 3, 4, 5]

watch(locale, () => load())
onMounted(() => load())
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
        <USelect
          v-model="period"
          :items="periodOptions"
          class="min-w-[150px]"
          @update:model-value="load"
        />
        <USelect
          v-model="scope"
          :items="scopeOptions"
          class="min-w-[200px]"
          @update:model-value="onScopeChange"
        />
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          square
          :aria-label="t('dashboard.reload')"
          :loading="loading"
          @click="load"
        />
        <UButton
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :label="t('dashboard.export')"
          @click="comingSoon"
        />
      </div>
    </div>

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
          :title="t('dashboard.chart.locationTitle')"
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
          :items="data?.maint ?? []"
          @see-all="comingSoon"
        />
        <DashboardApprovalPanel
          :title="t('dashboard.panel.approvalTitle')"
          :items="visibleAppr"
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
  </div>
</template>
