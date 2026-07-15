<script setup lang="ts">
import type { ContextMenuItem } from '@nuxt/ui'
import type { BadgeColor, RowAction } from '~/types'
import type { MaintenanceSchedule, MaintenanceRecord, AttentionItem } from '~/composables/api/useMaintenance'
import type { RecordPrefill } from '~/components/maintenance/RecordSlideover.vue'
import { MAINT_STATUS_TONE, MAINT_TYPE_TONE, dueDiffDays, dueKind, formatRupiah, type DueKind } from '~/constants/maintenanceMeta'
import { formatDateID, REQUEST_STATUS_TONE, type RequestStatus } from '~/constants/assignmentMeta'

definePageMeta({ middleware: 'can', permission: ['maintenance.view', 'request.create'] })

type TabKey = 'jadwal' | 'catatan' | 'laporan'

interface MyReportRow {
  id: string
  status: string
  created_at: string | null
  payload?: { asset_id?: string, problem_category_id?: string, description?: string | null } | null
  target_id?: string | null
}

const DOT_CLASS: Record<BadgeColor, string> = {
  primary: 'bg-primary',
  success: 'bg-success',
  info: 'bg-info',
  warning: 'bg-warning',
  error: 'bg-error',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}
const DUE_TEXT: Record<DueKind, string> = {
  overdue: 'text-error',
  today: 'text-error',
  soon: 'text-warning',
  normal: 'text-muted'
}

const { t } = useI18n()
const can = useCan()
const maintenanceApi = useMaintenance()
const assignmentApi = useAssignment()
const assetsApi = useAssets()
const referenceApi = useReference()

const canView = computed(() => can('maintenance.view'))
const canManage = computed(() => can('maintenance.manage'))
const canReport = computed(() => can('request.create'))

const tabs = computed(() => {
  const arr: { key: TabKey, label: string, icon: string }[] = []
  if (canView.value) arr.push({ key: 'jadwal', label: t('maintenance.tabs.jadwal'), icon: 'i-lucide-calendar' })
  if (canView.value) arr.push({ key: 'catatan', label: t('maintenance.tabs.catatan'), icon: 'i-lucide-clipboard-list' })
  if (canReport.value) arr.push({ key: 'laporan', label: t('maintenance.tabs.laporan'), icon: 'i-lucide-triangle-alert' })
  return arr
})

const tab = ref<TabKey>(tabs.value[0]?.key ?? 'jadwal')
watch(tabs, (list) => {
  if (list.length > 0 && !list.some(x => x.key === tab.value)) tab.value = list[0]!.key
})

function dueText(diff: number): string {
  if (diff < 0) return t('maintenance.due.overdue', { n: -diff })
  if (diff === 0) return t('maintenance.due.today')
  return t('maintenance.due.inDays', { n: diff })
}

// ---------------------------------------------------------------------------
// Jadwal (schedules)
// ---------------------------------------------------------------------------
const schedules = ref<MaintenanceSchedule[]>([])
const scheduleLoading = ref(true)
const scheduleError = ref(false)

async function loadSchedules() {
  scheduleLoading.value = true
  scheduleError.value = false
  try {
    const res = await maintenanceApi.schedules({ limit: 100 })
    schedules.value = res.data
  } catch {
    scheduleError.value = true
    schedules.value = []
  } finally {
    scheduleLoading.value = false
  }
}

const scheduleRows = computed(() => schedules.value.map((s) => {
  const diff = dueDiffDays(s.next_due_date)
  // An inactive schedule's due date is no longer actionable — its badge/urgency
  // must read as neutral regardless of how overdue next_due_date is.
  const kind = s.is_active ? dueKind(diff) : 'normal'
  return {
    item: s,
    asset: s.asset_name ?? s.asset_tag ?? '—',
    task: s.category_name ?? '—',
    dueLabel: diff === null ? '—' : dueText(diff),
    dueText: DUE_TEXT[kind],
    dateLabel: formatDateID(s.next_due_date),
    urgent: s.is_active && (kind === 'overdue' || kind === 'today')
  }
}))

