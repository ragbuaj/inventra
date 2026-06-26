<script setup lang="ts">
import type { ApprovalRequest, ReqType, ReqStatus, TimelineAction } from '~/mock/approval'
import type { BadgeColor } from '~/types'
import { useApproval } from '~/composables/api/useApproval'
import { loc, TYPE_META, STATUS_TONE, REQ_TYPE_KEYS, STATUS_FILTERS, YOU_ACTOR } from '~/mock/approval'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const TONE_SOFT: Record<BadgeColor, string> = {
  primary: 'bg-primary/15 text-primary',
  info: 'bg-info/15 text-info',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning',
  error: 'bg-error/15 text-error',
  neutral: 'bg-muted text-muted'
}
const TIMELINE_DOT: Record<TimelineAction | 'pending', string> = {
  submitted: 'bg-info',
  approved: 'bg-success',
  rejected: 'bg-error',
  pending: 'bg-warning'
}

const { t, locale } = useI18n()
const api = useApproval()

const requests = ref<ApprovalRequest[]>([])
const loading = ref(true)
const filter = ref<ReqStatus | 'all'>('pending')
const tipeFilter = ref<ReqType | 'all'>('all')
const selectedId = ref<string | null>('r1')
const note = ref('')
const deciding = ref(false)

const pendingCount = computed(() => requests.value.filter(r => r.status === 'pending').length)

const filterTabs = computed(() => STATUS_FILTERS.map(k => ({ key: k, label: t(`approval.filter.${k}`) })))
const tipeItems = computed(() => [
  { value: 'all', label: t('approval.allTypes') },
  ...REQ_TYPE_KEYS.map(k => ({ value: k, label: t(`approval.type.${k}`) }))
])

const filteredList = computed(() => requests.value.filter((r) => {
  if (filter.value !== 'all' && r.status !== filter.value) return false
  if (tipeFilter.value !== 'all' && r.tipe !== tipeFilter.value) return false
  return true
}))

const listRows = computed(() => filteredList.value.map((r) => {
  const meta = TYPE_META[r.tipe]
  return {
    id: r.id,
    icon: meta.icon,
    iconSoft: TONE_SOFT[meta.tone],
    tipeLabel: t(`approval.type.${r.tipe}`),
    sensitive: meta.sensitive,
    judul: r.judul,
    pengaju: r.pengaju,
    tgl: r.tgl,
    statusTone: STATUS_TONE[r.status],
    statusLabel: t(`approval.status.${r.status}`),
    selected: r.id === selectedId.value
  }
}))

const selected = computed(() => requests.value.find(r => r.id === selectedId.value) ?? null)

const detail = computed(() => {
  const r = selected.value
  if (!r) return null
  const meta = TYPE_META[r.tipe]
  const summary = (r.summary ?? []).map(s => ({ label: loc(s.label, locale.value), value: loc(s.value, locale.value) }))
  const diff = (r.diff ?? []).map(d => ({ label: loc(d.label, locale.value), before: loc(d.before, locale.value), after: loc(d.after, locale.value) }))

  const tl = r.timeline.map(e => ({
    action: t(`approval.action.${e.action}`),
    actor: `${loc(e.actor, locale.value)} · ${loc(e.role, locale.value)}`,
    date: e.date,
    note: loc(e.note, locale.value),
    dot: TIMELINE_DOT[e.action],
    line: true
  }))
  if (r.status === 'pending') {
    tl.push({ action: t('approval.action.pending'), actor: loc(YOU_ACTOR, locale.value), date: '—', note: '', dot: TIMELINE_DOT.pending, line: false })
  } else if (tl.length) {
    tl[tl.length - 1]!.line = false
  }

  const decided = r.status !== 'pending'
  const last = decided ? r.timeline[r.timeline.length - 1] : null
  const resultText = decided && last
    ? (r.status === 'approved'
        ? t('approval.resultApproved', { actor: loc(last.actor, locale.value), date: last.date })
        : t('approval.resultRejected', { actor: loc(last.actor, locale.value), date: last.date }))
    : ''

  return {
    req: r,
    icon: meta.icon,
    iconSoft: TONE_SOFT[meta.tone],
    tipeLabel: t(`approval.type.${r.tipe}`),
    sensitive: meta.sensitive,
    statusTone: STATUS_TONE[r.status],
    statusLabel: t(`approval.status.${r.status}`),
    role: loc(r.role, locale.value),
    isDiff: !!r.diff,
    summary,
    diff,
    alasan: loc(r.alasan, locale.value),
    timeline: tl,
    pending: r.status === 'pending',
    decided,
    resultText,
    resultTone: r.status === 'approved' ? 'success' as const : 'error' as const,
    resultIcon: r.status === 'approved' ? 'i-lucide-check' : 'i-lucide-x'
  }
})

