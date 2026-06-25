<script setup lang="ts">
import type { BarItem } from '~/utils/dashboard'
import { formatCount } from '~/utils/dashboard'

const props = defineProps<{
  title: string
  items: BarItem[]
  color?: 'primary' | 'info'
}>()

const barClass = computed(() => (props.color === 'info' ? 'bg-info' : 'bg-primary'))
</script>

<template>
  <div class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm">
    <div class="text-sm font-semibold mb-4">
      {{ title }}
    </div>
    <div class="flex flex-col gap-[13px]">
      <div
        v-for="bar in items"
        :key="bar.label"
      >
        <div class="flex justify-between text-[12.5px] font-medium mb-[5px] gap-2">
          <span class="text-muted truncate">{{ bar.label }}</span>
          <span class="text-default font-semibold flex-none">{{ formatCount(bar.count) }}</span>
        </div>
        <div class="h-2 rounded-full bg-elevated overflow-hidden">
          <div
            class="h-full rounded-full origin-left transition-[width] duration-500 ease-out"
            :class="barClass"
            :style="{ width: `${bar.w}%` }"
          />
        </div>
      </div>
    </div>
  </div>
</template>
