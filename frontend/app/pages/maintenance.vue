<script setup lang="ts">
import type { ScheduleItem, MaintRecord, DamageReport, MaintType, MaintStatus, DueLevel } from '~/mock/maintenance'
import {
  loc, dayDiff, dueLevel, TYPE_TONE, STATUS_TONE, MAINT_STATUS_KEYS, MAINT_TODAY,
  allAssets, myAssets, careCategories, vendors, problemKeys, maintenanceStore
} from '~/mock/maintenance'
import { fakeLatency } from '~/mock/helpers'
import type { BadgeColor } from '~/types'

// TODO(Task 11): this page is still on the pre-backend mock scaffold
// (`~/mock/maintenance`) — `~/composables/api/useMaintenance` now points at
// the real /api/v1/maintenance endpoints (Task 8) and this page is rewired to
// it, matching `docs/design/Maintenance.dc.html`, in a later task.
const mockApi = {
  async schedule(): Promise<ScheduleItem[]> {
    await fakeLatency(500)
    return maintenanceStore.schedule().map(s => ({ ...s }))
  },
  async records(): Promise<MaintRecord[]> {
    await fakeLatency(600)
    return maintenanceStore.records().map(r => ({ ...r }))
  },
  async reports(): Promise<DamageReport[]> {
    await fakeLatency(300)
    return maintenanceStore.reports().map(r => ({ ...r }))
  },
  async addRecord(rec: MaintRecord): Promise<MaintRecord> {
    await fakeLatency()
    return maintenanceStore.addRecord(rec)
  },
  async addReport(rep: DamageReport): Promise<DamageReport> {
    await fakeLatency()
    return maintenanceStore.addReport(rep)
  }
}

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const DOT_CLASS: Record<BadgeColor, string> = {
  primary: 'bg-primary',
  success: 'bg-success',
  info: 'bg-info',
  warning: 'bg-warning',
  error: 'bg-error',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}
const DUE_TEXT: Record<DueLevel, string> = {
  overdue: 'text-error',
  today: 'text-error',
  soon: 'text-warning',
  later: 'text-muted'
}

const { t, locale } = useI18n()

const tab = ref<'jadwal' | 'catatan' | 'laporan'>('jadwal')
const schedule = ref<ScheduleItem[]>([])
const records = ref<MaintRecord[]>([])
const reports = ref<DamageReport[]>([])
const loadingRecords = ref(true)

const cq = ref('')

// add-note slideover
const noteOpen = ref(false)
const savingNote = ref(false)
const na = reactive({ tag: '', tipe: 'preventive' as MaintType, kat: careCategories[0]!, tgl: '', status: 'scheduled' as MaintStatus, biaya: '', vendor: vendors[0]!, desc: '' })

// damage report form (staff)
const lkTag = ref('')
const lkProblem = ref('')
const lkDesc = ref('')
const lkMsg = ref(false)
const submittingReport = ref(false)
let lkTimer: ReturnType<typeof setTimeout> | undefined
onBeforeUnmount(() => {
  if (lkTimer) clearTimeout(lkTimer)
})

function formatDate(d: string): string {
  if (!d) return '—'
  const [y, m, day] = d.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}
function formatRp(v: number): string {
  return v ? `Rp ${v.toLocaleString('id-ID')}` : '—'
}
function dueText(diff: number): string {
  if (diff < 0) return t('maintenance.due.overdue', { n: -diff })
  if (diff === 0) return t('maintenance.due.today')
  return t('maintenance.due.inDays', { n: diff })
}

const tabs = computed(() => [
  { key: 'jadwal' as const, label: t('maintenance.tabs.jadwal'), icon: 'i-lucide-calendar' },
  { key: 'catatan' as const, label: t('maintenance.tabs.catatan'), icon: 'i-lucide-clipboard-list' },
  { key: 'laporan' as const, label: t('maintenance.tabs.laporan'), icon: 'i-lucide-triangle-alert' }
])

const scheduleRows = computed(() => schedule.value.map((s) => {
  const diff = dayDiff(s.due, MAINT_TODAY)
  const level = dueLevel(diff)
  return {
    item: s,
    asset: s.asset,
    task: loc(s.task, locale.value),
    vendor: loc(s.vendor, locale.value),
    typeTone: TYPE_TONE[s.tipe],
    typeLabel: t(`maintenance.type.${s.tipe}`),
    dueLabel: dueText(diff),
    dueText: DUE_TEXT[level],
    dateLabel: formatDate(s.due),
    urgent: level === 'overdue' || level === 'today'
  }
}))

