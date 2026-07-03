<script setup lang="ts">
import type { Asset, AssetClass, AssetStatus } from '~/types'
import type { CatalogCardAsset } from '~/components/asset/AssetCard.vue'
import { ASSET_CLASSES, ASSET_STATUSES, classMeta, statusMeta } from '~/constants/assetMeta'

definePageMeta({ middleware: 'can', permission: 'asset.view' })

const PAGE_SIZE = 20
const ALL = '__all__'
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const { t } = useI18n()
const toast = useToast()
const localePath = useLocalePath()
const assetsApi = useAssets()
const categoriesApi = useCategories()
const officesApi = useOffices()

const rows = ref<Asset[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const loadError = ref(false)

const search = ref('')
const debouncedSearch = ref('')
const fStatus = ref<string>(ALL)
const fKat = ref<string>(ALL)
const fKantor = ref<string>(ALL)
const fClass = ref<string>(ALL)
const view = ref<'table' | 'grid'>('table')
const selected = ref<Set<string>>(new Set())

// Price columns: shown by default (admin). Per-role/per-field masking is the
// backend field-permission concern — a row's purchase_cost/book_value simply
// comes back absent when the caller can't view it (see moneyCell below).
const showPrice = true

// Filter option lists + id→name maps (categories via useCategories().tree(),
// offices via the scoped useOffices().list()).
const categoryOptions = ref<{ value: string, label: string }[]>([])
const officeOptions = ref<{ value: string, label: string }[]>([])
const categoryMap = computed(() => new Map(categoryOptions.value.map(o => [o.value, o.label])))
const officeMap = computed(() => new Map(officeOptions.value.map(o => [o.value, o.label])))
function categoryName(id: string): string {
  return categoryMap.value.get(id) ?? '—'
}
function officeName(id: string): string {
  return officeMap.value.get(id) ?? '—'
}

interface MoneyCell { text: string, masked: boolean }
function moneyCell(v: string | null | undefined): MoneyCell {
  if (v === undefined) return { text: '—', masked: true }
  if (v === null) return { text: '—', masked: false }
  const n = Number(v)
  return { text: Number.isFinite(n) ? `Rp ${n.toLocaleString('id-ID')}` : '—', masked: false }
}
function formatDate(d: string | null | undefined): string {
  if (!d) return '—'
  const [y, m, day] = d.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}

const statusOptions = computed(() => [
  { value: ALL, label: t('assets.filter.allStatus') },
  ...ASSET_STATUSES.map(s => ({ value: s, label: t(statusMeta[s].labelKey) }))
])
const katOptions = computed(() => [{ value: ALL, label: t('assets.filter.allCategory') }, ...categoryOptions.value])
const kantorOptions = computed(() => [{ value: ALL, label: t('assets.filter.allOffice') }, ...officeOptions.value])
const classOptions = computed(() => [
  { value: ALL, label: t('assets.filter.allClass') },
  ...ASSET_CLASSES.map(c => ({ value: c, label: t(classMeta[c].labelKey) }))
])

const anyFilter = computed(() =>
  !!(search.value.trim() || fStatus.value !== ALL || fKat.value !== ALL || fKantor.value !== ALL || fClass.value !== ALL)
)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const pageInfo = computed(() => {
  const from = total.value === 0 ? 0 : (page.value - 1) * PAGE_SIZE + 1
  const to = Math.min(page.value * PAGE_SIZE, total.value)
  return t('assets.showing', { from, to, total: total.value })
})

const pageTags = computed(() => rows.value.map(r => r.asset_tag))
const allChecked = computed(() => pageTags.value.length > 0 && pageTags.value.every(tag => selected.value.has(tag)))
const selectionCount = computed(() => selected.value.size)

// Grid-view cards: resolved lookups + formatted labels, decoupled from the
// raw Asset shape (see AssetCard's CatalogCardAsset).
const cardRows = computed<CatalogCardAsset[]>(() => rows.value.map((r) => {
  const money = moneyCell(r.purchase_cost)
  return {
    tag: r.asset_tag,
    nama: r.name,
    kategori: categoryName(r.category_id),
    kantor: officeName(r.office_id),
    status: r.status,
    holder: '—',
    tglLabel: formatDate(r.purchase_date),
    hargaLabel: money.text,
    hargaMasked: money.masked
  }
}))

function toggle(tag: string) {
  const next = new Set(selected.value)
  if (next.has(tag)) next.delete(tag)
  else next.add(tag)
  selected.value = next
}
function toggleAll() {
  const next = new Set(selected.value)
  if (allChecked.value) pageTags.value.forEach(tag => next.delete(tag))
  else pageTags.value.forEach(tag => next.add(tag))
  selected.value = next
}
function clearSelection() {
  selected.value = new Set()
}
function resetFilters() {
  search.value = ''
  debouncedSearch.value = ''
  fStatus.value = ALL
  fKat.value = ALL
  fKantor.value = ALL
  fClass.value = ALL
  page.value = 1
}

function openDetail(tag: string) {
  navigateTo(localePath(`/assets/${tag}`))
}
function openEdit(tag: string) {
  navigateTo(localePath(`/assets/${tag}/edit`))
}
function openLabel(tags: string[]) {
  navigateTo(localePath(`/assets/label?tags=${tags.join(',')}`))
}
function comingSoon() {
  toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const res = await assetsApi.list({
      limit: PAGE_SIZE,
      offset: (page.value - 1) * PAGE_SIZE,
      search: debouncedSearch.value.trim() || undefined,
      status: fStatus.value !== ALL ? (fStatus.value as AssetStatus) : undefined,
      category_id: fKat.value !== ALL ? fKat.value : undefined,
      office_id: fKantor.value !== ALL ? fKantor.value : undefined,
      asset_class: fClass.value !== ALL ? (fClass.value as AssetClass) : undefined
    })
    rows.value = res.data
    total.value = res.total
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

async function loadFilterOptions() {
  const [categories, offices] = await Promise.all([
    categoriesApi.tree(),
    officesApi.list({ limit: 100 })
  ])
  categoryOptions.value = categories.map(c => ({ value: c.id, label: c.name }))
  officeOptions.value = offices.data.map(o => ({ value: o.id, label: o.name }))
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, (v) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    debouncedSearch.value = v
  }, 300)
})

