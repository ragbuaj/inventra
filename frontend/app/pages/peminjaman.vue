<script setup lang="ts">
import type { AvailableAsset } from '~/composables/api/useAssignment'
import type { ApprovalStep } from '~/composables/api/useApproval'
import type { BadgeColor } from '~/types'
import type { RequestStatus } from '~/constants/assignmentMeta'
import { REQUEST_STATUS_TONE } from '~/constants/assignmentMeta'

definePageMeta({ middleware: 'can', permission: 'request.create' })

const STATUS_DOT: Record<BadgeColor, string> = {
  primary: 'bg-primary',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-error',
  info: 'bg-info',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

interface MyRequestRow {
  id: string
  status: RequestStatus
  created_at: string | null
  decision_note: string | null
  target_id: string | null
  payload?: { asset_id?: string, due_date?: string | null, notes?: string | null } | null
}

const { t, locale } = useI18n()
const toast = useToast()
const assignmentApi = useAssignment()
const approvalApi = useApproval()
const assetsApi = useAssets()

// ---------------------------------------------------------------------------
// Ajukan Peminjaman (inline form, not a modal, per the mockup)
// ---------------------------------------------------------------------------
const availableAssets = ref<AvailableAsset[]>([])
const assetId = ref('')
const dueDate = ref('')
const notes = ref('')
const assetError = ref(false)
const notesError = ref(false)
const submitting = ref(false)

const assetItems = computed(() => availableAssets.value.map(a => ({ value: a.id, label: `${a.name} · ${a.asset_tag}` })))

function resetForm() {
  assetId.value = ''
  dueDate.value = ''
  notes.value = ''
  assetError.value = false
  notesError.value = false
}

async function loadAvailable() {
  try {
    const res = await assignmentApi.available()
    availableAssets.value = res.data
  } catch {
    availableAssets.value = []
  }
}

async function submitBorrow() {
  const reason = notes.value.trim()
  assetError.value = !assetId.value
  notesError.value = !reason
  if (assetError.value || notesError.value || submitting.value) return

  submitting.value = true
  try {
    const picked = availableAssets.value.find(a => a.id === assetId.value)
    await assignmentApi.borrow({
      asset_id: assetId.value,
      due_date: dueDate.value || null,
      notes: reason
    })
    resetForm()
    toast.add({ title: t('peminjaman.toast.sent'), description: picked ? t('peminjaman.toast.sentDesc', { name: picked.name }) : undefined, color: 'success' })
    await Promise.all([loadAvailable(), loadRequests()])
  } catch {
    // useApiClient already raised an error toast
  } finally {
    submitting.value = false
  }
}

// ---------------------------------------------------------------------------
// Pengajuan Peminjaman Saya
// ---------------------------------------------------------------------------
type FilterKey = 'pending' | 'approved' | 'rejected' | 'all'
const FILTER_STATUS: Record<FilterKey, string> = { pending: 'pending', approved: 'approved', rejected: 'rejected', all: '' }

const filter = ref<FilterKey>('all')
const requests = ref<MyRequestRow[]>([])
const loading = ref(true)
const loadError = ref(false)
const openId = ref<string | null>(null)
const timelineCache = ref(new Map<string, ApprovalStep[]>())
const timelineLoading = ref(new Set<string>())
// Best-effort asset name/tag resolution: the `mine` request list carries only
// target_id (no name/tag enrichment server-side — see task-12-report.md
// "Asset-name resolution" deviation). Resolved lazily per id and cached.
const assetNameCache = ref(new Map<string, { name: string, tag: string }>())
const cancellingId = ref<string | null>(null)

const filterTabs = computed<{ key: FilterKey, label: string }[]>(() => [
  { key: 'pending', label: t('peminjaman.filter.pending') },
  { key: 'approved', label: t('peminjaman.filter.approved') },
  { key: 'rejected', label: t('peminjaman.filter.rejected') },
  { key: 'all', label: t('peminjaman.filter.all') }
])

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-GB' : 'id-ID', {
    day: '2-digit', month: 'short', year: 'numeric'
  }).format(d)
}
function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const date = fmtDate(iso)
  const time = new Intl.DateTimeFormat('id-ID', { hour: '2-digit', minute: '2-digit', hour12: false }).format(d)
  return `${date} · ${time}`
}

function resolveAssetId(row: MyRequestRow): string | null {
  return row.payload?.asset_id ?? row.target_id ?? null
}

