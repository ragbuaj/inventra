<script setup lang="ts">
import type { ApprovalRequestRow, ApprovalRequestDetail } from '~/composables/api/useApproval'
import type { RequestType, RequestStatus } from '~/constants/approvalMeta'
import type { BadgeColor } from '~/types'
import { useApproval } from '~/composables/api/useApproval'
import { TYPE_META, STATUS_TONE, REQUEST_TYPE_KEYS, STATUS_FILTERS } from '~/constants/approvalMeta'
import { payloadToView } from '~/utils/approvalPayload'

definePageMeta({ middleware: 'can', permission: 'request.decide' })

const TONE_SOFT: Record<BadgeColor, string> = {
  primary: 'bg-primary/15 text-primary',
  info: 'bg-info/15 text-info',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning',
  error: 'bg-error/15 text-error',
  neutral: 'bg-muted text-muted'
}
const TIMELINE_DOT: Record<string, string> = {
  submitted: 'bg-info',
  approved: 'bg-success',
  rejected: 'bg-error',
  cancelled: 'bg-muted',
  pending: 'bg-warning'
}

const { t, locale } = useI18n()
const api = useApproval()
const categoriesApi = useCategories()
const office = useOfficePicker()
const referenceApi = useReference()
const assetsApi = useAssets()
const attachmentsApi = useAssetAttachments()

const rows = ref<ApprovalRequestRow[]>([])
const inboxRows = ref<ApprovalRequestRow[]>([])
const loading = ref(true)
const loadError = ref(false)
const filter = ref<RequestStatus | 'all'>('pending')
const typeFilter = ref<RequestType | 'all'>('all')
const selectedId = ref<string | null>(null)
const detail = ref<ApprovalRequestDetail | null>(null)
const detailLoading = ref(false)
const note = ref('')
const deciding = ref(false)
// Mobile drill-down (below lg): false = inbox list full-width, true = detail
// full-width with a back button. On lg+ both panes are always visible.
const showDetailMobile = ref(false)

// FK name lookups for the Data section (same inline pattern as master/employees).
// Office resolves on demand via useResolveCache — no more eager
// `{ limit: 100 }` list (a mutasi payload's from/to office id can be outside it).
const categoryMap = ref(new Map<string, string>())
const officeCache = useResolveCache(office.resolveFn)
const problemCategoryMap = ref(new Map<string, string>())
// Best-effort asset name/tag resolution for maintenance-type payloads (asset_id
// isn't enriched server-side) — mirrors peminjaman.vue's resolveAssetName.
const assetNameCache = ref(new Map<string, { name: string, tag: string }>())
const attachmentLoading = ref(false)

const inboxIds = computed(() => new Set(inboxRows.value.map(r => r.id)))
const pendingCount = computed(() => inboxRows.value.length)

const filterTabs = computed(() => STATUS_FILTERS.map(k => ({ key: k, label: t(`approval.filter.${k}`) })))
const tipeItems = computed(() => [
  { value: 'all', label: t('approval.allTypes') },
  ...REQUEST_TYPE_KEYS.map(k => ({ value: k, label: t(`approval.type.${k}`) }))
])

function fmtDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-GB' : 'id-ID', {
    day: '2-digit', month: 'short', year: 'numeric'
  }).format(d)
}
function fmtDateTime(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const date = fmtDate(iso)
  const time = new Intl.DateTimeFormat('id-ID', { hour: '2-digit', minute: '2-digit', hour12: false }).format(d)
  return `${date} · ${time}`
}

function rowTitle(r: ApprovalRequestRow): string {
  return `${t(`approval.type.${r.type}`)} · ${r.office_name ?? '—'}`
}

const listRows = computed(() => rows.value.map((r) => {
  const meta = TYPE_META[r.type]
  return {
    id: r.id,
    icon: meta?.icon ?? 'i-lucide-file-question',
    iconSoft: TONE_SOFT[meta?.tone ?? 'neutral'],
    tipeLabel: t(`approval.type.${r.type}`),
    sensitive: meta?.sensitive ?? false,
    judul: rowTitle(r),
    pengaju: r.requested_by_name ?? '—',
    tgl: fmtDate(r.created_at),
    statusTone: STATUS_TONE[r.status] ?? 'neutral',
    statusLabel: t(`approval.status.${r.status}`),
    selected: r.id === selectedId.value
  }
}))

async function loadInbox() {
  inboxRows.value = await api.inbox()
}