const dueItems = computed(() =>
  schedules.value
    .filter(s => s.is_active)
    .map(s => ({ s, diff: dueDiffDays(s.next_due_date) }))
    .filter((x): x is { s: MaintenanceSchedule, diff: number } => x.diff !== null && x.diff <= 3)
    .sort((a, b) => a.diff - b.diff)
    .map(({ s, diff }) => {
      const kind = dueKind(diff)
      return {
        id: s.id,
        asset: s.asset_name ?? s.asset_tag ?? '—',
        task: s.category_name ?? '—',
        dueLabel: dueText(diff),
        tone: (kind === 'overdue' || kind === 'today') ? 'error' as const : 'warning' as const
      }
    })
)

// ---------------------------------------------------------------------------
// Perlu Tindak Lanjut (attention) — approved deviation
// ---------------------------------------------------------------------------
const attentionItems = ref<AttentionItem[]>([])

async function loadAttention() {
  if (!canManage.value) {
    attentionItems.value = []
    return
  }
  try {
    const res = await maintenanceApi.attention()
    attentionItems.value = res.data
  } catch {
    attentionItems.value = []
  }
}

const attentionVisible = computed(() => canManage.value && attentionItems.value.length > 0)

// ---------------------------------------------------------------------------
// Catatan (records)
// ---------------------------------------------------------------------------
const records = ref<MaintenanceRecord[]>([])
const recordsLoading = ref(true)
const recordsError = ref(false)
const cq = ref('')
let cqTimer: ReturnType<typeof setTimeout> | undefined

async function loadRecords() {
  recordsLoading.value = true
  recordsError.value = false
  try {
    const res = await maintenanceApi.records({ q: cq.value.trim() || undefined, limit: 100 })
    records.value = res.data
  } catch {
    recordsError.value = true
    records.value = []
  } finally {
    recordsLoading.value = false
  }
}

watch(cq, () => {
  if (cqTimer) clearTimeout(cqTimer)
  cqTimer = setTimeout(loadRecords, 300)
})
onBeforeUnmount(() => {
  if (cqTimer) clearTimeout(cqTimer)
  if (reportTimer) clearTimeout(reportTimer)
})

const recordRows = computed(() => records.value.map(r => ({
  id: r.id,
  raw: r,
  assetName: r.asset_name ?? r.asset_tag ?? '—',
  assetTag: r.asset_tag,
  typeTone: MAINT_TYPE_TONE[r.type],
  typeLabel: t(`maintenance.type.${r.type}`),
  statusTone: MAINT_STATUS_TONE[r.status],
  statusLabel: t(`maintenance.status.${r.status}`),
  categoryLabel: r.category_name ?? '—',
  dateLabel: formatDateID(r.scheduled_date),
  costLabel: formatRupiah(r.cost),
  vendorLabel: r.vendor_name ?? r.performed_by ?? '—'
})))

// ---------------------------------------------------------------------------
// Slideovers
// ---------------------------------------------------------------------------
const scheduleSlideoverOpen = ref(false)
const scheduleSlideoverTarget = ref<MaintenanceSchedule | null>(null)

function openScheduleCreate() {
  scheduleSlideoverTarget.value = null
  scheduleSlideoverOpen.value = true
}
function openScheduleEdit(s: MaintenanceSchedule) {
  if (!canManage.value) return
  scheduleSlideoverTarget.value = s
  scheduleSlideoverOpen.value = true
}
function onScheduleSaved() {
  loadSchedules()
}

const recordSlideoverOpen = ref(false)
const recordSlideoverTarget = ref<MaintenanceRecord | null>(null)
const recordSlideoverPrefill = ref<RecordPrefill | null>(null)

function openRecordCreate(prefill: RecordPrefill | null = null) {
  recordSlideoverTarget.value = null
  recordSlideoverPrefill.value = prefill
  recordSlideoverOpen.value = true
}
function openRecordEdit(r: MaintenanceRecord) {
  if (!canManage.value) return
  recordSlideoverTarget.value = r
  recordSlideoverPrefill.value = null
  recordSlideoverOpen.value = true
}
function onRecordSaved() {
  loadRecords()
  loadSchedules()
  loadAttention()
}

// Per-row actions (kebab dropdown via RowActionsMenu, and the table's
// right-click context menu below) — both built from this same list via
// buildActionGroups so their grouping/dividers stay in sync (see Task 8,
// mirrors assets/index.vue's Task 7 pattern). The whole-row click still opens
// the edit slideover as a convenience; the kebab is the standardized
// affordance and reaches the same openRecordEdit().
function recordRowActions(r: MaintenanceRecord): RowAction[] {
  if (!canManage.value) return []
  return [{ label: t('maintenance.records.editRecord'), icon: 'i-lucide-pencil', onSelect: () => openRecordEdit(r) }]
}