async function resolveAssetName(id: string) {
  if (assetNameCache.value.has(id)) return
  try {
    const asset = await assetsApi.get(id)
    assetNameCache.value.set(id, { name: asset.name, tag: asset.asset_tag })
  } catch {
    // Best-effort only — the row falls back to showing the raw id.
  }
}

const rows = computed(() => requests.value.map((r) => {
  const aid = resolveAssetId(r)
  const known = aid ? assetNameCache.value.get(aid) : undefined
  return {
    id: r.id,
    assetName: known?.name ?? null,
    assetTag: known?.tag ?? aid,
    diajukan: fmtDate(r.created_at),
    tempo: r.payload?.due_date ? fmtDate(r.payload.due_date) : null,
    status: r.status,
    statusTone: REQUEST_STATUS_TONE[r.status],
    catatan: r.decision_note,
    canCancel: r.status === 'pending',
    open: openId.value === r.id
  }
}))

async function loadRequests() {
  loading.value = true
  loadError.value = false
  try {
    const status = FILTER_STATUS[filter.value]
    const res = await assignmentApi.myRequests(status ? { status } : {})
    requests.value = res.data as unknown as MyRequestRow[]
    // Best-effort name resolution for every row currently shown; failures are silent.
    for (const r of requests.value) {
      const aid = resolveAssetId(r)
      if (aid) resolveAssetName(aid)
    }
  } catch {
    loadError.value = true
    requests.value = []
  } finally {
    loading.value = false
  }
}

watch(filter, loadRequests)

async function toggleRow(id: string) {
  if (openId.value === id) {
    openId.value = null
    return
  }
  openId.value = id
  if (timelineCache.value.has(id)) return
  timelineLoading.value.add(id)
  try {
    const detail = await approvalApi.get(id)
    timelineCache.value.set(id, detail.steps)
    const aid = detail.payload?.asset_id as string | undefined ?? detail.target_id ?? undefined
    if (aid && !assetNameCache.value.has(aid)) await resolveAssetName(aid)
  } catch {
    timelineCache.value.set(id, [])
  } finally {
    timelineLoading.value.delete(id)
  }
}

function timelineFor(id: string): ApprovalStep[] {
  return timelineCache.value.get(id) ?? []
}

async function cancelRequest(id: string) {
  if (cancellingId.value) return
  cancellingId.value = id
  try {
    await assignmentApi.cancel(id)
    await loadRequests()
  } catch {
    // useApiClient already raised an error toast
  } finally {
    cancellingId.value = null
  }
}

onMounted(async () => {
  await Promise.all([loadAvailable(), loadRequests()])
})
</script>

