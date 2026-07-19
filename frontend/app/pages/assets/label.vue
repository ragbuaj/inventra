<script setup lang="ts">
import type { Asset } from '~/types'

definePageMeta({ middleware: 'can', permission: 'asset.view' })

const MAX_SELECTED = 500
const PICKER_LIMIT = 50
const DEBOUNCE_MS = 300

// Preview boxes at 5 px/mm, mirroring the backend size presets
// (backend/internal/asset/barcode.go sizePresets).
const SIZES: Record<string, { w: number, h: number }> = {
  '60x24': { w: 300, h: 120 },
  '50x30': { w: 250, h: 150 },
  '70x40': { w: 350, h: 200 },
  '100x50': { w: 500, h: 250 }
}

// Mirrors the backend app_settings defaults (label.company_name /
// label.disclaimer) for the on-screen preview only — the printed PDF always
// uses the server-side values.
const LABEL_COMPANY = 'PT Bank Tabungan Negara (Persero) Tbk'
const LABEL_DISCLAIMER = 'Tidak Untuk Diperjualbelikan & Apabila Dipindah posisi untuk disampaikan ke Pengelola Gedung'

// A4 sheet-fit constants — mirror the backend's `sheetFits` check
// (backend/internal/asset/barcode.go:104-107): cols*labelW + (cols-1)*gutter +
// 2*margin <= pageW, with pageW=210mm, margin=8mm/side (16mm total), gutter=3mm
// between columns. Batch prints whose column count violates this get a 400
// ErrSheetOverflow from the backend, so the UI must never offer/send one.
const A4_PAGE_W_MM = 210
const A4_MARGINS_MM = 16
const SHEET_GUTTER_MM = 3

function maxColsForLabelWidth(labelWmm: number): number {
  const usable = A4_PAGE_W_MM - A4_MARGINS_MM + SHEET_GUTTER_MM
  return Math.max(1, Math.floor(usable / (labelWmm + SHEET_GUTTER_MM)))
}

const { t } = useI18n()
const route = useRoute()
const toast = useToast()
const assetsApi = useAssets()
const { requestBlob } = useApiClient()

const size = ref('60x24')
const cols = ref(3)
const downloading = ref(false)

const sizeOptions = [
  { value: '60x24', label: '60 × 24 mm' },
  { value: '50x30', label: '50 × 30 mm' },
  { value: '70x40', label: '70 × 40 mm' },
  { value: '100x50', label: '100 × 50 mm' }
]

const sz = computed(() => SIZES[size.value] ?? SIZES['60x24']!)

// Label width in mm, parsed from the "WxH" size key (e.g. '70x40' → 70).
const sizeWidthMM = computed(() => Number(size.value.split('x')[0]))
const maxCols = computed(() => maxColsForLabelWidth(sizeWidthMM.value))

// Keep the selected column count within what the current size can fit on an
// A4 sheet — runs immediately so the default 70x40/3-column combo (which
// overflows: 70mm only fits 2 columns) is clamped on mount too, not just on
// later size changes.
watch(maxCols, (max) => {
  if (cols.value > max) cols.value = max
}, { immediate: true })

// --- Picker: server search over /assets (debounced), independent of selection. ---
const pickerQuery = ref('')
const debouncedPickerQuery = ref('')
const pickerResults = ref<Asset[]>([])
const pickerLoading = ref(true)
const pickerError = ref(false)

const pickerIds = computed(() => pickerResults.value.map(a => a.id))
const allChecked = computed(() =>
  pickerIds.value.length > 0 && pickerIds.value.every(id => selectedMap.value.has(id)))

let pickerSeq = 0
async function loadPicker() {
  const mine = ++pickerSeq
  pickerLoading.value = true
  pickerError.value = false
  try {
    const res = await assetsApi.list({ search: debouncedPickerQuery.value.trim() || undefined, limit: PICKER_LIMIT })
    if (mine !== pickerSeq) return
    pickerResults.value = res.data
    pickerLoading.value = false
  } catch {
    if (mine !== pickerSeq) return
    pickerError.value = true
    pickerLoading.value = false
  }
}

