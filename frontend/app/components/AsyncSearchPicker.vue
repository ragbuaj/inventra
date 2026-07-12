<script setup lang="ts">
import type { PickerItem } from '~/types'

const props = withDefaults(defineProps<{
  modelValue: string | null
  searchFn: (term: string) => Promise<PickerItem[]>
  resolveFn?: (id: string) => Promise<PickerItem | null>
  placeholder: string
  disabled?: boolean
  testid?: string
}>(), {
  resolveFn: undefined,
  disabled: false,
  testid: 'async'
})

const emit = defineEmits<{ 'update:modelValue': [id: string | null] }>()

const DEBOUNCE_MS = 300
const { t } = useI18n()

const query = ref('')
const results = ref<PickerItem[]>([])
const loading = ref(false)
const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
let seq = 0
let suppressNextSearch = false

async function runSearch(term: string) {
  const mine = ++seq
  loading.value = true
  try {
    const found = await props.searchFn(term)
    if (mine !== seq) return
    results.value = found
  } catch {
    if (mine === seq) results.value = []
  } finally {
    if (mine === seq) loading.value = false
  }
}

watch(query, (value) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (suppressNextSearch) {
    suppressNextSearch = false
    return
  }
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

// Resolve a preselected id into its display label (may be outside the search page).
watch(() => props.modelValue, async (id) => {
  if (!id) {
    if (!isOpen.value) query.value = ''
    return
  }
  if (props.resolveFn) {
    const item = await props.resolveFn(id)
    if (item && props.modelValue === id) {
      suppressNextSearch = query.value !== item.label
      query.value = item.label
    }
  }
}, { immediate: true })

function select(item: PickerItem) {
  if (debounceTimer) clearTimeout(debounceTimer)
  suppressNextSearch = query.value !== item.label
  query.value = item.label
  isOpen.value = false
  results.value = []
  emit('update:modelValue', item.id)
}

function onOutsideClick(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onOutsideClick))
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
      :data-testid="`${testid}-picker-input`"
      :placeholder="placeholder"
      :disabled="disabled"
      icon="i-lucide-search"
      class="w-full"
    />
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
        :data-testid="`${testid}-picker-empty`"
        class="py-6 px-4 text-center text-xs text-muted"
      >
        {{ t('common.pickerEmpty') }}
      </div>
      <ul
        v-else
        class="max-h-[260px] overflow-y-auto py-1"
      >
        <li
          v-for="item in results"
          :key="item.id"
          :data-testid="`${testid}-picker-item`"
          class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-muted"
          @click="select(item)"
        >
          <span class="min-w-0 flex-1">
            <span class="block text-[13px] font-medium truncate">{{ item.label }}</span>
            <span
              v-if="item.sublabel"
              class="block text-[11px] text-dimmed truncate"
            >{{ item.sublabel }}</span>
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>
