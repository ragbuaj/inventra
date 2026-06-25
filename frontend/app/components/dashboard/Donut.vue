<script setup lang="ts">
import { VisSingleContainer, VisDonut } from '@unovis/vue'
import type { StatusSegment } from '~/utils/dashboard'
import { formatCount } from '~/utils/dashboard'

const props = defineProps<{
  title: string
  total: number
  totalLabel: string
  segments: StatusSegment[]
}>()

const { t } = useI18n()

// Unovis accessors. We feed the raw segments; value = count, color = the segment's token CSS var
// (so light/dark theming follows the design tokens automatically).
const value = (d: StatusSegment) => d.count
const color = (d: StatusSegment) => d.color

const totalText = computed(() => formatCount(props.total))
</script>

<template>
  <div class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm">
    <div class="text-sm font-semibold mb-4">
      {{ title }}
    </div>
    <div class="flex items-center gap-[18px]">
      <!-- Ring + centered total overlay -->
      <div class="relative w-[128px] h-[128px] flex-none">
        <VisSingleContainer
          :data="segments"
          :height="128"
        >
          <VisDonut
            :value="value"
            :color="color"
            :arc-width="18"
            :corner-radius="2"
            :show-background="false"
          />
        </VisSingleContainer>
        <div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
          <span class="text-2xl font-bold tracking-tight leading-none">{{ totalText }}</span>
          <span class="text-[11px] text-muted mt-1">{{ totalLabel }}</span>
        </div>
      </div>

      <!-- Legend -->
      <div class="flex-1 flex flex-col gap-[9px] min-w-0">
        <div
          v-for="seg in segments"
          :key="seg.key"
          class="flex items-center gap-2 text-[12.5px] font-medium"
        >
          <span
            class="size-[9px] rounded-[3px] flex-none"
            :style="{ background: seg.color }"
          />
          <span class="flex-1 text-muted truncate">{{ t(`dashboard.status.${seg.key}`) }}</span>
          <span class="text-default font-semibold">{{ formatCount(seg.count) }}</span>
          <span class="text-dimmed w-9 text-right">{{ seg.pct }}%</span>
        </div>
      </div>
    </div>
  </div>
</template>
