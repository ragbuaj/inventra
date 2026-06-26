<script setup lang="ts">
import type { ReportKey, ReportResult } from '~/mock/reports'
import { useReports } from '~/composables/api/useReports'
import {
  REPORT_KEYS, REPORT_ICON, REPORT_CATEGORIES, REPORT_OFFICES, REPORT_STATUS_KEYS,
  ALL, rp, rpJt, computeReport
} from '~/mock/reports'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

type CellTone = 'default' | 'muted' | 'dimmed' | 'error'
type CellWeight = 'normal' | 'medium' | 'semibold' | 'bold'
interface Cell { text: string, align: 'left' | 'right', tone: CellTone, mono: boolean, weight: CellWeight }
interface Col { label: string, align: 'left' | 'right' }

const TONE_CLASS: Record<CellTone, string> = { default: '', muted: 'text-muted', dimmed: 'text-dimmed', error: 'text-error' }
const WEIGHT_CLASS: Record<CellWeight, string> = { normal: '', medium: 'font-medium', semibold: 'font-semibold', bold: 'font-bold' }

function cell(text: string, align: 'left' | 'right' = 'left', opts: Partial<Pick<Cell, 'tone' | 'mono' | 'weight'>> = {}): Cell {
  return { text, align, tone: opts.tone ?? 'default', mono: opts.mono ?? false, weight: opts.weight ?? 'normal' }
}
function bars(entries: [string, number][], display: (v: number) => string, widthIsValue = false): { label: string, display: string, w: string }[] {
  const max = Math.max(1, ...entries.map(e => e[1]))
  return entries.map(([label, v]) => ({ label, display: display(v), w: `${widthIsValue ? v : Math.round((v / max) * 100)}%` }))
}

const { t } = useI18n()
const api = useReports()

const report = ref<ReportKey>('aset')
const fPeriode = ref('2') // "this quarter" (mockup default index)
const fKantor = ref(ALL)
const fKat = ref(ALL)
const fStatus = ref(ALL)
const applied = ref(false)
const loading = ref(false)

const periodOptions = computed(() => ['last30', 'thisMonth', 'thisQuarter', 'ytd'].map((k, i) => ({ value: String(i), label: t(`reports.period.${k}`) })))
const kantorOptions = computed(() => [{ value: ALL, label: t('reports.allOffices') }, ...REPORT_OFFICES.map(k => ({ value: k, label: k }))])
const katOptions = computed(() => [{ value: ALL, label: t('reports.allCategories') }, ...REPORT_CATEGORIES.map(k => ({ value: k, label: k }))])
const statusOptions = computed(() => [{ value: ALL, label: t('reports.allStatus') }, ...REPORT_STATUS_KEYS.map(k => ({ value: k, label: t(`assets.status.${k}`) }))])

const reportCards = computed(() => REPORT_KEYS.map(k => ({
  key: k,
  icon: REPORT_ICON[k],
  label: t(`reports.card.${k}.label`),
  desc: t(`reports.card.${k}.desc`),
  active: report.value === k
})))

const showStatus = computed(() => report.value === 'aset')
const resultMeta = computed(() => t('reports.resultMeta', { period: t(`reports.period.${['last30', 'thisMonth', 'thisQuarter', 'ytd'][Number(fPeriode.value)] ?? 'thisQuarter'}`) }))

const result = computed<ReportResult | null>(() => applied.value ? computeReport(report.value, { kat: fKat.value, status: fStatus.value }) : null)
const hasData = computed(() => !!result.value && result.value.rows.length > 0)

