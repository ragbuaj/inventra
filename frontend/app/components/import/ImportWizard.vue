<script setup lang="ts">
import { useImports, importRowColumns, importRowValue } from '~/composables/api/useImports'
import type { ImportJob, ImportRow, ImportCellError } from '~/composables/api/useImports'

// Reusable, backend-driven bulk-import wizard. Reproduces the anatomy of
// docs/design/Import Aset.dc.html (stepper → upload card + template row →
// dashed drop-zone / selected-file card → columns badges → validate table
// with per-cell error highlighting → processing progress card → result card)
// but is generic across import targets (asset, employee, office, reference).
const props = defineProps<{ target: string, permission: string }>()

const { t, te } = useI18n()
const toast = useToast()
const can = useCan()
const imports = useImports()

// --- Asset columns badges (kept from the mockup for the asset target). Other
// targets don't hardcode columns — they get a generic "download template" note.
const ASSET_COLUMNS: [string, boolean][] = [
  ['asset_tag', false], ['nama', true], ['kategori', true], ['kantor', true],
  ['tgl_beli', true], ['harga', true], ['vendor', false], ['lokasi', false]
]
// Stable header order for the asset preview table (falls back to data keys).
const ASSET_COL_ORDER = ['asset_tag', 'nama', 'kategori', 'kantor', 'tgl_beli', 'harga', 'vendor', 'lokasi']

const MAX_BYTES = 10 * 1024 * 1024
const POLL_MS = 1500
// Statuses where the backend is still working and the client must keep polling.
const ACTIVE = ['pending', 'processing', 'confirmed', 'executing', 'awaiting_approval']
// Statuses/derived states that terminate the flow (no more polling, not "resumable").
const FINAL = ['completed', 'failed', 'cancelled']

const isAsset = computed(() => props.target === 'asset')
const allowed = computed(() => can(props.permission))

// --- State ------------------------------------------------------------------
const job = ref<ImportJob | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const confirming = ref(false)
const dlTemplate = ref(false)
const dlErrors = ref(false)
const resuming = ref(true)

const rows = ref<ImportRow[]>([])
const rowsTotal = ref(0)
const rowsLoading = ref(false)
const rowsError = ref(false)
const onlyErrors = ref(false)
const limit = ref(20)
const offset = ref(0)

let timer: ReturnType<typeof setTimeout> | undefined
// Set once onBeforeUnmount fires. A getJob request can still be in flight at
// that point; its resolution must not schedule a new timer or mutate state —
// onBeforeUnmount has already run and nothing will ever clear a timer set
// after it.
let disposed = false

// --- Derived phase / step ---------------------------------------------------
const status = computed(() => job.value?.status ?? '')
const rejected = computed(() => job.value?.approval_status === 'rejected')

type Phase = 'upload' | 'validating' | 'validated' | 'creating' | 'awaiting' | 'completed' | 'failed' | 'rejected'
const phase = computed<Phase>(() => {
  if (!job.value) return 'upload'
  if (rejected.value) return 'rejected'
  switch (status.value) {
    case 'pending':
    case 'processing': return 'validating'
    case 'validated': return 'validated'
    case 'confirmed':
    case 'executing': return 'creating'
    case 'awaiting_approval': return 'awaiting'
    case 'completed': return 'completed'
    case 'failed': return 'failed'
    default: return 'upload'
  }
})

const stepNo = computed(() => {
  switch (phase.value) {
    case 'upload': return 1
    case 'validating':
    case 'validated': return 2
    default: return 3
  }
})

const busy = computed(() => phase.value === 'validating' || phase.value === 'creating')
const progressKey = computed<'validate' | 'create'>(() => phase.value === 'creating' ? 'create' : 'validate')
const progressPct = computed<number | null>(() => {
  const p = job.value?.progress
  if (p && p.total > 0) return Math.min(100, Math.round((p.done / p.total) * 100))
  return null
})

