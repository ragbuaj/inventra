<script setup lang="ts">
import type { Asset, BadgeColor } from '~/types'
import type { AssetDepreciationEntry, AssetDepreciationResponse } from '~/composables/api/useDepreciation'
import type { MaintenanceRecord } from '~/composables/api/useMaintenance'
import type { AssetLocationHistory, AssetPICHistory } from '~/composables/api/useAssets'
import { classMeta } from '~/constants/assetMeta'
import { BASIS_META, type DepreciationBasis } from '~/constants/depreciationMeta'
import { MAINT_STATUS_TONE, MAINT_TYPE_TONE, formatRupiah as formatRupiahMaint } from '~/constants/maintenanceMeta'
import { formatDateID } from '~/constants/assignmentMeta'
import { formatRupiah } from '~/utils/format'
import { useCan } from '~/composables/useCan'

definePageMeta({ middleware: 'can', permission: 'asset.view' })

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const { t, locale } = useI18n()
const route = useRoute()
const localePath = useLocalePath()
const can = useCan()

const assetsApi = useAssets()
const attachmentsApi = useAssetAttachments()
const categoriesApi = useCategories()
const office = useOfficePicker()
const brand = useReferencePicker('brands')
const model = useReferencePicker('models')
const floorsApi = useFloors()
const referenceApi = useReference()
const deprApi = useDepreciation()
const maintenanceApi = useMaintenance()

const tag = computed(() => String(route.params.tag))
const asset = ref<Asset | null>(null)
const loading = ref(true)
const loadError = ref(false)
const notFound = ref(false)
const tab = ref<'info' | 'assign' | 'maint' | 'depr' | 'loc' | 'pic'>('info')

// FK id → name maps, populated by loadLookups() once the asset itself is
// known. Missing/unresolved ids render as "—" (see `name()` below). Office/
// brand/model resolve on-demand via useResolveCache — only ever one id apiece
// on this page, so no more eager `{ limit: 100 }` list (see loadLookups).
const categoryMap = ref(new Map<string, string>())
const officeCache = useResolveCache(office.resolveFn)
const brandCache = useResolveCache(brand.resolveFn)
const modelCache = useResolveCache(model.resolveFn)
const vendorMap = ref(new Map<string, string>())
const unitMap = ref(new Map<string, string>())
const roomLabel = ref('—')

interface Photo { id: string, url: string }
const photos = ref<Photo[]>([])
const activeIndex = ref(0)

function formatDate(tgl: string | null | undefined): string {
  if (!tgl) return '—'
  const [y, m, day] = tgl.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}

function name(id: string | null | undefined, map: Map<string, string>): string {
  if (!id) return '—'
  return map.get(id) ?? '—'
}

function brandModelLabel(): string {
  const a = asset.value
  if (!a) return '—'
  const brandLabel = a.brand_id ? brandCache.get(a.brand_id) : undefined
  const modelLabel = a.model_id ? modelCache.get(a.model_id) : undefined
  const parts = [brandLabel, modelLabel].filter((v): v is string => !!v && v !== '—')
  return parts.length > 0 ? parts.join(' ') : '—'
}

function boolText(v: boolean | undefined): string {
  if (v === true) return t('common.yes')
  if (v === false) return t('common.no')
  return '—'
}

interface MoneyCell { text: string, masked: boolean }
// Sensitive money fields (harga perolehan / akumulasi penyusutan / nilai buku)
// are stripped server-side by field-permission when the caller's role can't
// view them — the key comes back `undefined`, not `null` or `"0"`. Only that
// exact absence means "masked"; an explicit zero/negative value still prints.
function moneyCell(v: string | null | undefined): MoneyCell {
  if (v === undefined) return { text: '—', masked: true }
  if (v === null) return { text: '—', masked: false }
  const n = Number(v)
  return { text: Number.isFinite(n) ? `Rp ${n.toLocaleString('id-ID')}` : '—', masked: false }
}

const DEPR_METHOD_KEY: Record<string, string> = {
  straight_line: 'assets.detail.deprMethodValue.straight_line',
  declining_balance: 'assets.detail.deprMethodValue.declining_balance'
}
function deprMethodLabel(v: string | null | undefined): string {
  if (!v) return '—'
  const key = DEPR_METHOD_KEY[v]
  return key ? t(key) : v
}

function usefulLifeLabel(months: number | null | undefined): string {
  if (months == null) return '—'
  return t('assets.detail.months', { n: months })
}

// ---------------------------------------------------------------------------
// Depreciation tab — fetched once on first activation (and re-fetched if the
// route swaps to a different asset while the tab is already active).
// ---------------------------------------------------------------------------
const DEPR_BASIS_OPTIONS: DepreciationBasis[] = ['commercial', 'fiscal']
const deprBasis = ref<DepreciationBasis>('commercial')
const deprLoading = ref(false)
const deprError = ref(false)
const deprResp = ref<AssetDepreciationResponse | null>(null)
let deprLoadedForAssetId: string | null = null