function selectRequest(id: string) {
  selectedId.value = id
  note.value = ''
}

watch([filter, tipeFilter], () => {
  selectedId.value = null
  note.value = ''
})

async function decide(action: 'approved' | 'rejected') {
  if (!selected.value || deciding.value) return
  deciding.value = true
  await api.decide(selected.value.id, action, note.value)
  requests.value = await api.list()
  note.value = ''
  deciding.value = false
}

onMounted(async () => {
  loading.value = true
  requests.value = await api.list()
  loading.value = false
})
</script>

<template>
  <div class="flex flex-col h-[calc(100vh-9.5rem)] min-h-[560px] -m-1">
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
      <!-- LEFT: inbox -->
      <div class="w-[340px] flex-none border-e border-default flex flex-col min-h-0">
        <div class="flex-none p-3.5 border-b border-default">
          <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg mb-2.5">
            <button
              v-for="f in filterTabs"
              :key="f.key"
              class="flex-1 py-1.5 text-xs font-semibold rounded-md transition-colors"
              :class="filter === f.key ? 'bg-default text-default shadow-sm' : 'text-muted hover:text-default'"
              @click="filter = f.key"
            >
              {{ f.label }}
            </button>
          </div>
          <USelect
            v-model="tipeFilter"
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

      <!-- RIGHT: detail -->
      <div class="flex-1 flex flex-col min-w-0 bg-muted/30">
        <template v-if="detail">
          <div class="flex-1 overflow-y-auto p-6">
            <div class="max-w-[680px]">
              <!-- header -->
              <div class="flex items-center gap-2 flex-wrap mb-2.5">
                <UBadge
                  :color="TYPE_META[detail.req.tipe].tone"
                  variant="subtle"
                  class="rounded-full gap-1.5"
                >
                  <UIcon
                    :name="detail.icon"
                    class="size-3.5"
                  />
                  {{ detail.tipeLabel }}
                </UBadge>
                <UBadge
                  v-if="detail.sensitive"
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
                  :color="detail.statusTone"
                  variant="subtle"
                  class="rounded-full"
                >
                  {{ detail.statusLabel }}
                </UBadge>
              </div>
              <h2 class="text-[21px] font-bold tracking-tight mb-4">
                {{ detail.req.judul }}
              </h2>

              <!-- pengaju -->
              <div class="flex items-center gap-3 px-[15px] py-3.5 rounded-xl bg-default border border-default shadow-sm mb-[18px]">
                <span class="size-10 rounded-full bg-primary/15 text-primary flex items-center justify-center text-sm font-bold flex-none">{{ detail.req.ini }}</span>
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-semibold">
                    {{ detail.req.pengaju }}
                  </div>
                  <div class="text-[12.5px] text-muted">
                    {{ detail.role }} · {{ detail.req.kantor }}
                  </div>
                </div>
                <div class="text-right flex-none">
                  <div class="text-[11px] text-dimmed">
                    {{ t('approval.submitted') }}
                  </div>
                  <div class="text-[12.5px] font-medium text-muted">
                    {{ detail.req.tgl }}
                  </div>
                </div>
              </div>

              <!-- data -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.dataSection') }}
              </div>
              <div class="bg-default border border-default rounded-xl shadow-sm overflow-hidden mb-[18px]">
                <template v-if="detail.isDiff">
                  <div class="grid grid-cols-[140px_1fr_22px_1fr] items-center px-4 py-2.5 bg-muted text-[11px] font-semibold uppercase text-dimmed">
                    <span>{{ t('approval.thField') }}</span>
                    <span>{{ t('approval.thBefore') }}</span>
                    <span />
                    <span>{{ t('approval.thAfter') }}</span>
                  </div>
                  <div
                    v-for="(f, i) in detail.diff"
                    :key="i"
                    class="grid grid-cols-[140px_1fr_22px_1fr] items-center px-4 py-2.5 border-t border-default text-[13.5px]"
                  >
                    <span class="text-muted">{{ f.label }}</span>
                    <span class="text-dimmed line-through">{{ f.before }}</span>
                    <UIcon
                      name="i-lucide-arrow-right"
                      class="size-3.5 text-dimmed"
                    />
                    <span class="font-semibold">{{ f.after }}</span>
                  </div>
                </template>
                <template v-else>
                  <div
                    v-for="(f, i) in detail.summary"
                    :key="i"
                    class="flex items-center justify-between gap-3.5 px-4 py-2.5 border-t border-default first:border-t-0 text-[13.5px]"
                  >
                    <span class="text-muted">{{ f.label }}</span>
                    <span class="font-medium text-right">{{ f.value }}</span>
                  </div>
                </template>
              </div>

              <!-- alasan -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.reasonSection') }}
              </div>
              <div class="px-4 py-3.5 rounded-xl bg-default border border-default shadow-sm text-sm leading-relaxed mb-[18px]">
                {{ detail.alasan }}
              </div>

              <!-- lampiran -->
              <div class="text-xs font-semibold uppercase tracking-wider text-muted mb-2.5">
                {{ t('approval.attachSection') }}
              </div>
              <div
                v-if="detail.req.files.length > 0"
                class="flex flex-wrap gap-2 mb-5"
              >
                <span
                  v-for="(f, i) in detail.req.files"
                  :key="i"
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-[12.5px] font-medium rounded-[9px] bg-default border border-default"
                >
                  <UIcon
                    name="i-lucide-file-text"
                    class="size-3.5 text-muted"
                  />
                  {{ f }}
                </span>
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
                  v-for="(e, i) in detail.timeline"
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
            v-if="detail.pending"
            class="flex-none border-t border-default bg-default p-4 px-7"
          >
            <div class="max-w-[680px]">
              <div
                v-if="detail.sensitive"
                class="flex gap-2.5 items-center px-3 py-2.5 mb-3 rounded-[10px] bg-warning/10 border border-warning/30 text-warning text-[12.5px] leading-snug font-medium"
              >
                <UIcon
                  name="i-lucide-triangle-alert"
                  class="size-4 flex-none"
                />
                {{ t('approval.sensitiveWarn') }}
              </div>
              <div class="flex gap-3 items-end">
                <UFormField
                  :label="t('approval.noteLabel')"
                  class="flex-1"
                >
                  <UInput
                    v-model="note"
                    :placeholder="t('approval.notePlaceholder')"
                    class="w-full"
                  />
                </UFormField>
                <UButton
                  icon="i-lucide-x"
                  color="error"
                  :label="t('approval.reject')"
                  :loading="deciding"
                  @click="decide('rejected')"
                />
                <UButton
                  icon="i-lucide-check"
                  :label="t('approval.approve')"
                  :loading="deciding"
                  @click="decide('approved')"
                />
              </div>
            </div>
          </div>
          <div
            v-else
            class="flex-none border-t border-default bg-default p-4 px-7"
          >
            <div
              class="max-w-[680px] flex items-center gap-2.5 px-3.5 py-3 rounded-[11px] border"
              :class="detail.resultTone === 'success' ? 'bg-success/10 border-success/30 text-success' : 'bg-error/10 border-error/30 text-error'"
            >
              <UIcon
                :name="detail.resultIcon"
                class="size-[17px] flex-none"
              />
              <span class="text-[13.5px] font-semibold">{{ detail.resultText }}</span>
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
