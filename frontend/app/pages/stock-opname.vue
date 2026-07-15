<script setup lang="ts">
import type { ContextMenuItem } from '@nuxt/ui'
import type { BadgeColor, RowAction } from '~/types'
import type { OpnameItem, OpnameSession, OpnameSessionDetail } from '~/composables/api/useStockOpname'
import { ITEM_RESULT_TONE, SESSION_STATUS_TONE, type ItemResult, type SessionStatus } from '~/constants/stockOpnameMeta'
import { formatInt } from '~/utils/format'

definePageMeta({ middleware: 'can', permission: 'stockopname.view' })

const SEG_ORDER: ItemResult[] = ['found', 'damaged', 'misplaced', 'not_found']
const SEG_ICON: Record<ItemResult, string> = {
  pending: 'i-lucide-help-circle',
  found: 'i-lucide-check',
  damaged: 'i-lucide-wrench',
  misplaced: 'i-lucide-move',
  not_found: 'i-lucide-x'
}
const RESULT_ICON: Record<ItemResult, string> = {
  pending: 'i-lucide-clock',
  found: 'i-lucide-check-circle',
  damaged: 'i-lucide-wrench',
  misplaced: 'i-lucide-move',
  not_found: 'i-lucide-x-circle'
}
// Static per-segment active-state class (Tailwind can't see dynamically
// interpolated class names, so this must be a literal lookup table).
const SEG_ACTIVE_CLASS: Record<ItemResult, string> = {
  pending: 'bg-[var(--ui-text-dimmed)] text-white',
  found: 'bg-success text-white',
  damaged: 'bg-warning text-white',
  misplaced: 'bg-primary text-white',
  not_found: 'bg-error text-white'
}

const { t } = useI18n()
const can = useCan()
const toast = useToast()

const opnameApi = useStockOpname()

const canManage = computed(() => can('stockopname.manage'))

// ---------------------------------------------------------------------------
// View state — mirrors the mockup's list/detail toggle (no dedicated route).
// ---------------------------------------------------------------------------
type ViewMode = 'list' | 'detail'
const view = ref<ViewMode>('list')
const isList = computed(() => view.value === 'list')
const isDetail = computed(() => view.value === 'detail')

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------
const sessions = ref<OpnameSession[]>([])
const sessionKpis = ref<Record<string, { found: number, total: number }>>({})
const listLoading = ref(true)
const listError = ref(false)

async function loadList() {
  listLoading.value = true
  listError.value = false
  try {
    const page = await opnameApi.list()
    sessions.value = page.data
    // Best-effort per-session KPI fetch to drive the progress bar (list
    // endpoint itself carries no counts).
    const entries = await Promise.all(page.data.map(async (s) => {
      try {
        const d = await opnameApi.get(s.id)
        return [s.id, { found: d.found, total: d.total }] as const
      } catch {
        return [s.id, { found: 0, total: 0 }] as const
      }
    }))
    const map: Record<string, { found: number, total: number }> = {}
    for (const [id, kpi] of entries) map[id] = kpi
    sessionKpis.value = map
  } catch {
    listError.value = true
    sessions.value = []
  } finally {
    listLoading.value = false
  }
}

function kpiFor(id: string) {
  return sessionKpis.value[id] ?? { found: 0, total: 0 }
}

// ---------------------------------------------------------------------------
// Create session
// ---------------------------------------------------------------------------
const createOpen = ref(false)
const createSubmitting = ref(false)

function openCreate() {
  createOpen.value = true
}