const deprEntriesForBasis = computed<AssetDepreciationEntry[]>(() => {
  return (deprResp.value?.entries ?? []).filter(e => e.basis === deprBasis.value)
})

async function loadDepr() {
  const a = asset.value
  if (!a) return
  deprLoading.value = true
  deprError.value = false
  try {
    deprResp.value = await deprApi.assetSchedule(a.id)
    deprLoadedForAssetId = a.id
  } catch {
    deprError.value = true
    deprResp.value = null
  } finally {
    deprLoading.value = false
  }
}

function ensureDeprLoaded() {
  if (tab.value !== 'depr' || !asset.value) return
  if (deprLoadedForAssetId === asset.value.id) return
  loadDepr()
}

watch(tab, ensureDeprLoaded)

// ---------------------------------------------------------------------------
// Maintenance tab — fetched once on first activation (same lazy-load pattern
// as the depreciation tab above), re-fetched if the route swaps assets.
// ---------------------------------------------------------------------------
const maintLoading = ref(false)
const maintError = ref(false)
const maintRecords = ref<MaintenanceRecord[]>([])
let maintLoadedForAssetId: string | null = null

async function loadMaint() {
  const a = asset.value
  if (!a) return
  maintLoading.value = true
  maintError.value = false
  try {
    const res = await maintenanceApi.listByAsset(a.id)
    maintRecords.value = res.data
    maintLoadedForAssetId = a.id
  } catch {
    maintError.value = true
    maintRecords.value = []
  } finally {
    maintLoading.value = false
  }
}

function ensureMaintLoaded() {
  if (tab.value !== 'maint' || !asset.value) return
  if (maintLoadedForAssetId === asset.value.id) return
  loadMaint()
}

watch(tab, ensureMaintLoaded)

interface MaintRow { id: string, dateLabel: string, typeTone: BadgeColor, typeLabel: string, categoryLabel: string, statusTone: BadgeColor, statusLabel: string, costLabel: string, vendorLabel: string }
const maintRows = computed<MaintRow[]>(() => maintRecords.value.map(r => ({
  id: r.id,
  dateLabel: formatDateID(r.completed_date ?? r.scheduled_date) || '—',
  typeTone: MAINT_TYPE_TONE[r.type],
  typeLabel: t(`maintenance.type.${r.type}`),
  categoryLabel: r.category_name ?? '—',
  statusTone: MAINT_STATUS_TONE[r.status],
  statusLabel: t(`maintenance.status.${r.status}`),
  costLabel: formatRupiahMaint(r.cost),
  vendorLabel: r.vendor_name ?? '—'
})))

// ---------------------------------------------------------------------------
// Location + PIC history tabs — lazy-loaded on first activation (Fase 3).
// ---------------------------------------------------------------------------
const locLoading = ref(false)
const locError = ref(false)
const locRows = ref<AssetLocationHistory[]>([])
let locLoadedForAssetId: string | null = null

async function loadLoc() {
  const a = asset.value
  if (!a) return
  locLoading.value = true
  locError.value = false
  try {
    locRows.value = await assetsApi.locationHistory(a.id)
    locLoadedForAssetId = a.id
  } catch {
    locError.value = true
    locRows.value = []
  } finally {
    locLoading.value = false
  }
}
function ensureLocLoaded() {
  if (tab.value !== 'loc' || !asset.value) return
  if (locLoadedForAssetId === asset.value.id) return
  loadLoc()
}
watch(tab, ensureLocLoaded)

const picLoading = ref(false)
const picError = ref(false)
const picRows = ref<AssetPICHistory[]>([])
let picLoadedForAssetId: string | null = null

async function loadPic() {
  const a = asset.value
  if (!a) return
  picLoading.value = true
  picError.value = false
  try {
    picRows.value = await assetsApi.picHistory(a.id)
    picLoadedForAssetId = a.id
  } catch {
    picError.value = true
    picRows.value = []
  } finally {
    picLoading.value = false
  }
}
function ensurePicLoaded() {
  if (tab.value !== 'pic' || !asset.value) return
  if (picLoadedForAssetId === asset.value.id) return
  loadPic()
}
watch(tab, ensurePicLoaded)

function formatDateTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString(locale.value === 'en' ? 'en-US' : 'id-ID', { dateStyle: 'medium', timeStyle: 'short' })
}
function locationLabel(r: AssetLocationHistory): string {
  const parts = [r.office_name, r.floor_name, r.room_name].filter(Boolean)
  return parts.length ? parts.join(' / ') : '—'
}
function picEndLabel(r: AssetPICHistory): string {
  return r.released_at ? formatDateTime(r.released_at) : t('assets.detail.picActive')
}