const recordContextItems = ref<ContextMenuItem[][]>([])
function onRecordRowContextMenu(r: MaintenanceRecord) {
  recordContextItems.value = buildActionGroups(recordRowActions(r)) as ContextMenuItem[][]
}
// Safety net mirroring ResourceTable/disposals: a right-click that bubbles up
// from outside an actual record `tbody tr` (header, empty area) must clear
// any stale items left over from a previous row's right-click.
function onRecordsTableContextMenu(e: MouseEvent) {
  const tr = (e.target as HTMLElement | null)?.closest('tbody tr')
  if (!tr) recordContextItems.value = []
}

function makeNoteFromSchedule(s: MaintenanceSchedule) {
  openRecordCreate({
    asset: { id: s.asset_id, name: s.asset_name ?? '', asset_tag: s.asset_tag ?? '' },
    scheduleId: s.id,
    maintenanceCategoryId: s.maintenance_category_id ?? undefined,
    type: 'preventive'
  })
}

function makeNoteFromAttention(item: AttentionItem) {
  openRecordCreate({
    asset: { id: item.id, name: item.name, asset_tag: item.asset_tag },
    type: 'corrective'
  })
}

// ---------------------------------------------------------------------------
// Laporan Kerusakan (staff damage report)
// ---------------------------------------------------------------------------
interface MyAssetOption { value: string, label: string }
const myAssignedAssets = ref<MyAssetOption[]>([])

async function loadMyAssignedAssets() {
  // /assignments/mine resolves the caller's own employee id server-side (from
  // the JWT), so it's safe to call unconditionally: a caller with no linked
  // employee just gets back an empty list.
  try {
    const res = await assignmentApi.mine({ status: 'active' })
    myAssignedAssets.value = res.data.map(a => ({ value: a.asset_id, label: `${a.asset_name ?? '—'} · ${a.asset_tag ?? '—'}` }))
  } catch {
    myAssignedAssets.value = []
  }
}

// The problem-category form field is an async search picker (see
// usePickerSource.ts); `problemCategories` is still eagerly loaded to resolve
// problem_category_id -> label in the "Riwayat Laporan Saya" history cards
// below (a list-display concern, unchanged here).
const problemCategoryPicker = useReferencePicker('problem-categories')
const problemCategories = ref<{ id: string, name: string }[]>([])

async function loadProblemCategories() {
  try {
    const res = await referenceApi.list('problem-categories', { limit: 100 })
    problemCategories.value = res.data.map(r => ({ id: r.id, name: r.name }))
  } catch {
    problemCategories.value = []
  }
}

const reportAssetId = ref('')
const reportProblemId = ref('')
const reportDesc = ref('')
const reportPhoto = ref<File | null>(null)
const reportSubmitting = ref(false)
const reportMsg = ref(false)
let reportTimer: ReturnType<typeof setTimeout> | undefined

const reportReady = computed(() => !!(reportAssetId.value && reportProblemId.value))

function onPhotoChange(e: Event) {
  const input = e.target as HTMLInputElement
  reportPhoto.value = input.files?.[0] ?? null
}

async function submitReport() {
  if (!reportReady.value || reportSubmitting.value) return
  reportSubmitting.value = true
  try {
    await maintenanceApi.submitReport({
      asset_id: reportAssetId.value,
      problem_category_id: reportProblemId.value,
      description: reportDesc.value.trim() || null,
      photo: reportPhoto.value
    })
    reportAssetId.value = ''
    reportProblemId.value = ''
    reportDesc.value = ''
    reportPhoto.value = null
    reportMsg.value = true
    if (reportTimer) clearTimeout(reportTimer)
    reportTimer = setTimeout(() => {
      reportMsg.value = false
    }, 4000)
    await loadMyReports()
  } catch {
    // useApiClient already raised an error toast
  } finally {
    reportSubmitting.value = false
  }
}

const myReportsRaw = ref<MyReportRow[]>([])
const myReportsLoading = ref(true)
const assetNameCache = ref(new Map<string, { name: string, tag: string }>())

function resolveReportAssetId(r: MyReportRow): string | null {
  return r.payload?.asset_id ?? r.target_id ?? null
}

