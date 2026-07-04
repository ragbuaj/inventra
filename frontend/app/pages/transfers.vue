<script setup lang="ts">
import type { Asset, AssetStatus, BadgeColor, Floor, Office, ReferenceRow, Room } from '~/types'
import type { Transfer, TransferSubmitInput } from '~/composables/api/useTransfers'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'
import type { OfficeNode } from '~/utils/officeRegion'
import type { TransferHistoryRow, TransferHistoryStatus } from '~/utils/transferHistory'
import { CONDITION_KEYS, CONDITION_TONE, TRANSFER_STATUS_TONE, type TransferCondition } from '~/constants/transferMeta'
import { isInterRegion } from '~/utils/officeRegion'
import { mergeTransferHistory } from '~/utils/transferHistory'

definePageMeta({ middleware: 'can', permission: 'transfer.view' })

const AVAILABLE_STATUSES: AssetStatus[] = ['available']
// Nuxt UI's <SelectItem> forbids an empty-string value (reserved to mean
// "clear selection"), so the "no room selected" option uses this sentinel
// instead and is translated back to null/undefined at the API boundary.
const NONE = '__none__'

const DOT_CLASS: Record<BadgeColor, string> = {
  primary: 'bg-primary',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-error',
  info: 'bg-info',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const FLOW_STEPS: Array<{ key: 'diajukan' | 'approved' | 'in_transit' | 'received', tone: BadgeColor }> = [
  { key: 'diajukan', tone: 'warning' },
  { key: 'approved', tone: 'info' },
  { key: 'in_transit', tone: 'info' },
  { key: 'received', tone: 'success' }
]

// TransferHistoryStatus → the i18n subkey (transfer.status.<key> / transfer.statusFilter.<key>).
// Two of the seven statuses are request-only and use different wording than their
// TransferHistoryStatus literal (ditolak_pengajuan → rejected, dibatalkan → cancelled).
const STATUS_I18N_KEY: Record<TransferHistoryStatus, string> = {
  diajukan: 'diajukan',
  ditolak_pengajuan: 'rejected',
  dibatalkan: 'cancelled',
  approved: 'approved',
  in_transit: 'in_transit',
  received: 'received',
  returned: 'returned'
}
const HISTORY_STATUS_TONE: Record<TransferHistoryStatus, BadgeColor> = {
  diajukan: 'warning',
  ditolak_pengajuan: 'error',
  dibatalkan: 'neutral',
  approved: TRANSFER_STATUS_TONE.approved,
  in_transit: TRANSFER_STATUS_TONE.in_transit,
  received: TRANSFER_STATUS_TONE.received,
  returned: TRANSFER_STATUS_TONE.returned
}
const HISTORY_STATUS_KEYS: TransferHistoryStatus[] = ['diajukan', 'approved', 'in_transit', 'received', 'returned', 'ditolak_pengajuan', 'dibatalkan']

const { t, locale } = useI18n()
const auth = useAuthStore()
const can = useCan()
const toast = useToast()

const transfersApi = useTransfers()
const approvalApi = useApproval()
const officesApi = useOffices()
const floorsApi = useFloors()
const refApi = useReference()

type TabKey = 'ajukan' | 'inbox' | 'history'
const tab = ref<TabKey>('ajukan')

function fmtDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-GB' : 'id-ID', {
    day: '2-digit', month: 'short', year: 'numeric'
  }).format(d)
}
function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}
function initials(name: string | null): string {
  return (name ?? '—').split(/\s+/).filter(Boolean).map(w => w[0]).slice(0, 2).join('').toUpperCase() || '—'
}
function flattenRoomOptions(floors: Floor[], roomsByFloor: Record<string, Room[]>): Array<{ value: string, label: string }> {
  const opts: Array<{ value: string, label: string }> = []
  for (const f of floors) {
    for (const r of (roomsByFloor[f.id] ?? [])) {
      opts.push({ value: r.id, label: `${f.name} · ${r.name}` })
    }
  }
  return opts
}

// ---------------------------------------------------------------------------
// Lookups: offices + office-types (tier), shared by every tab.
// ---------------------------------------------------------------------------
const offices = ref<Office[]>([])
const officeTypeRows = ref<ReferenceRow[]>([])
const lookupsLoading = ref(true)
const lookupsError = ref(false)

