<script setup lang="ts">
import type { MockAsset } from '~/mock/assets'
import { useAssets } from '~/composables/api/useAssets'
import { ASSET_STATUS_KEYS, ASSET_CATEGORIES, ASSET_OFFICES, ASSET_LOCATIONS } from '~/mock/assets'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const PAGE_SIZE = 20
const ALL = '__all__'
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useAssets()
const localePath = useLocalePath()

const allRows = ref<MockAsset[]>([])
const loading = ref(true)

const search = ref('')
const fStatus = ref(ALL)
const fKat = ref(ALL)
const fKantor = ref(ALL)
const fLokasi = ref(ALL)
const dateFrom = ref('')
const dateTo = ref('')
const view = ref<'table' | 'grid'>('table')
const sortKey = ref<keyof MockAsset>('tag')
const sortDir = ref<'asc' | 'desc'>('asc')
const page = ref(1)
const selected = ref<Set<string>>(new Set())

// Price columns: shown by default (admin). Per-role/field masking is the backend
// field-permission concern (see spec) — the mockup's "preview role" widget is a demo control, omitted.
const showPrice = true

function formatRp(v: number): string {
  return v ? `Rp ${v.toLocaleString('id-ID')}` : '—'
}
function formatDate(tgl: string): string {
  const [y, m, day] = tgl.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}

const statusOptions = computed(() => [{ value: ALL, label: t('assets.filter.allStatus') }, ...ASSET_STATUS_KEYS.map(s => ({ value: s, label: t(`assets.status.${s}`) }))])
const katOptions = computed(() => [{ value: ALL, label: t('assets.filter.allCategory') }, ...ASSET_CATEGORIES.map(k => ({ value: k, label: k }))])
const kantorOptions = computed(() => [{ value: ALL, label: t('assets.filter.allOffice') }, ...ASSET_OFFICES.map(k => ({ value: k, label: k }))])
const lokasiOptions = computed(() => [{ value: ALL, label: t('assets.filter.allLocation') }, ...ASSET_LOCATIONS.map(k => ({ value: k, label: k }))])

const anyFilter = computed(() =>
  !!(search.value.trim() || fStatus.value !== ALL || fKat.value !== ALL || fKantor.value !== ALL || fLokasi.value !== ALL || dateFrom.value || dateTo.value)
)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return allRows.value.filter((r) => {
    if (q && !r.nama.toLowerCase().includes(q) && !r.tag.toLowerCase().includes(q) && !r.brand.toLowerCase().includes(q)) return false
    if (fStatus.value !== ALL && r.status !== fStatus.value) return false
    if (fKat.value !== ALL && r.kategori !== fKat.value) return false
    if (fKantor.value !== ALL && r.kantor !== fKantor.value) return false
    if (fLokasi.value !== ALL && r.lokasi !== fLokasi.value) return false
    if (dateFrom.value && r.tgl < dateFrom.value) return false
    if (dateTo.value && r.tgl > dateTo.value) return false
    return true
  })
})

const sorted = computed(() => {
  const key = sortKey.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...filtered.value].sort((a, b) => {
    const x = a[key]
    const y = b[key]
    if (typeof x === 'number' && typeof y === 'number') return (x - y) * dir
    return String(x).localeCompare(String(y), undefined, { numeric: true }) * dir
  })
})

const total = computed(() => sorted.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const pageRows = computed(() => {
  const p = Math.min(page.value, totalPages.value)
  const start = (p - 1) * PAGE_SIZE
  return sorted.value.slice(start, start + PAGE_SIZE)
})
const pageInfo = computed(() => {
  const p = Math.min(page.value, totalPages.value)
  const from = total.value === 0 ? 0 : (p - 1) * PAGE_SIZE + 1
  const to = Math.min(p * PAGE_SIZE, total.value)
  return t('assets.showing', { from, to, total: total.value })
})

const pageTags = computed(() => pageRows.value.map(r => r.tag))
const allChecked = computed(() => pageTags.value.length > 0 && pageTags.value.every(tag => selected.value.has(tag)))
const selectionCount = computed(() => selected.value.size)

function setSort(key: keyof MockAsset) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}
function sortIcon(key: keyof MockAsset): string | undefined {
  if (sortKey.value !== key) return undefined
  return sortDir.value === 'asc' ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'
}
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
  fStatus.value = ALL
  fKat.value = ALL
  fKantor.value = ALL
  fLokasi.value = ALL
  dateFrom.value = ''
  dateTo.value = ''
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

async function onDelete(asset: MockAsset) {
  const ok = await confirm({
    title: t('assets.deleteTitle'),
    description: t('assets.deleteBody', { tag: asset.tag })
  })
  if (!ok) return
  await api.remove(asset.tag)
  toggle(asset.tag) // drop from selection if present
  selected.value.delete(asset.tag)
  await refresh()
}

async function refresh() {
  const res = await api.list({ limit: 100 })
  allRows.value = res.data
}

watch([search, fStatus, fKat, fKantor, fLokasi, dateFrom, dateTo], () => {
  page.value = 1
})