const ringkas = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: t('assets.detail.fields.kategori'), value: name(a.category_id, categoryMap.value) },
    { label: t('assets.detail.fields.brandModel'), value: brandModelLabel() },
    { label: t('assets.detail.fields.kantor'), value: officeCache.get(a.office_id) },
    { label: t('assets.detail.fields.lokasi'), value: roomLabel.value },
    { label: t('assets.detail.fields.vendor'), value: name(a.vendor_id, vendorMap.value) }
  ]
})

interface InfoField { label: string, value: string, masked?: boolean }
const infoSections = computed<{ title: string, rows: InfoField[] }[]>(() => {
  const a = asset.value
  if (!a) return []
  const buy = moneyCell(a.purchase_cost)
  const accum = moneyCell(a.accumulated_depreciation)
  const book = moneyCell(a.book_value)
  return [
    { title: t('assets.detail.sections.identity'), rows: [
      { label: t('assets.detail.fields.kategori'), value: name(a.category_id, categoryMap.value) },
      { label: t('assets.detail.fields.brand'), value: brandCache.get(a.brand_id) },
      { label: t('assets.detail.fields.model'), value: modelCache.get(a.model_id) },
      { label: t('assets.detail.fields.serial'), value: a.serial_number || '—' },
      { label: t('assets.detail.fields.unit'), value: name(a.unit_id, unitMap.value) },
      { label: t('assets.detail.fields.assetClass'), value: t(classMeta[a.asset_class].labelKey) }
    ] },
    { title: t('assets.detail.sections.placement'), rows: [
      { label: t('assets.detail.fields.kantor'), value: officeCache.get(a.office_id) },
      { label: t('assets.detail.fields.lokasi'), value: roomLabel.value },
      { label: t('assets.detail.fields.holder'), value: '—' }
    ] },
    { title: t('assets.detail.sections.procurement'), rows: [
      { label: t('assets.detail.fields.vendor'), value: name(a.vendor_id, vendorMap.value) },
      { label: t('assets.detail.fields.buyDate'), value: formatDate(a.purchase_date) },
      { label: t('assets.detail.fields.poNumber'), value: a.po_number || '—' },
      { label: t('assets.detail.fields.fundingSource'), value: a.funding_source || '—' },
      { label: t('assets.detail.fields.warranty'), value: formatDate(a.warranty_expiry) },
      { label: t('assets.detail.fields.acquisitionBastNo'), value: a.acquisition_bast_no || '—' }
    ] },
    { title: t('assets.detail.sections.valuation'), rows: [
      { label: t('assets.detail.fields.buyPrice'), value: buy.text, masked: buy.masked },
      { label: t('assets.detail.fields.deprMethod'), value: deprMethodLabel(a.depreciation_method) },
      { label: t('assets.detail.fields.usefulLife'), value: usefulLifeLabel(a.useful_life_months) },
      { label: t('assets.detail.fields.accumDepr'), value: accum.text, masked: accum.masked },
      { label: t('assets.detail.fields.bookValue'), value: book.text, masked: book.masked },
      { label: t('assets.detail.fields.capitalized'), value: boolText(a.capitalized) },
      { label: t('assets.detail.fields.excludedFromValuation'), value: boolText(a.excluded_from_valuation) }
    ] },
    { title: t('assets.detail.sections.notes'), rows: [
      { label: t('assets.detail.fields.notes'), value: a.notes || '—' }
    ] }
  ]
})

const tabs = [
  { key: 'info', label: () => t('assets.detail.tabs.info') },
  { key: 'assign', label: () => t('assets.detail.tabs.assignment') },
  { key: 'maint', label: () => t('assets.detail.tabs.maintenance') },
  { key: 'depr', label: () => t('assets.detail.tabs.depreciation') },
  { key: 'loc', label: () => t('assets.detail.tabs.location') },
  { key: 'pic', label: () => t('assets.detail.tabs.pic') }
] as const

// Both dropdown actions submit maker-checker requests (POST /maintenance/reports
// and POST /requests), so each is gated behind request.create like the borrow
// button. The valuation-exception item is disabled once the asset is already
// excluded (the modal double-guards this).
const moreItems = computed(() => {
  if (!can('request.create')) return []
  return [
    [
      { label: t('assets.detail.requestMaintenance'), icon: 'i-lucide-wrench', onSelect: () => { maintOpen.value = true } },
      { label: t('assets.detail.requestValuationException'), icon: 'i-lucide-badge-dollar-sign', disabled: asset.value?.excluded_from_valuation === true, onSelect: () => { valexOpen.value = true } }
    ]
  ]
})

// ---------------------------------------------------------------------------
// Check-out / Ajukan Maintenance / Pengecualian Valuasi modals
// ---------------------------------------------------------------------------
const checkoutOpen = ref(false)
const maintOpen = ref(false)
const valexOpen = ref(false)