let pickerTimer: ReturnType<typeof setTimeout> | undefined
watch(pickerQuery, (v) => {
  if (pickerTimer) clearTimeout(pickerTimer)
  pickerTimer = setTimeout(() => {
    debouncedPickerQuery.value = v
  }, DEBOUNCE_MS)
})
watch(debouncedPickerQuery, () => loadPicker())

// --- Selection: Map keyed by asset id so we hold real Asset objects. ---
const selectedMap = ref<Map<string, Asset>>(new Map())
const selectedLabels = computed(() => Array.from(selectedMap.value.values()))
const perPage = computed(() => {
  const rowsPer = Math.max(1, Math.floor(1040 / (sz.value.h + 12)))
  return cols.value * rowsPer
})

function warnCap() {
  toast.add({ title: t('assets.label.maxSelected', { n: MAX_SELECTED }), color: 'warning', icon: 'i-lucide-triangle-alert' })
}

function toggle(asset: Asset) {
  const next = new Map(selectedMap.value)
  if (next.has(asset.id)) {
    next.delete(asset.id)
    selectedMap.value = next
    return
  }
  if (next.size >= MAX_SELECTED) {
    warnCap()
    return
  }
  next.set(asset.id, asset)
  selectedMap.value = next
}

function addMany(assets: Asset[]) {
  const next = new Map(selectedMap.value)
  let overflow = false
  for (const a of assets) {
    if (next.has(a.id)) continue
    if (next.size >= MAX_SELECTED) {
      overflow = true
      break
    }
    next.set(a.id, a)
  }
  selectedMap.value = next
  if (overflow) warnCap()
}

function toggleAll() {
  if (allChecked.value) {
    const next = new Map(selectedMap.value)
    for (const id of pickerIds.value) next.delete(id)
    selectedMap.value = next
  } else {
    addMany(pickerResults.value)
  }
}

// --- Label field resolvers: the BTN label prints the office CODE (bold row)
// and the category NAME — resolved on demand via useResolveCache, no eager
// `{ limit: 100 }` list (a selected batch's ids can outnumber 100). ---
const officesApi = useOffices()
const officeCodeCache = useResolveCache(async (id) => {
  try {
    const o = await officesApi.get(id)
    return { id: o.id, label: o.code }
  } catch {
    return null
  }
})
const categoryCache = useResolveCache(useCategoryPicker().resolveFn)

function purchaseYear(a: Asset): string {
  return (a.purchase_date ?? '').slice(0, 4)
}

// --- QR previews (the BTN label always prints a QR): lazy-fetched per asset,
// cached so re-renders never refetch an image already retrieved. ---
const barcodeUrls = ref(new Map<string, string>())
const barcodeInFlight = new Set<string>()

function qrSrcFor(id: string): string | undefined {
  return barcodeUrls.value.get(id)
}

async function ensureQr(id: string) {
  if (barcodeInFlight.has(id)) return
  barcodeInFlight.add(id)
  try {
    const blob = await requestBlob(`/assets/${id}/barcode?type=qr`)
    const url = URL.createObjectURL(blob)
    const next = new Map(barcodeUrls.value)
    next.set(id, url)
    barcodeUrls.value = next
  } catch {
    // Allow a retry later (e.g. after re-selecting the asset).
    barcodeInFlight.delete(id)
  }
}

watch(selectedLabels, () => {
  for (const asset of selectedLabels.value) ensureQr(asset.id)
}, { immediate: true })

// --- Initial selection from ?tags=... (e.g. navigated from the catalog). ---
const initialTags = String(route.query.tags ?? '').split(',').map(s => s.trim()).filter(Boolean)
async function resolveInitialTags() {
  if (initialTags.length === 0) return
  const results = await Promise.allSettled(initialTags.map(tagValue => assetsApi.getByTag(tagValue)))
  const next = new Map(selectedMap.value)
  for (const r of results) {
    if (r.status === 'fulfilled' && next.size < MAX_SELECTED) next.set(r.value.id, r.value)
  }
  selectedMap.value = next
}