watch([debouncedSearch, fStatus, fKat, fKantor, fClass], () => {
  page.value = 1
  load()
})
watch(page, () => load())

onMounted(() => {
  load()
  loadFilterOptions()
})
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-[18px]">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('assets.title') }}
        </h1>
        <p class="text-sm text-muted">
          {{ t('assets.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-2.5 flex-wrap">
        <UButton
          icon="i-lucide-scan-barcode"
          color="neutral"
          variant="outline"
          :label="t('assets.scan')"
          @click="comingSoon"
        />
        <UButton
          icon="i-lucide-upload"
          color="neutral"
          variant="outline"
          :label="t('assets.import')"
          :to="localePath('/assets/import')"
        />
        <UButton
          icon="i-lucide-plus"
          :label="t('assets.add')"
          :to="localePath('/assets/new')"
        />
      </div>
    </div>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] p-[14px] shadow-sm mb-4 flex items-center gap-2.5 flex-wrap">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('assets.searchPlaceholder')"
        class="flex-1 min-w-[220px]"
      />
      <USelect
        v-model="fStatus"
        :items="statusOptions"
        class="min-w-[140px]"
      />
      <USelect
        v-model="fKat"
        :items="katOptions"
        class="min-w-[150px]"
      />
      <USelect
        v-model="fKantor"
        :items="kantorOptions"
        class="min-w-[160px]"
      />
      <USelect
        v-model="fClass"
        :items="classOptions"
        class="min-w-[150px]"
      />
      <UButton
        v-if="anyFilter"
        color="error"
        variant="ghost"
        icon="i-lucide-x"
        :label="t('assets.reset')"
        @click="resetFilters"
      />
      <div class="flex-1" />
      <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg flex-none">
        <UButton
          icon="i-lucide-table"
          :color="view === 'table' ? 'primary' : 'neutral'"
          :variant="view === 'table' ? 'soft' : 'ghost'"
          size="sm"
          square
          :aria-label="t('assets.viewTable')"
          @click="view = 'table'"
        />
        <UButton
          icon="i-lucide-layout-grid"
          :color="view === 'grid' ? 'primary' : 'neutral'"
          :variant="view === 'grid' ? 'soft' : 'ghost'"
          size="sm"
          square
          :aria-label="t('assets.viewGrid')"
          @click="view = 'grid'"
        />
      </div>
    </div>

    <!-- Bulk bar -->
    <div
      v-if="selectionCount > 0"
      class="flex items-center gap-3 px-4 py-[11px] mb-3.5 bg-primary/10 border border-primary/30 rounded-[11px]"
    >
      <span class="text-[13.5px] font-semibold text-primary">{{ t('assets.selected', { n: selectionCount }) }}</span>
      <div class="flex-1" />
      <UButton
        icon="i-lucide-printer"
        size="sm"
        :label="t('assets.printLabels')"
        @click="openLabel([...selected])"
      />
      <UButton
        icon="i-lucide-download"
        color="neutral"
        variant="outline"
        size="sm"
        :label="t('assets.export')"
        @click="comingSoon"
      />
      <UButton
        icon="i-lucide-x"
        color="neutral"
        variant="ghost"
        size="sm"
        square
        :aria-label="t('common.cancel')"
        @click="clearSelection"
      />
    </div>

    <!-- Loading -->
    <div
      v-if="loading"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <USkeleton class="h-[42px] w-full rounded-none" />
      <div
        v-for="n in 8"
        :key="n"
        class="flex items-center gap-4 px-[18px] py-3.5 border-t border-default"
      >
        <USkeleton class="size-4 rounded" />
        <USkeleton class="h-3 w-[130px] rounded" />
        <USkeleton class="h-3 flex-1 rounded" />
        <USkeleton class="h-5 w-[84px] rounded-full" />
        <USkeleton class="h-3 w-[90px] rounded" />
      </div>
    </div>

    <!-- Load error -->
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

    <!-- Empty -->
    <div
      v-else-if="total === 0"
      class="bg-default border border-default rounded-2xl shadow-sm py-[60px] px-6 text-center"
    >
      <div class="size-[60px] mx-auto mb-4 rounded-2xl bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-package"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ anyFilter ? t('assets.emptyFilter') : t('assets.emptyNoData') }}
      </div>
      <div class="text-sm text-muted max-w-[340px] mx-auto mb-[18px]">
        {{ anyFilter ? t('assets.emptyFilterSub') : t('assets.emptyNoDataSub') }}
      </div>
      <UButton
        v-if="anyFilter"
        color="neutral"
        variant="outline"
        :label="t('assets.reset')"
        @click="resetFilters"
      />
      <UButton
        v-else
        icon="i-lucide-plus"
        :label="t('assets.add')"
        :to="localePath('/assets/new')"
      />
    </div>

    <!-- Table view -->
    <div
      v-else-if="view === 'table'"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
          <thead>
            <tr class="bg-muted text-muted">
              <th class="px-3.5 py-[11px] w-[42px]">
                <UCheckbox
                  :model-value="allChecked"
                  @update:model-value="toggleAll"
                />
              </th>
              <th
                v-for="col in [
                  { key: 'tag', label: t('assets.columns.tag') },
                  { key: 'nama', label: t('assets.columns.nama') },
                  { key: 'kategori', label: t('assets.columns.kategori') },
                  { key: 'brand', label: t('assets.columns.brand') },
                  { key: 'status', label: t('assets.columns.status') },
                  { key: 'kantor', label: t('assets.columns.kantor') },
                  { key: 'holder', label: t('assets.columns.holder') },
                  { key: 'tgl', label: t('assets.columns.date') }
                ]"
                :key="col.key"
                class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide"
              >
                {{ col.label }}
              </th>
              <template v-if="showPrice">
                <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assets.columns.harga') }}
                </th>
                <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assets.columns.buku') }}
                </th>
              </template>
              <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                {{ t('assets.columns.aksi') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="r in rows"
              :key="r.asset_tag"
              class="border-t border-default hover:bg-muted transition-colors"
              :class="selected.has(r.asset_tag) ? 'bg-primary/5' : ''"
            >
              <td class="px-3.5 py-3">
                <UCheckbox
                  :model-value="selected.has(r.asset_tag)"
                  @update:model-value="toggle(r.asset_tag)"
                />
              </td>
              <td class="px-3.5 py-3 font-mono text-[12.5px] text-muted">
                <NuxtLink
                  :to="localePath(`/assets/${r.asset_tag}`)"
                  class="hover:text-primary"
                >
                  {{ r.asset_tag }}
                </NuxtLink>
              </td>
              <td class="px-3.5 py-3 font-medium">
                {{ r.name }}
              </td>
              <td class="px-3.5 py-3">
                <UBadge
                  color="neutral"
                  variant="subtle"
                  class="rounded-full"
                >
                  {{ categoryName(r.category_id) }}
                </UBadge>
              </td>
              <td class="px-3.5 py-3 text-muted">
                —
              </td>
              <td class="px-3.5 py-3">
                <AssetStatusBadge :status="r.status" />
              </td>
              <td class="px-3.5 py-3 text-muted">
                {{ officeName(r.office_id) }}
              </td>
              <td class="px-3.5 py-3 text-dimmed">
                —
              </td>
              <td class="px-3.5 py-3 text-muted">
                {{ formatDate(r.purchase_date) }}
              </td>
              <template v-if="showPrice">
                <td class="px-3.5 py-3 text-right tabular-nums">
                  <span
                    v-if="moneyCell(r.purchase_cost).masked"
                    class="inline-flex items-center gap-1 text-dimmed justify-end"
                    :title="t('assets.masked')"
                  >
                    {{ moneyCell(r.purchase_cost).text }}
                    <UIcon
                      name="i-lucide-lock"
                      class="size-3"
                    />
                  </span>
                  <template v-else>
                    {{ moneyCell(r.purchase_cost).text }}
                  </template>
                </td>
                <td class="px-3.5 py-3 text-right tabular-nums text-muted">
                  <span
                    v-if="moneyCell(r.book_value).masked"
                    class="inline-flex items-center gap-1 text-dimmed justify-end"
                    :title="t('assets.masked')"
                  >
                    {{ moneyCell(r.book_value).text }}
                    <UIcon
                      name="i-lucide-lock"
                      class="size-3"
                    />
                  </span>
                  <template v-else>
                    {{ moneyCell(r.book_value).text }}
                  </template>
                </td>
              </template>
              <td class="px-3.5 py-3 text-right">
                <div class="inline-flex gap-0.5">
                  <UButton
                    icon="i-lucide-eye"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('common.view', 'Lihat')"
                    @click="openDetail(r.asset_tag)"
                  />
                  <UButton
                    icon="i-lucide-pencil"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('common.edit')"
                    @click="openEdit(r.asset_tag)"
                  />
                  <UButton
                    icon="i-lucide-printer"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('assets.printLabels')"
                    @click="openLabel([r.asset_tag])"
                  />
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex items-center justify-between flex-wrap gap-2.5 px-4 py-3 border-t border-default">
        <span class="text-[13px] text-muted">{{ pageInfo }}</span>
        <div class="flex items-center gap-1.5">
          <UButton
            icon="i-lucide-chevron-left"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page <= 1"
            :aria-label="t('common.actions')"
            @click="page = Math.max(1, page - 1)"
          />
          <UButton
            v-for="p in totalPages"
            :key="p"
            :color="p === Math.min(page, totalPages) ? 'primary' : 'neutral'"
            :variant="p === Math.min(page, totalPages) ? 'solid' : 'outline'"
            size="sm"
            class="min-w-[34px] justify-center"
            @click="page = p"
          >
            {{ p }}
          </UButton>
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page >= totalPages"
            :aria-label="t('common.actions')"
            @click="page = Math.min(totalPages, page + 1)"
          />
        </div>
      </div>
    </div>

    <!-- Grid view -->
    <div v-else>
      <div class="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(280px,1fr))]">
        <AssetCard
          v-for="r in cardRows"
          :key="r.tag"
          :asset="r"
          :selected="selected.has(r.tag)"
          :show-price="showPrice"
          @toggle="toggle(r.tag)"
          @open="openDetail(r.tag)"
        />
      </div>
      <div class="flex items-center justify-between flex-wrap gap-2.5 mt-4">
        <span class="text-[13px] text-muted">{{ pageInfo }}</span>
        <div class="flex items-center gap-1.5">
          <UButton
            icon="i-lucide-chevron-left"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page <= 1"
            :aria-label="t('common.actions')"
            @click="page = Math.max(1, page - 1)"
          />
          <UButton
            v-for="p in totalPages"
            :key="p"
            :color="p === Math.min(page, totalPages) ? 'primary' : 'neutral'"
            :variant="p === Math.min(page, totalPages) ? 'solid' : 'outline'"
            size="sm"
            class="min-w-[34px] justify-center"
            @click="page = p"
          >
            {{ p }}
          </UButton>
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page >= totalPages"
            :aria-label="t('common.actions')"
            @click="page = Math.min(totalPages, page + 1)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