async function resolveAssetName(id: string) {
  if (assetNameCache.value.has(id)) return
  try {
    const asset = await assetsApi.get(id)
    assetNameCache.value.set(id, { name: asset.name, tag: asset.asset_tag })
  } catch {
    // Best-effort only (e.g. 403 out-of-scope) — the row falls back to the raw id/tag.
  }
}

async function loadMyReports() {
  myReportsLoading.value = true
  try {
    const res = await maintenanceApi.myReports({ limit: 50 })
    myReportsRaw.value = res.data as unknown as MyReportRow[]
    for (const r of myReportsRaw.value) {
      const aid = resolveReportAssetId(r)
      if (aid) await resolveAssetName(aid)
    }
  } catch {
    myReportsRaw.value = []
  } finally {
    myReportsLoading.value = false
  }
}

const REPORT_STATUS_LABEL_KEY: Record<RequestStatus, string> = {
  pending: 'maintenance.report.awaiting',
  approved: 'maintenance.report.status.approved',
  rejected: 'maintenance.report.status.rejected',
  cancelled: 'maintenance.report.status.cancelled'
}

const myReportRows = computed(() => myReportsRaw.value.map((r) => {
  const aid = resolveReportAssetId(r)
  const known = aid ? assetNameCache.value.get(aid) : undefined
  const problemId = r.payload?.problem_category_id
  const problemLabel = problemId ? (problemCategories.value.find(c => c.id === problemId)?.name ?? problemId) : '—'
  const status = (r.status as RequestStatus) in REQUEST_STATUS_TONE ? (r.status as RequestStatus) : 'pending'
  return {
    id: r.id,
    assetName: known?.name ?? null,
    assetTag: known?.tag ?? aid ?? '—',
    statusTone: REQUEST_STATUS_TONE[status],
    statusLabel: t(REPORT_STATUS_LABEL_KEY[status]),
    problemLabel,
    dateLabel: formatDateID(r.created_at),
    desc: r.payload?.description ?? ''
  }
}))