const view = computed(() => {
  const r = result.value
  if (!r) return null
  const TOTAL = t('reports.total')
  const days = t('reports.unit.days')
  const daysShort = t('reports.unit.daysShort')

  if (r.kind === 'aset') {
    return {
      kpis: [
        { label: t('reports.kpi.assetCount'), value: String(r.rows.length), sub: t('reports.sub.assetUnits') },
        { label: t('reports.kpi.assetAcq'), value: rpJt(r.totalHarga), sub: t('reports.sub.acquisition') },
        { label: t('reports.kpi.assetBook'), value: rpJt(r.totalBuku), sub: t('reports.sub.afterDeprec') }
      ],
      chartTitle: t('reports.chart.aset'),
      chartBars: bars(Object.entries(r.byCategory), rpJt),
      cols: [t('reports.col.code'), t('reports.col.name'), t('reports.col.category'), t('reports.col.buyPrice'), t('reports.col.accumDeprec'), t('reports.col.bookValue')].map((label, i): Col => ({ label, align: i >= 3 ? 'right' : 'left' })),
      rows: r.rows.map(a => [cell(a.kode, 'left', { mono: true, tone: 'muted' }), cell(a.nama, 'left', { weight: 'medium' }), cell(a.kat, 'left', { tone: 'muted' }), cell(rp(a.harga), 'right'), cell(rp(a.akum), 'right', { tone: 'muted' }), cell(rp(a.buku), 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell('', 'left'), cell(rp(r.totalHarga), 'right', { weight: 'bold' }), cell(rp(r.totalAkum), 'right', { weight: 'bold' }), cell(rp(r.totalBuku), 'right', { weight: 'bold' })]
    }
  }
  if (r.kind === 'depr') {
    return {
      kpis: [
        { label: t('reports.kpi.deprCurrent'), value: rpJt(84000000), sub: t('reports.sub.currentYear') },
        { label: t('reports.kpi.deprAccum'), value: rpJt(408000000), sub: '2024–2026' },
        { label: t('reports.kpi.deprBook'), value: rpJt(168000000), sub: t('reports.sub.endOf2026') }
      ],
      chartTitle: t('reports.chart.depr'),
      chartBars: bars(r.rows.map(d => [d.period, d.deprec] as [string, number]), rpJt),
      cols: [t('reports.col.period'), t('reports.col.opening'), t('reports.col.deprec'), t('reports.col.closing')].map((label, i): Col => ({ label, align: i >= 1 ? 'right' : 'left' })),
      rows: r.rows.map(d => [cell(d.period, 'left', { mono: true, weight: 'semibold' }), cell(rp(d.opening), 'right', { tone: 'muted' }), cell(rp(d.deprec), 'right', { tone: 'error' }), cell(rp(d.closing), 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'right'), cell(rp(r.totalDeprec), 'right', { weight: 'bold' }), cell('', 'right')]
    }
  }
  if (r.kind === 'util') {
    return {
      kpis: [
        { label: t('reports.kpi.utilAvg'), value: `${r.avg}%`, sub: t('reports.sub.ofCapacity') },
        { label: t('reports.kpi.utilLoaned'), value: String(r.loaned), sub: t('reports.sub.assetsUsed') },
        { label: t('reports.kpi.utilDays'), value: `${r.totalHari} ${days}`, sub: t('reports.sub.accumulated') }
      ],
      chartTitle: t('reports.chart.util'),
      chartBars: bars(Object.entries(r.avgByCategory), v => `${v}%`, true),
      cols: [t('reports.col.assetName'), t('reports.col.category'), t('reports.col.daysLoaned'), t('reports.col.loanCount'), t('reports.col.utilization')].map((label, i): Col => ({ label, align: i >= 2 ? 'right' : 'left' })),
      rows: r.rows.map(u => [cell(u.nama, 'left', { weight: 'medium' }), cell(u.kat, 'left', { tone: 'muted' }), cell(`${u.hari} ${daysShort}`, 'right', { tone: 'muted' }), cell(`${u.pinjam}×`, 'right', { tone: 'muted' }), cell(`${u.util}%`, 'right', { weight: 'semibold' })]),
      footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell(`${r.totalHari} ${daysShort}`, 'right', { weight: 'bold' }), cell(`${r.totalPinjam}×`, 'right', { weight: 'bold' }), cell(`${r.avg}%`, 'right', { weight: 'bold' })]
    }
  }
  return {
    kpis: [
      { label: t('reports.kpi.costTotal'), value: rpJt(r.total), sub: t('reports.sub.selectedPeriod') },
      { label: t('reports.kpi.costPrev'), value: rpJt(r.preventive), sub: 'preventive' },
      { label: t('reports.kpi.costCorr'), value: rpJt(r.corrective), sub: 'corrective' }
    ],
    chartTitle: t('reports.chart.biaya'),
    chartBars: bars(Object.entries(r.byCategory), rpJt),
    cols: [t('reports.col.asset'), t('reports.col.category'), t('reports.col.type'), t('reports.col.actions'), t('reports.col.totalCost')].map((label, i): Col => ({ label, align: i >= 3 ? 'right' : 'left' })),
    rows: r.rows.map(b => [cell(b.nama, 'left', { weight: 'medium' }), cell(b.kat, 'left', { tone: 'muted' }), cell(b.tipe, 'left', { tone: 'muted' }), cell(String(b.n), 'right', { tone: 'muted' }), cell(rp(b.biaya), 'right', { weight: 'semibold' })]),
    footer: [cell(TOTAL, 'left', { weight: 'bold' }), cell('', 'left'), cell('', 'left'), cell(String(r.totalN), 'right', { weight: 'bold' }), cell(rp(r.total), 'right', { weight: 'bold' })]
  }
})

const resultTitle = computed(() => t(`reports.card.${report.value}.label`))

async function apply() {
  loading.value = true
  await api.run(report.value, { kat: fKat.value, status: fStatus.value })
  applied.value = true
  loading.value = false
}
function resetFilters() {
  fKat.value = ALL
  fStatus.value = ALL
  fKantor.value = ALL
}
const toast = useToast()
function exportSoon() {
  toast.add({ title: t('reports.exportSoon'), color: 'neutral', icon: 'i-lucide-info' })
}
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
        class="flex flex-col gap-2 p-[15px] rounded-[13px] border text-left transition-colors hover:border-primary"
        :class="c.active ? 'border-primary bg-primary/5 shadow-none' : 'border-default bg-default shadow-sm'"
        @click="report = c.key"
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
        <USelect
          v-model="fPeriode"
          value-key="value"
          :items="periodOptions"
          class="min-w-[150px]"
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.office') }}</span>
        <USelect
          v-model="fKantor"
          value-key="value"
          :items="kantorOptions"
          class="min-w-[150px]"
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.category') }}</span>
        <USelect
          v-model="fKat"
          value-key="value"
          :items="katOptions"
          class="min-w-[150px]"
        />
      </div>
      <div
        v-if="showStatus"
        class="flex flex-col gap-1"
      >
        <span class="text-[11px] font-medium uppercase tracking-wide text-dimmed">{{ t('reports.filter.status') }}</span>
        <USelect
          v-model="fStatus"
          value-key="value"
          :items="statusOptions"
          class="min-w-[150px]"
        />
      </div>
      <div class="flex-1" />
      <UButton
        icon="i-lucide-filter"
        :label="t('reports.apply')"
        :loading="loading"
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
        <div class="flex gap-2.5">
          <UButton
            icon="i-lucide-file-text"
            color="neutral"
            variant="outline"
            size="sm"
            :label="t('reports.pdf')"
            @click="exportSoon"
          />
          <UButton
            icon="i-lucide-file-spreadsheet"
            color="neutral"
            variant="outline"
            size="sm"
            :label="t('reports.excel')"
            @click="exportSoon"
          />
        </div>
      </div>

      <!-- KPI -->
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
            <table class="w-full border-collapse text-[13px] whitespace-nowrap">
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
              <tfoot>
                <tr class="border-t-2 border-default bg-muted">
                  <td
                    v-for="(c, ci) in view.footer"
                    :key="ci"
                    class="px-4 py-3 text-[13px] font-bold tabular-nums"
                    :class="c.align === 'right' ? 'text-right' : 'text-left'"
                  >
                    {{ c.text }}
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