const officeNameMap = computed(() => new Map(offices.value.map(o => [o.id, o.name])))
const officeNodes = computed(() => new Map<string, OfficeNode>(offices.value.map(o => [o.id, { id: o.id, parent_id: o.parent_id, office_type_id: o.office_type_id }])))
const officeTypeTierMap = computed(() => new Map(officeTypeRows.value.map(r => [r.id, typeof r.tier === 'string' ? r.tier : null])))
function tierOf(officeTypeId: string): string | null {
  return officeTypeTierMap.value.get(officeTypeId) ?? null
}
function officeName(id: string | null): string | null {
  return id ? (officeNameMap.value.get(id) ?? null) : null
}

const myOfficeId = computed(() => auth.user?.office_id ?? null)
const myOfficeLabel = computed(() => myOfficeId.value ? (officeNameMap.value.get(myOfficeId.value) ?? myOfficeId.value) : null)

async function loadLookups() {
  lookupsLoading.value = true
  lookupsError.value = false
  try {
    const [offs, types] = await Promise.all([
      officesApi.list({ limit: 100 }),
      refApi.list('office-types', { limit: 100 })
    ])
    offices.value = offs.data
    officeTypeRows.value = types.data
  } catch {
    lookupsError.value = true
  } finally {
    lookupsLoading.value = false
  }
}

// ---------------------------------------------------------------------------
// Ajukan Mutasi
// ---------------------------------------------------------------------------
const pickerKey = ref(0)
const selectedAsset = ref<Asset | null>(null)
const toOfficeId = ref('')
const toRoomId = ref(NONE)
const transferDate = ref('')
const condition = ref<TransferCondition>('baik')
const reason = ref('')
const submitting = ref(false)
const ajMsg = ref<{ text: string, type: 'ok' | 'error' } | null>(null)
let ajTimer: ReturnType<typeof setTimeout> | undefined

const destFloors = ref<Floor[]>([])
const destRoomsByFloor = ref<Record<string, Room[]>>({})

const fromOfficeId = computed(() => selectedAsset.value?.office_id ?? null)
const fromOfficeLabel = computed(() => fromOfficeId.value ? (officeNameMap.value.get(fromOfficeId.value) ?? fromOfficeId.value) : '—')
const toOfficeOptions = computed(() => offices.value.filter(o => o.id !== fromOfficeId.value).map(o => ({ value: o.id, label: o.name })))
const destRoomOptions = computed(() => [{ value: NONE, label: t('transfer.form.roomNone') }, ...flattenRoomOptions(destFloors.value, destRoomsByFloor.value)])
const conditionItems = computed(() => CONDITION_KEYS.map(k => ({ value: k, label: t(`transfer.condition.${k}`) })))

const interRegionResult = computed<boolean | null>(() => {
  if (!fromOfficeId.value || !toOfficeId.value) return null
  return isInterRegion(fromOfficeId.value, toOfficeId.value, officeNodes.value, tierOf)
})

const canManage = computed(() => can('transfer.manage'))
const ajReady = computed(() => !!(selectedAsset.value && toOfficeId.value && transferDate.value && condition.value))

function onSelectAsset(asset: Asset) {
  selectedAsset.value = asset
  toOfficeId.value = ''
  toRoomId.value = NONE
}

watch(toOfficeId, async (id) => {
  toRoomId.value = NONE
  destFloors.value = []
  destRoomsByFloor.value = {}
  if (!id) return
  try {
    const floors = await floorsApi.listByOffice(id)
    destFloors.value = floors
    const entries = await Promise.all(floors.map(async f => [f.id, await floorsApi.roomsByFloor(f.id)] as const))
    const map: Record<string, Room[]> = {}
    for (const [fid, rooms] of entries) map[fid] = rooms
    destRoomsByFloor.value = map
  } catch {
    // Best-effort — the destination room stays "not set" if this fails.
  }
})

function resetForm() {
  selectedAsset.value = null
  toOfficeId.value = ''
  toRoomId.value = NONE
  transferDate.value = ''
  condition.value = 'baik'
  reason.value = ''
  ajMsg.value = null
  pickerKey.value++
}

