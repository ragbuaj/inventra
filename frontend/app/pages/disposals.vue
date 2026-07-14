<script setup lang="ts">
import type { ContextMenuItem } from '@nuxt/ui'
import type { Asset, AssetAttachment, AssetStatus, BadgeColor, Office, RowAction } from '~/types'
import type { Disposal, DisposalSubmitInput } from '~/composables/api/useDisposals'
import type { ApprovalRequestRow, ApprovalStep } from '~/composables/api/useApproval'
import type { PreviewStep } from '~/composables/api/useApprovalPreview'
import type { AssetDepreciationResponse } from '~/composables/api/useDepreciation'
import type { DisposalHistoryRow, DisposalHistoryStatus } from '~/utils/disposalHistory'
import { METHOD_KEYS, METHOD_TONE, type DisposalMethod } from '~/constants/disposalMeta'
import { mergeDisposalHistory } from '~/utils/disposalHistory'
import { formatRupiah } from '~/utils/format'

definePageMeta({ middleware: 'can', permission: 'disposal.view' })

const PICKER_STATUSES: AssetStatus[] = ['available', 'under_maintenance']

type ChainState = 'idle' | 'loading' | 'ready' | 'not_configured' | 'masked'
type TimelineStatus = 'done' | 'current' | 'queued'

interface SubmittedSnapshot {
  requestId: string
  assetName: string
  assetTag: string
  bastNo: string | null
  method: DisposalMethod
  proceeds: string
  gainLoss: number | null
}

const { t, locale } = useI18n()
const auth = useAuthStore()
const can = useCan()
const toast = useToast()

const disposalsApi = useDisposals()
const approvalApi = useApproval()
const previewApi = useApprovalPreview()
const attachmentsApi = useAssetAttachments()
const officesApi = useOffices()
const deprApi = useDepreciation()

const canManage = computed(() => can('disposal.manage'))

type TabKey = 'ajukan' | 'history'
const tab = ref<TabKey>('ajukan')

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-GB' : 'id-ID', {
    day: '2-digit', month: 'short', year: 'numeric'
  }).format(d)
}

function formatSigned(v: number): string {
  const sign = v >= 0 ? '+' : '−'
  return `${sign} ${formatRupiah(Math.abs(v))}`
}

// ---------------------------------------------------------------------------
// Lookups (office names for the asset picker's search results)
// ---------------------------------------------------------------------------
const offices = ref<Office[]>([])
const officeNameMap = computed(() => new Map(offices.value.map(o => [o.id, o.name])))

async function loadOffices() {
  try {
    offices.value = await officesApi.tree()
  } catch {
    // Best-effort — the picker just shows '—' for office names if this fails.
  }
}

// ---------------------------------------------------------------------------
// Ajukan Penghapusan — pre-submit form
// ---------------------------------------------------------------------------
const pickerKey = ref(0)
const selectedAsset = ref<Asset | null>(null)
const method = ref<DisposalMethod>('sale')
const proceedsRaw = ref('')
const disposalDate = ref('')
const bastNo = ref('')
const reason = ref('')
const submitting = ref(false)

const evidenceUploads = ref<AssetAttachment[]>([])
const evidenceUploading = ref(false)
const evidenceInput = ref<HTMLInputElement | null>(null)

function openEvidencePicker() {
  if (!selectedAsset.value || !canManage.value) return
  evidenceInput.value?.click()
}

const methodItems = computed(() => METHOD_KEYS.map(k => ({ value: k, label: t(`disposal.method.${k}`) })))

const formReady = computed(() => !!(selectedAsset.value && disposalDate.value && method.value))

function onSelectAsset(asset: Asset) {
  selectedAsset.value = asset
  evidenceUploads.value = []
}

async function onEvidenceFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (!files.length || !selectedAsset.value || !canManage.value) return
  evidenceUploading.value = true
  try {
    for (const file of files) {
      const att = await attachmentsApi.upload(selectedAsset.value.id, file)
      evidenceUploads.value.push(att)
    }
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    evidenceUploading.value = false
  }
}

async function removeEvidence(att: AssetAttachment) {
  if (!selectedAsset.value || !canManage.value) return
  try {
    await attachmentsApi.remove(selectedAsset.value.id, att.id)
    evidenceUploads.value = evidenceUploads.value.filter(a => a.id !== att.id)
  } catch {
    // useApiClient surfaces the error toast
  }
}

// Ringkasan Valuasi (read-only) — money fields absent from the payload mean
// field-permission masked them for the caller's role.
const acquisitionMasked = computed(() => !!selectedAsset.value && selectedAsset.value.purchase_cost === undefined)
const accumMasked = computed(() => !!selectedAsset.value && selectedAsset.value.accumulated_depreciation === undefined)
const bookValueMasked = computed(() => !!selectedAsset.value && selectedAsset.value.book_value === undefined)

// ---------------------------------------------------------------------------
// Depreciation schedule for the selected asset — fetched alongside the
// approval-chain preview (see the `selectedAsset` watcher below). Drives the
// fiscal valuation cell/gain-loss row and the book-value-based preview
// amount (mirrors the server's basis switch on submit).
// ---------------------------------------------------------------------------
const deprSchedule = ref<AssetDepreciationResponse | null>(null)

// Commercial book value: prefer the freshly-computed value from the
// depreciation module over the (possibly stale) asset.book_value column.
const commercialBookValue = computed<string | null>(() => {
  if (!selectedAsset.value) return null
  return deprSchedule.value?.computed_book_value ?? selectedAsset.value.book_value ?? null
})