// --- Generate + download the label PDF. ---
async function downloadLabels() {
  if (selectedLabels.value.length === 0) return
  downloading.value = true
  try {
    // A single selected label prints on a continuous roll; more than one
    // normally uses the on-screen column count as a tiled sheet grid (matches
    // the "Label Tunggal"/"Label Batch" preview distinction above). But when
    // the current size only fits 1 column on an A4 sheet (e.g. 100x50), a
    // "sheet" with 1 column is pointless and the backend's A4 fit check would
    // reject anything above that — print it as a roll instead, same as a
    // single label.
    const isBatch = selectedLabels.value.length > 1
    const useRoll = !isBatch || maxCols.value <= 1
    const blob = await requestBlob('/assets/labels', {
      method: 'POST',
      body: {
        asset_ids: selectedLabels.value.map(a => a.id),
        template: 'btn',
        layout: useRoll ? 'roll' : 'sheet',
        size: size.value,
        ...(useRoll ? {} : { columns: cols.value })
      }
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = 'labels.pdf'
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
  } catch {
    // Failure is already surfaced by useApiClient's error toast.
  } finally {
    downloading.value = false
  }
}

onMounted(() => {
  loadPicker()
  resolveInitialTags()
})

onUnmounted(() => {
  if (pickerTimer) clearTimeout(pickerTimer)
  for (const url of barcodeUrls.value.values()) URL.revokeObjectURL(url)
})
</script>

<template>
  <div>
    <div class="mb-5">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('assets.label.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('assets.label.subtitle') }}
      </p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-[300px_1fr] gap-5 items-start">
      <!-- Left: select + layout -->
      <div class="flex flex-col gap-4">
        <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
          <div class="px-4 py-3 border-b border-default flex items-center justify-between">
            <span class="text-[13px] font-semibold">{{ t('assets.label.selectAssets') }}</span>
            <span class="text-[11.5px] text-muted">{{ t('assets.label.selected', { n: selectedMap.size }) }}</span>
          </div>
          <div class="p-3 border-b border-default">
            <UInput
              v-model="pickerQuery"
              icon="i-lucide-search"
              :placeholder="t('assets.label.searchPlaceholder')"
              class="w-full"
              size="sm"
            />
            <label class="flex items-center gap-2 mt-2.5 text-[12.5px] cursor-pointer">
              <UCheckbox
                :model-value="allChecked"
                :disabled="pickerLoading || pickerIds.length === 0"
                @update:model-value="toggleAll"
              />
              {{ t('assets.label.selectAll') }}
            </label>
          </div>

          <div
            v-if="pickerLoading"
            class="p-3 space-y-2"
          >
            <USkeleton
              v-for="n in 5"
              :key="n"
              class="h-[38px] w-full rounded-lg"
            />
          </div>

          <div
            v-else-if="pickerError"
            class="flex flex-col items-center gap-2.5 py-8 text-muted"
          >
            <UIcon
              name="i-lucide-circle-alert"
              class="size-5"
            />
            <span class="text-xs">{{ t('common.loadError') }}</span>
            <UButton
              color="neutral"
              variant="subtle"
              size="xs"
              @click="loadPicker"
            >
              {{ t('common.retry') }}
            </UButton>
          </div>

          <div
            v-else-if="pickerResults.length === 0"
            class="py-8 px-4 text-center text-xs text-muted"
          >
            {{ t('assets.label.pickerEmpty') }}
          </div>

          <div
            v-else
            class="max-h-[280px] overflow-y-auto p-2"
          >
            <label
              v-for="a in pickerResults"
              :key="a.id"
              class="flex items-start gap-2.5 px-2 py-2 rounded-lg cursor-pointer hover:bg-muted"
            >
              <UCheckbox
                :model-value="selectedMap.has(a.id)"
                class="mt-0.5"
                @update:model-value="toggle(a)"
              />
              <span class="min-w-0">
                <span class="block text-[12.5px] font-medium truncate">{{ a.name }}</span>
                <span class="block text-[11px] font-mono text-dimmed truncate">{{ a.asset_tag }}</span>
              </span>
            </label>
          </div>
        </div>

        <div class="bg-default border border-default rounded-[14px] shadow-sm p-4 space-y-4">
          <div class="text-[13px] font-semibold">
            {{ t('assets.label.layout') }}
          </div>
          <UFormField :label="t('assets.label.size')">
            <USelect
              v-model="size"
              :items="sizeOptions"
              class="w-full"
              size="sm"
            />
          </UFormField>
          <div>
            <div class="text-xs text-muted mb-1.5">
              {{ t('assets.label.columns') }}
            </div>
            <div class="flex gap-1.5">
              <UButton
                v-for="n in [2, 3, 4]"
                :key="n"
                :color="cols === n ? 'primary' : 'neutral'"
                :variant="cols === n ? 'soft' : 'outline'"
                size="sm"
                class="flex-1 justify-center"
                :disabled="n > maxCols"
                @click="() => { cols = n }"
              >
                {{ n }}
              </UButton>
            </div>
            <div class="text-[11px] text-dimmed mt-1.5">
              {{ t('assets.label.maxColsHint', { n: maxCols }) }}
            </div>
          </div>
          <p class="text-[11px] text-dimmed">
            {{ t('assets.label.btnTemplateHint') }}
          </p>
        </div>
      </div>

      <!-- Right: preview -->
      <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
        <div class="flex items-center justify-between gap-3 flex-wrap px-5 py-3.5 border-b border-default">
          <div>
            <div class="text-sm font-semibold">
              {{ selectedLabels.length <= 1 ? t('assets.label.single') : t('assets.label.batch') }}
            </div>
            <div class="text-[12px] text-muted">
              {{ t('assets.label.count', { n: selectedLabels.length }) }} · {{ t('assets.label.perPage', { n: perPage }) }}
            </div>
          </div>
          <div class="flex items-center gap-2.5">
            <UButton
              icon="i-lucide-download"
              color="neutral"
              variant="outline"
              size="sm"
              :label="t('assets.label.pdf')"
              :disabled="selectedLabels.length === 0"
              :loading="downloading"
              @click="downloadLabels"
            />
            <UButton
              icon="i-lucide-printer"
              size="sm"
              :label="t('assets.label.print')"
              :disabled="selectedLabels.length === 0"
              :loading="downloading"
              @click="downloadLabels"
            />
          </div>
        </div>

        <div
          v-if="selectedLabels.length === 0"
          class="py-16 px-6 text-center"
        >
          <div class="size-[54px] mx-auto mb-3.5 rounded-2xl bg-muted text-dimmed flex items-center justify-center">
            <UIcon
              name="i-lucide-printer"
              class="size-7"
            />
          </div>
          <div class="text-base font-semibold mb-1.5">
            {{ t('assets.label.emptyTitle') }}
          </div>
          <div class="text-sm text-muted max-w-[320px] mx-auto">
            {{ t('assets.label.emptySub') }}
          </div>
        </div>

        <div
          v-else
          class="p-5 overflow-x-auto"
        >
          <div
            class="grid gap-3 justify-start"
            :style="{ gridTemplateColumns: `repeat(${Math.min(cols, Math.max(1, selectedLabels.length))}, ${sz.w}px)` }"
          >
            <AssetLabel
              v-for="lbl in selectedLabels"
              :key="lbl.id"
              :tag="lbl.asset_tag"
              :nama="lbl.name"
              :kategori="categoryCache.get(lbl.category_id)"
              :kantor="officeCodeCache.get(lbl.office_id)"
              :tahun="purchaseYear(lbl)"
              :company="LABEL_COMPANY"
              :disclaimer="LABEL_DISCLAIMER"
              :size="sz"
              :qr-src="qrSrcFor(lbl.id)"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
