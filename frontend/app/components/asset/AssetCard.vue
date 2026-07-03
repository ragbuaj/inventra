<script setup lang="ts">
import type { AssetStatus } from '~/types'

/** Catalog-page view-model for a grid card — decoupled from the raw `Asset`
 * shape so lookups (category/office names) are resolved once by the page. */
export interface CatalogCardAsset {
  tag: string
  nama: string
  kategori: string
  brand: string
  kantor: string
  status: AssetStatus
  holder: string
  tglLabel: string
  hargaLabel: string
  hargaMasked: boolean
}

defineProps<{
  asset: CatalogCardAsset
  selected: boolean
  showPrice: boolean
}>()

defineEmits<{ toggle: [], open: [] }>()
</script>

<template>
  <div
    class="bg-default border rounded-[13px] shadow-sm overflow-hidden transition-colors"
    :class="selected ? 'border-primary' : 'border-default'"
  >
    <div class="flex items-start justify-between gap-2.5 px-[15px] pt-3.5">
      <div class="flex items-center gap-2.5 min-w-0">
        <UCheckbox
          :model-value="selected"
          @update:model-value="$emit('toggle')"
          @click.stop
        />
        <span class="text-xs font-mono text-muted truncate">{{ asset.tag }}</span>
      </div>
      <AssetStatusBadge :status="asset.status" />
    </div>
    <button
      type="button"
      class="block w-full text-left px-[15px] pt-2.5 pb-3.5 cursor-pointer"
      @click="$emit('open')"
    >
      <div class="text-[15px] font-semibold mb-0.5">
        {{ asset.nama }}
      </div>
      <div class="text-[12.5px] text-muted mb-[11px]">
        {{ asset.brand }}
      </div>
      <div class="flex flex-wrap gap-1.5 mb-3">
        <UBadge
          color="neutral"
          variant="subtle"
          class="rounded-full"
        >
          {{ asset.kategori }}
        </UBadge>
        <span class="inline-flex items-center gap-1 px-2.5 py-0.5 text-[11.5px] font-medium rounded-full bg-muted text-muted">
          <UIcon
            name="i-lucide-building-2"
            class="size-3"
          />
          {{ asset.kantor }}
        </span>
      </div>
      <div class="flex items-center justify-between text-[12.5px] text-muted pt-[11px] border-t border-default">
        <span :class="asset.holder === '—' ? 'text-dimmed' : 'text-default'">{{ asset.holder }}</span>
        <span>{{ asset.tglLabel }}</span>
      </div>
      <div
        v-if="showPrice"
        class="mt-2 flex items-center gap-1.5 text-[13.5px] font-semibold"
      >
        <span
          v-if="asset.hargaMasked"
          class="inline-flex items-center gap-1 text-dimmed font-normal"
          :title="$t('assets.masked')"
        >
          {{ asset.hargaLabel }}
          <UIcon
            name="i-lucide-lock"
            class="size-3"
          />
        </span>
        <template v-else>
          {{ asset.hargaLabel }}
        </template>
      </div>
    </button>
  </div>
</template>
