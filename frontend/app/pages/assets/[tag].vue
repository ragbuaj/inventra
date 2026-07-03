<script setup lang="ts">
import type { AssetCondition, MockAsset } from '~/mock/assets'
import { useAssets } from '~/composables/api/useAssets'
import {
  ASSET_CONDITION_TONE, MAINTENANCE_TYPE_TONE, MAINTENANCE_STATUS_TONE,
  sampleAssignments, sampleMaintenance, depreciationSchedule
} from '~/mock/assets'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const { t } = useI18n()
const route = useRoute()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useAssets()
const localePath = useLocalePath()

const tag = computed(() => String(route.params.tag))
const asset = ref<MockAsset | null>(null)
const loading = ref(true)
const tab = ref<'info' | 'assign' | 'maint' | 'depr'>('info')

function formatRp(v: number): string {
  return v ? `Rp ${v.toLocaleString('id-ID')}` : 'Rp 0'
}
function formatDate(tgl: string): string {
  const [y, m, day] = tgl.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}
function plusYears(tgl: string, years: number): string {
  const [y, m, day] = tgl.split('-')
  return formatDate(`${Number(y) + years}-${m}-${day}`)
}

const condition = computed<AssetCondition>(() => asset.value?.status === 'maintenance' ? 'Perlu Servis' : 'Baik')
const brandModel = computed(() => {
  const parts = (asset.value?.brand ?? '').split(' ')
  return { brand: parts[0] ?? '', model: parts.slice(1).join(' ') || (parts[0] ?? '') }
})

const ringkas = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: t('assets.detail.fields.kategori'), value: a.kategori },
    { label: t('assets.detail.fields.brandModel'), value: a.brand },
    { label: t('assets.detail.fields.kantor'), value: a.kantor },
    { label: t('assets.detail.fields.lokasi'), value: a.lokasi },
    { label: t('assets.detail.fields.vendor'), value: 'PT Sinar Komputindo' },
    { label: t('assets.detail.fields.kondisi'), value: t(condition.value === 'Baik' ? 'assets.detail.condition.baik' : 'assets.detail.condition.perluServis') }
  ]
})

interface InfoField { label: string, value: string, sensitive?: boolean }
const infoSections = computed<{ title: string, rows: InfoField[] }[]>(() => {
  const a = asset.value
  if (!a) return []
  return [
    { title: t('assets.detail.sections.identity'), rows: [
      { label: t('assets.detail.fields.kategori'), value: a.kategori },
      { label: t('assets.detail.fields.brand'), value: brandModel.value.brand },
      { label: t('assets.detail.fields.model'), value: brandModel.value.model },
      { label: t('assets.detail.fields.serial'), value: `SN-${a.tag.split('-').slice(-2).join('-')}` }
    ] },
    { title: t('assets.detail.sections.placement'), rows: [
      { label: t('assets.detail.fields.kantor'), value: a.kantor },
      { label: t('assets.detail.fields.lokasi'), value: a.lokasi },
      { label: t('assets.detail.fields.holder'), value: a.holder },
      { label: t('assets.detail.fields.kondisi'), value: t(condition.value === 'Baik' ? 'assets.detail.condition.baik' : 'assets.detail.condition.perluServis') }
    ] },
    { title: t('assets.detail.sections.procurement'), rows: [
      { label: t('assets.detail.fields.vendor'), value: 'PT Sinar Komputindo' },
      { label: t('assets.detail.fields.buyDate'), value: formatDate(a.tgl) },
      { label: t('assets.detail.fields.invoice'), value: `INV/${a.tgl.slice(0, 4)}/${a.tag.slice(-4)}` },
      { label: t('assets.detail.fields.warranty'), value: plusYears(a.tgl, 3) }
    ] },
    { title: t('assets.detail.sections.valuation'), rows: [
      { label: t('assets.detail.fields.buyPrice'), value: formatRp(a.harga), sensitive: true },
      { label: t('assets.detail.fields.deprMethod'), value: t('assets.detail.straightLine') },
      { label: t('assets.detail.fields.usefulLife'), value: t('assets.detail.years', { n: 4 }) },
      { label: t('assets.detail.fields.accumDepr'), value: formatRp(Math.max(0, a.harga - a.buku)), sensitive: true },
      { label: t('assets.detail.fields.bookValue'), value: formatRp(a.buku), sensitive: true }
    ] }
  ]
})

