<script setup lang="ts">
import type { Asset, AssetStatus, PickerItem } from '~/types'

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

const assetsApi = useAssets()
const byId = new Map<string, Asset>()
const selected = ref<string | null>(null)

async function searchFn(term: string): Promise<PickerItem[]> {
  const pages = await Promise.all(
    props.statuses.map(status => assetsApi.list({ search: term, status, limit: 20 }))
  )
  const merged = new Map<string, Asset>()
  for (const page of pages) for (const a of page.data) merged.set(a.id, a)
  byId.clear()
  const items: PickerItem[] = []
  for (const a of merged.values()) {
    byId.set(a.id, a)
    items.push({ id: a.id, label: a.name, sublabel: `${a.asset_tag} · ${props.officeNames?.get(a.office_id) ?? '—'}` })
  }
  return items
}

function onUpdate(id: string | null) {
  selected.value = id
  const asset = id ? byId.get(id) : undefined
  if (asset) emit('select', asset)
}
</script>

<template>
  <div>
    <AsyncSearchPicker
      v-model="selected"
      testid="asset"
      :search-fn="searchFn"
      :placeholder="placeholder"
      :disabled="disabled"
      @update:model-value="onUpdate"
    />
    <p
      v-if="hint"
      data-testid="asset-picker-hint"
      class="text-xs text-muted mt-1"
    >
      {{ hint }}
    </p>
  </div>
</template>