const steps = computed(() => ([
  { n: 1, label: t('assets.import.steps.upload'), sub: t('assets.import.steps.uploadSub') },
  { n: 2, label: t('assets.import.steps.validate'), sub: t('assets.import.steps.validateSub') },
  { n: 3, label: t('assets.import.steps.result'), sub: t('assets.import.steps.resultSub') }
]))

// --- Counts + preview table columns ----------------------------------------
const totalRows = computed(() => job.value?.total_rows ?? 0)
const validCount = computed(() => job.value?.success_rows ?? 0)
const errorCount = computed(() => job.value?.failed_rows ?? 0)

const dataColumns = computed<string[]>(() => {
  const set = new Set<string>()
  for (const r of rows.value) for (const k of importRowColumns(r)) set.add(k)
  if (isAsset.value) {
    const ordered = ASSET_COL_ORDER.filter(c => set.has(c))
    const extra = [...set].filter(c => !ASSET_COL_ORDER.includes(c))
    return [...ordered, ...extra]
  }
  return [...set]
})

const pageFrom = computed(() => (rowsTotal.value === 0 ? 0 : offset.value + 1))
const pageTo = computed(() => Math.min(offset.value + limit.value, rowsTotal.value))
const canPrev = computed(() => offset.value > 0)
const canNext = computed(() => offset.value + limit.value < rowsTotal.value)

function cellError(r: ImportRow, col: string): ImportCellError | undefined {
  return r.errors.find(e => e.column === col)
}
function cellClass(r: ImportRow, col: string): string {
  return cellError(r, col) ? 'bg-error/10 text-error' : ''
}
// Asset-specific cell styling to match the Import Aset mockup (asset_tag is
// monospaced). Other targets fall back to the plain generic cell.
function cellExtraClass(col: string): string {
  return isAsset.value && col === 'asset_tag' ? 'font-mono text-[12px]' : ''
}
// Asset-specific cell display: the harga column carries a "Rp" prefix when it
// holds a numeric value, mirroring the mockup. Everything else renders raw.
function cellDisplay(r: ImportRow, col: string): string {
  const v = importRowValue(r, col)
  if (isAsset.value && col === 'harga' && v && /^[\d.,]+$/.test(v)) return `Rp ${v}`
  return v || '—'
}
function cellErrorMsg(r: ImportRow, col: string): string {
  const e = cellError(r, col)
  return e ? errKeyMsg(e.error_key) : ''
}
function rowNote(r: ImportRow): string {
  if (r.errors.length === 0) return ''
  return r.errors.map(e => errKeyMsg(e.error_key)).join('; ')
}
function errKeyMsg(key: string): string {
  const k = `import.wizard.cellErrors.${key}`
  return te(k) ? t(k) : key
}
function jobErrorMsg(): string {
  const k = `import.wizard.jobErrors.${job.value?.error_key ?? 'unknown'}`
  return te(k) ? t(k) : t('import.wizard.jobErrors.unknown')
}

// --- Polling ----------------------------------------------------------------
function stopPolling() {
  if (timer) {
    clearTimeout(timer)
    timer = undefined
  }
}
function afterJob(j: ImportJob) {
  job.value = j
  if (j.status === 'validated') {
    void loadRows()
  } else if (ACTIVE.includes(j.status) && j.approval_status !== 'rejected') {
    schedulePoll()
  }
}
function schedulePoll() {
  // No-op once torn down — otherwise a getJob resolving after unmount could
  // arm a timer that nothing will ever clear again.
  if (disposed) return
  stopPolling()
  timer = setTimeout(poll, POLL_MS)
}
async function poll() {
  if (!job.value) return
  try {
    const j = await imports.getJob(job.value.id)
    if (disposed) return
    afterJob(j)
  } catch {
    // Transport errors already surface a toast via useApiClient; stop polling.
  }
}

