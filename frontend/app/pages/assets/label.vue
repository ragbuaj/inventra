<script setup lang="ts">
import { assetStore } from '~/mock/assets'

definePageMeta({ middleware: 'can', permission: 'asset.view' })

const { t } = useI18n()
const route = useRoute()
const toast = useToast()

const SIZES: Record<string, { w: number, h: number, qr: number, bar: number }> = {
  '50x30': { w: 196, h: 116, qr: 60, bar: 30 },
  '70x40': { w: 248, h: 146, qr: 74, bar: 36 },
  '100x50': { w: 320, h: 168, qr: 88, bar: 42 }
}

const all = assetStore.all().map(a => ({ tag: a.tag, nama: a.nama, kantor: a.kantor }))

const search = ref('')
const size = ref('70x40')
const cols = ref(3)
const mode = ref<'barcode' | 'qr' | 'both'>('both')
const fields = reactive({ nama: true, kode: true, kantor: true })

const initialTags = String(route.query.tags ?? '').split(',').map(s => s.trim()).filter(Boolean)
const selected = ref<Set<string>>(new Set(initialTags))

const sizeOptions = [
  { value: '50x30', label: '50 × 30 mm' },
  { value: '70x40', label: '70 × 40 mm' },
  { value: '100x50', label: '100 × 50 mm' }
]
const modeOptions = computed(() => [
  { value: 'barcode', label: t('assets.label.modeBarcode') },
  { value: 'qr', label: t('assets.label.modeQr') },
  { value: 'both', label: t('assets.label.modeBoth') }
])

const sz = computed(() => SIZES[size.value] ?? SIZES['70x40']!)
const showQr = computed(() => mode.value === 'qr' || mode.value === 'both')
const showBarcode = computed(() => mode.value === 'barcode' || mode.value === 'both')

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return all.filter(a => !q || a.nama.toLowerCase().includes(q) || a.tag.toLowerCase().includes(q))
})
const filteredTags = computed(() => filtered.value.map(a => a.tag))
const allChecked = computed(() => filteredTags.value.length > 0 && filteredTags.value.every(tag => selected.value.has(tag)))

const selectedLabels = computed(() => all.filter(a => selected.value.has(a.tag)))
const perPage = computed(() => {
  const rowsPer = Math.max(1, Math.floor(1040 / (sz.value.h + 12)))
  return cols.value * rowsPer
})

function toggle(tag: string) {
  const next = new Set(selected.value)
  if (next.has(tag)) next.delete(tag)
  else next.add(tag)
  selected.value = next
}
function toggleAll() {
  const next = new Set(selected.value)
  if (allChecked.value) filteredTags.value.forEach(tag => next.delete(tag))
  else filteredTags.value.forEach(tag => next.add(tag))
  selected.value = next
}
function comingSoon() {
  toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}
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
            <span class="text-[11.5px] text-muted">{{ t('assets.label.selected', { n: selected.size }) }}</span>
          </div>
          <div class="p-3 border-b border-default">
            <UInput
              v-model="search"
              icon="i-lucide-search"
              :placeholder="t('assets.label.searchPlaceholder')"
              class="w-full"
              size="sm"
            />
            <label class="flex items-center gap-2 mt-2.5 text-[12.5px] cursor-pointer">
              <UCheckbox
                :model-value="allChecked"
                @update:model-value="toggleAll"
              />
              {{ t('assets.label.selectAll') }}
            </label>
          </div>
          <div class="max-h-[280px] overflow-y-auto p-2">
            <label
              v-for="a in filtered"
              :key="a.tag"
              class="flex items-start gap-2.5 px-2 py-2 rounded-lg cursor-pointer hover:bg-muted"
            >
              <UCheckbox
                :model-value="selected.has(a.tag)"
                class="mt-0.5"
                @update:model-value="toggle(a.tag)"
              />
              <span class="min-w-0">
                <span class="block text-[12.5px] font-medium truncate">{{ a.nama }}</span>
                <span class="block text-[11px] font-mono text-dimmed truncate">{{ a.tag }}</span>
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
                @click="cols = n"
              >
                {{ n }}
              </UButton>
            </div>
          </div>
          <div>
            <div class="text-xs text-muted mb-1.5">
              {{ t('assets.label.show') }}
            </div>
            <div class="flex gap-0.5 p-0.5 bg-muted rounded-lg">
              <UButton
                v-for="m in modeOptions"
                :key="m.value"
                :color="mode === m.value ? 'primary' : 'neutral'"
                :variant="mode === m.value ? 'soft' : 'ghost'"
                size="sm"
                class="flex-1 justify-center"
                @click="mode = m.value as 'barcode' | 'qr' | 'both'"
              >
                {{ m.label }}
              </UButton>
            </div>
          </div>
          <div>
            <div class="text-xs text-muted mb-1.5">
              {{ t('assets.label.fields') }}
            </div>
            <div class="space-y-1.5">
              <label class="flex items-center gap-2 text-[12.5px] cursor-pointer">
                <UCheckbox v-model="fields.nama" /> {{ t('assets.label.fieldNama') }}
              </label>
              <label class="flex items-center gap-2 text-[12.5px] cursor-pointer">
                <UCheckbox v-model="fields.kode" /> {{ t('assets.label.fieldKode') }}
              </label>
              <label class="flex items-center gap-2 text-[12.5px] cursor-pointer">
                <UCheckbox v-model="fields.kantor" /> {{ t('assets.label.fieldKantor') }}
              </label>
            </div>
          </div>
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
              @click="comingSoon"
            />
            <UButton
              icon="i-lucide-printer"
              size="sm"
              :label="t('assets.label.print')"
              :disabled="selectedLabels.length === 0"
              @click="comingSoon"
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
              :key="lbl.tag"
              :tag="lbl.tag"
              :nama="lbl.nama"
              :kantor="lbl.kantor"
              :size="sz"
              :show-qr="showQr"
              :show-barcode="showBarcode"
              :fields="fields"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