const depr = computed(() => asset.value ? depreciationSchedule(asset.value) : [])
const annualDepr = computed(() => asset.value ? formatRp(Math.round(asset.value.harga / 4)) : '')

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
  ],
  [
    { label: t('assets.detail.deleteAsset'), icon: 'i-lucide-trash-2', color: 'error' as const, onSelect: onDelete }
  ]
])

function comingSoon() {
  toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}
async function onDelete() {
  if (!asset.value) return
  const ok = await confirm({ title: t('assets.deleteTitle'), description: t('assets.deleteBody', { tag: asset.value.tag }) })
  if (!ok) return
  await api.remove(asset.value.tag)
  toast.add({ title: t('assets.detail.deletedToast'), color: 'success', icon: 'i-lucide-trash-2' })
  navigateTo(localePath('/assets'))
}

onMounted(async () => {
  loading.value = true
  asset.value = (await api.get(tag.value)) ?? null
  loading.value = false
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
      v-else-if="!asset"
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

    <template v-else>
      <!-- Header block -->
      <div class="flex items-start justify-between gap-5 flex-wrap mb-5">
        <div class="min-w-0">
          <div class="flex items-center gap-2.5 flex-wrap mb-1.5">
            <h1 class="text-2xl font-bold tracking-tight">
              {{ asset.nama }}
            </h1>
            <AssetStatusBadge :status="asset.status" />
          </div>
          <div class="flex items-center gap-3">
            <span class="font-mono text-[13px] text-muted">{{ asset.tag }}</span>
          </div>
        </div>
        <div class="flex items-center gap-2.5 flex-wrap">
          <UButton
            icon="i-lucide-pencil"
            :label="t('common.edit')"
            :to="localePath(`/assets/${asset.tag}/edit`)"
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
            :to="localePath(`/assets/label?tags=${asset.tag}`)"
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
              <span class="px-3 py-1.5 text-xs font-mono text-muted bg-default border border-default rounded-md">
                {{ t('assets.detail.gallery') }}
              </span>
            </div>
            <div class="flex gap-2 mt-2.5">
              <div
                v-for="n in 4"
                :key="n"
                class="flex-1 h-[52px] rounded-[9px] bg-muted border-2 border-transparent"
              />
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
                      v-if="f.sensitive"
                      name="i-lucide-lock"
                      class="size-2.5 text-dimmed"
                    />
                  </span>
                  <span class="text-sm font-medium">{{ f.value }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Assignment tab -->
          <div
            v-else-if="tab === 'assign'"
            class="overflow-x-auto"
          >
            <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
              <thead>
                <tr class="bg-muted text-muted">
                  <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.assignmentCols.holder') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.assignmentCols.from') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.assignmentCols.to') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.assignmentCols.cond') }}
                  </th>
                  <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.assignmentCols.note') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(a, i) in sampleAssignments"
                  :key="i"
                  class="border-t border-default hover:bg-muted"
                >
                  <td class="px-[18px] py-3">
                    <div class="flex items-center gap-2.5">
                      <span class="size-7 rounded-full bg-muted text-muted flex items-center justify-center text-[11px] font-semibold">{{ a.initials }}</span>
                      <span class="font-medium">{{ a.holder }}</span>
                    </div>
                  </td>
                  <td class="px-3.5 py-3 text-muted">
                    {{ a.from }}
                  </td>
                  <td
                    class="px-3.5 py-3"
                    :class="a.to ? 'text-muted' : 'text-primary font-medium'"
                  >
                    {{ a.to ?? t('assets.detail.assignmentCols.now') }}
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      :color="ASSET_CONDITION_TONE[a.cond]"
                      variant="subtle"
                      class="rounded-full"
                    >
                      {{ t(a.cond === 'Baik' ? 'assets.detail.condition.baik' : 'assets.detail.condition.perluServis') }}
                    </UBadge>
                  </td>
                  <td class="px-[18px] py-3 text-muted">
                    {{ a.note }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Maintenance tab -->
          <div
            v-else-if="tab === 'maint'"
            class="overflow-x-auto"
          >
            <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
              <thead>
                <tr class="bg-muted text-muted">
                  <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.maintenanceCols.date') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.maintenanceCols.type') }}
                  </th>
                  <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.maintenanceCols.status') }}
                  </th>
                  <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.maintenanceCols.cost') }}
                  </th>
                  <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase">
                    {{ t('assets.detail.maintenanceCols.vendor') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(m, i) in sampleMaintenance"
                  :key="i"
                  class="border-t border-default hover:bg-muted"
                >
                  <td class="px-[18px] py-3 font-medium">
                    {{ m.date }}
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      :color="MAINTENANCE_TYPE_TONE[m.type]"
                      variant="subtle"
                      class="rounded-full"
                    >
                      {{ t(`assets.detail.maintenanceType.${m.type}`) }}
                    </UBadge>
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      :color="MAINTENANCE_STATUS_TONE[m.status]"
                      variant="subtle"
                      class="rounded-full"
                    >
                      {{ t(`assets.detail.maintenanceStatus.${m.status}`) }}
                    </UBadge>
                  </td>
                  <td class="px-3.5 py-3 text-right tabular-nums">
                    {{ m.cost ? formatRp(m.cost) : '—' }}
                  </td>
                  <td class="px-[18px] py-3 text-muted">
                    {{ m.vendor }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Depreciation tab -->
          <div v-else>
            <div class="flex flex-wrap gap-[18px] px-5 py-4 border-b border-default bg-muted">
              <div>
                <div class="text-xs text-muted">
                  {{ t('assets.detail.deprMethodLabel') }}
                </div>
                <div class="text-sm font-semibold">
                  {{ t('assets.detail.straightLine') }}
                </div>
              </div>
              <div>
                <div class="text-xs text-muted">
                  {{ t('assets.detail.usefulLifeLabel') }}
                </div>
                <div class="text-sm font-semibold">
                  {{ t('assets.detail.years', { n: 4 }) }}
                </div>
              </div>
              <div>
                <div class="text-xs text-muted">
                  {{ t('assets.detail.annualLabel') }}
                </div>
                <div class="text-sm font-semibold">
                  {{ annualDepr }}
                </div>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
                <thead>
                  <tr class="bg-muted text-muted">
                    <th class="text-left px-[18px] py-[11px] text-xs font-semibold uppercase">
                      {{ t('assets.detail.depreciationCols.period') }}
                    </th>
                    <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase">
                      {{ t('assets.detail.depreciationCols.opening') }}
                    </th>
                    <th class="text-right px-3.5 py-[11px] text-xs font-semibold uppercase">
                      {{ t('assets.detail.depreciationCols.depreciation') }}
                    </th>
                    <th class="text-right px-[18px] py-[11px] text-xs font-semibold uppercase">
                      {{ t('assets.detail.depreciationCols.bookValue') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="d in depr"
                    :key="d.period"
                    class="border-t border-default"
                    :class="d.current ? 'bg-primary/5' : ''"
                  >
                    <td class="px-[18px] py-3 font-medium">
                      <span class="inline-flex items-center gap-2">
                        {{ d.period }}
                        <UBadge
                          v-if="d.current"
                          color="primary"
                          variant="subtle"
                          size="sm"
                          class="rounded-full"
                        >{{ t('assets.detail.depreciationCols.current') }}</UBadge>
                      </span>
                    </td>
                    <td class="px-3.5 py-3 text-right tabular-nums text-muted">
                      {{ formatRp(d.open) }}
                    </td>
                    <td class="px-3.5 py-3 text-right tabular-nums text-muted">
                      {{ formatRp(d.deprec) }}
                    </td>
                    <td class="px-[18px] py-3 text-right tabular-nums font-medium">
                      {{ formatRp(d.close) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