async function loadTab() {
  loading.value = true
  loadError.value = false
  try {
    if (filter.value === 'pending') {
      await loadInbox()
      rows.value = typeFilter.value === 'all'
        ? inboxRows.value
        : inboxRows.value.filter(r => r.type === typeFilter.value)
    } else {
      const q = filter.value === 'all' ? {} : { status: filter.value }
      const page = await api.list({ ...q, type: typeFilter.value === 'all' ? undefined : typeFilter.value, limit: 100 })
      rows.value = page.data
    }
  } catch {
    loadError.value = true
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function resolveAssetName(id: string) {
  if (assetNameCache.value.has(id)) return
  try {
    const asset = await assetsApi.get(id)
    if (asset?.name && asset?.asset_tag) assetNameCache.value.set(id, { name: asset.name, tag: asset.asset_tag })
  } catch {
    // Best-effort only — the payload row falls back to the raw id.
  }
}

async function selectRequest(id: string) {
  selectedId.value = id
  showDetailMobile.value = true
  note.value = ''
  detailLoading.value = true
  try {
    detail.value = await api.get(id)
    if (detail.value.type === 'maintenance') {
      const aid = (detail.value.payload?.asset_id as string | undefined) ?? detail.value.target_id ?? undefined
      if (aid) await resolveAssetName(aid)
    }
  } catch {
    detail.value = null
  } finally {
    detailLoading.value = false
  }
}

watch([filter, typeFilter], async () => {
  selectedId.value = null
  detail.value = null
  note.value = ''
  showDetailMobile.value = false
  await loadTab()
})

const view = computed(() => {
  const d = detail.value
  if (!d) return null
  const meta = TYPE_META[d.type]
  const dataView = payloadToView(d, t, {
    categoryName: id => categoryMap.value.get(id),
    officeName: id => officeCache.get(id),
    assetName: (id) => {
      const known = assetNameCache.value.get(id)
      return known ? `${known.name} · ${known.tag}` : undefined
    },
    problemCategoryName: id => problemCategoryMap.value.get(id)
  })
  const maintPayload = d.type === 'maintenance' ? (d.payload as Record<string, unknown> | null ?? null) : null
  const attachmentId = typeof maintPayload?.attachment_id === 'string' ? maintPayload.attachment_id : undefined

  const initials = (d.requested_by_name ?? '—').split(/\s+/).map(w => w[0]).slice(0, 2).join('').toUpperCase()

  type TimelineEntry = { action: string, actor: string, date: string, note: string, dot: string, line: boolean }
  const tl: TimelineEntry[] = [{
    action: t('approval.action.submitted'),
    actor: `${d.requested_by_name ?? '—'} · ${d.requested_by_role ?? '—'}`,
    date: fmtDateTime(d.created_at),
    note: '',
    dot: TIMELINE_DOT.submitted!,
    line: true
  }]
  for (const s of d.steps) {
    if (s.decision === 'approved' || s.decision === 'rejected') {
      tl.push({
        action: t(`approval.action.${s.decision}`),
        actor: `${s.approver_name ?? '—'} · ${t(`approval.level.${s.required_level}`)}`,
        date: fmtDateTime(s.decided_at),
        note: s.note ?? '',
        dot: TIMELINE_DOT[s.decision]!,
        line: true
      })
    }
  }
  if (d.status === 'cancelled') {
    tl.push({
      action: t('approval.action.cancelled'),
      actor: d.requested_by_name ?? '—',
      date: '—',
      note: d.decision_note ?? '',
      dot: TIMELINE_DOT.cancelled!,
      line: false
    })
  } else if (d.status === 'pending') {
    const cur = d.steps.find(s => s.step_order === d.current_step)
    tl.push({
      action: cur
        ? t('approval.action.pendingStep', { n: cur.step_order, level: t(`approval.level.${cur.required_level}`) })
        : t('approval.action.pending'),
      actor: '—',
      date: '—',
      note: '',
      dot: TIMELINE_DOT.pending!,
      line: false
    })
  }
  if (tl.length && d.status !== 'pending' && d.status !== 'cancelled') tl[tl.length - 1]!.line = false

  const decided = d.status === 'approved' || d.status === 'rejected'
  const lastStep = [...d.steps].reverse().find(s => s.decision === 'approved' || s.decision === 'rejected')
  const resultText = d.status === 'cancelled'
    ? t('approval.resultCancelled')
    : decided && lastStep
      ? (d.status === 'approved'
          ? t('approval.resultApproved', { actor: lastStep.approver_name ?? '—', date: fmtDateTime(lastStep.decided_at) })
          : t('approval.resultRejected', { actor: lastStep.approver_name ?? '—', date: fmtDateTime(lastStep.decided_at) }))
      : ''

  return {
    req: d,
    icon: meta?.icon ?? 'i-lucide-file-question',
    tone: meta?.tone ?? 'neutral',
    iconSoft: TONE_SOFT[meta?.tone ?? 'neutral'],
    tipeLabel: t(`approval.type.${d.type}`),
    sensitive: meta?.sensitive ?? false,
    statusTone: STATUS_TONE[d.status] ?? 'neutral',
    statusLabel: t(`approval.status.${d.status}`),
    judul: rowTitle(d),
    pengaju: d.requested_by_name ?? '—',
    role: d.requested_by_role ?? '—',
    kantor: d.office_name ?? '—',
    ini: initials || '—',
    tgl: fmtDate(d.created_at),
    isDiff: dataView.layout === 'diff',
    dataRows: dataView.rows,
    attachmentId,
    alasan: d.reason ?? '—',
    timeline: tl,
    pending: d.status === 'pending',
    eligible: d.status === 'pending' && inboxIds.value.has(d.id),
    decided: decided || d.status === 'cancelled',
    resultText,
    resultTone: d.status === 'approved' ? 'success' as const : d.status === 'rejected' ? 'error' as const : 'neutral' as const,
    resultIcon: d.status === 'approved' ? 'i-lucide-check' : d.status === 'rejected' ? 'i-lucide-x' : 'i-lucide-ban'
  }
})

async function decide(action: 'approve' | 'reject') {
  const d = detail.value
  if (!d || deciding.value) return
  deciding.value = true
  try {
    if (action === 'approve') await api.approve(d.id, note.value || undefined)
    else await api.reject(d.id, note.value || undefined)
    note.value = ''
    await loadTab()
    await useInboxStore().refresh()
    await selectRequest(d.id)
  } catch {
    // useApiClient already raised a toast; re-sync state (403 SoD / 409 stale step).
    await loadTab()
    await useInboxStore().refresh()
    if (selectedId.value) await selectRequest(selectedId.value)
  } finally {
    deciding.value = false
  }
}

async function loadLookups() {
  try {
    categoryMap.value = new Map((await categoriesApi.tree()).map(c => [c.id, c.name]))
  } catch {
    // Best-effort; the mapper falls back to raw ids.
  }
  try {
    const problems = await referenceApi.list('problem-categories', { limit: 100 })
    problemCategoryMap.value = new Map(problems.data.map(r => [r.id, r.name]))
  } catch {
    // Best-effort — falls back to the raw id.
  }
}

async function viewAttachment() {
  const d = detail.value
  const p = (d?.payload ?? null) as Record<string, unknown> | null
  const attachmentId = typeof p?.attachment_id === 'string' ? p.attachment_id : undefined
  const assetId = typeof p?.asset_id === 'string' ? p.asset_id : (d?.target_id ?? undefined)
  if (!d || d.type !== 'maintenance' || !attachmentId || !assetId || attachmentLoading.value) return
  attachmentLoading.value = true
  try {
    const blob = await attachmentsApi.contentBlob(assetId, attachmentId)
    const url = URL.createObjectURL(blob)
    window.open(url, '_blank')
  } catch {
    // useApiClient already raised an error toast
  } finally {
    attachmentLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadTab(), loadLookups(), useInboxStore().refresh()])
})
</script>