const dueItems = computed(() =>
  schedule.value
    .map(s => ({ s, diff: dayDiff(s.due, MAINT_TODAY) }))
    .filter(x => x.diff <= 3)
    .sort((a, b) => a.diff - b.diff)
    .map(({ s, diff }) => {
      const level = dueLevel(diff)
      return {
        asset: s.asset,
        task: loc(s.task, locale.value),
        dueLabel: dueText(diff),
        tone: level === 'overdue' || level === 'today' ? 'error' as const : 'warning' as const
      }
    })
)

const recordRows = computed(() => {
  const q = cq.value.trim().toLowerCase()
  return records.value
    .filter((r) => {
      if (!q) return true
      return r.nama.toLowerCase().includes(q) || r.tag.toLowerCase().includes(q) || loc(r.vendor, locale.value).toLowerCase().includes(q)
    })
    .map(r => ({
      ...r,
      typeTone: TYPE_TONE[r.tipe],
      typeLabel: t(`maintenance.type.${r.tipe}`),
      statusTone: STATUS_TONE[r.status],
      statusLabel: t(`maintenance.status.${r.status}`),
      kategoriLabel: loc(r.kategori, locale.value),
      vendorLabel: loc(r.vendor, locale.value),
      tanggalLabel: formatDate(r.tanggal),
      biayaLabel: formatRp(r.biaya)
    }))
})

const reportRows = computed(() => reports.value.map(r => ({
  ...r,
  problemLabel: t(`maintenance.problems.${r.problemKey}`),
  dateLabel: formatDate(r.date)
})))

// select item lists
const assetItems = computed(() => allAssets.map(a => ({ value: a.tag, label: `${a.nama} · ${a.tag}` })))
const myAssetItems = computed(() => myAssets.map(a => ({ value: a.tag, label: `${a.nama} · ${a.tag}` })))
const typeItems = computed(() => (['preventive', 'corrective'] as MaintType[]).map(k => ({ value: k, label: t(`maintenance.type.${k}`) })))
const careItems = computed(() => careCategories.map(c => ({ value: c, label: c })))
const vendorItems = computed(() => vendors.map(v => ({ value: v, label: v })))
const statusItems = computed(() => MAINT_STATUS_KEYS.map(k => ({ value: k, label: t(`maintenance.status.${k}`) })))
const problemItems = computed(() => problemKeys.map(k => ({ value: k, label: t(`maintenance.problems.${k}`) })))

const naReady = computed(() => !!(na.tag && na.tgl))
const lkReady = computed(() => !!(lkTag.value && lkProblem.value))

function openNote(item?: ScheduleItem) {
  na.tag = item?.tag ?? ''
  na.tipe = item?.tipe ?? 'preventive'
  na.kat = careCategories[0]!
  na.tgl = ''
  na.status = 'scheduled'
  na.biaya = ''
  na.vendor = vendors[0]!
  na.desc = ''
  noteOpen.value = true
}

async function saveNote() {
  if (!naReady.value) return
  savingNote.value = true
  const asset = allAssets.find(a => a.tag === na.tag)
  const rec: MaintRecord = {
    tag: na.tag,
    nama: asset?.nama ?? na.tag,
    tipe: na.tipe,
    kategori: na.kat,
    tanggal: na.tgl,
    status: na.status,
    biaya: Number(String(na.biaya).replace(/\D/g, '')) || 0,
    vendor: na.vendor
  }
  await mockApi.addRecord(rec)
  records.value = await mockApi.records()
  savingNote.value = false
  noteOpen.value = false
}

async function submitReport() {
  if (!lkReady.value) return
  submittingReport.value = true
  const asset = myAssets.find(a => a.tag === lkTag.value)
  const rep: DamageReport = {
    tag: lkTag.value,
    nama: asset?.nama ?? lkTag.value,
    problemKey: lkProblem.value,
    desc: lkDesc.value,
    date: MAINT_TODAY
  }
  await mockApi.addReport(rep)
  reports.value = await mockApi.reports()
  submittingReport.value = false
  lkTag.value = ''
  lkProblem.value = ''
  lkDesc.value = ''
  lkMsg.value = true
  if (lkTimer) clearTimeout(lkTimer)
  lkTimer = setTimeout(() => {
    lkMsg.value = false
  }, 4000)
}