onMounted(async () => {
  loading.value = true
  await refresh()
  loading.value = false
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
        v-model="fLokasi"
        :items="lokasiOptions"
        class="min-w-[150px]"
      />
      <div class="flex items-center gap-1.5 ps-2 border-s border-default">
        <UInput
          v-model="dateFrom"
          type="date"
          :aria-label="t('assets.dateFrom')"
        />
        <span class="text-dimmed">–</span>
        <UInput
          v-model="dateTo"
          type="date"
          :aria-label="t('assets.dateTo')"
        />
      </div>
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
                  { key: 'tag', label: t('assets.columns.tag'), sortable: true },
                  { key: 'nama', label: t('assets.columns.nama'), sortable: true },
                  { key: 'kategori', label: t('assets.columns.kategori'), sortable: true },
                  { key: 'brand', label: t('assets.columns.brand'), sortable: false },
                  { key: 'status', label: t('assets.columns.status'), sortable: true },
                  { key: 'kantor', label: t('assets.columns.kantor'), sortable: false },
                  { key: 'holder', label: t('assets.columns.holder'), sortable: false },
                  { key: 'tgl', label: t('assets.columns.date'), sortable: true }
                ]"
                :key="col.key"
                class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide"
                :class="col.sortable ? 'cursor-pointer select-none' : ''"
                @click="col.sortable && setSort(col.key as keyof MockAsset)"
              >
                <span class="inline-flex items-center gap-1.5">
                  {{ col.label }}
                  <UIcon
                    v-if="col.sortable && sortIcon(col.key as keyof MockAsset)"
                    :name="sortIcon(col.key as keyof MockAsset)!"
                    class="size-3.5 text-primary"
                  />
                </span>
              </th>
              <template v-if="showPrice">
                <th
                  class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide cursor-pointer select-none"
                  @click="setSort('harga')"
                >
                  <span class="inline-flex items-center gap-1.5">
                    {{ t('assets.columns.harga') }}
                    <UIcon
                      v-if="sortIcon('harga')"
                      :name="sortIcon('harga')!"
                      class="size-3.5 text-primary"
                    />
                  </span>
                </th>
                <th
                  class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide cursor-pointer select-none"
                  @click="setSort('buku')"
                >
                  <span class="inline-flex items-center gap-1.5">
                    {{ t('assets.columns.buku') }}
                    <UIcon
                      v-if="sortIcon('buku')"
                      :name="sortIcon('buku')!"
                      class="size-3.5 text-primary"
                    />
                  </span>
                </th>
              </template>
              <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                {{ t('assets.columns.aksi') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="r in pageRows"
              :key="r.tag"
              class="border-t border-default hover:bg-muted transition-colors"
              :class="selected.has(r.tag) ? 'bg-primary/5' : ''"
            >
              <td class="px-3.5 py-3">
                <UCheckbox
                  :model-value="selected.has(r.tag)"
                  @update:model-value="toggle(r.tag)"
                />
              </td>
              <td class="px-3.5 py-3 font-mono text-[12.5px] text-muted">
                <NuxtLink
                  :to="localePath(`/assets/${r.tag}`)"
                  class="hover:text-primary"
                >
                  {{ r.tag }}
                </NuxtLink>
              </td>
              <td class="px-3.5 py-3 font-medium">
                {{ r.nama }}
              </td>
              <td class="px-3.5 py-3">
                <UBadge
                  color="neutral"
                  variant="subtle"
                  class="rounded-full"
                >
                  {{ r.kategori }}
                </UBadge>
              </td>
              <td class="px-3.5 py-3 text-muted">
                {{ r.brand }}
              </td>
              <td class="px-3.5 py-3">
                <AssetStatusBadge :status="r.status" />
              </td>
              <td class="px-3.5 py-3 text-muted">
                {{ r.kantor }}
              </td>
              <td
                class="px-3.5 py-3"
                :class="r.holder === '—' ? 'text-dimmed' : ''"
              >
                {{ r.holder }}
              </td>
              <td class="px-3.5 py-3 text-muted">
                {{ formatDate(r.tgl) }}
              </td>
              <template v-if="showPrice">
                <td class="px-3.5 py-3 text-right tabular-nums">
                  {{ formatRp(r.harga) }}
                </td>
                <td class="px-3.5 py-3 text-right tabular-nums text-muted">
                  {{ formatRp(r.buku) }}
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
                    @click="openDetail(r.tag)"
                  />
                  <UButton
                    icon="i-lucide-pencil"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('common.edit')"
                    @click="openEdit(r.tag)"
                  />
                  <UButton
                    icon="i-lucide-printer"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('assets.printLabels')"
                    @click="openLabel([r.tag])"
                  />
                  <UButton
                    icon="i-lucide-trash-2"
                    color="error"
                    variant="ghost"
                    size="xs"
                    :aria-label="t('common.delete')"
                    @click="onDelete(r)"
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
          v-for="r in pageRows"
          :key="r.tag"
          :asset="r"
          :selected="selected.has(r.tag)"
          :show-price="showPrice"
          :format-date="formatDate"
          :format-rp="formatRp"
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