<template>
  <div class="flex flex-col -m-1 lg:h-[calc(100vh-9.5rem)] lg:min-h-[560px]">
    <!-- Header -->
    <div class="flex items-center gap-2.5 mb-3 px-1">
      <h1 class="text-lg font-semibold tracking-tight">
        {{ t('approval.title') }}
      </h1>
      <UBadge
        v-if="pendingCount > 0"
        color="warning"
        variant="subtle"
        class="rounded-full font-bold"
      >
        {{ t('approval.pending', { n: pendingCount }) }}
      </UBadge>
    </div>

    <!-- Two-pane -->
    <div class="flex-1 flex min-h-0 border border-default rounded-[14px] overflow-hidden bg-default shadow-sm">
      <!-- LEFT: inbox (mobile: hidden while the detail is open) -->
      <div
        class="w-full lg:w-[340px] flex-none lg:border-e border-default flex-col min-h-0"
        :class="showDetailMobile ? 'hidden lg:flex' : 'flex'"
      >
        <div class="flex-none p-3.5 border-b border-default">
          <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg mb-2.5">
            <button
              v-for="f in filterTabs"
              :key="f.key"
              class="flex-1 py-1.5 text-xs font-semibold rounded-md transition-colors"
              :class="filter === f.key ? 'bg-default text-default shadow-sm' : 'text-muted hover:text-default'"
              :data-testid="`approval-tab-${f.key}`"
              @click="filter = f.key"
            >
              {{ f.label }}
            </button>
          </div>
          <USelect
            v-model="typeFilter"
            value-key="value"
            :items="tipeItems"
            class="w-full"
          />
        </div>

        <div class="flex-1 overflow-y-auto p-2.5">
          <div
            v-if="loading"
            class="flex flex-col gap-2"
          >
            <USkeleton
              v-for="n in 5"
              :key="n"
              class="h-[74px] w-full rounded-[11px]"
            />
          </div>

          <template v-else-if="listRows.length > 0">
            <button
              v-for="r in listRows"
              :key="r.id"
              class="flex gap-2.5 w-full p-3 mb-2 rounded-[11px] border text-left transition-colors hover:border-primary"
              :class="r.selected ? 'border-primary bg-primary/5' : 'border-default bg-default'"
              data-testid="approval-card"
              @click="selectRequest(r.id)"
            >
              <span
                class="size-9 rounded-[9px] flex items-center justify-center flex-none"
                :class="r.iconSoft"
              >
                <UIcon
                  :name="r.icon"
                  class="size-[18px]"
                />
              </span>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-1.5 mb-0.5">
                  <span class="text-[11px] font-semibold">{{ r.tipeLabel }}</span>
                  <UIcon
                    v-if="r.sensitive"
                    name="i-lucide-triangle-alert"
                    class="size-3 text-warning"
                  />
                </div>
                <div class="text-[13.5px] font-semibold leading-tight mb-1.5 line-clamp-2">
                  {{ r.judul }}
                </div>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-[11.5px] text-dimmed truncate">{{ r.pengaju }} · {{ r.tgl }}</span>
                  <UBadge
                    :color="r.statusTone"
                    variant="subtle"
                    size="sm"
                    class="rounded-full flex-none"
                  >
                    {{ r.statusLabel }}
                  </UBadge>
                </div>
              </div>
            </button>
          </template>

          <div
            v-else-if="loadError"
            class="py-[50px] px-5 text-center"
            data-testid="approval-load-error"
          >
            <div class="text-sm font-semibold mb-2">
              {{ t('approval.loadError') }}
            </div>
            <UButton
              size="sm"
              variant="soft"
              :label="t('approval.retry')"
              @click="loadTab"
            />
          </div>

          <div
            v-else
            class="py-[50px] px-5 text-center"
          >
            <div class="size-12 mx-auto mb-3 rounded-[13px] bg-muted text-dimmed flex items-center justify-center">
              <UIcon
                name="i-lucide-inbox"
                class="size-6"
              />
            </div>
            <div class="text-sm font-semibold mb-1">
              {{ t('approval.inboxEmptyTitle') }}
            </div>
            <div class="text-[12.5px] text-muted">
              {{ t('approval.inboxEmptySub') }}
            </div>
          </div>
        </div>
      </div>

      <!-- RIGHT: detail (mobile: only visible after selecting an item) -->
      <div
        class="flex-1 flex-col min-w-0 bg-muted/30"
        :class="showDetailMobile ? 'flex' : 'hidden lg:flex'"
      >
        <!-- mobile back bar -->
        <div class="lg:hidden flex-none border-b border-default bg-default px-3 py-2">
          <UButton
            icon="i-lucide-arrow-left"
            color="neutral"
            variant="ghost"
            size="sm"
            :label="t('common.back')"
            data-testid="approval-back"
            @click="() => { showDetailMobile = false }"
          />
        </div>
        <template v-if="detailLoading">
          <div class="flex-1 overflow-y-auto p-4 sm:p-6">
            <div class="max-w-[680px] space-y-4">
              <USkeleton class="h-8 w-2/3 rounded-lg" />
              <USkeleton class="h-20 w-full rounded-xl" />
              <USkeleton class="h-40 w-full rounded-xl" />
            </div>
          </div>
        </template>
        <template v-else-if="view">
          <div class="flex-1 overflow-y-auto p-4 sm:p-6">
            <div class="max-w-[680px]">
              <!-- header -->
              <div class="flex items-center gap-2 flex-wrap mb-2.5">
                <UBadge
                  :color="view.tone"
                  variant="subtle"
                  class="rounded-full gap-1.5"
                >
                  <UIcon
                    :name="view.icon"
                    class="size-3.5"
                  />
                  {{ view.tipeLabel }}
                </UBadge>
                <UBadge
                  v-if="view.sensitive"
                  color="warning"
                  variant="subtle"
                  class="rounded-full gap-1.5"
                >
                  <UIcon
                    name="i-lucide-triangle-alert"
                    class="size-3"
                  />
                  {{ t('approval.sensitive') }}
                </UBadge>
                <div class="flex-1" />
                <UBadge
                  :color="view.statusTone"
                  variant="subtle"
                  class="rounded-full"
                >
                  {{ view.statusLabel }}
                </UBadge>
              </div>
              <h2 class="text-[21px] font-bold tracking-tight mb-4">
                {{ view.judul }}
              </h2>

              <!-- pengaju -->
              <div class="flex items-center gap-3 px-[15px] py-3.5 rounded-xl bg-default border border-default shadow-sm mb-[18px]">
                <span class="size-10 rounded-full bg-primary/15 text-primary flex items-center justify-center text-sm font-bold flex-none">{{ view.ini }}</span>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-semibold">
                    {{ view.pengaju }}
                  </div>
                  <div class="text-[12.5px] text-muted">
                    {{ view.role }} · {{ view.kantor }}
                  </div>
                </div>
                <div class="text-right flex-none">
                  <div class="text-[11px] text-dimmed">
                    {{ t('approval.submitted') }}
                  </div>
                  <div class="text-[12.5px] font-medium text-muted">
                    {{ view.tgl }}
                  </div>
                </div>
              </div>

              <!-- data -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.dataSection') }}
              </div>
              <div
                v-if="view.dataRows.length === 0"
                class="px-4 py-3.5 rounded-xl bg-default border border-default shadow-sm text-[13px] text-dimmed mb-[18px]"
              >
                {{ t('approval.noData') }}
              </div>
              <div
                v-else
                class="bg-default border border-default rounded-xl shadow-sm overflow-hidden mb-[18px]"
              >
                <template v-if="view.isDiff">
                  <div class="hidden sm:grid grid-cols-[140px_1fr_22px_1fr] items-center px-4 py-2.5 bg-muted text-[11px] font-semibold uppercase text-dimmed">
                    <span>{{ t('approval.thField') }}</span>
                    <span>{{ t('approval.thBefore') }}</span>
                    <span />
                    <span>{{ t('approval.thAfter') }}</span>
                  </div>
                  <div
                    v-for="(f, i) in view.dataRows"
                    :key="i"
                    class="grid grid-cols-1 gap-y-1 sm:grid-cols-[140px_1fr_22px_1fr] sm:gap-y-0 items-center px-4 py-2.5 border-t first:border-t-0 sm:first:border-t border-default text-[13.5px]"
                  >
                    <span class="text-muted">{{ f.label }}</span>
                    <span class="text-dimmed line-through">{{ 'before' in f ? f.before : '' }}</span>
                    <UIcon
                      name="i-lucide-arrow-right"
                      class="size-3.5 text-dimmed hidden sm:block"
                    />
                    <span class="font-semibold">{{ 'after' in f ? f.after : '' }}</span>
                  </div>
                </template>
                <template v-else>
                  <div
                    v-for="(f, i) in view.dataRows"
                    :key="i"
                    class="flex items-center justify-between gap-3.5 px-4 py-2.5 border-t border-default first:border-t-0 text-[13.5px]"
                  >
                    <span class="text-muted">{{ f.label }}</span>
                    <span class="font-medium text-right">{{ 'value' in f ? f.value : '' }}</span>
                  </div>
                </template>
              </div>

              <!-- alasan -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.reasonSection') }}
              </div>
              <div class="px-4 py-3.5 rounded-xl bg-default border border-default shadow-sm text-sm leading-relaxed mb-[18px]">
                {{ view.alasan }}
              </div>

              <!-- lampiran -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.attachSection') }}
              </div>
              <div
                v-if="view.attachmentId"
                class="mb-5"
              >
                <UButton
                  icon="i-lucide-paperclip"
                  color="neutral"
                  variant="outline"
                  size="sm"
                  :label="t('approval.viewAttachment')"
                  :loading="attachmentLoading"
                  data-testid="approval-view-attachment"
                  @click="viewAttachment"
                />
              </div>
              <div
                v-else
                class="text-[13px] text-dimmed mb-5"
              >
                {{ t('approval.noAttach') }}
              </div>

              <!-- timeline -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-3">
                {{ t('approval.timelineSection') }}
              </div>
              <div class="ps-1.5">
                <div
                  v-for="(e, i) in view.timeline"
                  :key="i"
                  class="flex gap-3"
                >
                  <div class="flex flex-col items-center flex-none">
                    <span
                      class="size-[11px] rounded-full mt-0.5 ring-2 ring-[var(--ui-bg)]"
                      :class="e.dot"
                    />
                    <span
                      v-if="e.line"
                      class="w-0.5 flex-1 bg-default my-1 min-h-[18px]"
                    />
                  </div>
                  <div class="pb-4 min-w-0">
                    <div class="text-[13px] font-semibold">
                      {{ e.action }}
                    </div>
                    <div class="text-xs text-muted mt-px">
                      {{ e.actor }} · {{ e.date }}
                    </div>
                    <div
                      v-if="e.note"
                      class="mt-1.5 px-2.5 py-2 rounded-lg bg-muted text-[12.5px] leading-snug text-muted"
                    >
                      “{{ e.note }}”
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- footer action -->
          <div
            v-if="view.pending"
            class="flex-none border-t border-default bg-default p-4 lg:px-7"
          >
            <div class="max-w-[680px]">
              <div
                v-if="!view.eligible"
                class="flex gap-2.5 items-center px-3 py-2.5 rounded-[10px] bg-muted border border-default text-muted text-[12.5px] leading-snug font-medium"
                data-testid="approval-not-eligible"
              >
                <UIcon
                  name="i-lucide-lock"
                  class="size-4 flex-none"
                />
                {{ t('approval.notEligible') }}
              </div>
              <template v-else>
                <div
                  v-if="view.sensitive"
                  class="flex gap-2.5 items-center px-3 py-2.5 mb-3 rounded-[10px] bg-warning/10 border border-warning/30 text-warning text-[12.5px] leading-snug font-medium"
                >
                  <UIcon
                    name="i-lucide-triangle-alert"
                    class="size-4 flex-none"
                  />
                  {{ t('approval.sensitiveWarn') }}
                </div>
                <div class="flex flex-wrap gap-3 items-end">
                  <UFormField
                    :label="t('approval.noteLabel')"
                    class="w-full sm:w-auto sm:flex-1"
                  >
                    <UInput
                      v-model="note"
                      :placeholder="t('approval.notePlaceholder')"
                      class="w-full"
                      data-testid="approval-note"
                    />
                  </UFormField>
                  <UButton
                    icon="i-lucide-x"
                    color="error"
                    :label="t('approval.reject')"
                    :loading="deciding"
                    class="flex-1 justify-center sm:flex-none"
                    data-testid="approval-reject"
                    @click="decide('reject')"
                  />
                  <UButton
                    icon="i-lucide-check"
                    :label="t('approval.approve')"
                    :loading="deciding"
                    class="flex-1 justify-center sm:flex-none"
                    data-testid="approval-approve"
                    @click="decide('approve')"
                  />
                </div>
              </template>
            </div>
          </div>
          <div
            v-else
            class="flex-none border-t border-default bg-default p-4 lg:px-7"
          >
            <div
              class="max-w-[680px] flex items-center gap-2.5 px-3.5 py-3 rounded-[11px] border"
              :class="view.resultTone === 'success' ? 'bg-success/10 border-success/30 text-success'
                : view.resultTone === 'error' ? 'bg-error/10 border-error/30 text-error'
                  : 'bg-muted border-default text-muted'"
            >
              <UIcon
                :name="view.resultIcon"
                class="size-[17px] flex-none"
              />
              <span class="text-[13.5px] font-semibold">{{ view.resultText }}</span>
            </div>
          </div>
        </template>

        <!-- placeholder -->
        <div
          v-else
          class="flex-1 flex flex-col items-center justify-center gap-2.5 p-10 text-center"
        >
          <div class="size-[60px] rounded-2xl bg-muted text-dimmed flex items-center justify-center">
            <UIcon
              name="i-lucide-message-square"
              class="size-7"
            />
          </div>
          <div class="text-base font-semibold">
            {{ t('approval.phTitle') }}
          </div>
          <div class="text-sm text-muted max-w-[300px]">
            {{ t('approval.phSub') }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