onMounted(async () => {
  schedule.value = await mockApi.schedule()
  reports.value = await mockApi.reports()
  loadingRecords.value = true
  records.value = await mockApi.records()
  loadingRecords.value = false
})
</script>

<template>
  <div class="max-w-[1000px] mx-auto">
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('maintenance.title') }}
        </h1>
        <p class="text-sm text-muted">
          {{ t('maintenance.subtitle') }}
        </p>
      </div>
    </div>

    <!-- Overdue banner -->
    <div
      v-if="dueItems.length > 0"
      class="border border-warning/30 rounded-[13px] bg-warning/10 p-4 mb-5"
    >
      <div class="flex items-center justify-between gap-3 flex-wrap mb-2.5">
        <div class="flex items-center gap-2.5 text-warning">
          <UIcon
            name="i-lucide-triangle-alert"
            class="size-[18px]"
          />
          <span class="text-sm font-semibold">{{ t('maintenance.dueBannerTitle') }}</span>
        </div>
        <UButton
          color="warning"
          variant="outline"
          size="xs"
          :label="t('maintenance.seeSchedule')"
          @click="tab = 'jadwal'"
        />
      </div>
      <div class="flex flex-col gap-2">
        <div
          v-for="(d, i) in dueItems"
          :key="i"
          class="flex items-center gap-2.5 px-3 py-2.5 rounded-[10px] bg-default border"
          :class="d.tone === 'error' ? 'border-error/35' : 'border-default'"
        >
          <span
            class="size-[30px] rounded-lg flex items-center justify-center flex-none"
            :class="d.tone === 'error' ? 'bg-error/15 text-error' : 'bg-warning/15 text-warning'"
          >
            <UIcon
              name="i-lucide-wrench"
              class="size-[15px]"
            />
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-[13.5px] font-semibold truncate">
              {{ d.asset }}
            </div>
            <div class="text-[12.5px] text-muted">
              {{ d.task }}
            </div>
          </div>
          <UBadge
            :color="d.tone"
            variant="subtle"
            class="rounded-full flex-none"
          >
            {{ d.dueLabel }}
          </UBadge>
        </div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-default mb-5">
      <button
        v-for="tb in tabs"
        :key="tb.key"
        class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-sm border-b-2 transition-colors"
        :class="tab === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
        @click="tab = tb.key"
      >
        <UIcon
          :name="tb.icon"
          class="size-[15px]"
        />
        {{ tb.label }}
      </button>
    </div>

    <!-- JADWAL -->
    <div
      v-if="tab === 'jadwal'"
      class="flex flex-col gap-2.5"
    >
      <div
        v-for="(s, i) in scheduleRows"
        :key="i"
        class="flex items-center gap-3.5 px-4 py-3.5 bg-default border rounded-xl shadow-sm"
        :class="s.urgent ? 'border-error/35' : 'border-default'"
      >
        <span
          class="size-10 rounded-[10px] flex items-center justify-center flex-none"
          :class="s.urgent ? 'bg-error/15 text-error' : 'bg-warning/15 text-warning'"
        >
          <UIcon
            name="i-lucide-wrench"
            class="size-[19px]"
          />
        </span>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-sm font-semibold">{{ s.asset }}</span>
            <UBadge
              :color="s.typeTone"
              variant="subtle"
              class="rounded-full"
            >
              {{ s.typeLabel }}
            </UBadge>
          </div>
          <div class="text-[12.5px] text-muted mt-0.5">
            {{ s.task }} · {{ s.vendor }}
          </div>
        </div>
        <div class="flex items-center gap-3 flex-none">
          <div class="text-right">
            <div
              class="text-[12.5px] font-semibold"
              :class="s.dueText"
            >
              {{ s.dueLabel }}
            </div>
            <div class="text-[11.5px] text-dimmed">
              {{ s.dateLabel }}
            </div>
          </div>
          <UButton
            icon="i-lucide-plus"
            color="neutral"
            variant="outline"
            size="xs"
            :label="t('maintenance.makeNote')"
            @click="openNote(s.item)"
          />
        </div>
      </div>
    </div>

    <!-- CATATAN -->
    <div v-else-if="tab === 'catatan'">
      <div class="flex items-center gap-2.5 flex-wrap mb-3.5">
        <UInput
          v-model="cq"
          icon="i-lucide-search"
          :placeholder="t('maintenance.records.searchPlaceholder')"
          class="flex-1 min-w-[220px]"
        />
        <UButton
          icon="i-lucide-plus"
          :label="t('maintenance.records.addNote')"
          @click="openNote()"
        />
      </div>

      <div
        v-if="loadingRecords"
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
          <USkeleton class="h-5 w-[90px] rounded-full" />
        </div>
      </div>

      <div
        v-else-if="recordRows.length === 0"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-wrench"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('maintenance.records.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('maintenance.records.emptySub') }}
        </div>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colAsset') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colType') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colCategory') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colDate') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colStatus') }}
                </th>
                <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colCost') }}
                </th>
                <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('maintenance.records.colVendor') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(r, i) in recordRows"
                :key="i"
                class="border-t border-default hover:bg-muted transition-colors"
              >
                <td class="px-4 py-3">
                  <div class="font-medium">
                    {{ r.nama }}
                  </div>
                  <div class="font-mono text-[11.5px] text-dimmed">
                    {{ r.tag }}
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <UBadge
                    :color="r.typeTone"
                    variant="subtle"
                    class="rounded-full"
                  >
                    {{ r.typeLabel }}
                  </UBadge>
                </td>
                <td class="px-3.5 py-3 text-muted">
                  {{ r.kategoriLabel }}
                </td>
                <td class="px-3.5 py-3 text-muted">
                  {{ r.tanggalLabel }}
                </td>
                <td class="px-3.5 py-3">
                  <UBadge
                    :color="r.statusTone"
                    variant="subtle"
                    class="rounded-full gap-1.5"
                  >
                    <span
                      class="size-1.5 rounded-full"
                      :class="DOT_CLASS[r.statusTone]"
                    />
                    {{ r.statusLabel }}
                  </UBadge>
                </td>
                <td class="px-3.5 py-3 text-right tabular-nums">
                  {{ r.biayaLabel }}
                </td>
                <td class="px-4 py-3 text-muted">
                  {{ r.vendorLabel }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- LAPORAN KERUSAKAN -->
    <div
      v-else
      class="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start"
    >
      <!-- form -->
      <div>
        <div class="flex items-center gap-2 mb-3">
          <UBadge
            color="info"
            variant="subtle"
            class="rounded-full"
          >
            {{ t('maintenance.report.staffBadge') }}
          </UBadge>
        </div>
        <div
          v-if="lkMsg"
          class="flex gap-2.5 items-center px-3.5 py-3 mb-4 rounded-[11px] border bg-success/10 border-success/30 text-success text-[13px] font-medium"
        >
          <UIcon
            name="i-lucide-circle-check"
            class="size-[17px] flex-none"
          />
          {{ t('maintenance.report.submitted') }}
        </div>
        <div class="bg-default border border-default rounded-[14px] shadow-sm p-5 flex flex-col gap-[15px]">
          <div class="text-[15px] font-semibold">
            {{ t('maintenance.report.formTitle') }}
          </div>
          <UFormField
            :label="t('maintenance.report.asset')"
            required
          >
            <USelect
              v-model="lkTag"
              value-key="value"
              :items="myAssetItems"
              :placeholder="t('maintenance.report.selectPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('maintenance.report.problem')"
            required
          >
            <USelect
              v-model="lkProblem"
              value-key="value"
              :items="problemItems"
              :placeholder="t('maintenance.report.selectPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('maintenance.report.description')">
            <UTextarea
              v-model="lkDesc"
              :rows="3"
              :placeholder="t('maintenance.report.descPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('maintenance.report.photo')">
            <div class="border-[1.5px] border-dashed border-default rounded-[11px] p-[18px] text-center cursor-pointer hover:border-primary transition-colors">
              <div class="size-9 mx-auto mb-2 rounded-[9px] bg-muted text-muted flex items-center justify-center">
                <UIcon
                  name="i-lucide-camera"
                  class="size-[18px]"
                />
              </div>
              <div class="text-[12.5px] font-medium text-muted">
                {{ t('maintenance.report.photoDrop') }}
              </div>
            </div>
          </UFormField>
          <UButton
            icon="i-lucide-send"
            block
            :label="t('maintenance.report.submit')"
            :disabled="!lkReady"
            :loading="submittingReport"
            @click="submitReport"
          />
          <div class="text-xs leading-relaxed text-dimmed flex gap-2 items-start">
            <UIcon
              name="i-lucide-info"
              class="size-3.5 flex-none mt-0.5"
            />
            {{ t('maintenance.report.queueNote') }}
          </div>
        </div>
      </div>

      <!-- history -->
      <div>
        <div class="text-sm font-semibold mb-3">
          {{ t('maintenance.report.historyTitle') }}
        </div>
        <div
          v-if="reportRows.length > 0"
          class="flex flex-col gap-2.5"
        >
          <div
            v-for="(r, i) in reportRows"
            :key="i"
            class="bg-default border border-default rounded-xl shadow-sm px-4 py-3.5"
          >
            <div class="flex items-start justify-between gap-2.5">
              <div class="min-w-0">
                <div class="text-[13.5px] font-semibold">
                  {{ r.nama }}
                </div>
                <div class="font-mono text-[11.5px] text-dimmed">
                  {{ r.tag }}
                </div>
              </div>
              <UBadge
                color="warning"
                variant="subtle"
                class="rounded-full flex-none"
              >
                {{ t('maintenance.report.awaiting') }}
              </UBadge>
            </div>
            <div class="mt-2.5 flex flex-wrap gap-1.5 items-center">
              <UBadge
                color="neutral"
                variant="subtle"
                class="rounded-full"
              >
                {{ r.problemLabel }}
              </UBadge>
              <span class="text-xs text-dimmed self-center">{{ r.dateLabel }}</span>
            </div>
            <div
              v-if="r.desc"
              class="mt-2 text-[12.5px] leading-relaxed text-muted"
            >
              {{ r.desc }}
            </div>
          </div>
        </div>
        <div
          v-else
          class="bg-default border border-dashed border-default rounded-[14px] py-[46px] px-6 text-center"
        >
          <div class="size-[50px] mx-auto mb-3 rounded-[13px] bg-muted text-dimmed flex items-center justify-center">
            <UIcon
              name="i-lucide-file-text"
              class="size-6"
            />
          </div>
          <div class="text-[15px] font-semibold mb-1">
            {{ t('maintenance.report.emptyTitle') }}
          </div>
          <div class="text-[13px] leading-relaxed text-muted max-w-[260px] mx-auto">
            {{ t('maintenance.report.emptySub') }}
          </div>
        </div>
      </div>
    </div>

    <!-- ADD NOTE SLIDEOVER -->
    <FormSlideover
      v-model:open="noteOpen"
      :title="t('maintenance.note.title')"
      :subtitle="t('maintenance.note.subtitle')"
      :loading="savingNote"
      @submit="saveNote"
    >
      <div class="flex flex-col gap-[15px]">
        <UFormField
          :label="t('maintenance.note.asset')"
          required
        >
          <USelect
            v-model="na.tag"
            value-key="value"
            :items="assetItems"
            :placeholder="t('maintenance.report.selectPlaceholder')"
            class="w-full"
          />
        </UFormField>
        <div class="grid grid-cols-2 gap-3.5">
          <UFormField :label="t('maintenance.note.type')">
            <USelect
              v-model="na.tipe"
              value-key="value"
              :items="typeItems"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('maintenance.note.category')">
            <USelect
              v-model="na.kat"
              value-key="value"
              :items="careItems"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('maintenance.note.date')"
            required
          >
            <UInput
              v-model="na.tgl"
              type="date"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('maintenance.note.status')">
            <USelect
              v-model="na.status"
              value-key="value"
              :items="statusItems"
              class="w-full"
            />
          </UFormField>
        </div>
        <UFormField :label="t('maintenance.note.cost')">
          <UInput
            v-model="na.biaya"
            inputmode="numeric"
            placeholder="0"
            class="w-full"
          >
            <template #leading>
              <span class="text-[13px] font-medium text-dimmed">Rp</span>
            </template>
          </UInput>
        </UFormField>
        <UFormField :label="t('maintenance.note.vendor')">
          <USelect
            v-model="na.vendor"
            value-key="value"
            :items="vendorItems"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('maintenance.note.description')">
          <UTextarea
            v-model="na.desc"
            :rows="3"
            :placeholder="t('maintenance.note.descPlaceholder')"
            class="w-full"
          />
        </UFormField>
      </div>
    </FormSlideover>
  </div>
</template>