// --- File selection + upload -----------------------------------------------
function openPicker() {
  fileInput.value?.click()
}
function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f) return
  if (!/\.(csv|xlsx)$/i.test(f.name)) {
    toast.add({ title: t('import.wizard.wrongType'), color: 'error', icon: 'i-lucide-file-x' })
    input.value = ''
    return
  }
  if (f.size > MAX_BYTES) {
    toast.add({ title: t('import.wizard.fileTooLarge'), color: 'error', icon: 'i-lucide-file-x' })
    input.value = ''
    return
  }
  selectedFile.value = f
}
function removeFile() {
  selectedFile.value = null
  if (fileInput.value) fileInput.value.value = ''
}
function fileMeta(f: File): string {
  return `${(f.size / 1024).toFixed(0)} KB`
}
async function startUpload() {
  if (!selectedFile.value || uploading.value) return
  uploading.value = true
  try {
    afterJob(await imports.uploadImport(props.target, selectedFile.value))
  } catch {
    // toast handled by useApiClient
  } finally {
    uploading.value = false
  }
}

// --- Preview rows -----------------------------------------------------------
async function loadRows() {
  if (!job.value) return
  rowsLoading.value = true
  rowsError.value = false
  try {
    const res = await imports.getRows(job.value.id, {
      onlyErrors: onlyErrors.value,
      limit: limit.value,
      offset: offset.value
    })
    if (disposed) return
    rows.value = res.data
    rowsTotal.value = res.total
  } catch {
    if (!disposed) rowsError.value = true
  } finally {
    if (!disposed) rowsLoading.value = false
  }
}
watch(onlyErrors, () => {
  offset.value = 0
  void loadRows()
})
function prevPage() {
  if (!canPrev.value) return
  offset.value = Math.max(0, offset.value - limit.value)
  void loadRows()
}
function nextPage() {
  if (!canNext.value) return
  offset.value += limit.value
  void loadRows()
}

// --- Confirm ----------------------------------------------------------------
async function confirm() {
  if (!job.value || confirming.value) return
  confirming.value = true
  try {
    afterJob(await imports.confirmJob(job.value.id))
  } catch {
    // toast handled by useApiClient
  } finally {
    confirming.value = false
  }
}

// --- Downloads (Bearer-auth blobs → object URL → <a download>) --------------
function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
async function downloadTemplate() {
  if (dlTemplate.value) return
  dlTemplate.value = true
  try {
    const blob = await imports.getTemplate(props.target, 'csv')
    triggerDownload(blob, `${props.target}-template.csv`)
  } catch {
    // toast handled by useApiClient
  } finally {
    dlTemplate.value = false
  }
}
async function downloadErrors() {
  if (!job.value || dlErrors.value) return
  dlErrors.value = true
  try {
    const blob = await imports.getErrorReport(job.value.id)
    triggerDownload(blob, `${props.target}-errors.csv`)
  } catch {
    // toast handled by useApiClient
  } finally {
    dlErrors.value = false
  }
}

// --- Reset ------------------------------------------------------------------
function reset() {
  stopPolling()
  job.value = null
  selectedFile.value = null
  rows.value = []
  rowsTotal.value = 0
  offset.value = 0
  onlyErrors.value = false
  rowsError.value = false
  if (fileInput.value) fileInput.value.value = ''
}

// --- Resume on mount --------------------------------------------------------
onBeforeUnmount(() => {
  disposed = true
  stopPolling()
})
onMounted(async () => {
  try {
    const res = await imports.listJobs(props.target, { limit: 1 })
    const latest = res.data[0]
    // Resume any non-final job. list() does NOT enrich approval_status or
    // progress (only the single-job GET does, via enrichJob on the backend) —
    // so fetch the enriched view and decide the initial phase off THAT, not
    // the stale list row. Otherwise an already-rejected asset batch would
    // briefly render the "awaiting approval" card, and resumed progress would
    // be missing.
    if (latest && !FINAL.includes(latest.status)) {
      try {
        const enriched = await imports.getJob(latest.id)
        if (!disposed) afterJob(enriched)
      } catch {
        // Enriched fetch failed — start fresh rather than rendering
        // decisions off the un-enriched list row.
      }
    }
  } catch {
    // A failed resume just starts the wizard at step 1.
  } finally {
    if (!disposed) resuming.value = false
  }
})
</script>