const valexAsset = computed(() => {
  const a = asset.value
  if (!a) return null
  return {
    id: a.id,
    name: a.name,
    asset_tag: a.asset_tag,
    office_id: a.office_id,
    excluded_from_valuation: a.excluded_from_valuation
  }
})

function onCheckoutSubmitted() {
  checkoutOpen.value = false
  // Check-out changes the asset's status (available -> assigned): reload the
  // whole detail so the badge, buttons, and tabs reflect the new state.
  load()
}

function onMaintSubmitted() {
  maintOpen.value = false
}

function onValexSubmitted() {
  valexOpen.value = false
}

// ---------------------------------------------------------------------------
// Ajukan Peminjaman (Task 13) — trigger + locked-asset modal
// ---------------------------------------------------------------------------
const borrowOpen = ref(false)

const borrowAsset = computed(() => {
  const a = asset.value
  if (!a) return null
  return {
    id: a.id,
    name: a.name,
    asset_tag: a.asset_tag,
    category: name(a.category_id, categoryMap.value),
    office: officeCache.get(a.office_id),
    location: roomLabel.value
  }
})

function onBorrowSubmitted() {
  borrowOpen.value = false
}

// Guards against a stale, out-of-order response overwriting a newer load
// (e.g. a fast route-param change re-triggers the fetch). `mine` is threaded
// into every downstream async helper (loadLookups/resolveRoom/loadGallery) so
// each bails after its own awaits if a newer load has since started.
let seq = 0

async function resolveRoom(a: Asset, mine: number) {
  if (!a.room_id) {
    if (mine === seq) roomLabel.value = '—'
    return
  }
  try {
    const floors = await floorsApi.listByOffice(a.office_id)
    if (mine !== seq) return
    // Each floor's room lookup is independent — one floor's rejection must
    // not abort resolution of a room that lives on another floor.
    const perFloor = await Promise.all(floors.map(async floor => ({ floor, rooms: await floorsApi.roomsByFloor(floor.id).catch(() => []) })))
    if (mine !== seq) return
    for (const { floor, rooms } of perFloor) {
      const room = rooms.find(r => r.id === a.room_id)
      if (room) {
        roomLabel.value = `${floor.name} — ${room.name}`
        return
      }
    }
    roomLabel.value = '—'
  } catch {
    if (mine === seq) roomLabel.value = '—'
  }
}

async function loadLookups(a: Asset, mine: number) {
  await Promise.all([
    categoriesApi.tree().then((cats) => { if (mine === seq) categoryMap.value = new Map(cats.map(c => [c.id, c.name])) }).catch(() => {}),
    referenceApi.list('vendors', { limit: 100 }).then((res) => { if (mine === seq) vendorMap.value = new Map(res.data.map(v => [v.id, v.name])) }).catch(() => {}),
    referenceApi.list('units', { limit: 100 }).then((res) => { if (mine === seq) unitMap.value = new Map(res.data.map(u => [u.id, u.name])) }).catch(() => {}),
    resolveRoom(a, mine)
  ])
}

function revokePhotos() {
  for (const p of photos.value) URL.revokeObjectURL(p.url)
}

async function loadGallery(assetId: string, mine: number) {
  revokePhotos()
  photos.value = []
  activeIndex.value = 0
  try {
    const res = await attachmentsApi.list(assetId)
    if (mine !== seq) return
    const imageRows = res.data.filter(r => r.kind === 'photo' && r.has_thumbnail)
    const loaded = await Promise.all(imageRows.map(async (row): Promise<Photo | null> => {
      try {
        const blob = await attachmentsApi.thumbnailBlob(assetId, row.id)
        return { id: row.id, url: URL.createObjectURL(blob) }
      } catch {
        return null
      }
    }))
    if (mine !== seq) {
      // A newer load has since taken over — discard these object URLs so
      // they don't leak, and leave the current (newer) photos.value alone.
      for (const p of loaded) {
        if (p) URL.revokeObjectURL(p.url)
      }
      return
    }
    photos.value = loaded.filter((p): p is Photo => p !== null)
  } catch {
    if (mine === seq) photos.value = []
  }
}

async function load() {
  const mine = ++seq
  loading.value = true
  loadError.value = false
  notFound.value = false
  try {
    const a = await assetsApi.getByTag(tag.value)
    if (mine !== seq) return
    asset.value = a
    loading.value = false
    deprLoadedForAssetId = null
    deprResp.value = null
    maintLoadedForAssetId = null
    maintRecords.value = []
    loadLookups(a, mine)
    loadGallery(a.id, mine)
    ensureDeprLoaded()
    ensureMaintLoaded()
  } catch (err) {
    if (mine !== seq) return
    const status = (err as { statusCode?: number } | undefined)?.statusCode
    if (status === 404) notFound.value = true
    else loadError.value = true
    loading.value = false
  }
}

