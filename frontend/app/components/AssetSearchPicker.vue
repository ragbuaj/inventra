<script setup lang="ts">
import type { Asset, AssetStatus } from '~/types'

const props = withDefaults(defineProps<{
  statuses: AssetStatus[]
  placeholder: string
  hint?: string
  disabled?: boolean
  officeNames?: Map<string, string>
}>(), {
  hint: undefined,
  disabled: false,
  officeNames: () => new Map()
})

const emit = defineEmits<{
  select: [asset: Asset]
}>()

const DEBOUNCE_MS = 300

const { t } = useI18n()
const assetsApi = useAssets()

const query = ref('')
const results = ref<Asset[]>([])
const loading = ref(false)
const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
let seq = 0

async function runSearch(term: string) {
  const mine = ++seq
  loading.value = true
  try {
    const pages = await Promise.all(
      props.statuses.map(status => assetsApi.list({ search: term, status, limit: 20 }))
    )
    if (mine !== seq) return
    const merged = new Map<string, Asset>()
    for (const page of pages) {
      for (const asset of page.data) merged.set(asset.id, asset)
    }
    results.value = Array.from(merged.values())
  } catch {
    if (mine !== seq) return
    results.value = []
  } finally {
    if (mine === seq) loading.value = false
  }
}

watch(query, (value) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (props.disabled) return

  const term = value.trim()
  if (!term) {
    results.value = []
    loading.value = false
    isOpen.value = false
    return
  }

  isOpen.value = true
  debounceTimer = setTimeout(() => runSearch(term), DEBOUNCE_MS)
})

function officeLabel(asset: Asset): string {
  return props.officeNames.get(asset.office_id) ?? '—'
}

function select(asset: Asset) {
  query.value = asset.name
  isOpen.value = false
  results.value = []
  emit('select', asset)
}

function onOutsideClick(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('mousedown', onOutsideClick)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', onOutsideClick)
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>

<template>
  <div
    ref="containerRef"
    class="relative"
  >
    <UInput
      v-model="query"
      data-testid="asset-picker-input"
      :placeholder="placeholder"
      :disabled="disabled"
      icon="i-lucide-search"
      class="w-full"
    />
    <p
      v-if="hint"
      data-testid="asset-picker-hint"
      class="text-xs text-muted mt-1"
    >
      {{ hint }}
    </p>

    <div
      v-if="isOpen"
      class="absolute z-10 mt-1 w-full bg-default border border-default rounded-lg shadow-lg overflow-hidden"
    >
      <div
        v-if="loading"
        class="p-3 space-y-2"
      >
        <USkeleton
          v-for="n in 3"
          :key="n"
          class="h-[34px] w-full rounded-lg"
        />
      </div>

      <div
        v-else-if="results.length === 0"
        class="py-6 px-4 text-center text-xs text-muted"
      >
        {{ t('common.assetPickerEmpty') }}
      </div>

      <ul
        v-else
        class="max-h-[260px] overflow-y-auto py-1"
      >
        <li
          v-for="asset in results"
          :key="asset.id"
          data-testid="asset-picker-item"
          class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-muted"
          @click="select(asset)"
        >
          <span class="size-2 rounded-full bg-success shrink-0" />
          <span class="min-w-0 flex-1">
            <span class="block text-[13px] font-medium truncate">{{ asset.name }}</span>
            <span class="block text-[11px] text-dimmed truncate">{{ asset.asset_tag }} · {{ officeLabel(asset) }}</span>
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>