// Fiscal book value = closing of the most recent fiscal-basis entry (entries
// come back ordered ascending by period); "—" when nothing has been computed
// on the fiscal basis yet.
const fiscalBookValue = computed<string | null>(() => {
  const fiscalEntries = (deprSchedule.value?.entries ?? []).filter(e => e.basis === 'fiscal')
  return fiscalEntries.length > 0 ? fiscalEntries[fiscalEntries.length - 1]!.closing : null
})

// The amount the server's basis switch would use for the approval-chain
// preview: the computed book value when known, else the acquisition cost.
const chainPreviewAmount = computed<string | null>(() => {
  if (!selectedAsset.value) return null
  return deprSchedule.value?.computed_book_value ?? selectedAsset.value.purchase_cost ?? null
})

// ---------------------------------------------------------------------------
// Laba/Rugi card — pure client-side computation, no fetch involved.
// ---------------------------------------------------------------------------
type GainLossState = 'empty' | 'masked' | 'result'
const gainLossState = computed<GainLossState>(() => {
  if (!selectedAsset.value || proceedsRaw.value === '') return 'empty'
  if (bookValueMasked.value || commercialBookValue.value == null) return 'masked'
  return 'result'
})
const gainLossValue = computed<number | null>(() => {
  if (gainLossState.value !== 'result' || commercialBookValue.value == null) return null
  // Uses the server-computed book value (BookValueAsOf) — the same basis the
  // backend records gain_loss from — so the maker's preview matches what
  // approval will actually persist.
  return Number(proceedsRaw.value) - Number(commercialBookValue.value)
})
const gainLossVariant = computed<'gain' | 'loss' | 'breakEven' | null>(() => {
  const v = gainLossValue.value
  if (v === null) return null
  if (v > 0) return 'gain'
  if (v < 0) return 'loss'
  return 'breakEven'
})

// Fiscal gain/loss = proceeds − fiscal book value; "—" until both a sale
// value is entered and a fiscal-basis closing exists for the asset.
const fiscalGainLoss = computed<number | null>(() => {
  if (proceedsRaw.value === '' || fiscalBookValue.value === null) return null
  return Number(proceedsRaw.value) - Number(fiscalBookValue.value)
})

// ---------------------------------------------------------------------------
// Jenjang Persetujuan card
// ---------------------------------------------------------------------------
const chainState = ref<ChainState>('idle')
const chainSteps = ref<PreviewStep[]>([])

watch(selectedAsset, async (asset) => {
  chainSteps.value = []
  deprSchedule.value = null
  if (!asset) {
    chainState.value = 'idle'
    return
  }
  // Best-effort: pulls the fiscal valuation + computed book value used below
  // and by the valuation/gain-loss cards. Its failure must not block the
  // approval-chain preview itself.
  try {
    deprSchedule.value = await deprApi.assetSchedule(asset.id)
  } catch {
    deprSchedule.value = null
  }
  if (asset.purchase_cost == null) {
    chainState.value = 'masked'
    return
  }
  chainState.value = 'loading'
  try {
    chainSteps.value = await previewApi.preview('asset_disposal', chainPreviewAmount.value ?? asset.purchase_cost)
    chainState.value = 'ready'
  } catch {
    chainState.value = 'not_configured'
  }
})

const chainRows = computed(() => {
  if (chainState.value !== 'ready') return []
  return chainSteps.value.map((s, i) => ({
    num: i + 2,
    role: t(`approval.level.${s.required_level}`),
    note: t('disposal.chain.note.always')
  }))
})

// ---------------------------------------------------------------------------
// Post-submit view
// ---------------------------------------------------------------------------
const submitted = ref(false)
const submittedSnapshot = ref<SubmittedSnapshot | null>(null)
const requestId = ref<string | null>(null)

const timelineLoading = ref(false)
const timelineError = ref(false)
const timelineSteps = ref<ApprovalStep[]>([])
const timelineCurrentStep = ref(1)
const timelineMakerName = ref('')
const timelineMakerDate = ref<string | null>(null)

async function loadTimeline(id: string) {
  timelineLoading.value = true
  timelineError.value = false
  try {
    const detail = await approvalApi.get(id)
    timelineSteps.value = detail.steps
    timelineCurrentStep.value = detail.current_step
    timelineMakerName.value = detail.requested_by_name ?? auth.user?.name ?? '—'
    timelineMakerDate.value = detail.created_at
  } catch {
    timelineError.value = true
  } finally {
    timelineLoading.value = false
  }
}

interface TimelineRow {
  role: string
  status: TimelineStatus
  meta: string
}

const timelineRows = computed<TimelineRow[]>(() => {
  const rows: TimelineRow[] = [{
    role: timelineMakerName.value,
    status: 'done',
    meta: `${timelineMakerName.value} · ${fmtDate(timelineMakerDate.value)}`
  }]
  for (const step of timelineSteps.value) {
    const role = t(`approval.level.${step.required_level}`)
    if (step.decision === 'approved' || step.decision === 'rejected') {
      rows.push({ role, status: 'done', meta: `${step.approver_name ?? '—'} · ${fmtDate(step.decided_at)}` })
    } else if (step.step_order === timelineCurrentStep.value) {
      rows.push({ role, status: 'current', meta: t('disposal.timeline.awaitingReview') })
    } else {
      rows.push({ role, status: 'queued', meta: t('disposal.timeline.queuedNote') })
    }
  }
  return rows
})