onMounted(async () => {
  const tasks: Promise<unknown>[] = []
  if (canView.value) {
    tasks.push(loadSchedules())
    tasks.push(loadRecords())
  }
  if (canManage.value) tasks.push(loadAttention())
  if (canReport.value) {
    tasks.push(loadMyAssignedAssets())
    tasks.push(loadProblemCategories())
    tasks.push(loadMyReports())
  }
  await Promise.all(tasks)
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

    <!-- Due banner -->
    <div
      v-if="dueItems.length > 0"
      class="border border-warning/30 rounded-[13px] bg-warning/10 p-4 mb-5"
      data-testid="due-banner"
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
          data-testid="due-banner-see-schedule"
          @click="() => { tab = 'jadwal' }"
        />
      </div>
      <div class="flex flex-col gap-2">
        <div
          v-for="d in dueItems"
          :key="d.id"
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

    <!-- Perlu Tindak Lanjut (approved deviation) -->
    <div
      v-if="attentionVisible"
      class="border border-error/25 rounded-[13px] bg-error/5 p-4 mb-5"
      data-testid="attention-section"
    >
      <div class="flex items-center gap-2.5 text-error mb-2.5">
        <UIcon
          name="i-lucide-flag"
          class="size-[18px]"
        />
        <span class="text-sm font-semibold">{{ t('maintenance.attention.title') }}</span>
      </div>
      <div class="flex flex-col gap-2">
        <div
          v-for="item in attentionItems"
          :key="item.id"
          class="flex items-center gap-2.5 px-3 py-2.5 rounded-[10px] bg-default border border-default"
          :data-testid="`attention-item-${item.id}`"
        >
          <span class="size-[30px] rounded-lg bg-error/15 text-error flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-wrench"
              class="size-[15px]"
            />
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-[13.5px] font-semibold truncate">
              {{ item.name }}
            </div>
            <div class="text-[12.5px] text-muted font-mono">
              {{ item.asset_tag }} · {{ item.office_name ?? '—' }}
            </div>
          </div>
          <UButton
            size="xs"
            color="error"
            variant="outline"
            icon="i-lucide-plus"
            :label="t('maintenance.makeNote')"
            :data-testid="`attention-note-${item.id}`"
            @click="makeNoteFromAttention(item)"
          />
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
    <div v-if="tab === 'jadwal'">
      <div
        v-if="canManage"
        class="flex justify-end mb-2.5"
      >
        <UButton
          icon="i-lucide-plus"
          :label="t('maintenance.schedule.addButton')"
          data-testid="jadwal-add-button"
          @click="openScheduleCreate"
        />
      </div>

      <div
        v-if="scheduleLoading"
        class="flex flex-col gap-2.5"
        data-testid="jadwal-loading"
      >
        <div
          v-for="n in 3"
          :key="n"
          class="flex items-center gap-3.5 px-4 py-3.5 bg-default border border-default rounded-xl shadow-sm"
        >
          <USkeleton class="size-10 rounded-[10px] flex-none" />
          <div class="flex-1 flex flex-col gap-2">
            <USkeleton class="h-3 w-1/3 rounded" />
            <USkeleton class="h-3 w-1/2 rounded" />
          </div>
          <USkeleton class="h-8 w-24 rounded-lg flex-none" />
        </div>
      </div>

      <div
        v-else-if="scheduleError"
        class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
        data-testid="jadwal-load-error"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          data-testid="jadwal-retry"
          @click="loadSchedules"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else-if="scheduleRows.length === 0"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-wrench"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('maintenance.jadwal.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('maintenance.jadwal.emptySub') }}
        </div>
      </div>

      <div
        v-else
        class="flex flex-col gap-2.5"
      >
        <div
          v-for="s in scheduleRows"
          :key="s.item.id"
          class="flex items-center gap-3.5 px-4 py-3.5 bg-default border rounded-xl shadow-sm"
          :class="[s.urgent ? 'border-error/35' : 'border-default', canManage ? 'cursor-pointer' : '']"
          :data-testid="`schedule-card-${s.item.id}`"
          @click="openScheduleEdit(s.item)"
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
                :color="MAINT_TYPE_TONE.preventive"
                variant="subtle"
                class="rounded-full"
              >
                {{ t('maintenance.type.preventive') }}
              </UBadge>
            </div>
            <div class="text-[12.5px] text-muted mt-0.5">
              {{ s.task }}
            </div>
          </div>
          <div class="flex items-center gap-3 flex-none">
            <div class="text-right">
              <UBadge
                v-if="!s.item.is_active"
                color="neutral"
                variant="subtle"
                class="rounded-full"
                :data-testid="`schedule-inactive-${s.item.id}`"
              >
                {{ t('maintenance.schedule.inactive') }}
              </UBadge>
              <div
                v-else
                class="text-[12.5px] font-semibold"
                :class="s.dueText"
              >
                {{ s.dueLabel }}
              </div>
              <div
                class="text-[11.5px]"
                :class="s.item.is_active ? 'text-dimmed' : 'text-dimmed/70'"
              >
                {{ s.dateLabel }}
              </div>
            </div>
            <UButton
              v-if="canManage"
              icon="i-lucide-plus"
              color="neutral"
              variant="outline"
              size="xs"
              :label="t('maintenance.makeNote')"
              :data-testid="`schedule-make-note-${s.item.id}`"
              @click.stop="makeNoteFromSchedule(s.item)"
            />
          </div>
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
          v-if="canManage"
          icon="i-lucide-plus"
          :label="t('maintenance.records.addNote')"
          data-testid="catatan-add-button"
          @click="openRecordCreate(null)"
        />
      </div>

      <div
        v-if="recordsLoading"
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
        data-testid="catatan-loading"
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
        v-else-if="recordsError"
        class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
        data-testid="catatan-load-error"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          data-testid="catatan-retry"
          @click="loadRecords"
        >
          {{ t('common.retry') }}
        </UButton>
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
        <UContextMenu
          :items="recordContextItems"
          :disabled="recordRows.length === 0"
        >
          <div
            class="overflow-x-auto"
            @contextmenu="onRecordsTableContextMenu"
          >
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
                  <th class="text-right px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('common.actions') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in recordRows"
                  :key="r.id"
                  class="border-t border-default transition-colors"
                  :class="canManage ? 'cursor-pointer hover:bg-muted' : ''"
                  :data-testid="`record-row-${r.id}`"
                  @click="openRecordEdit(r.raw)"
                  @contextmenu="onRecordRowContextMenu(r.raw)"
                >
                  <td class="px-4 py-3">
                    <div class="font-medium">
                      {{ r.assetName }}
                    </div>
                    <div class="font-mono text-[11.5px] text-dimmed">
                      {{ r.assetTag }}
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
                    {{ r.categoryLabel }}
                  </td>
                  <td class="px-3.5 py-3 text-muted">
                    {{ r.dateLabel }}
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
                    {{ r.costLabel }}
                  </td>
                  <td class="px-4 py-3 text-muted">
                    {{ r.vendorLabel }}
                  </td>
                  <td
                    class="px-4 py-3 text-right"
                    :data-testid="`record-actions-${r.id}`"
                    @click.stop
                  >
                    <RowActionsMenu :items="recordRowActions(r.raw)" />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </UContextMenu>
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
          v-if="reportMsg"
          class="flex gap-2.5 items-center px-3.5 py-3 mb-4 rounded-[11px] border bg-success/10 border-success/30 text-success text-[13px] font-medium"
          data-testid="report-success"
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
            <USelectMenu
              v-model="reportAssetId"
              data-testid="report-asset-picker"
              value-key="value"
              :items="myAssignedAssets"
              :placeholder="t('maintenance.report.selectPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('maintenance.report.problem')"
            required
          >
            <AsyncSearchPicker
              :model-value="reportProblemId || null"
              :search-fn="problemCategoryPicker.searchFn"
              :resolve-fn="problemCategoryPicker.resolveFn"
              :placeholder="t('common.searchProblemCategory')"
              testid="report-problem"
              clearable
              @update:model-value="reportProblemId = $event ?? ''"
            />
          </UFormField>
          <UFormField :label="t('maintenance.report.description')">
            <UTextarea
              v-model="reportDesc"
              data-testid="report-description"
              :rows="3"
              :placeholder="t('maintenance.report.descPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('maintenance.report.photo')">
            <label
              for="maintenance-report-photo"
              class="block border-[1.5px] border-dashed border-default rounded-[11px] p-[18px] text-center cursor-pointer hover:border-primary transition-colors"
            >
              <div class="size-9 mx-auto mb-2 rounded-[9px] bg-muted text-muted flex items-center justify-center">
                <UIcon
                  name="i-lucide-camera"
                  class="size-[18px]"
                />
              </div>
              <div class="text-[12.5px] font-medium text-muted">
                {{ reportPhoto ? reportPhoto.name : t('maintenance.report.photoDrop') }}
              </div>
            </label>
            <input
              id="maintenance-report-photo"
              type="file"
              accept="image/*"
              class="hidden"
              data-testid="report-photo-input"
              @change="onPhotoChange"
            >
          </UFormField>
          <UButton
            icon="i-lucide-send"
            block
            :label="t('maintenance.report.submit')"
            :disabled="!reportReady"
            :loading="reportSubmitting"
            data-testid="report-submit"
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
          v-if="myReportsLoading"
          class="flex flex-col gap-2.5"
        >
          <div
            v-for="n in 2"
            :key="n"
            class="bg-default border border-default rounded-xl shadow-sm px-4 py-3.5 flex flex-col gap-2"
          >
            <USkeleton class="h-3 w-2/3 rounded" />
            <USkeleton class="h-3 w-1/3 rounded" />
          </div>
        </div>
        <div
          v-else-if="myReportRows.length > 0"
          class="flex flex-col gap-2.5"
        >
          <div
            v-for="r in myReportRows"
            :key="r.id"
            class="bg-default border border-default rounded-xl shadow-sm px-4 py-3.5"
            :data-testid="`report-history-${r.id}`"
          >
            <div class="flex items-start justify-between gap-2.5">
              <div class="min-w-0">
                <div class="text-[13.5px] font-semibold">
                  {{ r.assetName ?? r.assetTag }}
                </div>
                <div
                  v-if="r.assetName"
                  class="font-mono text-[11.5px] text-dimmed"
                >
                  {{ r.assetTag }}
                </div>
              </div>
              <UBadge
                :color="r.statusTone"
                variant="subtle"
                class="rounded-full flex-none"
              >
                {{ r.statusLabel }}
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

    <MaintenanceScheduleSlideover
      v-model:open="scheduleSlideoverOpen"
      :schedule="scheduleSlideoverTarget"
      @saved="onScheduleSaved"
    />
    <MaintenanceRecordSlideover
      v-model:open="recordSlideoverOpen"
      :record="recordSlideoverTarget"
      :prefill="recordSlideoverPrefill"
      @saved="onRecordSaved"
    />
  </div>
</template>