onMounted(() => {
  load()
})

// Detail→detail navigation (e.g. via the global search palette) reuses this
// same route component — a plain onMounted() only fires once, so without
// this watcher the page would keep showing the previously-loaded asset.
// Guard against `undefined` on route-leave (params are cleared briefly
// while Vue Router tears the route down).
watch(() => route.params.tag, (newTag) => {
  if (!newTag) return
  load()
})

onUnmounted(() => {
  revokePhotos()
})
</script>

<template>
  <div>
    <div
      v-if="loading"
      class="flex items-center justify-center py-24"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-6 animate-spin text-muted"
      />
    </div>

    <div
      v-else-if="notFound"
      class="bg-default border border-default rounded-2xl shadow-sm py-16 px-6 text-center"
    >
      <div class="text-[17px] font-semibold mb-2">
        {{ t('assets.errNotFound') }}
      </div>
      <UButton
        :to="localePath('/assets')"
        color="neutral"
        variant="outline"
        icon="i-lucide-arrow-left"
        :label="t('assets.detail.backToCatalog')"
      />
    </div>

    <div
      v-else-if="loadError"
      class="bg-default border border-default rounded-[13px] shadow-sm flex flex-col items-center justify-center gap-3 py-16 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('common.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>

    <template v-else-if="asset">
      <!-- Header block -->
      <div class="flex items-start justify-between gap-5 flex-wrap mb-5">
        <div class="min-w-0">
          <div class="flex items-center gap-2.5 flex-wrap mb-1.5">
            <h1 class="text-2xl font-bold tracking-tight">
              {{ asset.name }}
            </h1>
            <AssetStatusBadge :status="asset.status" />
          </div>
          <div class="flex items-center gap-3">
            <span class="font-mono text-[13px] text-muted">{{ asset.asset_tag }}</span>
          </div>
        </div>
        <div class="flex items-center gap-2.5 flex-wrap">
          <span
            v-if="can('request.create')"
            :title="asset.status !== 'available' ? t('peminjaman.action.borrowDisabled') : undefined"
          >
            <UButton
              icon="i-lucide-hand"
              :label="t('peminjaman.action.borrow')"
              :disabled="asset.status !== 'available'"
              @click="() => { borrowOpen = true }"
            />
          </span>
          <UButton
            icon="i-lucide-pencil"
            :label="t('common.edit')"
            :to="localePath(`/assets/${asset.asset_tag}/edit`)"
          />
          <span
            v-if="can('assignment.manage')"
            :title="asset.status !== 'available' ? t('assets.detail.checkoutModal.disabledTip') : undefined"
          >
            <UButton
              icon="i-lucide-clipboard-check"
              color="neutral"
              variant="outline"
              :label="t('assets.detail.checkout')"
              :disabled="asset.status !== 'available'"
              data-testid="checkout-open"
              @click="() => { checkoutOpen = true }"
            />
          </span>
          <UButton
            icon="i-lucide-printer"
            color="neutral"
            variant="outline"
            :label="t('assets.detail.printLabel')"
            :to="localePath(`/assets/label?tags=${asset.asset_tag}`)"
          />
          <UDropdownMenu
            v-if="moreItems.length > 0"
            :items="moreItems"
            :content="{ align: 'end' }"
          >
            <UButton
              icon="i-lucide-ellipsis-vertical"
              color="neutral"
              variant="outline"
              square
              :aria-label="t('common.actions')"
            />
          </UDropdownMenu>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-5 items-start">
        <!-- Left: gallery + key info -->
        <div class="flex flex-col gap-4">
          <div class="bg-default border border-default rounded-[14px] p-3.5 shadow-sm">
            <div class="relative h-[200px] rounded-[11px] overflow-hidden bg-muted flex items-center justify-center [background-image:repeating-linear-gradient(45deg,var(--ui-bg-muted),var(--ui-bg-muted)_11px,var(--ui-bg-elevated)_11px,var(--ui-bg-elevated)_22px)]">
              <img
                v-if="photos.length"
                :src="photos[activeIndex]?.url"
                :alt="t('assets.detail.gallery')"
                class="absolute inset-0 w-full h-full object-cover"
              >
              <span
                v-else
                class="px-3 py-1.5 text-xs font-mono text-muted bg-default border border-default rounded-md"
              >
                {{ t('assets.detail.noPhotos') }}
              </span>
            </div>
            <div
              v-if="photos.length"
              class="flex gap-2 mt-2.5"
            >
              <button
                v-for="(p, i) in photos"
                :key="p.id"
                type="button"
                class="flex-1 h-[52px] rounded-[9px] overflow-hidden border-2 p-0 cursor-pointer"
                :class="i === activeIndex ? 'border-primary' : 'border-transparent'"
                @click="activeIndex = i"
              >
                <img
                  :src="p.url"
                  :alt="t('assets.detail.gallery')"
                  class="w-full h-full object-cover"
                >
              </button>
            </div>
          </div>
          <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
            <div class="px-4 py-3 border-b border-default text-[13px] font-semibold">
              {{ t('assets.detail.keyInfo') }}
            </div>
            <div class="px-4 pt-1.5 pb-3">
              <div
                v-for="(r, i) in ringkas"
                :key="i"
                class="flex items-center justify-between gap-3 py-2.5 border-b border-default last:border-b-0"
              >
                <span class="text-[13px] text-muted">{{ r.label }}</span>
                <span class="text-[13px] font-medium text-right">{{ r.value }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Right: tabs -->
        <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
          <div class="flex gap-0.5 px-2 border-b border-default overflow-x-auto">
            <button
              v-for="tb in tabs"
              :key="tb.key"
              type="button"
              class="px-3.5 py-3.5 -mb-px whitespace-nowrap text-[13.5px] border-b-2 cursor-pointer transition-colors"
              :class="tab === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
              @click="tab = tb.key"
            >
              {{ tb.label() }}
            </button>
          </div>

          <!-- Info tab -->
          <div
            v-if="tab === 'info'"
            class="p-5"
          >
            <div
              v-for="(sec, si) in infoSections"
              :key="si"
              class="mb-[22px] last:mb-0"
            >
              <div class="flex items-center gap-2 mb-2.5">
                <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ sec.title }}</span>
                <div class="flex-1 h-px bg-default" />
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-7 gap-y-2.5">
                <div
                  v-for="(f, fi) in sec.rows"
                  :key="fi"
                  class="flex flex-col gap-0.5"
                >
                  <span class="inline-flex items-center gap-1.5 text-xs text-muted">
                    {{ f.label }}
                    <UIcon
                      v-if="f.masked"
                      name="i-lucide-lock"
                      class="size-2.5 text-dimmed"
                    />
                  </span>
                  <span
                    class="text-sm font-medium"
                    :class="f.masked ? 'text-dimmed' : ''"
                    :title="f.masked ? t('assets.masked') : undefined"
                  >{{ f.value }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Depreciation tab -->
          <div
            v-else-if="tab === 'depr'"
            class="p-5"
          >
            <div
              v-if="deprLoading"
              class="flex items-center justify-center py-16"
            >
              <UIcon
                name="i-lucide-loader-circle"
                class="size-6 animate-spin text-muted"
              />
            </div>

            <div
              v-else-if="deprError"
              data-testid="depr-tab-error"
              class="flex flex-col items-center gap-3 py-16 text-center"
            >
              <UIcon
                name="i-lucide-circle-alert"
                class="size-6 text-muted"
              />
              <span class="text-sm text-muted">{{ t('common.loadError') }}</span>
              <UButton
                color="neutral"
                variant="subtle"
                @click="loadDepr"
              >
                {{ t('common.retry') }}
              </UButton>
            </div>

            <div
              v-else-if="deprResp?.masked"
              data-testid="depr-tab-masked"
              class="flex flex-col items-center gap-2.5 py-16 text-center"
            >
              <UIcon
                name="i-lucide-lock"
                class="size-6 text-dimmed"
              />
              <span class="text-sm text-muted">{{ t('depreciation.assetDetail.masked') }}</span>
            </div>

            <div
              v-else-if="!deprResp || deprResp.entries.length === 0"
              data-testid="depr-tab-empty"
              class="flex flex-col items-center gap-2.5 py-16 text-center"
            >
              <UIcon
                name="i-lucide-inbox"
                class="size-6 text-dimmed"
              />
              <span class="text-sm text-muted">{{ t('depreciation.assetDetail.empty') }}</span>
            </div>

            <div v-else>
              <div class="flex items-center justify-between gap-3 flex-wrap mb-3.5">
                <span class="text-[13px] font-semibold">{{ t('depreciation.assetDetail.title') }}</span>
                <div
                  class="flex gap-0.5 p-1 bg-muted rounded-[11px]"
                  data-testid="depr-tab-basis-toggle"
                >
                  <button
                    v-for="opt in DEPR_BASIS_OPTIONS"
                    :key="opt"
                    type="button"
                    class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
                    :class="deprBasis === opt ? 'bg-default shadow-sm text-default' : 'text-muted hover:text-default'"
                    :data-testid="`depr-tab-basis-${opt}`"
                    @click="deprBasis = opt"
                  >
                    {{ t(BASIS_META[opt].labelKey) }}
                  </button>
                </div>
              </div>

              <div
                v-if="deprEntriesForBasis.length === 0"
                data-testid="depr-tab-basis-empty"
                class="py-10 text-center text-sm text-dimmed"
              >
                {{ t('depreciation.assetDetail.empty') }}
              </div>
              <div
                v-else
                class="overflow-x-auto"
              >
                <table class="w-full border-collapse text-[13px] whitespace-nowrap">
                  <thead>
                    <tr class="bg-muted text-muted">
                      <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                        {{ t('depreciation.assetDetail.column.period') }}
                      </th>
                      <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                        {{ t('depreciation.assetDetail.column.opening') }}
                      </th>
                      <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                        {{ t('depreciation.assetDetail.column.expense') }}
                      </th>
                      <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                        {{ t('depreciation.assetDetail.column.closing') }}
                      </th>
                      <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                        {{ t('depreciation.assetDetail.column.method') }}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="(e, i) in deprEntriesForBasis"
                      :key="i"
                      data-testid="depr-tab-row"
                      class="border-t border-default"
                    >
                      <td class="px-4 py-3 font-medium">
                        {{ e.period }}
                      </td>
                      <td class="px-3 py-3 text-right tabular-nums text-muted">
                        {{ formatRupiah(e.opening) }}
                      </td>
                      <td class="px-3 py-3 text-right tabular-nums text-error">
                        {{ formatRupiah(e.amount) }}
                      </td>
                      <td class="px-3 py-3 text-right tabular-nums font-semibold">
                        {{ formatRupiah(e.closing) }}
                      </td>
                      <td class="px-4 py-3">
                        {{ deprMethodLabel(e.method) }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- Maintenance tab -->
          <div
            v-else-if="tab === 'maint'"
            class="p-5"
          >
            <div
              v-if="maintLoading"
              class="flex items-center justify-center py-16"
            >
              <UIcon
                name="i-lucide-loader-circle"
                class="size-6 animate-spin text-muted"
              />
            </div>

            <div
              v-else-if="maintError"
              data-testid="maint-tab-error"
              class="flex flex-col items-center gap-3 py-16 text-center"
            >
              <UIcon
                name="i-lucide-circle-alert"
                class="size-6 text-muted"
              />
              <span class="text-sm text-muted">{{ t('common.loadError') }}</span>
              <UButton
                color="neutral"
                variant="subtle"
                @click="loadMaint"
              >
                {{ t('common.retry') }}
              </UButton>
            </div>

            <div
              v-else-if="maintRows.length === 0"
              data-testid="maint-tab-empty"
              class="flex flex-col items-center gap-2.5 py-16 text-center"
            >
              <UIcon
                name="i-lucide-wrench"
                class="size-6 text-dimmed"
              />
              <span class="text-sm text-muted">{{ t('assets.detail.maintenanceEmpty') }}</span>
            </div>

            <div
              v-else
              class="overflow-x-auto"
            >
              <table class="w-full border-collapse text-[13px] whitespace-nowrap">
                <thead>
                  <tr class="bg-muted text-muted">
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colDate') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colType') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colCategory') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colStatus') }}
                    </th>
                    <th class="text-right px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colCost') }}
                    </th>
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('maintenance.records.colVendor') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in maintRows"
                    :key="r.id"
                    data-testid="maint-tab-row"
                    class="border-t border-default"
                  >
                    <td class="px-4 py-3 text-muted">
                      {{ r.dateLabel }}
                    </td>
                    <td class="px-3 py-3">
                      <UBadge
                        :color="r.typeTone"
                        variant="subtle"
                        class="rounded-full"
                      >
                        {{ r.typeLabel }}
                      </UBadge>
                    </td>
                    <td class="px-3 py-3">
                      {{ r.categoryLabel }}
                    </td>
                    <td class="px-3 py-3">
                      <UBadge
                        :color="r.statusTone"
                        variant="subtle"
                        class="rounded-full"
                      >
                        {{ r.statusLabel }}
                      </UBadge>
                    </td>
                    <td class="px-3 py-3 text-right tabular-nums">
                      {{ r.costLabel }}
                    </td>
                    <td class="px-4 py-3 text-muted">
                      {{ r.vendorLabel }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Location history tab (Fase 3) -->
          <div
            v-else-if="tab === 'loc'"
            class="p-5"
          >
            <div
              v-if="locLoading"
              class="flex items-center justify-center py-16"
            >
              <UIcon
                name="i-lucide-loader-circle"
                class="size-6 animate-spin text-muted"
              />
            </div>
            <div
              v-else-if="locError"
              data-testid="loc-tab-error"
              class="flex flex-col items-center gap-3 py-16 text-center"
            >
              <UIcon
                name="i-lucide-circle-alert"
                class="size-6 text-muted"
              />
              <span class="text-sm text-muted">{{ t('common.loadError') }}</span>
              <UButton
                color="neutral"
                variant="subtle"
                @click="loadLoc"
              >
                {{ t('common.retry') }}
              </UButton>
            </div>
            <div
              v-else-if="locRows.length === 0"
              data-testid="loc-tab-empty"
              class="flex flex-col items-center gap-2.5 py-16 text-center"
            >
              <UIcon
                name="i-lucide-map-pin"
                class="size-6 text-dimmed"
              />
              <span class="text-sm text-muted">{{ t('assets.detail.locationEmpty') }}</span>
            </div>
            <div
              v-else
              class="overflow-x-auto"
            >
              <table class="w-full border-collapse text-[13px] whitespace-nowrap">
                <thead>
                  <tr class="bg-muted text-muted">
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.locCols.date') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.locCols.location') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.locCols.source') }}
                    </th>
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.locCols.by') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in locRows"
                    :key="r.id"
                    data-testid="loc-tab-row"
                    class="border-t border-default"
                  >
                    <td class="px-4 py-3 text-muted">
                      {{ formatDateTime(r.moved_at) }}
                    </td>
                    <td class="px-3 py-3">
                      {{ locationLabel(r) }}
                    </td>
                    <td class="px-3 py-3">
                      <UBadge
                        color="neutral"
                        variant="subtle"
                        class="rounded-full"
                      >
                        {{ t(`assets.detail.locSource.${r.source}`) }}
                      </UBadge>
                    </td>
                    <td class="px-4 py-3 text-muted">
                      {{ r.moved_by_name || '—' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- PIC history tab (Fase 3) -->
          <div
            v-else-if="tab === 'pic'"
            class="p-5"
          >
            <div
              v-if="picLoading"
              class="flex items-center justify-center py-16"
            >
              <UIcon
                name="i-lucide-loader-circle"
                class="size-6 animate-spin text-muted"
              />
            </div>
            <div
              v-else-if="picError"
              data-testid="pic-tab-error"
              class="flex flex-col items-center gap-3 py-16 text-center"
            >
              <UIcon
                name="i-lucide-circle-alert"
                class="size-6 text-muted"
              />
              <span class="text-sm text-muted">{{ t('common.loadError') }}</span>
              <UButton
                color="neutral"
                variant="subtle"
                @click="loadPic"
              >
                {{ t('common.retry') }}
              </UButton>
            </div>
            <div
              v-else-if="picRows.length === 0"
              data-testid="pic-tab-empty"
              class="flex flex-col items-center gap-2.5 py-16 text-center"
            >
              <UIcon
                name="i-lucide-user-round"
                class="size-6 text-dimmed"
              />
              <span class="text-sm text-muted">{{ t('assets.detail.picEmpty') }}</span>
            </div>
            <div
              v-else
              class="overflow-x-auto"
            >
              <table class="w-full border-collapse text-[13px] whitespace-nowrap">
                <thead>
                  <tr class="bg-muted text-muted">
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.picCols.pic') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.picCols.from') }}
                    </th>
                    <th class="text-left px-3 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.picCols.to') }}
                    </th>
                    <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                      {{ t('assets.detail.picCols.by') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="r in picRows"
                    :key="r.id"
                    data-testid="pic-tab-row"
                    class="border-t border-default"
                  >
                    <td class="px-4 py-3">
                      {{ r.pic_name }} <span class="text-dimmed">({{ r.pic_code }})</span>
                    </td>
                    <td class="px-3 py-3 text-muted">
                      {{ formatDateTime(r.assigned_at) }}
                    </td>
                    <td class="px-3 py-3 text-muted">
                      {{ picEndLabel(r) }}
                    </td>
                    <td class="px-4 py-3 text-muted">
                      {{ r.assigned_by_name || '—' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Assignment tab — module not yet built (Phase: Assignment). -->
          <div
            v-else
            class="p-5"
          >
            <UCard>
              <div class="flex flex-col items-center gap-2.5 py-10 text-center">
                <UIcon
                  name="i-lucide-inbox"
                  class="size-6 text-dimmed"
                />
                <span class="text-sm text-muted">{{ t('assets.detail.moduleNotAvailable') }}</span>
              </div>
            </UCard>
          </div>
        </div>
      </div>
    </template>

    <AssignmentAjukanPeminjamanModal
      v-model:open="borrowOpen"
      :asset="borrowAsset"
      @submitted="onBorrowSubmitted"
    />

    <AssetCheckoutModal
      v-model:open="checkoutOpen"
      :asset="borrowAsset"
      @submitted="onCheckoutSubmitted"
    />

    <AssetRequestMaintenanceModal
      v-model:open="maintOpen"
      :asset="borrowAsset"
      @submitted="onMaintSubmitted"
    />

    <AssetValuationExceptionModal
      v-model:open="valexOpen"
      :asset="valexAsset"
      @submitted="onValexSubmitted"
    />
  </div>
</template>