async function onCreateConfirm(payload: { officeId: string, name: string, period: string }) {
  createSubmitting.value = true
  try {
    const created = await opnameApi.create({
      office_id: payload.officeId,
      name: payload.name || undefined,
      period: payload.period
    })
    createOpen.value = false
    toast.add({ title: t('stockOpname.submitSuccess', { name: created.name ?? payload.name }), color: 'success' })
    await loadList()
    await openDetail(created.id)
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    createSubmitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Detail
// ---------------------------------------------------------------------------
const activeId = ref<string | null>(null)
const detailLoading = ref(true)
const detailError = ref(false)
const detailSession = ref<OpnameSessionDetail | null>(null)

const itemsLoading = ref(true)
const itemsError = ref(false)
const allItems = ref<OpnameItem[]>([])

async function loadDetail(id: string) {
  detailLoading.value = true
  detailError.value = false
  try {
    detailSession.value = await opnameApi.get(id)
  } catch {
    detailError.value = true
    detailSession.value = null
  } finally {
    detailLoading.value = false
  }
}

async function loadItems(id: string) {
  itemsLoading.value = true
  itemsError.value = false
  try {
    const page = await opnameApi.items(id)
    allItems.value = page.data
  } catch {
    itemsError.value = true
    allItems.value = []
  } finally {
    itemsLoading.value = false
  }
}

async function openDetail(id: string) {
  activeId.value = id
  view.value = 'detail'
  await Promise.all([loadDetail(id), loadItems(id)])
}

function backToList() {
  view.value = 'list'
  activeId.value = null
}

async function reloadDetail() {
  if (!activeId.value) return
  await Promise.all([loadDetail(activeId.value), loadItems(activeId.value)])
}

const statusKey = computed<SessionStatus | null>(() => detailSession.value ? (detailSession.value.status as SessionStatus) : null)
const isOpenStatus = computed(() => statusKey.value === 'open')
const isCounting = computed(() => statusKey.value === 'counting')
const isReconciling = computed(() => statusKey.value === 'reconciling')
const isClosed = computed(() => statusKey.value === 'closed')
const isEditable = computed(() => isCounting.value)

// ---------------------------------------------------------------------------
// Item table — client-side search + room filter (per contract: backend item
// list only filters by `result`, so search/room narrow the loaded page).
// ---------------------------------------------------------------------------
const itemQuery = ref('')
const resultFilter = ref('all')
const roomFilter = ref('all')

const roomOptions = computed(() => {
  const names = new Set<string>()
  for (const it of allItems.value) {
    if (it.room_name) names.add(it.room_name)
  }
  return [{ value: 'all', label: t('stockOpname.allRooms') }, ...Array.from(names).map(n => ({ value: n, label: n }))]
})

const resultFilterOptions = computed(() => [
  { value: 'all', label: t('stockOpname.allResults') },
  ...(['found', 'not_found', 'damaged', 'misplaced', 'pending'] as ItemResult[]).map(k => ({ value: k, label: t(`stockOpname.result.${k}`) }))
])

const filteredItems = computed(() => {
  const q = itemQuery.value.trim().toLowerCase()
  return allItems.value.filter((it) => {
    if (resultFilter.value !== 'all' && it.result !== resultFilter.value) return false
    if (roomFilter.value !== 'all' && it.room_name !== roomFilter.value) return false
    if (q) {
      const hay = `${it.asset_name ?? ''} ${it.asset_tag ?? ''}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

function locationOf(it: OpnameItem): string {
  const parts = [it.floor_name, it.room_name].filter(Boolean)
  return parts.length ? parts.join(' · ') : '—'
}

async function setItemResult(item: OpnameItem, result: ItemResult) {
  if (!activeId.value || !isEditable.value) return
  try {
    const res = await opnameApi.setResult(activeId.value, item.id, { result })
    const idx = allItems.value.findIndex(i => i.id === item.id)
    if (idx !== -1) allItems.value[idx] = { ...allItems.value[idx]!, result: res.result }
    if (detailSession.value) await loadDetail(activeId.value)
  } catch {
    // useApiClient surfaces the error toast
  }
}

// Per-row actions (kebab dropdown via RowActionsMenu, and the table's
// right-click context menu below) — both built from this same list via
// buildActionGroups so their grouping/dividers stay in sync (see Task 8,
// mirrors assets/index.vue's Task 7 pattern). The segmented control stays as
// a fast-path; this menu is the standardized affordance and maps 1:1 onto
// SEG_ORDER via the existing setItemResult() setter. A locked (non-editable)
// session shows the read-only status badge instead — no menu.
function itemRowActions(item: OpnameItem): RowAction[] {
  if (!isEditable.value) return []
  return SEG_ORDER.map(seg => ({
    label: t('stockOpname.action.setResult', { result: t(`stockOpname.result.${seg}`) }),
    icon: SEG_ICON[seg],
    onSelect: () => setItemResult(item, seg)
  }))
}

const contextItems = ref<ContextMenuItem[][]>([])
function onItemRowContextMenu(item: OpnameItem) {
  contextItems.value = buildActionGroups(itemRowActions(item)) as ContextMenuItem[][]
}
// Safety net mirroring ResourceTable/disposals: a right-click that bubbles up
// from outside an actual item `tbody tr` (header, empty area) must clear any
// stale items left over from a previous row's right-click.
function onItemsTableContextMenu(e: MouseEvent) {
  const tr = (e.target as HTMLElement | null)?.closest('tbody tr')
  if (!tr) contextItems.value = []
}

// ---------------------------------------------------------------------------
// Scan bar
// ---------------------------------------------------------------------------
const manualCode = ref('')
const scanning = ref(false)

async function submitManualScan() {
  const code = manualCode.value.trim()
  if (!code || !activeId.value || scanning.value) return
  scanning.value = true
  try {
    await opnameApi.scan(activeId.value, code)
    manualCode.value = ''
    toast.add({ title: t('stockOpname.scan.foundMessage', { name: code }), color: 'success' })
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    scanning.value = false
  }
}

function onManualKey(e: KeyboardEvent) {
  if (e.key === 'Enter') submitManualScan()
}

// ---------------------------------------------------------------------------
// Variance panel
// ---------------------------------------------------------------------------
const varianceKinds: Array<'not_found' | 'damaged' | 'misplaced'> = ['not_found', 'damaged', 'misplaced']
const varianceItems = computed(() => allItems.value.filter(it => varianceKinds.includes(it.result as 'not_found' | 'damaged' | 'misplaced')))
const damagedCount = computed(() => allItems.value.filter(it => it.result === 'damaged').length)
const misplacedCount = computed(() => allItems.value.filter(it => it.result === 'misplaced').length)
const notFoundCount = computed(() => allItems.value.filter(it => it.result === 'not_found').length)

const VARIANCE_ICON: Record<'not_found' | 'damaged' | 'misplaced', string> = {
  not_found: 'i-lucide-x',
  damaged: 'i-lucide-wrench',
  misplaced: 'i-lucide-move'
}
const VARIANCE_TONE: Record<'not_found' | 'damaged' | 'misplaced', BadgeColor> = {
  not_found: 'error',
  damaged: 'warning',
  misplaced: 'primary'
}

async function followupNotFound(item: OpnameItem) {
  if (!activeId.value) return
  try {
    await opnameApi.followup(activeId.value, item.id, {})
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  }
}

async function followupDamaged(item: OpnameItem) {
  if (!activeId.value) return
  try {
    await opnameApi.followup(activeId.value, item.id, {})
    toast.add({ title: t('stockOpname.followup.maintenanceCreated', { name: item.asset_name ?? '—' }), color: 'success' })
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  }
}

function isFollowedUp(item: OpnameItem): boolean {
  return !!(item.followup_request_id || item.followup_record_id)
}

// Follow-up modal (misplaced → transfer)
const followupOpen = ref(false)
const followupItem = ref<OpnameItem | null>(null)
const followupOfficeId = ref('')
const followupSubmitting = ref(false)

function openFollowup(item: OpnameItem) {
  followupItem.value = item
  followupOfficeId.value = ''
  followupOpen.value = true
}

async function onFollowupConfirm(payload: { toOfficeId: string, toRoomId: string | null, reason: string }) {
  if (!activeId.value || !followupItem.value) return
  followupSubmitting.value = true
  try {
    await opnameApi.followup(activeId.value, followupItem.value.id, {
      to_office_id: payload.toOfficeId,
      to_room_id: payload.toRoomId,
      reason: payload.reason || null
    })
    followupOpen.value = false
    toast.add({ title: t('stockOpname.followup.success', { name: followupItem.value.asset_name ?? '—' }), color: 'success' })
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    followupSubmitting.value = false
  }
}

function onFollowupClick(item: OpnameItem, kind: 'not_found' | 'damaged' | 'misplaced') {
  if (kind === 'not_found') {
    followupNotFound(item)
  } else if (kind === 'misplaced') {
    openFollowup(item)
  } else if (kind === 'damaged') {
    followupDamaged(item)
  }
}

function followupLabel(item: OpnameItem): string {
  if (item.result === 'not_found') return t('stockOpname.variance.followupDisposal')
  if (item.result === 'damaged') return t('stockOpname.variance.followupMaintenance')
  return t('stockOpname.variance.followupTransfer')
}

// ---------------------------------------------------------------------------
// Transitions: start / reconcile / close
// ---------------------------------------------------------------------------
const transitioning = ref(false)

async function doStart() {
  if (!activeId.value || transitioning.value) return
  transitioning.value = true
  try {
    await opnameApi.start(activeId.value)
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    transitioning.value = false
  }
}

async function doReconcile() {
  if (!activeId.value || transitioning.value) return
  transitioning.value = true
  try {
    await opnameApi.reconcile(activeId.value)
    await reloadDetail()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    transitioning.value = false
  }
}

// ---------------------------------------------------------------------------
// Finish modal (close + report export)
// ---------------------------------------------------------------------------
const finishOpen = ref(false)
const finishSubmitting = ref(false)
const exporting = ref(false)

function openFinish() {
  finishOpen.value = true
}

async function onFinishConfirm() {
  if (!activeId.value || finishSubmitting.value) return
  finishSubmitting.value = true
  try {
    await opnameApi.close(activeId.value)
    finishOpen.value = false
    await reloadDetail()
    await loadList()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    finishSubmitting.value = false
  }
}

async function downloadReport(format: 'pdf' | 'xlsx') {
  if (!activeId.value) return
  exporting.value = true
  try {
    const blob = await opnameApi.reportUrl(activeId.value, format)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `berita-acara-opname-${activeId.value}.${format}`
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  loadList()
})
</script>

<template>
  <div>
    <!-- ============ LIST VIEW ============ -->
    <div
      v-if="isList"
      class="max-w-[1000px] mx-auto"
    >
      <div class="flex items-start justify-between gap-4 flex-wrap mb-[18px]">
        <div>
          <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
            {{ t('stockOpname.pageTitle') }}
          </h1>
          <p class="text-sm text-muted">
            {{ t('stockOpname.pageSub') }}
          </p>
        </div>
        <UButton
          v-if="canManage"
          icon="i-lucide-plus"
          :label="t('stockOpname.create.action')"
          data-testid="opname-create-open"
          @click="openCreate"
        />
      </div>

      <div
        v-if="listLoading"
        class="flex flex-col gap-3"
      >
        <USkeleton
          v-for="n in 3"
          :key="n"
          class="h-[76px] w-full rounded-[13px]"
        />
      </div>

      <div
        v-else-if="listError"
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
          data-testid="opname-retry"
          @click="loadList"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else-if="sessions.length === 0"
        data-testid="opname-empty"
        class="bg-default border border-default rounded-2xl shadow-sm py-[60px] px-6 text-center"
      >
        <div class="size-14 mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-clipboard-check"
            class="size-[27px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('stockOpname.empty.title') }}
        </div>
        <div class="text-sm text-muted mb-[18px]">
          {{ t('stockOpname.empty.sub') }}
        </div>
        <UButton
          v-if="canManage"
          icon="i-lucide-plus"
          :label="t('stockOpname.create.action')"
          @click="openCreate"
        />
      </div>

      <div
        v-else
        class="flex flex-col gap-3"
      >
        <StockopnameSessionCard
          v-for="s in sessions"
          :key="s.id"
          :session="s"
          :found="kpiFor(s.id).found"
          :total="kpiFor(s.id).total"
          @open="openDetail(s.id)"
        />
      </div>
    </div>

    <!-- ============ DETAIL VIEW ============ -->
    <div
      v-else-if="isDetail"
      class="max-w-[1060px] mx-auto"
    >
      <button
        type="button"
        class="inline-flex items-center gap-1.5 text-[13px] text-muted hover:text-primary mb-3"
        data-testid="opname-back"
        @click="backToList"
      >
        <UIcon
          name="i-lucide-arrow-left"
          class="size-[14px]"
        />
        {{ t('stockOpname.pageTitle') }}
      </button>

      <div
        v-if="detailLoading"
        class="flex flex-col gap-4"
      >
        <USkeleton class="h-10 w-1/2 rounded-lg" />
        <div class="grid grid-cols-4 gap-3.5">
          <USkeleton
            v-for="n in 4"
            :key="n"
            class="h-24 rounded-[13px]"
          />
        </div>
      </div>

      <div
        v-else-if="detailError"
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
          data-testid="opname-detail-retry"
          @click="reloadDetail"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <template v-else-if="detailSession">
        <!-- Detail header -->
        <div class="flex items-start justify-between gap-4 flex-wrap mb-[18px]">
          <div>
            <div class="flex items-center gap-2.5 flex-wrap mb-1">
              <h1 class="text-[22px] font-bold tracking-tight">
                {{ detailSession.name ?? '—' }}
              </h1>
              <UBadge
                :color="SESSION_STATUS_TONE[statusKey!]"
                variant="subtle"
                class="rounded-full gap-1.5"
              >
                {{ t(`stockOpname.status.${statusKey}`) }}
              </UBadge>
            </div>
            <div class="flex items-center gap-3.5 flex-wrap text-[13px] text-muted">
              <span class="inline-flex items-center gap-1.5">
                <UIcon
                  name="i-lucide-building-2"
                  class="size-[14px]"
                />
                {{ detailSession.office_name ?? '—' }}
              </span>
              <span class="inline-flex items-center gap-1.5">
                <UIcon
                  name="i-lucide-calendar"
                  class="size-[14px]"
                />
                {{ detailSession.period }}
              </span>
            </div>
          </div>
          <div class="flex gap-2.5 flex-wrap">
            <span
              v-if="isClosed"
              class="inline-flex items-center gap-1.5 px-3.5 py-2 text-[13px] font-medium text-success bg-success/10 rounded-lg"
            >
              <UIcon
                name="i-lucide-lock"
                class="size-[15px]"
              />
              {{ t('stockOpname.finish.lockedNote') }}
            </span>
            <UButton
              color="neutral"
              variant="outline"
              icon="i-lucide-file-text"
              :label="t('stockOpname.exportAction')"
              data-testid="opname-export"
              :loading="exporting"
              @click="downloadReport('pdf')"
            />
            <UButton
              v-if="canManage && isOpenStatus"
              icon="i-lucide-play"
              :label="t('stockOpname.startAction')"
              data-testid="opname-start"
              :loading="transitioning"
              @click="doStart"
            />
            <UButton
              v-if="canManage && isCounting"
              icon="i-lucide-git-compare"
              :label="t('stockOpname.reconcileAction')"
              data-testid="opname-reconcile"
              :loading="transitioning"
              @click="doReconcile"
            />
            <UButton
              v-if="canManage && isReconciling"
              icon="i-lucide-check-circle"
              :label="t('stockOpname.finish.action')"
              data-testid="opname-finish-open"
              @click="openFinish"
            />
          </div>
        </div>

        <!-- KPI tiles -->
        <div class="grid grid-cols-4 gap-3.5 mb-4">
          <div
            data-testid="opname-kpi-total"
            class="bg-default border border-default rounded-[13px] shadow-sm p-4"
          >
            <div class="flex items-center gap-2 text-[12.5px] text-muted">
              <UIcon
                name="i-lucide-package"
                class="size-[15px]"
              />
              {{ t('stockOpname.kpi.total') }}
            </div>
            <div class="text-[26px] font-bold tracking-tight mt-2">
              {{ formatInt(detailSession.total) }}
            </div>
          </div>
          <div
            data-testid="opname-kpi-found"
            class="bg-default border border-default rounded-[13px] shadow-sm p-4"
          >
            <div class="flex items-center gap-2 text-[12.5px] text-muted">
              <UIcon
                name="i-lucide-check-circle"
                class="size-[15px] text-success"
              />
              {{ t('stockOpname.kpi.found') }}
            </div>
            <div class="text-[26px] font-bold tracking-tight mt-2">
              {{ formatInt(detailSession.found) }}
            </div>
          </div>
          <div
            data-testid="opname-kpi-pending"
            class="bg-default border border-default rounded-[13px] shadow-sm p-4"
          >
            <div class="flex items-center gap-2 text-[12.5px] text-muted">
              <UIcon
                name="i-lucide-clock"
                class="size-[15px] text-warning"
              />
              {{ t('stockOpname.kpi.pending') }}
            </div>
            <div class="text-[26px] font-bold tracking-tight mt-2">
              {{ formatInt(detailSession.pending) }}
            </div>
          </div>
          <div
            data-testid="opname-kpi-variance"
            class="bg-default border rounded-[13px] shadow-sm p-4"
            :class="detailSession.variance > 0 ? 'border-error/35' : 'border-default'"
          >
            <div class="flex items-center gap-2 text-[12.5px] text-muted">
              <UIcon
                name="i-lucide-alert-triangle"
                class="size-[15px] text-error"
              />
              {{ t('stockOpname.kpi.variance') }}
            </div>
            <div
              class="text-[26px] font-bold tracking-tight mt-2"
              :class="detailSession.variance > 0 ? 'text-error' : ''"
            >
              {{ formatInt(detailSession.variance) }}
            </div>
          </div>
        </div>

        <!-- Scan bar (counting only) -->
        <div
          v-if="isCounting"
          class="flex gap-3.5 items-stretch flex-wrap mb-[18px]"
        >
          <div class="flex-1 min-w-[240px] flex flex-col justify-center gap-1.5 p-3.5 bg-default border border-default rounded-[13px] shadow-sm">
            <label class="text-xs font-medium text-muted">{{ t('stockOpname.scan.manualLabel') }}</label>
            <div class="flex gap-2">
              <UInput
                v-model="manualCode"
                data-testid="opname-scan-input"
                class="flex-1 font-mono"
                :placeholder="t('stockOpname.scan.placeholder')"
                @keydown="onManualKey"
              />
              <UButton
                color="neutral"
                variant="outline"
                data-testid="opname-scan-check"
                :loading="scanning"
                @click="submitManualScan"
              >
                {{ t('stockOpname.scan.check') }}
              </UButton>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-4 items-start">
          <!-- Item table -->
          <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
            <div class="flex items-center gap-2.5 flex-wrap p-3.5 border-b border-default">
              <UInput
                v-model="itemQuery"
                data-testid="opname-item-search"
                icon="i-lucide-search"
                :placeholder="t('stockOpname.searchPlaceholder')"
                class="flex-1 min-w-[160px]"
              />
              <USelect
                v-model="resultFilter"
                data-testid="opname-item-result-filter"
                value-key="value"
                :items="resultFilterOptions"
                class="min-w-[140px]"
              />
              <USelect
                v-model="roomFilter"
                data-testid="opname-item-room-filter"
                value-key="value"
                :items="roomOptions"
                class="min-w-[140px]"
              />
            </div>

            <div
              v-if="itemsLoading"
              class="p-4 flex flex-col gap-2.5"
            >
              <USkeleton
                v-for="n in 4"
                :key="n"
                class="h-10 w-full rounded-lg"
              />
            </div>
            <div
              v-else-if="itemsError"
              class="py-10 px-6 text-center"
            >
              <p class="text-sm text-muted mb-3">
                {{ t('common.loadError') }}
              </p>
              <UButton
                size="sm"
                color="neutral"
                variant="outline"
                icon="i-lucide-rotate-cw"
                @click="() => { activeId && loadItems(activeId) }"
              >
                {{ t('common.retry') }}
              </UButton>
            </div>
            <div
              v-else-if="filteredItems.length === 0"
              class="py-10 px-6 text-center text-[13px] text-dimmed"
            >
              {{ t('stockOpname.itemsNoMatch') }}
            </div>
            <UContextMenu
              v-else
              :items="contextItems"
              :disabled="filteredItems.length === 0"
            >
              <div
                class="overflow-x-auto"
                @contextmenu="onItemsTableContextMenu"
              >
                <table class="w-full border-collapse text-[13px] whitespace-nowrap">
                  <thead>
                    <tr class="bg-muted text-muted">
                      <th class="text-left px-4 py-[10px] text-[11.5px] font-semibold uppercase tracking-wide">
                        {{ t('stockOpname.column.asset') }}
                      </th>
                      <th class="text-left px-3 py-[10px] text-[11.5px] font-semibold uppercase tracking-wide">
                        {{ t('stockOpname.column.location') }}
                      </th>
                      <th class="text-left px-3 py-[10px] text-[11.5px] font-semibold uppercase tracking-wide">
                        {{ t('stockOpname.column.result') }}
                      </th>
                      <th class="text-right px-4 py-[10px] text-[11.5px] font-semibold uppercase tracking-wide">
                        {{ t('common.actions') }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="it in filteredItems"
                      :key="it.id"
                      data-testid="opname-item-row"
                      class="border-t border-default hover:bg-muted/60 transition-colors"
                      @contextmenu="onItemRowContextMenu(it)"
                    >
                      <td class="px-4 py-2.5">
                        <div class="font-medium">
                          {{ it.asset_name ?? '—' }}
                        </div>
                        <div class="font-mono text-[11px] text-dimmed">
                          {{ it.asset_tag ?? '—' }}
                        </div>
                      </td>
                      <td class="px-3 py-2.5 text-muted">
                        {{ locationOf(it) }}
                      </td>
                      <td class="px-3 py-2.5">
                        <div
                          v-if="isEditable"
                          class="inline-flex gap-0.5 p-0.5 bg-muted rounded-lg"
                        >
                          <button
                            v-for="seg in SEG_ORDER"
                            :key="seg"
                            type="button"
                            :data-testid="`opname-result-${seg}`"
                            :title="t(`stockOpname.result.${seg}`)"
                            class="flex items-center justify-center size-[26px] rounded-md"
                            :class="it.result === seg ? SEG_ACTIVE_CLASS[seg] : 'text-dimmed hover:bg-default'"
                            @click="setItemResult(it, seg)"
                          >
                            <UIcon
                              :name="SEG_ICON[seg]"
                              class="size-[13px]"
                            />
                          </button>
                        </div>
                        <UBadge
                          v-else
                          :color="ITEM_RESULT_TONE[it.result as ItemResult]"
                          variant="subtle"
                          class="rounded-full gap-1.5"
                        >
                          <UIcon
                            :name="RESULT_ICON[it.result as ItemResult]"
                            class="size-3"
                          />
                          {{ t(`stockOpname.result.${it.result}`) }}
                        </UBadge>
                      </td>
                      <td
                        class="px-3 py-2.5 text-right"
                        @click.stop
                      >
                        <RowActionsMenu :items="itemRowActions(it)" />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </UContextMenu>
          </div>

          <!-- Variance panel -->
          <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
            <div class="flex items-center gap-2.5 p-3.5 border-b border-default">
              <span class="size-7 rounded-lg bg-warning/15 text-warning flex items-center justify-center flex-none">
                <UIcon
                  name="i-lucide-alert-triangle"
                  class="size-[15px]"
                />
              </span>
              <div class="flex-1 min-w-0">
                <div class="text-sm font-semibold">
                  {{ t('stockOpname.variance.title') }}
                </div>
                <div class="text-[11px] text-dimmed">
                  {{ t('stockOpname.variance.count', { n: varianceItems.length }) }}
                </div>
              </div>
            </div>

            <div
              v-if="varianceItems.length === 0"
              class="py-11 px-5 text-center"
            >
              <div class="size-11 mx-auto mb-2.5 rounded-[11px] bg-success/15 text-success flex items-center justify-center">
                <UIcon
                  name="i-lucide-check-circle"
                  class="size-[21px]"
                />
              </div>
              <div class="text-[13.5px] font-semibold mb-1">
                {{ t('stockOpname.variance.empty') }}
              </div>
              <div class="text-xs text-muted">
                {{ t('stockOpname.variance.emptySub') }}
              </div>
            </div>

            <div
              v-else
              class="max-h-[520px] overflow-y-auto"
            >
              <div
                v-for="it in varianceItems"
                :key="it.id"
                class="p-3.5 border-b border-default"
              >
                <div class="flex items-start gap-2.5">
                  <span
                    class="size-[26px] rounded-lg flex items-center justify-center flex-none mt-px"
                    :class="{
                      'bg-error/15 text-error': it.result === 'not_found',
                      'bg-warning/15 text-warning': it.result === 'damaged',
                      'bg-primary/15 text-primary': it.result === 'misplaced'
                    }"
                  >
                    <UIcon
                      :name="VARIANCE_ICON[it.result as 'not_found' | 'damaged' | 'misplaced']"
                      class="size-[13px]"
                    />
                  </span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[13px] font-medium truncate">
                      {{ it.asset_name ?? '—' }}
                    </div>
                    <div class="font-mono text-[11px] text-dimmed">
                      {{ it.asset_tag ?? '—' }}
                    </div>
                    <UBadge
                      :color="VARIANCE_TONE[it.result as 'not_found' | 'damaged' | 'misplaced']"
                      variant="subtle"
                      size="sm"
                      class="rounded-full mt-1"
                    >
                      {{ t(`stockOpname.result.${it.result}`) }}
                    </UBadge>
                  </div>
                </div>
                <UButton
                  block
                  size="sm"
                  color="neutral"
                  variant="soft"
                  class="mt-2.5"
                  :data-testid="`opname-followup-${it.result}`"
                  :disabled="!canManage || isFollowedUp(it)"
                  @click="onFollowupClick(it, it.result as 'not_found' | 'damaged' | 'misplaced')"
                >
                  {{ followupLabel(it) }}
                </UButton>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Create session modal -->
    <StockopnameCreateSessionModal
      v-model:open="createOpen"
      :submitting="createSubmitting"
      @confirm="onCreateConfirm"
    />

    <!-- Follow-up modal (misplaced → transfer) -->
    <StockopnameFollowupModal
      v-model:open="followupOpen"
      :item="followupItem"
      :submitting="followupSubmitting"
      @confirm="onFollowupConfirm"
    />

    <!-- Finish modal -->
    <StockopnameFinishModal
      v-model:open="finishOpen"
      :session="detailSession"
      :damaged="damagedCount"
      :misplaced="misplacedCount"
      :not-found="notFoundCount"
      :submitting="finishSubmitting"
      :exporting="exporting"
      @confirm="onFinishConfirm"
      @export="downloadReport"
    />
  </div>
</template>
