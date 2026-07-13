<script setup lang="ts">
import type { PickerItem } from '~/types'

const props = withDefaults(defineProps<{
  modelValue: string | null
  searchFn: (term: string) => Promise<PickerItem[]>
  resolveFn?: (id: string) => Promise<PickerItem | null>
  placeholder: string
  disabled?: boolean
  testid?: string
  clearable?: boolean
}>(), {
  resolveFn: undefined,
  disabled: false,
  testid: 'async',
  clearable: false
})

const emit = defineEmits<{ 'update:modelValue': [id: string | null] }>()

const DEBOUNCE_MS = 300
const { t } = useI18n()

const query = ref('')
const results = ref<PickerItem[]>([])
const loading = ref(false)
const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const activeIndex = ref(-1)
const listboxId = useId()

let debounceTimer: ReturnType<typeof setTimeout> | undefined
let seq = 0
let suppressNextSearch = false

function optionId(index: number) {
  return `${listboxId}-opt-${index}`
}

watch(results, () => {
  activeIndex.value = -1
})

watch(isOpen, (open) => {
  if (open) activeIndex.value = -1
})

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

function clear() {
  if (debounceTimer) clearTimeout(debounceTimer)
  suppressNextSearch = true
  query.value = ''
  isOpen.value = false
  results.value = []
  emit('update:modelValue', null)
}

function focusInput() {
  containerRef.value?.querySelector('input')?.focus()
}

function moveActive(direction: 1 | -1) {
  const len = results.value.length
  if (len === 0) return
  if (direction === 1) {
    activeIndex.value = activeIndex.value < len - 1 ? activeIndex.value + 1 : 0
  } else {
    activeIndex.value = activeIndex.value > 0 ? activeIndex.value - 1 : len - 1
  }
}

function onKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      if (!isOpen.value) {
        isOpen.value = true
        return
      }
      moveActive(1)
      break
    case 'ArrowUp':
      event.preventDefault()
      if (!isOpen.value) {
        isOpen.value = true
        return
      }
      moveActive(-1)
      break
    case 'Enter':
      if (activeIndex.value >= 0 && activeIndex.value < results.value.length) {
        event.preventDefault()
        select(results.value[activeIndex.value]!)
      }
      break
    case 'Escape':
      isOpen.value = false
      activeIndex.value = -1
      focusInput()
      break
    case 'Home':
      if (results.value.length > 0) {
        event.preventDefault()
        activeIndex.value = 0
      }
      break
    case 'End':
      if (results.value.length > 0) {
        event.preventDefault()
        activeIndex.value = results.value.length - 1
      }
      break
  }
}

const showClear = computed(() => props.clearable && !!props.modelValue)

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
      role="combobox"
      :aria-expanded="isOpen"
      aria-haspopup="listbox"
      :aria-controls="listboxId"
      :aria-activedescendant="activeIndex >= 0 ? optionId(activeIndex) : undefined"
      @keydown="onKeydown"
    >
      <template
        v-if="showClear"
        #trailing
      >
        <UButton
          type="button"
          color="neutral"
          variant="link"
          size="sm"
          icon="i-lucide-x"
          :data-testid="`${testid}-picker-clear`"
          :aria-label="t('common.clearSelection')"
          @click="clear"
        />
      </template>
    </UInput>
    <div
      v-if="isOpen"
      class="absolute z-10 mt-1 w-full bg-default border border-default rounded-lg shadow-lg overflow-hidden"
    >
      <div
        v-if="loading"
        class="p-3 space-y-2"
        role="status"
        aria-live="polite"
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
        role="status"
        aria-live="polite"
      >
        {{ t('common.pickerEmpty') }}
      </div>
      <ul
        v-else
        :id="listboxId"
        role="listbox"
        class="max-h-[260px] overflow-y-auto py-1"
      >
        <li
          v-for="(item, index) in results"
          :id="optionId(index)"
          :key="item.id"
          role="option"
          :aria-selected="index === activeIndex"
          :data-testid="`${testid}-picker-item`"
          class="flex items-center gap-2.5 px-3 py-2 cursor-pointer hover:bg-muted"
          :class="{ 'bg-muted': index === activeIndex }"
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