const TIMELINE_DOT: Record<TimelineStatus, BadgeColor> = { done: 'success', current: 'warning', queued: 'neutral' }
const TIMELINE_ICON: Record<TimelineStatus, string> = { done: 'i-lucide-check', current: 'i-lucide-clock', queued: 'i-lucide-clock' }
const TIMELINE_LABEL_KEY: Record<TimelineStatus, string> = { done: 'disposal.timeline.done', current: 'disposal.timeline.current', queued: 'disposal.timeline.queued' }

async function submitDisposal() {
  if (!canManage.value) return
  if (!formReady.value || submitting.value) return
  submitting.value = true
  try {
    const asset = selectedAsset.value!
    const input: DisposalSubmitInput = {
      asset_id: asset.id,
      method: method.value,
      disposal_date: disposalDate.value,
      proceeds: proceedsRaw.value !== '' ? proceedsRaw.value : null,
      bast_no: bastNo.value.trim() || null,
      reason: reason.value.trim() || null
    }
    const res = await disposalsApi.submit(input)
    submittedSnapshot.value = {
      requestId: res.request_id,
      assetName: asset.name,
      assetTag: asset.asset_tag,
      bastNo: bastNo.value.trim() || null,
      method: method.value,
      proceeds: proceedsRaw.value,
      gainLoss: gainLossValue.value
    }
    requestId.value = res.request_id
    submitted.value = true
    await Promise.all([loadTimeline(res.request_id), loadHistory()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  submitted.value = false
  submittedSnapshot.value = null
  requestId.value = null
  selectedAsset.value = null
  method.value = 'sale'
  proceedsRaw.value = ''
  disposalDate.value = ''
  bastNo.value = ''
  reason.value = ''
  evidenceUploads.value = []
  chainState.value = 'idle'
  chainSteps.value = []
  deprSchedule.value = null
  timelineSteps.value = []
  timelineCurrentStep.value = 1
  timelineMakerName.value = ''
  timelineMakerDate.value = null
  pickerKey.value++
}

// ---------------------------------------------------------------------------
// Riwayat
// ---------------------------------------------------------------------------
const historyRequests = ref<ApprovalRequestRow[]>([])
const historyDisposals = ref<Disposal[]>([])
const historyLoading = ref(true)
const historyError = ref(false)
const historyQuery = ref('')
const historyStatus = ref<'all' | DisposalHistoryStatus>('all')

async function loadHistory() {
  historyLoading.value = true
  historyError.value = false
  try {
    const [reqPage, dpPage] = await Promise.all([
      approvalApi.list({ type: 'asset_disposal', limit: 100 }),
      disposalsApi.list({ limit: 100 })
    ])
    historyRequests.value = reqPage.data
    historyDisposals.value = dpPage.data
  } catch {
    historyError.value = true
    historyRequests.value = []
    historyDisposals.value = []
  } finally {
    historyLoading.value = false
  }
}

const mergedHistory = computed<DisposalHistoryRow[]>(() => mergeDisposalHistory(historyRequests.value, historyDisposals.value, {
  fmtDate,
  assetName: () => null,
  canAttach: canManage.value
}))

// Deviation (h): no "Disetujui" option — an approved disposal is always
// executed immediately, so DisposalHistoryStatus itself has no such state.
const HISTORY_STATUS_KEYS: DisposalHistoryStatus[] = ['menunggu', 'ditolak', 'dibatalkan', 'selesai']
const HISTORY_STATUS_TONE: Record<DisposalHistoryStatus, BadgeColor> = {
  menunggu: 'warning',
  ditolak: 'error',
  dibatalkan: 'neutral',
  selesai: 'neutral'
}

const statusFilterItems = computed(() => [
  { value: 'all', label: t('disposal.statusFilter.all') },
  ...HISTORY_STATUS_KEYS.map(k => ({ value: k, label: t(`disposal.statusFilter.${k}`) }))
])

const filteredHistory = computed(() => {
  const q = historyQuery.value.trim().toLowerCase()
  return mergedHistory.value.filter((row) => {
    if (historyStatus.value !== 'all' && row.status !== historyStatus.value) return false
    if (q) {
      const hay = `${row.assetLabel} ${row.assetTag ?? ''}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

// ---------------------------------------------------------------------------
// Lampirkan BAST modal
// ---------------------------------------------------------------------------
const attachOpen = ref(false)
const attachTarget = ref<DisposalHistoryRow | null>(null)
const attachBastNo = ref('')
const attachDocNo = ref('')
const attachDocDate = ref('')
const attachCounterparty = ref('')
const attachFile = ref<File | null>(null)
const attachSubmitting = ref(false)

function openAttach(row: DisposalHistoryRow) {
  attachTarget.value = row
  attachBastNo.value = ''
  attachDocNo.value = ''
  attachDocDate.value = ''
  attachCounterparty.value = ''
  attachFile.value = null
  attachOpen.value = true
}

function onAttachFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  attachFile.value = input.files?.[0] ?? null
}

async function confirmAttach() {
  const target = attachTarget.value
  if (!target || attachSubmitting.value) return
  attachSubmitting.value = true
  try {
    await disposalsApi.attachDocument(target.id, {
      bast_no: attachBastNo.value.trim() || undefined,
      doc_no: attachDocNo.value.trim() || undefined,
      doc_date: attachDocDate.value || undefined,
      counterparty: attachCounterparty.value.trim() || undefined,
      file: attachFile.value
    })
    attachOpen.value = false
    toast.add({ title: t('disposal.attachBast.success', { name: target.assetLabel }), color: 'success' })
    await loadHistory()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    attachSubmitting.value = false
  }
}

// Per-row actions (kebab dropdown via RowActionsMenu, and the table's
// right-click context menu below) — both built from this same list via
// buildActionGroups so their grouping/dividers stay in sync (see Task 8,
// mirrors assets/index.vue's Task 7 pattern). "Lampirkan BAST" only applies
// to executed (selesai) disposals without a BAST yet (row.canAttach).
function rowActions(row: DisposalHistoryRow): RowAction[] {
  return row.canAttach
    ? [{ label: t('disposal.attachBast.title'), icon: 'i-lucide-paperclip', onSelect: () => openAttach(row) }]
    : []
}

const contextItems = ref<ContextMenuItem[][]>([])
function onRowContextMenu(row: DisposalHistoryRow) {
  contextItems.value = buildActionGroups(rowActions(row)) as ContextMenuItem[][]
}
// Safety net mirroring ResourceTable/assets-index: a right-click that bubbles
// up from outside a `tbody tr` (header row, empty table area) must clear any
// stale items left over from a previous row's right-click — otherwise the
// menu would surface the previous row's actions.
function onTableContextMenu(e: MouseEvent) {
  const tr = (e.target as HTMLElement | null)?.closest('tbody tr')
  if (!tr) contextItems.value = []
}

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------
const tabs = computed(() => [
  { key: 'ajukan' as const, label: t('disposal.tabs.ajukan'), icon: 'i-lucide-trash-2' },
  { key: 'history' as const, label: t('disposal.tabs.history'), icon: 'i-lucide-history' }
])

onMounted(() => {
  loadOffices()
  loadHistory()
})
</script>

<template>
  <div class="max-w-[1000px] mx-auto">
    <!-- Header -->
    <div class="mb-[18px]">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('disposal.pageTitle') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('disposal.pageSub') }}
      </p>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-default mb-[22px]">
      <button
        v-for="tb in tabs"
        :key="tb.key"
        class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-sm border-b-2 transition-colors"
        :class="tab === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
        :data-testid="`disposal-tab-${tb.key}`"
        @click="tab = tb.key"
      >
        <UIcon
          :name="tb.icon"
          class="size-[15px]"
        />
        {{ tb.label }}
      </button>
    </div>

    <!-- ============ AJUKAN PENGHAPUSAN ============ -->
    <div v-if="tab === 'ajukan'">
      <!-- POST-SUBMIT -->
      <div
        v-if="submitted && submittedSnapshot"
        class="max-w-[640px]"
      >
        <div class="flex gap-3 items-center px-4 py-3.5 mb-[18px] rounded-xl bg-success/10 border border-success/30">
          <UIcon
            name="i-lucide-circle-check"
            class="size-5 flex-none text-success"
          />
          <div>
            <div class="text-[14.5px] font-semibold text-success">
              {{ t('disposal.submitted.title') }}
            </div>
            <div class="text-[13px] text-muted mt-px">
              {{ t('disposal.submitted.sub') }}
            </div>
          </div>
        </div>

        <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden mb-4">
          <div class="flex items-center gap-3 px-[18px] py-4 border-b border-default">
            <span class="size-10 rounded-[10px] bg-error/10 text-error flex items-center justify-center flex-none">
              <UIcon
                name="i-lucide-trash-2"
                class="size-[19px]"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="text-[14.5px] font-semibold">
                {{ submittedSnapshot.assetName }}
              </div>
              <div class="font-mono text-xs text-dimmed">
                {{ submittedSnapshot.assetTag }} · {{ submittedSnapshot.bastNo ?? '—' }}
              </div>
            </div>
            <UBadge
              color="warning"
              variant="subtle"
              class="rounded-full gap-1.5"
              data-testid="disposal-submitted-status"
            >
              <span class="size-1.5 rounded-full bg-warning" />
              {{ t('disposal.submitted.status') }}
            </UBadge>
          </div>
          <div class="px-[18px] py-4 grid grid-cols-3 gap-3.5">
            <div>
              <div class="text-xs text-muted">
                {{ t('disposal.submitted.method') }}
              </div>
              <div class="text-sm font-semibold mt-0.5">
                {{ t(`disposal.method.${submittedSnapshot.method}`) }}
              </div>
            </div>
            <div>
              <div class="text-xs text-muted">
                {{ t('disposal.submitted.value') }}
              </div>
              <div class="text-sm font-semibold mt-0.5">
                {{ formatRupiah(submittedSnapshot.proceeds || null) }}
              </div>
            </div>
            <div>
              <div class="text-xs text-muted">
                {{ t('disposal.submitted.gainLoss') }}
              </div>
              <div
                class="text-sm font-semibold mt-0.5"
                :class="submittedSnapshot.gainLoss === null ? '' : (submittedSnapshot.gainLoss > 0 ? 'text-success' : (submittedSnapshot.gainLoss < 0 ? 'text-error' : ''))"
              >
                {{ submittedSnapshot.gainLoss === null ? '—' : formatSigned(submittedSnapshot.gainLoss) }}
              </div>
            </div>
          </div>
        </div>

        <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-3.5">
          {{ t('disposal.timeline.title') }}
        </div>
        <div
          v-if="timelineLoading"
          class="flex flex-col gap-3 pl-1.5 mb-2"
        >
          <USkeleton class="h-10 w-full rounded-lg" />
          <USkeleton class="h-10 w-full rounded-lg" />
        </div>
        <div
          v-else-if="timelineError"
          class="bg-default border border-default rounded-xl shadow-sm py-6 px-6 text-center mb-2"
        >
          <p class="text-sm text-muted mb-3">
            {{ t('common.loadError') }}
          </p>
          <UButton
            size="sm"
            color="neutral"
            variant="outline"
            icon="i-lucide-rotate-cw"
            @click="requestId && loadTimeline(requestId)"
          >
            {{ t('common.retry') }}
          </UButton>
        </div>
        <div
          v-else
          class="pl-1.5"
        >
          <div
            v-for="(row, i) in timelineRows"
            :key="i"
            data-testid="disposal-timeline-row"
            :data-status="row.status"
            class="flex gap-3"
          >
            <div class="flex flex-col items-center flex-none">
              <span
                class="size-[26px] rounded-full flex items-center justify-center flex-none"
                :class="row.status === 'done' ? 'bg-success text-white' : (row.status === 'current' ? 'bg-warning text-white' : 'bg-muted text-dimmed')"
              >
                <UIcon
                  :name="TIMELINE_ICON[row.status]"
                  class="size-3.5"
                />
              </span>
              <span
                v-if="i < timelineRows.length - 1"
                class="w-0.5 flex-1 bg-default my-0.5 min-h-[22px]"
              />
            </div>
            <div class="pb-[18px] min-w-0 flex-1">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-[13.5px] font-semibold">{{ row.role }}</span>
                <UBadge
                  :color="TIMELINE_DOT[row.status]"
                  variant="subtle"
                  size="sm"
                  class="rounded-full"
                >
                  {{ t(TIMELINE_LABEL_KEY[row.status]) }}
                </UBadge>
              </div>
              <div class="text-xs text-muted mt-0.5">
                {{ row.meta }}
              </div>
            </div>
          </div>
        </div>
        <UButton
          color="neutral"
          variant="outline"
          icon="i-lucide-plus"
          :label="t('disposal.timeline.newRequest')"
          data-testid="disposal-reset"
          @click="resetForm"
        />
      </div>

      <!-- PRE-SUBMIT FORM -->
      <div
        v-else
        class="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-[18px] items-start"
      >
        <!-- LEFT: form -->
        <div class="flex flex-col gap-4 min-w-0">
          <!-- Aset yang Dilepas -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-5">
            <div class="text-[13.5px] font-semibold mb-3.5">
              {{ t('disposal.form.assetSection') }}
            </div>
            <UFormField
              :label="t('disposal.form.asset')"
              required
            >
              <AssetSearchPicker
                :key="pickerKey"
                data-testid="disposal-asset-picker"
                :statuses="PICKER_STATUSES"
                :placeholder="t('disposal.form.assetPlaceholder')"
                :hint="t('disposal.form.assetHint')"
                :office-names="officeNameMap"
                @select="onSelectAsset"
              />
            </UFormField>

            <div
              v-if="selectedAsset"
              data-testid="disposal-valuation"
              class="mt-4 p-3.5 rounded-[11px] bg-muted"
            >
              <div class="text-[11px] font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('disposal.valuation.title') }}
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <div class="text-[11.5px] text-muted">
                    {{ t('disposal.valuation.acquisition') }}
                  </div>
                  <div
                    v-if="acquisitionMasked"
                    data-testid="disposal-valuation-acquisition"
                    class="text-[13.5px] font-semibold mt-px inline-flex items-center gap-1 text-dimmed"
                    :title="t('assets.masked')"
                  >
                    •••
                    <UIcon
                      name="i-lucide-lock"
                      class="size-3"
                    />
                  </div>
                  <div
                    v-else
                    data-testid="disposal-valuation-acquisition"
                    class="text-[13.5px] font-semibold mt-px"
                  >
                    {{ formatRupiah(selectedAsset.purchase_cost) }}
                  </div>
                </div>
                <div>
                  <div class="text-[11.5px] text-muted">
                    {{ t('disposal.valuation.accumulatedDepreciation') }}
                  </div>
                  <div
                    v-if="accumMasked"
                    data-testid="disposal-valuation-accum"
                    class="text-[13.5px] font-semibold mt-px inline-flex items-center gap-1 text-dimmed"
                    :title="t('assets.masked')"
                  >
                    •••
                    <UIcon
                      name="i-lucide-lock"
                      class="size-3"
                    />
                  </div>
                  <div
                    v-else
                    data-testid="disposal-valuation-accum"
                    class="text-[13.5px] font-semibold mt-px text-error"
                  >
                    − {{ formatRupiah(selectedAsset.accumulated_depreciation) }}
                  </div>
                </div>
                <div>
                  <div class="inline-flex items-center gap-1 text-[11.5px] text-muted">
                    {{ t('disposal.valuation.bookValueCommercial') }}
                    <span class="px-1 py-0 text-[9px] font-semibold rounded bg-info/15 text-info">{{ t('disposal.valuation.psakChip') }}</span>
                  </div>
                  <div
                    v-if="bookValueMasked"
                    data-testid="disposal-valuation-book-commercial"
                    class="text-[15px] font-bold mt-px inline-flex items-center gap-1 text-dimmed"
                    :title="t('assets.masked')"
                  >
                    •••
                    <UIcon
                      name="i-lucide-lock"
                      class="size-3"
                    />
                  </div>
                  <div
                    v-else
                    data-testid="disposal-valuation-book-commercial"
                    class="text-[15px] font-bold mt-px text-info"
                  >
                    {{ formatRupiah(commercialBookValue) }}
                  </div>
                </div>
                <div>
                  <div class="inline-flex items-center gap-1 text-[11.5px] text-muted">
                    {{ t('disposal.valuation.bookValueFiscal') }}
                    <span class="px-1 py-0 text-[9px] font-semibold rounded bg-muted text-muted">{{ t('disposal.valuation.fiscalChip') }}</span>
                  </div>
                  <div
                    data-testid="disposal-valuation-book-fiscal"
                    class="text-[15px] font-bold mt-px"
                  >
                    {{ fiscalBookValue !== null ? formatRupiah(fiscalBookValue) : '—' }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Detail Pelepasan -->
          <div class="bg-default border border-default rounded-[14px] shadow-sm p-5">
            <div class="text-[13.5px] font-semibold mb-3.5">
              {{ t('disposal.form.detailSection') }}
            </div>
            <div class="flex flex-col gap-3.5">
              <div class="grid grid-cols-2 gap-3.5">
                <UFormField
                  :label="t('disposal.form.method')"
                  required
                >
                  <USelect
                    v-model="method"
                    data-testid="disposal-method"
                    value-key="value"
                    :items="methodItems"
                    class="w-full"
                  />
                </UFormField>
                <UFormField :label="t('disposal.form.value')">
                  <NumberInput
                    v-model="proceedsRaw"
                    money
                    data-testid="disposal-proceeds"
                    class="w-full"
                  />
                  <template #hint>
                    {{ t('disposal.form.valueHint') }}
                  </template>
                </UFormField>
              </div>
              <div class="grid grid-cols-2 gap-3.5">
                <UFormField
                  :label="t('disposal.form.date')"
                  required
                >
                  <DateField
                    v-model="disposalDate"
                    testid="disposal-date"
                  />
                </UFormField>
                <UFormField :label="t('disposal.form.bastNo')">
                  <UInput
                    v-model="bastNo"
                    data-testid="disposal-bast-no"
                    class="w-full font-mono"
                  />
                </UFormField>
              </div>
              <UFormField :label="t('disposal.form.reason')">
                <UTextarea
                  v-model="reason"
                  :rows="3"
                  :placeholder="t('disposal.form.reasonPlaceholder')"
                  class="w-full"
                />
              </UFormField>
              <UFormField :label="t('disposal.form.attachments')">
                <input
                  ref="evidenceInput"
                  type="file"
                  multiple
                  class="hidden"
                  data-testid="disposal-evidence-input"
                  @change="onEvidenceFileChange"
                >
                <button
                  type="button"
                  data-testid="disposal-evidence-dropzone"
                  :disabled="!selectedAsset || evidenceUploading || !canManage"
                  class="w-full flex flex-col items-center justify-center gap-1.5 py-[18px] px-4 rounded-[11px] border-[1.5px] border-dashed border-strong text-center transition-colors"
                  :class="!selectedAsset || !canManage ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:border-primary'"
                  @click="openEvidencePicker"
                >
                  <UIcon
                    name="i-lucide-upload-cloud"
                    class="size-[17px] text-muted"
                  />
                  <span class="text-[12.5px] font-medium text-muted">{{ t('disposal.form.dropText') }}</span>
                </button>
                <p class="text-xs text-dimmed mt-1">
                  {{ t('disposal.form.evidenceHint') }}
                </p>
                <div
                  v-if="evidenceUploads.length"
                  class="flex flex-wrap gap-2 mt-2.5"
                >
                  <span
                    v-for="att in evidenceUploads"
                    :key="att.id"
                    data-testid="disposal-evidence-chip"
                    class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-full bg-muted border border-default text-xs font-medium"
                  >
                    <UIcon
                      name="i-lucide-file"
                      class="size-3.5 text-muted"
                    />
                    <span class="max-w-[160px] truncate">{{ att.original_filename }}</span>
                    <button
                      type="button"
                      data-testid="disposal-evidence-remove"
                      class="text-dimmed hover:text-error"
                      @click="removeEvidence(att)"
                    >
                      <UIcon
                        name="i-lucide-x"
                        class="size-3"
                      />
                    </button>
                  </span>
                </div>
              </UFormField>
            </div>
          </div>
        </div>

        <!-- RIGHT: gain/loss + approval chain (sticky per mockup) -->
        <div
          data-testid="disposal-summary-column"
          class="flex flex-col gap-4 lg:sticky lg:top-4 self-start"
        >
          <!-- Laba/Rugi -->
          <div
            data-testid="disposal-gainloss-card"
            class="rounded-[14px] shadow-sm overflow-hidden border p-[18px]"
            :class="{
              'border-success/30 bg-success/10': gainLossVariant === 'gain',
              'border-error/30 bg-error/10': gainLossVariant === 'loss',
              'border-default bg-default': gainLossVariant === null || gainLossVariant === 'breakEven'
            }"
          >
            <div class="flex items-center gap-2 mb-3">
              <UIcon
                :name="gainLossVariant === 'loss' ? 'i-lucide-trending-down' : 'i-lucide-trending-up'"
                class="size-4"
                :class="{
                  'text-success': gainLossVariant === 'gain',
                  'text-error': gainLossVariant === 'loss',
                  'text-muted': gainLossVariant === null || gainLossVariant === 'breakEven'
                }"
              />
              <span
                class="text-[13px] font-semibold"
                :class="{
                  'text-success': gainLossVariant === 'gain',
                  'text-error': gainLossVariant === 'loss',
                  'text-muted': gainLossVariant === null || gainLossVariant === 'breakEven'
                }"
              >
                {{ gainLossVariant === 'gain' ? t('disposal.gainLoss.gain') : gainLossVariant === 'loss' ? t('disposal.gainLoss.loss') : gainLossVariant === 'breakEven' ? t('disposal.gainLoss.breakEven') : t('disposal.gainLoss.title') }}
              </span>
            </div>

            <div v-if="gainLossState === 'empty'">
              <p
                data-testid="disposal-gainloss-empty"
                class="text-[13px] leading-relaxed text-muted"
              >
                {{ t('disposal.gainLoss.empty') }}
              </p>
            </div>
            <div v-else-if="gainLossState === 'masked'">
              <div
                data-testid="disposal-gainloss-masked"
                class="text-2xl font-bold text-dimmed"
              >
                —
              </div>
              <p class="text-xs text-muted mt-2">
                {{ t('disposal.gainLoss.bookValueHidden') }}
              </p>
            </div>
            <div v-else>
              <div
                data-testid="disposal-gainloss-value"
                class="text-2xl font-bold"
                :class="{ 'text-success': gainLossVariant === 'gain', 'text-error': gainLossVariant === 'loss' }"
              >
                {{ formatSigned(gainLossValue!) }}
              </div>
              <div class="flex flex-col gap-1.5 mt-3.5">
                <div class="flex items-center justify-between text-[12.5px]">
                  <span class="text-muted">{{ t('disposal.gainLoss.value') }}</span>
                  <span class="font-medium tabular-nums">{{ formatRupiah(proceedsRaw) }}</span>
                </div>
                <div class="flex items-center justify-between text-[12.5px]">
                  <span class="text-muted">− {{ t('disposal.gainLoss.bookValue') }}</span>
                  <span class="font-medium tabular-nums">{{ formatRupiah(commercialBookValue) }}</span>
                </div>
                <div class="h-px bg-default my-0.5" />
                <div class="flex items-center justify-between text-[11.5px] text-dimmed">
                  <span>{{ t('disposal.gainLoss.fiscal') }}</span>
                  <span data-testid="disposal-gainloss-fiscal-value">{{ fiscalGainLoss === null ? '—' : formatSigned(fiscalGainLoss) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Jenjang Persetujuan -->
          <div
            v-if="selectedAsset"
            data-testid="disposal-chain-card"
            class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden"
          >
            <div class="flex items-center gap-2.5 px-4 py-3.5 border-b border-default">
              <span class="size-7 rounded-lg bg-violet-500/15 text-violet-600 dark:text-violet-400 flex items-center justify-center flex-none">
                <UIcon
                  name="i-lucide-shield-check"
                  class="size-3.5"
                />
              </span>
              <div class="flex-1 min-w-0">
                <div class="text-[13px] font-semibold">
                  {{ t('disposal.chain.title') }}
                </div>
                <div
                  v-if="!acquisitionMasked"
                  class="text-[11px] text-dimmed truncate"
                >
                  {{ t('disposal.chain.basedOnBookValue', { value: formatRupiah(chainPreviewAmount) }) }}
                </div>
              </div>
            </div>

            <div class="p-1.5">
              <div class="flex items-center gap-2.5 px-2.5 py-2">
                <span class="size-[26px] rounded-full bg-muted text-muted flex items-center justify-center text-[11px] font-bold flex-none">1</span>
                <div class="flex-1 min-w-0">
                  <div class="text-[13px] font-medium">
                    {{ auth.user?.name ?? '—' }}
                  </div>
                  <div class="text-[11px] text-muted">
                    {{ t('disposal.chain.note.maker') }}
                  </div>
                </div>
              </div>

              <div
                v-if="chainState === 'masked'"
                data-testid="disposal-chain-masked"
                class="px-2.5 py-2 text-[12.5px] text-muted"
              >
                {{ t('disposal.chain.acquisitionHidden') }}
              </div>
              <div
                v-else-if="chainState === 'not_configured'"
                data-testid="disposal-chain-not-configured"
                class="px-2.5 py-2 text-[12.5px] text-muted"
              >
                {{ t('disposal.chain.notConfigured') }}
              </div>
              <div
                v-else-if="chainState === 'loading'"
                class="flex flex-col gap-2 px-2.5 py-2"
              >
                <USkeleton class="h-8 w-full rounded-lg" />
                <USkeleton class="h-8 w-full rounded-lg" />
              </div>
              <div
                v-else-if="chainState === 'ready'"
                data-testid="disposal-chain-steps"
              >
                <div
                  v-for="row in chainRows"
                  :key="row.num"
                  class="flex items-center gap-2.5 px-2.5 py-2"
                >
                  <span class="size-[26px] rounded-full bg-primary/15 text-primary flex items-center justify-center text-[11px] font-bold flex-none">{{ row.num }}</span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[13px] font-medium">
                      {{ row.role }}
                    </div>
                    <div class="text-[11px] text-muted">
                      {{ row.note }}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="flex gap-2 items-start px-4 py-3 border-t border-default bg-warning/10">
              <UIcon
                name="i-lucide-alert-triangle"
                class="size-[15px] flex-none mt-0.5 text-warning"
              />
              <span class="text-xs leading-relaxed text-warning">{{ t('disposal.chain.sensitiveNote') }}</span>
            </div>
          </div>

          <div
            v-if="!canManage"
            data-testid="disposal-no-manage"
            class="flex gap-2 items-center px-3 py-2.5 rounded-[10px] bg-muted border border-default text-muted text-[12.5px] leading-snug font-medium"
          >
            <UIcon
              name="i-lucide-lock"
              class="size-4 flex-none"
            />
            {{ t('disposal.form.noManagePermission') }}
          </div>

          <UButton
            block
            icon="i-lucide-send"
            :label="t('disposal.form.submit')"
            :loading="submitting"
            :disabled="!formReady || !canManage"
            data-testid="disposal-submit"
            @click="submitDisposal"
          />
        </div>
      </div>
    </div>

    <!-- ============ RIWAYAT ============ -->
    <div v-else>
      <div class="flex items-center gap-2.5 flex-wrap mb-3.5">
        <UInput
          v-model="historyQuery"
          data-testid="disposal-history-search"
          icon="i-lucide-search"
          :placeholder="t('disposal.history.searchPlaceholder')"
          class="flex-1 min-w-[220px]"
        />
        <USelect
          v-model="historyStatus"
          data-testid="disposal-history-status"
          value-key="value"
          :items="statusFilterItems"
          class="min-w-[190px]"
        />
      </div>

      <div
        v-if="historyLoading"
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
        v-else-if="historyError"
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
          @click="loadHistory"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else-if="filteredHistory.length === 0"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-trash-2"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('disposal.history.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('disposal.history.emptySub') }}
        </div>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <UContextMenu
          :items="contextItems"
          :disabled="filteredHistory.length === 0"
        >
          <div
            class="overflow-x-auto"
            @contextmenu="onTableContextMenu"
          >
            <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
              <thead>
                <tr class="bg-muted text-muted">
                  <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.asset') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.method') }}
                  </th>
                  <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.value') }}
                  </th>
                  <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.gainLoss') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.date') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                    {{ t('disposal.history.column.status') }}
                  </th>
                  <th class="px-4 py-[11px]" />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in filteredHistory"
                  :key="row.key"
                  data-testid="disposal-history-row"
                  class="border-t border-default hover:bg-muted/60 transition-colors"
                  @contextmenu="onRowContextMenu(row)"
                >
                  <td class="px-4 py-3">
                    <div class="font-medium">
                      {{ row.assetLabel }}
                    </div>
                    <div
                      v-if="row.assetTag"
                      class="font-mono text-[11.5px] text-dimmed"
                    >
                      {{ row.assetTag }}
                    </div>
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      v-if="row.methodKey"
                      :color="METHOD_TONE[row.methodKey]"
                      variant="subtle"
                      class="rounded-full"
                    >
                      {{ t(`disposal.method.${row.methodKey}`) }}
                    </UBadge>
                    <span
                      v-else
                      class="text-dimmed"
                    >—</span>
                  </td>
                  <td class="px-3.5 py-3 text-right tabular-nums">
                    {{ row.proceeds && Number(row.proceeds) !== 0 ? formatRupiah(row.proceeds) : '—' }}
                  </td>
                  <td
                    class="px-3.5 py-3 text-right tabular-nums font-semibold"
                    :class="row.gainLoss === null ? 'text-muted' : (Number(row.gainLoss) > 0 ? 'text-success' : (Number(row.gainLoss) < 0 ? 'text-error' : 'text-muted'))"
                  >
                    {{ row.gainLoss === null ? '—' : formatSigned(Number(row.gainLoss)) }}
                  </td>
                  <td class="px-3.5 py-3 text-muted">
                    {{ row.dateLabel }}
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      :color="HISTORY_STATUS_TONE[row.status]"
                      variant="subtle"
                      class="rounded-full gap-1.5"
                    >
                      {{ t(`disposal.status.${row.status}`) }}
                    </UBadge>
                  </td>
                  <td class="px-4 py-3 text-right">
                    <div class="flex justify-end">
                      <RowActionsMenu :items="rowActions(row)" />
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </UContextMenu>
        <div class="px-4 py-3 border-t border-default text-[13px] text-muted">
          {{ t('disposal.history.info', { n: filteredHistory.length }) }}
        </div>
      </div>
    </div>

    <!-- Lampirkan BAST modal -->
    <UModal
      v-model:open="attachOpen"
      :title="t('disposal.attachBast.title')"
      :description="t('disposal.attachBast.hint')"
    >
      <template #body>
        <div class="space-y-4">
          <UFormField :label="t('disposal.attachBast.bastNo')">
            <UInput
              v-model="attachBastNo"
              data-testid="disposal-attach-bast-no"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField :label="t('disposal.attachBast.docNo')">
            <UInput
              v-model="attachDocNo"
              data-testid="disposal-attach-doc-no"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('disposal.attachBast.docDate')">
            <DateField
              v-model="attachDocDate"
              testid="disposal-attach-doc-date"
            />
          </UFormField>
          <UFormField :label="t('disposal.attachBast.counterparty')">
            <UInput
              v-model="attachCounterparty"
              data-testid="disposal-attach-counterparty"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('disposal.attachBast.file')">
            <input
              data-testid="disposal-attach-file"
              type="file"
              class="block w-full text-[13px]"
              @change="onAttachFileChange"
            >
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            color="neutral"
            variant="ghost"
            @click="attachOpen = false"
          >
            {{ t('disposal.attachBast.cancel') }}
          </UButton>
          <UButton
            :loading="attachSubmitting"
            data-testid="disposal-attach-confirm"
            @click="confirmAttach"
          >
            {{ t('disposal.attachBast.confirm') }}
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>