<template>
  <div>
    <!-- Stepper -->
    <div class="flex items-center gap-2 mb-6 max-w-[640px]">
      <template
        v-for="(st, i) in steps"
        :key="st.n"
      >
        <div class="flex items-center gap-3">
          <div
            class="size-9 rounded-full border flex items-center justify-center flex-none text-sm font-semibold"
            :class="stepNo > st.n ? 'bg-primary text-inverted border-primary'
              : stepNo === st.n ? 'bg-primary/10 text-primary border-primary'
                : 'bg-muted text-dimmed border-default'"
          >
            <UIcon
              v-if="stepNo > st.n"
              name="i-lucide-check"
              class="size-4"
            />
            <span v-else>{{ st.n }}</span>
          </div>
          <div class="hidden sm:block">
            <div
              class="text-[13px] font-semibold"
              :class="stepNo >= st.n ? 'text-default' : 'text-muted'"
            >
              {{ st.label }}
            </div>
            <div class="text-[11.5px] text-dimmed">
              {{ st.sub }}
            </div>
          </div>
        </div>
        <div
          v-if="i < 2"
          class="flex-1 h-px"
          :class="stepNo > st.n ? 'bg-primary' : 'bg-default'"
        />
      </template>
    </div>

    <!-- Resuming (mount fetch in flight) -->
    <div
      v-if="resuming"
      class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-7 animate-spin text-primary mb-3"
      />
      <div class="text-sm text-muted">
        {{ t('import.wizard.resuming') }}
      </div>
    </div>

    <!-- Processing (validate / create) -->
    <div
      v-else-if="busy"
      class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center"
    >
      <div class="max-w-[420px] mx-auto">
        <UIcon
          name="i-lucide-loader-circle"
          class="size-7 animate-spin text-primary mb-3"
        />
        <div class="text-base font-semibold mb-1">
          {{ progressKey === 'create' ? t('assets.import.processing.create') : t('assets.import.processing.validate') }}
        </div>
        <div class="text-sm text-muted mb-4">
          {{ progressKey === 'create' ? t('assets.import.processing.createSub') : t('assets.import.processing.validateSub') }}
        </div>
        <div class="h-2 rounded-full bg-muted overflow-hidden">
          <div
            v-if="progressPct !== null"
            class="h-full bg-primary transition-[width] duration-150"
            :style="{ width: `${progressPct}%` }"
          />
          <div
            v-else
            class="h-full w-1/3 bg-primary animate-pulse rounded-full"
          />
        </div>
        <div
          v-if="progressPct !== null"
          class="text-xs text-dimmed mt-2"
        >
          {{ progressPct }}%
        </div>
      </div>
    </div>

    <!-- Step 1: Upload -->
    <div
      v-else-if="phase === 'upload'"
      class="space-y-4"
    >
      <input
        ref="fileInput"
        type="file"
        accept=".csv,.xlsx"
        class="hidden"
        data-testid="import-file-input"
        @change="onFileChange"
      >

      <div class="bg-default border border-default rounded-[14px] shadow-sm p-5 flex items-center justify-between gap-4 flex-wrap">
        <div class="flex items-center gap-3">
          <span class="size-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-file-spreadsheet"
              class="size-5"
            />
          </span>
          <div>
            <div class="text-sm font-semibold">
              {{ t('assets.import.templateTitle') }}
            </div>
            <div class="text-[12.5px] text-muted">
              {{ t('assets.import.templateSub') }}
            </div>
          </div>
        </div>
        <UButton
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :loading="dlTemplate"
          :label="t('assets.import.templateTitle')"
          @click="downloadTemplate"
        />
      </div>

      <div class="bg-default border border-default rounded-[14px] shadow-sm p-5">
        <button
          v-if="!selectedFile"
          type="button"
          class="w-full flex flex-col items-center justify-center gap-2 py-10 px-4 rounded-[12px] border-2 border-dashed border-default text-center cursor-pointer hover:border-primary transition-colors"
          @click="openPicker"
        >
          <UIcon
            name="i-lucide-upload-cloud"
            class="size-8 text-dimmed"
          />
          <span class="text-sm font-medium">{{ t('assets.import.dropTitle') }}</span>
          <span class="text-xs text-dimmed">{{ t('import.wizard.dropSub') }}</span>
        </button>
        <div
          v-else
          class="flex items-center gap-3 p-3.5 rounded-[11px] border border-default bg-muted"
        >
          <span class="size-9 rounded-lg bg-success/10 text-success flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-file-check"
              class="size-4"
            />
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium truncate">
              {{ selectedFile.name }}
            </div>
            <div class="text-xs text-muted">
              {{ fileMeta(selectedFile) }}
            </div>
          </div>
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            size="sm"
            square
            :aria-label="t('common.delete')"
            @click="removeFile"
          />
        </div>

        <div class="mt-5">
          <div class="text-[13px] font-semibold mb-2">
            {{ t('assets.import.columnsTitle') }}
          </div>
          <div
            v-if="isAsset"
            class="flex flex-wrap gap-2"
          >
            <UBadge
              v-for="([name, req]) in ASSET_COLUMNS"
              :key="name"
              :color="req ? 'primary' : 'neutral'"
              variant="subtle"
              class="rounded-md font-mono"
            >
              {{ name }}{{ req ? ' *' : '' }}
            </UBadge>
          </div>
          <div class="text-xs text-dimmed mt-2.5">
            {{ isAsset ? t('assets.import.columnsNote') : t('import.wizard.columnsNoteGeneric') }}
          </div>
        </div>
      </div>

      <div class="flex justify-end">
        <UButton
          icon="i-lucide-circle-check-big"
          :disabled="!selectedFile || !allowed"
          :loading="uploading"
          :label="t('assets.import.validate')"
          @click="startUpload"
        />
      </div>
    </div>

    <!-- Step 2: Validate -->
    <div
      v-else-if="phase === 'validated'"
      class="space-y-4"
      data-testid="import-step-validate"
    >
      <div class="bg-default border border-default rounded-[14px] shadow-sm p-4 flex items-center gap-4 flex-wrap">
        <span class="text-[13px] text-muted">{{ t('assets.import.totalRows') }}: <span class="font-semibold text-default">{{ totalRows }}</span></span>
        <UBadge
          color="success"
          variant="subtle"
          class="rounded-full"
          data-testid="import-valid-count"
        >
          {{ validCount }} {{ t('assets.import.valid') }}
        </UBadge>
        <UBadge
          color="error"
          variant="subtle"
          class="rounded-full"
          data-testid="import-error-count"
        >
          {{ errorCount }} {{ t('assets.import.error') }}
        </UBadge>
        <div class="flex-1" />
        <label class="flex items-center gap-2 text-[13px] cursor-pointer">
          <UCheckbox v-model="onlyErrors" />
          {{ t('assets.import.onlyErrors') }}
        </label>
      </div>

      <!-- Rows loading -->
      <div
        v-if="rowsLoading"
        class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center"
      >
        <UIcon
          name="i-lucide-loader-circle"
          class="size-6 animate-spin text-primary"
        />
        <div class="text-sm text-muted mt-2">
          {{ t('import.wizard.loading') }}
        </div>
      </div>

      <!-- Rows error -->
      <div
        v-else-if="rowsError"
        class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center"
      >
        <UIcon
          name="i-lucide-triangle-alert"
          class="size-6 text-error"
        />
        <div class="text-sm text-muted mt-2 mb-3">
          {{ t('import.wizard.rowsError') }}
        </div>
        <UButton
          icon="i-lucide-refresh-cw"
          color="neutral"
          variant="outline"
          size="sm"
          :label="t('import.wizard.retry')"
          @click="loadRows"
        />
      </div>

      <!-- Rows empty -->
      <div
        v-else-if="rows.length === 0"
        class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center text-sm text-muted"
      >
        {{ t('import.wizard.emptyRows') }}
      </div>

      <!-- Rows table -->
      <div
        v-else
        class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden"
      >
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase w-[40px]">
                  #
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  {{ t('assets.import.status') }}
                </th>
                <th
                  v-for="col in dataColumns"
                  :key="col"
                  class="text-left px-3 py-2.5 text-xs font-semibold uppercase font-mono"
                >
                  {{ col }}
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  {{ t('assets.import.note') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="r in rows"
                :key="r.row_no"
                class="border-t border-default"
              >
                <td class="px-3 py-2.5 text-dimmed">
                  {{ r.row_no }}
                </td>
                <td class="px-3 py-2.5">
                  <UBadge
                    :color="r.valid ? 'success' : 'error'"
                    variant="subtle"
                    class="rounded-full"
                  >
                    {{ r.valid ? t('assets.import.valid') : t('assets.import.error') }}
                  </UBadge>
                </td>
                <td
                  v-for="col in dataColumns"
                  :key="col"
                  class="px-3 py-2.5"
                  :class="[cellClass(r, col), cellExtraClass(col)]"
                  :title="cellErrorMsg(r, col)"
                >
                  {{ cellDisplay(r, col) }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="r.errors.length ? 'text-error' : 'text-dimmed'"
                >
                  {{ rowNote(r) || '—' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div
          v-if="rowsTotal > limit"
          class="flex items-center justify-between gap-3 px-3 py-2.5 border-t border-default text-[12.5px] text-muted"
        >
          <span>{{ t('import.wizard.pageInfo', { from: pageFrom, to: pageTo, total: rowsTotal }) }}</span>
          <div class="flex items-center gap-2">
            <UButton
              icon="i-lucide-chevron-left"
              color="neutral"
              variant="outline"
              size="xs"
              :disabled="!canPrev"
              :label="t('import.wizard.prev')"
              @click="prevPage"
            />
            <UButton
              trailing-icon="i-lucide-chevron-right"
              color="neutral"
              variant="outline"
              size="xs"
              :disabled="!canNext"
              :label="t('import.wizard.next')"
              @click="nextPage"
            />
          </div>
        </div>
      </div>

      <div class="flex justify-between">
        <UButton
          icon="i-lucide-arrow-left"
          color="neutral"
          variant="outline"
          :label="t('assets.import.back')"
          @click="reset"
        />
        <UButton
          icon="i-lucide-circle-check-big"
          :disabled="!allowed || validCount === 0"
          :loading="confirming"
          :label="`${isAsset ? t('assets.import.create') : t('import.wizard.confirm')} (${validCount})`"
          data-testid="import-confirm-button"
          @click="confirm"
        />
      </div>
    </div>

    <!-- Step 3: Result — asset awaiting approval -->
    <div
      v-else-if="phase === 'awaiting'"
      class="bg-default border border-default rounded-[14px] shadow-sm p-8 text-center max-w-[560px] mx-auto"
      data-testid="import-awaiting-approval"
    >
      <div class="size-[54px] mx-auto mb-4 rounded-2xl bg-warning/10 text-warning flex items-center justify-center">
        <UIcon
          name="i-lucide-clock"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ t('import.wizard.approval.title') }}
      </div>
      <div class="text-sm text-muted mb-4">
        {{ t('import.wizard.approval.desc') }}
      </div>
      <div
        v-if="job?.request_id"
        class="inline-flex items-center gap-2 rounded-[10px] border border-default bg-muted px-3 py-2 mb-5"
      >
        <span class="text-xs text-muted">{{ t('import.wizard.approval.reference') }}</span>
        <span class="text-[12px] font-mono font-semibold">{{ job.request_id }}</span>
      </div>
      <div class="flex items-center justify-center gap-2 text-[13px] text-muted">
        <UIcon
          name="i-lucide-loader-circle"
          class="size-4 animate-spin text-primary"
        />
        {{ t('import.wizard.approval.waiting') }}
      </div>
    </div>

    <!-- Step 3: Result — rejected -->
    <div
      v-else-if="phase === 'rejected'"
      class="bg-default border border-default rounded-[14px] shadow-sm p-8 text-center max-w-[560px] mx-auto"
    >
      <div class="size-[54px] mx-auto mb-4 rounded-2xl bg-error/10 text-error flex items-center justify-center">
        <UIcon
          name="i-lucide-circle-x"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ t('import.wizard.rejected.title') }}
      </div>
      <div class="text-sm text-muted mb-5">
        {{ t('import.wizard.rejected.desc') }}
      </div>
      <div class="flex items-center justify-center gap-2.5 flex-wrap">
        <UButton
          v-if="errorCount > 0"
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :loading="dlErrors"
          :label="t('assets.import.downloadFailed')"
          @click="downloadErrors"
        />
        <UButton
          :label="t('assets.import.finish')"
          @click="reset"
        />
      </div>
    </div>

    <!-- Step 3: Result — failed job -->
    <div
      v-else-if="phase === 'failed'"
      class="bg-default border border-default rounded-[14px] shadow-sm p-8 text-center max-w-[560px] mx-auto"
    >
      <div class="size-[54px] mx-auto mb-4 rounded-2xl bg-error/10 text-error flex items-center justify-center">
        <UIcon
          name="i-lucide-triangle-alert"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ t('import.wizard.failed.title') }}
      </div>
      <div class="text-sm text-muted mb-5">
        {{ jobErrorMsg() }}
      </div>
      <div class="flex items-center justify-center gap-2.5 flex-wrap">
        <UButton
          icon="i-lucide-arrow-left"
          color="neutral"
          variant="outline"
          :label="t('assets.import.back')"
          @click="reset"
        />
      </div>
    </div>

    <!-- Step 3: Result — completed (master-data & asset post-approval) -->
    <div
      v-else
      class="bg-default border border-default rounded-[14px] shadow-sm p-8 text-center max-w-[560px] mx-auto"
    >
      <div class="size-[54px] mx-auto mb-4 rounded-2xl bg-success/10 text-success flex items-center justify-center">
        <UIcon
          name="i-lucide-circle-check-big"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ t('import.wizard.result.title') }}
      </div>
      <div class="text-sm text-muted mb-5">
        {{ t('import.wizard.result.desc') }}
      </div>
      <div class="grid grid-cols-2 gap-3 mb-5">
        <div
          class="rounded-[12px] border border-default p-4"
          data-testid="import-result-created"
        >
          <div class="text-2xl font-bold text-success">
            {{ validCount }}
          </div>
          <div class="text-[13px] text-muted">
            {{ t('assets.import.createdLabel') }}
          </div>
        </div>
        <div
          class="rounded-[12px] border border-default p-4"
          data-testid="import-result-failed"
        >
          <div class="text-2xl font-bold text-error">
            {{ errorCount }}
          </div>
          <div class="text-[13px] text-muted">
            {{ t('assets.import.failedLabel') }}
          </div>
        </div>
      </div>
      <div class="flex items-center justify-center gap-2.5 flex-wrap">
        <UButton
          v-if="errorCount > 0"
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :loading="dlErrors"
          :label="t('assets.import.downloadFailed')"
          @click="downloadErrors"
        />
        <UButton
          :label="t('assets.import.finish')"
          @click="reset"
        />
      </div>
    </div>
  </div>
</template>
