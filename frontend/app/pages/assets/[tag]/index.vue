<script setup lang="ts">
import type { Asset } from '~/types'
import { classMeta } from '~/constants/assetMeta'

definePageMeta({ middleware: 'can', permission: 'asset.view' })

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const { t } = useI18n()
const route = useRoute()
const toast = useToast()
const localePath = useLocalePath()

const assetsApi = useAssets()
const attachmentsApi = useAssetAttachments()
const categoriesApi = useCategories()
const officesApi = useOffices()
const floorsApi = useFloors()
const referenceApi = useReference()

const tag = computed(() => String(route.params.tag))
const asset = ref<Asset | null>(null)
const loading = ref(true)
const loadError = ref(false)
const notFound = ref(false)
const tab = ref<'info' | 'assign' | 'maint' | 'depr'>('info')

// FK id → name maps, populated by loadLookups() once the asset itself is
// known. Missing/unresolved ids render as "—" (see `name()` below).
const categoryMap = ref(new Map<string, string>())
const officeMap = ref(new Map<string, string>())
const brandMap = ref(new Map<string, string>())
const modelMap = ref(new Map<string, string>())
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
  const brand = a.brand_id ? brandMap.value.get(a.brand_id) : undefined
  const model = a.model_id ? modelMap.value.get(a.model_id) : undefined
  const parts = [brand, model].filter((v): v is string => !!v)
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

const ringkas = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: t('assets.detail.fields.kategori'), value: name(a.category_id, categoryMap.value) },
    { label: t('assets.detail.fields.brandModel'), value: brandModelLabel() },
    { label: t('assets.detail.fields.kantor'), value: name(a.office_id, officeMap.value) },
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
      { label: t('assets.detail.fields.brand'), value: name(a.brand_id, brandMap.value) },
      { label: t('assets.detail.fields.model'), value: name(a.model_id, modelMap.value) },
      { label: t('assets.detail.fields.serial'), value: a.serial_number || '—' },
      { label: t('assets.detail.fields.unit'), value: name(a.unit_id, unitMap.value) },
      { label: t('assets.detail.fields.assetClass'), value: t(classMeta[a.asset_class].labelKey) }
    ] },
    { title: t('assets.detail.sections.placement'), rows: [
      { label: t('assets.detail.fields.kantor'), value: name(a.office_id, officeMap.value) },
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
  { key: 'depr', label: () => t('assets.detail.tabs.depreciation') }
] as const

const moreItems = computed(() => [
  [
    { label: t('assets.detail.requestMaintenance'), icon: 'i-lucide-wrench', onSelect: comingSoon },
    { label: t('assets.detail.requestValuationException'), icon: 'i-lucide-badge-dollar-sign', onSelect: comingSoon }
  ]
])

function comingSoon() {
  toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
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
    officesApi.list({ limit: 100 }).then((res) => { if (mine === seq) officeMap.value = new Map(res.data.map(o => [o.id, o.name])) }).catch(() => {}),
    referenceApi.list('brands', { limit: 100 }).then((res) => { if (mine === seq) brandMap.value = new Map(res.data.map(b => [b.id, b.name])) }).catch(() => {}),
    referenceApi.list('models', { limit: 100 }).then((res) => { if (mine === seq) modelMap.value = new Map(res.data.map(m => [m.id, m.name])) }).catch(() => {}),
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
    loadLookups(a, mine)
    loadGallery(a.id, mine)
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
          <UButton
            icon="i-lucide-pencil"
            :label="t('common.edit')"
            :to="localePath(`/assets/${asset.asset_tag}/edit`)"
          />
          <UButton
            icon="i-lucide-clipboard-check"
            color="neutral"
            variant="outline"
            :label="t('assets.detail.checkout')"
            @click="comingSoon"
          />
          <UButton
            icon="i-lucide-printer"
            color="neutral"
            variant="outline"
            :label="t('assets.detail.printLabel')"
            :to="localePath(`/assets/label?tags=${asset.asset_tag}`)"
          />
          <UDropdownMenu
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

          <!-- Assignment / Maintenance / Depreciation tabs — module not yet built (Phase: Assignment/Maintenance/Depreciation). -->
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
  </div>
</template>