<template>
  <div class="max-w-[760px] mx-auto">
    <!-- Header -->
    <div class="mb-[18px]">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('peminjaman.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('peminjaman.subtitle') }}
      </p>
    </div>

    <!-- (1) Ajukan Peminjaman -->
    <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden mb-6">
      <div class="flex items-center gap-2.5 px-5 py-4 border-b border-default">
        <span class="size-[34px] rounded-[9px] bg-primary/10 text-primary flex items-center justify-center flex-none">
          <UIcon
            name="i-lucide-handshake"
            class="size-[17px]"
          />
        </span>
        <div>
          <div class="font-semibold text-[15px]">
            {{ t('peminjaman.form.title') }}
          </div>
          <div class="text-[12.5px] text-muted mt-px">
            {{ t('peminjaman.form.subtitle') }}
          </div>
        </div>
      </div>

      <div class="p-5 flex flex-col gap-4">
        <UFormField
          :label="t('peminjaman.form.aset')"
          :hint="t('peminjaman.form.asetHint')"
          :error="assetError ? t('peminjaman.form.asetErr') : undefined"
          required
        >
          <USelectMenu
            v-model="assetId"
            data-testid="peminjaman-asset-picker"
            value-key="value"
            :items="assetItems"
            icon="i-lucide-search"
            :placeholder="t('peminjaman.form.asetPlaceholder')"
            :search-input="{ placeholder: t('peminjaman.form.asetPlaceholder') }"
            class="w-full"
            @update:model-value="assetError = false"
          />
        </UFormField>

        <UFormField :label="t('peminjaman.form.tempo')">
          <UInput
            v-model="dueDate"
            data-testid="peminjaman-due-date"
            type="date"
            class="w-full"
          />
          <template #hint>
            <span class="text-xs text-dimmed">{{ t('peminjaman.form.tempoHint') }}</span>
          </template>
        </UFormField>

        <UFormField
          :label="t('peminjaman.form.alasan')"
          :error="notesError ? t('peminjaman.form.alasanErr') : undefined"
          required
        >
          <UTextarea
            v-model="notes"
            data-testid="peminjaman-notes"
            :rows="3"
            :placeholder="t('peminjaman.form.alasanPlaceholder')"
            class="w-full"
            @update:model-value="notesError = false"
          />
        </UFormField>

        <div class="flex gap-2.5 items-start px-3.5 py-3 rounded-[11px] bg-primary/10 border border-primary/25">
          <UIcon
            name="i-lucide-info"
            class="size-4 flex-none mt-px text-primary"
          />
          <span class="text-[12.5px] leading-relaxed font-medium text-primary">{{ t('peminjaman.infoBanner') }}</span>
        </div>
      </div>

      <div class="flex justify-end gap-2.5 px-5 py-3.5 border-t border-default">
        <UButton
          color="neutral"
          variant="outline"
          data-testid="peminjaman-reset"
          @click="resetForm"
        >
          {{ t('peminjaman.form.cancel') }}
        </UButton>
        <UButton
          icon="i-lucide-send"
          :loading="submitting"
          data-testid="peminjaman-submit"
          @click="submitBorrow"
        >
          {{ t('peminjaman.form.submit') }}
        </UButton>
      </div>
    </div>

    <!-- (2) Pengajuan Peminjaman Saya -->
    <div class="flex items-center justify-between gap-3 flex-wrap mb-3.5">
      <div class="font-semibold text-base">
        {{ t('peminjaman.list.title') }}
      </div>
      <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg">
        <button
          v-for="f in filterTabs"
          :key="f.key"
          class="px-3 py-1.5 text-xs font-semibold rounded-md transition-colors"
          :class="filter === f.key ? 'bg-default text-default shadow-sm' : 'text-muted hover:text-default'"
          :data-testid="`peminjaman-filter-${f.key}`"
          @click="filter = f.key"
        >
          {{ f.label }}
        </button>
      </div>
    </div>

    <div
      v-if="loading"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <USkeleton class="h-[42px] w-full rounded-none" />
      <div
        v-for="n in 3"
        :key="n"
        class="flex items-center gap-4 px-4 py-3.5 border-t border-default"
      >
        <USkeleton class="h-3 w-[150px] rounded" />
        <USkeleton class="h-3 flex-1 rounded" />
        <USkeleton class="h-5 w-[90px] rounded-full" />
      </div>
    </div>

    <div
      v-else-if="loadError"
      class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
      data-testid="peminjaman-load-error"
    >
      <p class="text-sm text-muted mb-3">
        {{ t('common.loadError') }}
      </p>
      <UButton
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-rotate-cw"
        data-testid="peminjaman-retry"
        @click="loadRequests"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>

    <div
      v-else-if="rows.length === 0"
      class="bg-default border border-default rounded-[14px] shadow-sm py-[54px] px-6 text-center"
    >
      <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-clipboard-check"
          class="size-[26px]"
        />
      </div>
      <div class="text-base font-semibold mb-1.5">
        {{ t('peminjaman.list.emptyTitle') }}
      </div>
      <div class="text-sm text-muted">
        {{ t('peminjaman.list.emptySub') }}
      </div>
    </div>

    <div
      v-else
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full border-collapse text-[13.5px]">
          <thead>
            <tr class="bg-muted text-muted">
              <th class="w-[30px] px-2 py-[11px]" />
              <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide whitespace-nowrap">
                {{ t('peminjaman.list.colAsset') }}
              </th>
              <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide whitespace-nowrap">
                {{ t('peminjaman.list.colDiajukan') }}
              </th>
              <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide whitespace-nowrap">
                {{ t('peminjaman.list.colTempo') }}
              </th>
              <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide whitespace-nowrap">
                {{ t('peminjaman.list.colStatus') }}
              </th>
              <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                {{ t('peminjaman.list.colCatatan') }}
              </th>
              <th class="text-right px-4 py-[11px] text-xs font-semibold uppercase tracking-wide whitespace-nowrap">
                {{ t('peminjaman.list.colAksi') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <template
              v-for="row in rows"
              :key="row.id"
            >
              <tr
                class="border-t border-default cursor-pointer hover:bg-muted transition-colors"
                :class="row.open ? 'bg-muted' : ''"
                :data-testid="`peminjaman-row-${row.id}`"
                @click="toggleRow(row.id)"
              >
                <td class="px-2 py-3">
                  <UIcon
                    name="i-lucide-chevron-right"
                    class="size-[15px] text-dimmed transition-transform"
                    :class="row.open ? 'rotate-90' : ''"
                  />
                </td>
                <td class="px-3 py-3">
                  <div class="font-medium whitespace-nowrap">
                    {{ row.assetName ?? row.assetTag ?? '—' }}
                  </div>
                  <div class="font-mono text-[11px] text-dimmed">
                    {{ row.assetName ? row.assetTag : '' }}
                  </div>
                </td>
                <td class="px-3 py-3 text-muted whitespace-nowrap">
                  {{ row.diajukan }}
                </td>
                <td
                  class="px-3 py-3 whitespace-nowrap"
                  :class="row.tempo ? 'text-muted' : 'text-dimmed'"
                >
                  {{ row.tempo ?? '—' }}
                </td>
                <td class="px-3 py-3">
                  <UBadge
                    :color="row.statusTone"
                    variant="subtle"
                    class="rounded-full gap-1.5"
                    :data-testid="`peminjaman-status-${row.id}`"
                  >
                    <span
                      class="size-1.5 rounded-full"
                      :class="STATUS_DOT[row.statusTone]"
                    />
                    {{ t(`peminjaman.status.${row.status}`) }}
                  </UBadge>
                </td>
                <td class="px-3 py-3 text-muted max-w-[200px]">
                  <span class="block truncate">{{ row.catatan ?? '—' }}</span>
                </td>
                <td class="px-4 py-3 text-right whitespace-nowrap">
                  <UButton
                    v-if="row.canCancel"
                    size="xs"
                    color="error"
                    variant="outline"
                    icon="i-lucide-x"
                    :loading="cancellingId === row.id"
                    :data-testid="`peminjaman-cancel-${row.id}`"
                    @click.stop="cancelRequest(row.id)"
                  >
                    {{ t('peminjaman.list.cancelAction') }}
                  </UButton>
                  <span
                    v-else
                    class="text-xs text-dimmed"
                  >—</span>
                </td>
              </tr>
              <tr
                v-if="row.open"
                class="bg-muted"
              >
                <td
                  colspan="7"
                  class="px-5 py-4"
                  :data-testid="`peminjaman-timeline-${row.id}`"
                >
                  <div class="text-[11px] font-semibold uppercase tracking-wider text-dimmed mb-3">
                    {{ t('peminjaman.list.timelineTitle') }}
                  </div>
                  <div
                    v-if="timelineLoading.has(row.id)"
                    class="flex flex-col gap-2"
                  >
                    <USkeleton class="h-4 w-2/3 rounded" />
                    <USkeleton class="h-4 w-1/2 rounded" />
                  </div>
                  <div
                    v-else-if="timelineFor(row.id).length === 0"
                    class="text-[12.5px] text-dimmed"
                  >
                    {{ t('peminjaman.list.timelineEmpty') }}
                  </div>
                  <div
                    v-else
                    class="flex flex-col gap-3"
                  >
                    <div
                      v-for="(step, i) in timelineFor(row.id)"
                      :key="i"
                      class="flex gap-3"
                    >
                      <span class="size-[22px] rounded-full bg-default border border-default flex items-center justify-center flex-none text-[11px] font-semibold">
                        {{ step.step_order }}
                      </span>
                      <div class="min-w-0 flex-1">
                        <div class="text-[13px] font-semibold">
                          {{ t(`peminjaman.list.decision.${step.decision}`) }}
                        </div>
                        <div class="text-xs text-muted mt-px">
                          {{ step.approver_name ?? t('peminjaman.list.timelineWaiting') }} · {{ fmtDateTime(step.decided_at) }}
                        </div>
                        <div
                          v-if="step.note"
                          class="mt-1.5 px-2.5 py-2 rounded-lg bg-default border border-default text-[12.5px] leading-snug text-muted"
                        >
                          "{{ step.note }}"
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
      <div class="px-4 py-3 border-t border-default text-[13px] text-muted">
        {{ t('peminjaman.list.total', { n: rows.length }) }}
      </div>
    </div>
  </div>
</template>