async function submitTransfer() {
  if (!canManage.value) return
  if (!ajReady.value || submitting.value) {
    ajMsg.value = { text: t('transfer.form.error'), type: 'error' }
    return
  }
  submitting.value = true
  try {
    const input: TransferSubmitInput = {
      asset_id: selectedAsset.value!.id,
      to_office_id: toOfficeId.value,
      to_room_id: toRoomId.value === NONE ? null : toRoomId.value,
      reason: reason.value.trim() || null,
      condition_sent: condition.value,
      transfer_date: transferDate.value
    }
    const assetName = selectedAsset.value!.name
    const officeLabel = officeNameMap.value.get(toOfficeId.value) ?? toOfficeId.value
    await transfersApi.submit(input)
    resetForm()
    ajMsg.value = { text: t('transfer.submitSuccess', { name: assetName, office: officeLabel }), type: 'ok' }
    if (ajTimer) clearTimeout(ajTimer)
    ajTimer = setTimeout(() => {
      ajMsg.value = null
    }, 4500)
    await loadHistory()
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    submitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Kotak Masuk
// ---------------------------------------------------------------------------
const inboxRows = ref<Transfer[]>([])
const inboxLoading = ref(true)
const inboxError = ref(false)

async function loadInbox() {
  inboxLoading.value = true
  inboxError.value = false
  try {
    const page = await transfersApi.list({ status: 'in_transit', limit: 100 })
    const myId = myOfficeId.value
    inboxRows.value = myId ? page.data.filter(r => r.to_office_id === myId) : page.data
  } catch {
    inboxError.value = true
    inboxRows.value = []
  } finally {
    inboxLoading.value = false
  }
}

function inboxInter(row: Transfer): boolean | null {
  return isInterRegion(row.from_office_id, row.to_office_id, officeNodes.value, tierOf)
}

// My office's rooms — used by the receive (accept) modal's room select.
const myFloors = ref<Floor[]>([])
const myRoomsByFloor = ref<Record<string, Room[]>>({})
const myRoomOptions = computed(() => [{ value: NONE, label: t('transfer.form.roomNone') }, ...flattenRoomOptions(myFloors.value, myRoomsByFloor.value)])

async function loadMyRooms() {
  const officeId = myOfficeId.value
  if (!officeId) return
  try {
    const floors = await floorsApi.listByOffice(officeId)
    myFloors.value = floors
    const entries = await Promise.all(floors.map(async f => [f.id, await floorsApi.roomsByFloor(f.id)] as const))
    const map: Record<string, Room[]> = {}
    for (const [fid, rooms] of entries) map[fid] = rooms
    myRoomsByFloor.value = map
  } catch {
    // Best-effort — room select just stays empty.
  }
}

const acceptOpen = ref(false)
const acceptTarget = ref<Transfer | null>(null)
const acceptReceivedDate = ref('')
const acceptRoomId = ref(NONE)
const acceptBastNo = ref('')
const acceptFile = ref<File | null>(null)
const acceptSubmitting = ref(false)

function openAccept(row: Transfer) {
  acceptTarget.value = row
  acceptReceivedDate.value = todayISO()
  acceptRoomId.value = NONE
  acceptBastNo.value = ''
  acceptFile.value = null
  acceptOpen.value = true
}

function onAcceptFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  acceptFile.value = input.files?.[0] ?? null
}

async function confirmAccept() {
  const target = acceptTarget.value
  if (!target || acceptSubmitting.value) return
  acceptSubmitting.value = true
  try {
    await transfersApi.receive(target.id, {
      received_date: acceptReceivedDate.value || undefined,
      to_room_id: acceptRoomId.value === NONE ? undefined : acceptRoomId.value,
      bast_no: acceptBastNo.value.trim() || undefined,
      file: acceptFile.value
    })
    acceptOpen.value = false
    toast.add({ title: t('transfer.inbox.acceptSuccess', { name: target.asset_name ?? '—' }), color: 'success' })
    await Promise.all([loadInbox(), loadHistory()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    acceptSubmitting.value = false
  }
}

const rejectOpen = ref(false)
const rejectTarget = ref<Transfer | null>(null)
const rejectNote = ref('')
const rejectSubmitting = ref(false)

function openReject(row: Transfer) {
  rejectTarget.value = row
  rejectNote.value = ''
  rejectOpen.value = true
}

async function confirmReject() {
  const target = rejectTarget.value
  if (!target || rejectSubmitting.value) return
  rejectSubmitting.value = true
  try {
    await transfersApi.rejectReceive(target.id, rejectNote.value.trim() || undefined)
    rejectOpen.value = false
    toast.add({ title: t('transfer.inbox.rejectSuccess', { name: target.asset_name ?? '—' }), color: 'error' })
    await Promise.all([loadInbox(), loadHistory()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    rejectSubmitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Riwayat
// ---------------------------------------------------------------------------
const historyRequests = ref<ApprovalRequestRow[]>([])
const historyTransfers = ref<Transfer[]>([])
const historyLoading = ref(true)
const historyError = ref(false)
const historyQuery = ref('')
const historyStatus = ref<'all' | TransferHistoryStatus>('all')

async function loadHistory() {
  historyLoading.value = true
  historyError.value = false
  try {
    const [reqPage, trPage] = await Promise.all([
      approvalApi.list({ type: 'asset_transfer', limit: 100 }),
      transfersApi.list({ limit: 100 })
    ])
    historyRequests.value = reqPage.data
    historyTransfers.value = trPage.data
  } catch {
    historyError.value = true
    historyRequests.value = []
    historyTransfers.value = []
  } finally {
    historyLoading.value = false
  }
}

const mergedHistory = computed<TransferHistoryRow[]>(() => mergeTransferHistory(historyRequests.value, historyTransfers.value, {
  fmtDate,
  assetName: () => null,
  officeName,
  interRegion: (a, b) => isInterRegion(a, b, officeNodes.value, tierOf),
  canShip: () => can('transfer.manage')
}))

const statusFilterItems = computed(() => [
  { value: 'all', label: t('transfer.statusFilter.all') },
  ...HISTORY_STATUS_KEYS.map(k => ({ value: k, label: t(`transfer.statusFilter.${STATUS_I18N_KEY[k]}`) }))
])

const filteredHistory = computed(() => {
  const q = historyQuery.value.trim().toLowerCase()
  return mergedHistory.value.filter((row) => {
    if (historyStatus.value !== 'all' && row.status !== historyStatus.value) return false
    if (q) {
      const hay = `${row.assetLabel} ${row.assetTag ?? ''} ${row.fromLabel} ${row.toLabel}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

const shipOpen = ref(false)
const shipTarget = ref<TransferHistoryRow | null>(null)
const shipDate = ref('')
const shipSubmitting = ref(false)
const shipCondition = computed<TransferCondition | null>(() => {
  const raw = shipTarget.value?.raw as Transfer | undefined
  return raw?.condition_sent ?? null
})

function openShip(row: TransferHistoryRow) {
  shipTarget.value = row
  shipDate.value = ''
  shipOpen.value = true
}

async function confirmShip() {
  const row = shipTarget.value
  if (!row || shipSubmitting.value) return
  const raw = row.raw as Transfer
  shipSubmitting.value = true
  try {
    await transfersApi.ship(raw.id, shipDate.value || undefined)
    shipOpen.value = false
    toast.add({ title: t('transfer.ship.success', { name: raw.asset_name ?? row.assetLabel }), color: 'success' })
    await Promise.all([loadHistory(), loadInbox()])
  } catch {
    // useApiClient surfaces the error toast
  } finally {
    shipSubmitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------
const tabs = computed(() => [
  { key: 'ajukan' as const, label: t('transfer.tabs.ajukan'), icon: 'i-lucide-repeat', badge: 0 },
  { key: 'inbox' as const, label: t('transfer.tabs.inbox'), icon: 'i-lucide-inbox', badge: inboxRows.value.length },
  { key: 'history' as const, label: t('transfer.tabs.history'), icon: 'i-lucide-history', badge: 0 }
])

onMounted(() => {
  loadLookups()
  loadInbox()
  loadHistory()
  loadMyRooms()
})

onBeforeUnmount(() => {
  if (ajTimer) clearTimeout(ajTimer)
})
</script>

<template>
  <div class="max-w-[1000px] mx-auto">
    <!-- Header -->
    <div class="mb-[18px]">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('transfer.pageTitle') }}
      </h1>
      <div
        v-if="myOfficeId"
        data-testid="transfer-my-office"
        class="flex items-center gap-1.5 text-[13px] font-medium text-muted"
      >
        <UIcon
          name="i-lucide-building-2"
          class="size-[15px]"
        />
        {{ myOfficeLabel }}
      </div>
    </div>

    <!-- Flow legend -->
    <div class="flex items-center gap-2 flex-wrap px-[15px] py-3 mb-[18px] bg-default border border-default rounded-xl shadow-sm">
      <span class="text-[11px] font-semibold uppercase tracking-wider text-dimmed me-0.5">
        {{ t('transfer.flow.label') }}
      </span>
      <template
        v-for="(f, i) in FLOW_STEPS"
        :key="f.key"
      >
        <UBadge
          :color="f.tone"
          variant="subtle"
          class="rounded-full gap-1.5"
        >
          <span
            class="size-1.5 rounded-full"
            :class="DOT_CLASS[f.tone]"
          />
          {{ t(`transfer.flow.${f.key}`) }}
        </UBadge>
        <UIcon
          v-if="i < FLOW_STEPS.length - 1"
          name="i-lucide-arrow-right"
          class="size-3.5 text-dimmed"
        />
      </template>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-default mb-[22px]">
      <button
        v-for="tb in tabs"
        :key="tb.key"
        class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-sm border-b-2 transition-colors"
        :class="tab === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
        :data-testid="`transfer-tab-${tb.key}`"
        @click="tab = tb.key"
      >
        <UIcon
          :name="tb.icon"
          class="size-[15px]"
        />
        {{ tb.label }}
        <span
          v-if="tb.key === 'inbox' && tb.badge > 0"
          class="min-w-[19px] h-[19px] px-1.5 inline-flex items-center justify-center text-[11px] font-bold rounded-full"
          :class="tab === tb.key ? 'bg-primary/15 text-primary' : 'bg-muted text-muted'"
        >{{ tb.badge }}</span>
      </button>
    </div>

    <!-- ============ AJUKAN MUTASI ============ -->
    <div
      v-if="tab === 'ajukan'"
      class="max-w-[640px]"
    >
      <div
        v-if="ajMsg"
        class="flex gap-2.5 items-center px-3.5 py-3 mb-[18px] rounded-[11px] border text-[13px] font-medium"
        :class="ajMsg.type === 'ok' ? 'bg-success/10 border-success/30 text-success' : 'bg-error/10 border-error/30 text-error'"
      >
        <UIcon
          :name="ajMsg.type === 'ok' ? 'i-lucide-circle-check' : 'i-lucide-circle-alert'"
          class="size-[17px] flex-none"
        />
        {{ ajMsg.text }}
      </div>

      <div
        v-if="lookupsLoading"
        class="bg-default border border-default rounded-[14px] shadow-sm p-[22px] flex flex-col gap-4"
      >
        <USkeleton class="h-10 w-full rounded-lg" />
        <USkeleton class="h-10 w-full rounded-lg" />
        <USkeleton class="h-10 w-full rounded-lg" />
      </div>
      <div
        v-else-if="lookupsError"
        class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          @click="loadLookups"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[14px] shadow-sm p-[22px] flex flex-col gap-4"
      >
        <UFormField
          :label="t('transfer.form.asset')"
          required
        >
          <AssetSearchPicker
            :key="pickerKey"
            data-testid="transfer-asset-picker"
            :statuses="AVAILABLE_STATUSES"
            :placeholder="t('transfer.form.assetPlaceholder')"
            :hint="t('transfer.form.assetHint')"
            :office-names="officeNameMap"
            @select="onSelectAsset"
          />
        </UFormField>

        <div class="grid grid-cols-2 gap-4">
          <UFormField :label="t('transfer.form.fromOffice')">
            <div class="flex items-center gap-2 px-3 py-2.5 text-[13.5px] font-medium text-muted bg-muted border border-dashed border-strong rounded-[10px]">
              <UIcon
                name="i-lucide-building-2"
                class="size-[15px] flex-none"
              />
              <span class="truncate">{{ fromOfficeLabel }}</span>
            </div>
          </UFormField>
          <UFormField
            :label="t('transfer.form.toOffice')"
            required
          >
            <USelect
              v-model="toOfficeId"
              data-testid="transfer-to-office"
              value-key="value"
              :items="toOfficeOptions"
              :placeholder="t('transfer.form.toOfficePlaceholder')"
              class="w-full"
            />
          </UFormField>
        </div>

        <div
          v-if="interRegionResult === true"
          data-testid="transfer-inter-region-alert"
          class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-violet-500/10 border border-violet-500/30"
        >
          <UIcon
            name="i-lucide-shield-alert"
            class="size-[17px] flex-none mt-0.5 text-violet-600 dark:text-violet-400"
          />
          <div>
            <div class="text-[13px] font-semibold text-violet-600 dark:text-violet-400">
              {{ t('transfer.interRegion.title') }}
            </div>
            <div class="text-[12.5px] leading-relaxed text-muted mt-0.5">
              {{ t('transfer.interRegion.note') }}
            </div>
          </div>
        </div>
        <div
          v-else-if="interRegionResult === false"
          data-testid="transfer-in-subtree-note"
          class="flex gap-2.5 items-center px-3.5 py-2.5 rounded-[11px] bg-success/10 border border-success/25"
        >
          <UIcon
            name="i-lucide-check"
            class="size-[15px] flex-none text-success"
          />
          <span class="text-[12.5px] font-medium text-success">{{ t('transfer.inSubtreeNote') }}</span>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <UFormField :label="t('transfer.form.toRoom')">
            <USelect
              v-model="toRoomId"
              value-key="value"
              :items="destRoomOptions"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('transfer.form.date')"
            required
          >
            <UInput
              v-model="transferDate"
              data-testid="transfer-date"
              type="date"
              class="w-full"
            />
          </UFormField>
        </div>

        <UFormField :label="t('transfer.form.condition')">
          <USelect
            v-model="condition"
            data-testid="transfer-condition"
            value-key="value"
            :items="conditionItems"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('transfer.form.reason')">
          <UTextarea
            v-model="reason"
            :rows="3"
            :placeholder="t('transfer.form.reasonPlaceholder')"
            class="w-full"
          />
        </UFormField>

        <div class="border-t border-default pt-4">
          <div
            v-if="!canManage"
            data-testid="transfer-no-manage"
            class="flex gap-2 items-center px-3 py-2.5 mb-3 rounded-[10px] bg-muted border border-default text-muted text-[12.5px] leading-snug font-medium"
          >
            <UIcon
              name="i-lucide-lock"
              class="size-4 flex-none"
            />
            {{ t('transfer.form.noManagePermission') }}
          </div>
          <div class="flex justify-end gap-2.5">
            <UButton
              color="neutral"
              variant="outline"
              :label="t('transfer.form.reset')"
              @click="resetForm"
            />
            <UButton
              icon="i-lucide-repeat"
              :label="t('transfer.form.submit')"
              :loading="submitting"
              :disabled="!ajReady || !canManage"
              data-testid="transfer-submit"
              @click="submitTransfer"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- ============ KOTAK MASUK ============ -->
    <div v-else-if="tab === 'inbox'">
      <div
        v-if="inboxLoading"
        class="flex flex-col gap-3"
      >
        <USkeleton
          v-for="n in 2"
          :key="n"
          class="h-[150px] w-full rounded-[13px]"
        />
      </div>

      <div
        v-else-if="inboxError"
        class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
      >
        <p class="text-sm text-muted mb-3">
          {{ t('common.loadError') }}
        </p>
        <UButton
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-cw"
          @click="loadInbox"
        >
          {{ t('common.retry') }}
        </UButton>
      </div>

      <div
        v-else-if="inboxRows.length > 0"
        class="flex flex-col gap-3"
      >
        <div
          v-for="row in inboxRows"
          :key="row.id"
          data-testid="transfer-inbox-card"
          class="bg-default border rounded-[13px] shadow-sm overflow-hidden"
          :class="inboxInter(row) ? 'border-violet-500/35' : 'border-default'"
        >
          <div class="flex items-start gap-3.5 p-4">
            <span class="size-[42px] rounded-[11px] bg-info/15 text-info flex items-center justify-center flex-none">
              <UIcon
                name="i-lucide-package"
                class="size-5"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 flex-wrap mb-0.5">
                <span class="font-semibold text-[14.5px]">{{ row.asset_name ?? '—' }}</span>
                <UBadge
                  color="info"
                  variant="subtle"
                  class="rounded-full gap-1.5"
                >
                  <span class="size-1.5 rounded-full bg-info" />
                  {{ t('transfer.status.in_transit') }}
                </UBadge>
                <span
                  v-if="inboxInter(row)"
                  class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[11px] font-semibold bg-violet-500/15 text-violet-600 dark:text-violet-400"
                >
                  <UIcon
                    name="i-lucide-globe"
                    class="size-3"
                  />
                  {{ t('transfer.interRegion.badge') }}
                </span>
              </div>
              <div class="font-mono text-[11.5px] text-dimmed">
                {{ row.asset_tag ?? '—' }}
              </div>
              <div class="flex items-center gap-2 mt-2 flex-wrap text-[13px]">
                <span class="text-muted">{{ row.from_office_name ?? officeName(row.from_office_id) ?? '—' }}</span>
                <UIcon
                  name="i-lucide-arrow-right"
                  class="size-3.5 text-primary"
                />
                <span class="font-semibold">{{ row.to_office_name ?? officeName(row.to_office_id) ?? '—' }}</span>
              </div>
              <div class="flex items-center gap-3.5 mt-2 flex-wrap text-[12.5px] text-muted">
                <span>{{ t('transfer.byLabel') }} <span class="font-medium text-default">{{ row.requested_by_name ?? '—' }}</span></span>
                <span>{{ fmtDate(row.transfer_date ?? row.created_at) }}</span>
                <span
                  v-if="row.condition_sent"
                  class="inline-flex items-center gap-1.5"
                >
                  {{ t('transfer.conditionSentLabel') }}
                  <UBadge
                    :color="CONDITION_TONE[row.condition_sent]"
                    variant="subtle"
                    size="sm"
                    class="rounded-full"
                  >
                    {{ t(`transfer.condition.${row.condition_sent}`) }}
                  </UBadge>
                </span>
              </div>
              <div
                v-if="row.reason"
                class="mt-2 px-3 py-2 rounded-lg bg-muted text-[12.5px] leading-relaxed text-muted"
              >
                “{{ row.reason }}”
              </div>
            </div>
          </div>
          <div
            v-if="can('transfer.manage')"
            class="flex justify-end gap-2.5 px-4 py-3 border-t border-default bg-muted/50"
          >
            <UButton
              color="error"
              variant="outline"
              icon="i-lucide-x"
              :label="t('transfer.inbox.reject')"
              data-testid="transfer-reject-receive"
              @click="openReject(row)"
            />
            <UButton
              icon="i-lucide-check"
              :label="t('transfer.inbox.accept')"
              data-testid="transfer-accept"
              @click="openAccept(row)"
            />
          </div>
        </div>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[14px] shadow-sm py-[56px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-inbox"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('transfer.inbox.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('transfer.inbox.emptySub') }}
        </div>
      </div>
    </div>

    <!-- ============ RIWAYAT ============ -->
    <div v-else>
      <div class="flex items-center gap-2.5 flex-wrap mb-3.5">
        <UInput
          v-model="historyQuery"
          data-testid="transfer-history-search"
          icon="i-lucide-search"
          :placeholder="t('transfer.history.searchPlaceholder')"
          class="flex-1 min-w-[220px]"
        />
        <USelect
          v-model="historyStatus"
          value-key="value"
          :items="statusFilterItems"
          class="min-w-[170px]"
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
            name="i-lucide-history"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('transfer.history.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('transfer.history.emptySub') }}
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
                  {{ t('transfer.history.column.asset') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('transfer.history.column.route') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('transfer.history.column.date') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('transfer.history.column.actor') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('transfer.history.column.status') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('transfer.history.column.bast') }}
                </th>
                <th class="px-4 py-[11px]" />
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in filteredHistory"
                :key="row.key"
                data-testid="transfer-history-row"
                class="border-t border-default hover:bg-muted/60 transition-colors"
                :class="row.status === 'in_transit' ? 'bg-info/5' : ''"
              >
                <td class="px-4 py-3">
                  <div class="flex items-center gap-1.5">
                    <span class="font-medium">{{ row.assetLabel }}</span>
                    <UIcon
                      v-if="row.interRegion"
                      name="i-lucide-globe"
                      :title="t('transfer.interRegion.badge')"
                      class="size-3.5 text-violet-600 dark:text-violet-400"
                    />
                  </div>
                  <div
                    v-if="row.assetTag"
                    class="font-mono text-[11.5px] text-dimmed"
                  >
                    {{ row.assetTag }}
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted">{{ row.fromLabel }}</span>
                    <UIcon
                      name="i-lucide-arrow-right"
                      class="size-3.5 text-dimmed"
                    />
                    <span class="font-medium">{{ row.toLabel }}</span>
                  </div>
                </td>
                <td class="px-3.5 py-3 text-muted">
                  {{ row.dateLabel }}
                </td>
                <td class="px-3.5 py-3">
                  <div class="flex items-center gap-2">
                    <span class="size-[26px] rounded-full bg-muted text-muted flex items-center justify-center text-[10px] font-semibold flex-none">{{ initials(row.actorName) }}</span>
                    <span>{{ row.actorName ?? '—' }}</span>
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <UBadge
                    data-testid="transfer-history-status"
                    :color="HISTORY_STATUS_TONE[row.status]"
                    variant="subtle"
                    class="rounded-full gap-1.5"
                  >
                    <span
                      class="size-1.5 rounded-full"
                      :class="DOT_CLASS[HISTORY_STATUS_TONE[row.status]]"
                    />
                    {{ t(`transfer.status.${STATUS_I18N_KEY[row.status]}`) }}
                  </UBadge>
                </td>
                <td class="px-3.5 py-3 font-mono text-[12.5px]">
                  {{ row.bastNo ?? '—' }}
                </td>
                <td class="px-4 py-3 text-right">
                  <UButton
                    v-if="row.canShip"
                    size="xs"
                    icon="i-lucide-send"
                    :label="t('transfer.ship.action')"
                    data-testid="transfer-ship"
                    @click="openShip(row)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="px-4 py-3 border-t border-default text-[13px] text-muted">
          {{ t('transfer.history.info', { n: filteredHistory.length }) }}
        </div>
      </div>
    </div>

    <!-- Terima (accept) modal -->
    <UModal
      v-model:open="acceptOpen"
      :title="t('transfer.inbox.acceptModal.title')"
    >
      <template #body>
        <div class="space-y-4">
          <UFormField :label="t('transfer.inbox.acceptModal.receivedDate')">
            <UInput
              v-model="acceptReceivedDate"
              data-testid="transfer-accept-received-date"
              type="date"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('transfer.inbox.acceptModal.room')">
            <USelect
              v-model="acceptRoomId"
              data-testid="transfer-accept-room"
              value-key="value"
              :items="myRoomOptions"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('transfer.inbox.acceptModal.bastNo')">
            <UInput
              v-model="acceptBastNo"
              data-testid="transfer-accept-bast"
              :placeholder="t('transfer.inbox.acceptModal.bastNoPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('transfer.inbox.acceptModal.file')">
            <input
              data-testid="transfer-accept-file"
              type="file"
              class="block w-full text-[13px]"
              @change="onAcceptFileChange"
            >
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            color="neutral"
            variant="ghost"
            @click="acceptOpen = false"
          >
            {{ t('common.cancel') }}
          </UButton>
          <UButton
            :loading="acceptSubmitting"
            data-testid="transfer-accept-confirm"
            @click="confirmAccept"
          >
            {{ t('transfer.inbox.acceptModal.confirm') }}
          </UButton>
        </div>
      </template>
    </UModal>

    <!-- Tolak Terima (reject-receive) modal -->
    <UModal
      v-model:open="rejectOpen"
      :title="t('transfer.inbox.rejectModal.title')"
    >
      <template #body>
        <UFormField :label="t('transfer.inbox.rejectModal.note')">
          <UTextarea
            v-model="rejectNote"
            data-testid="transfer-reject-note"
            :rows="3"
            :placeholder="t('transfer.inbox.rejectModal.notePlaceholder')"
            class="w-full"
          />
        </UFormField>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            color="neutral"
            variant="ghost"
            @click="rejectOpen = false"
          >
            {{ t('common.cancel') }}
          </UButton>
          <UButton
            color="error"
            :loading="rejectSubmitting"
            data-testid="transfer-reject-confirm"
            @click="confirmReject"
          >
            {{ t('transfer.inbox.rejectModal.confirm') }}
          </UButton>
        </div>
      </template>
    </UModal>

    <!-- Kirim (ship) confirm modal -->
    <UModal
      v-model:open="shipOpen"
      :title="t('transfer.ship.title')"
      :description="t('transfer.ship.subtitle')"
    >
      <template #body>
        <div class="space-y-4">
          <div
            v-if="shipCondition"
            class="flex items-center justify-between px-3.5 py-2.5 rounded-[10px] bg-muted text-[13px]"
          >
            <span class="text-muted">{{ t('transfer.ship.conditionLabel') }}</span>
            <UBadge
              :color="CONDITION_TONE[shipCondition]"
              variant="subtle"
              class="rounded-full"
            >
              {{ t(`transfer.condition.${shipCondition}`) }}
            </UBadge>
          </div>
          <UFormField :label="t('transfer.ship.dateLabel')">
            <UInput
              v-model="shipDate"
              data-testid="transfer-ship-date"
              type="date"
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
            @click="shipOpen = false"
          >
            {{ t('transfer.ship.cancel') }}
          </UButton>
          <UButton
            :loading="shipSubmitting"
            data-testid="transfer-ship-confirm"
            @click="confirmShip"
          >
            {{ t('transfer.ship.confirm') }}
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>
