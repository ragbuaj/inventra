<script setup lang="ts">
type IconTone = 'primary' | 'info' | 'neutral' | 'error' | 'warning'
type TrendTone = 'success' | 'muted' | 'error' | 'warning'

const props = defineProps<{
  label: string
  value: string
  icon: string
  iconTone: IconTone
  trendIcon: string
  trendText: string
  trendTone: TrendTone
}>()

const iconClass: Record<IconTone, string> = {
  primary: 'bg-primary/10 text-primary',
  info: 'bg-info/10 text-info',
  neutral: 'bg-elevated text-dimmed',
  error: 'bg-error/10 text-error',
  warning: 'bg-warning/10 text-warning'
}

const trendClass: Record<TrendTone, string> = {
  success: 'text-success',
  muted: 'text-muted',
  error: 'text-error',
  warning: 'text-warning'
}

const iconTokens = computed(() => iconClass[props.iconTone])
const trendTokens = computed(() => trendClass[props.trendTone])
</script>

<template>
  <div class="bg-default border border-default rounded-[14px] p-[18px] shadow-sm">
    <div class="flex items-start justify-between gap-[10px]">
      <span class="text-[13px] font-medium text-muted">{{ label }}</span>
      <span
        class="size-8 rounded-[9px] flex items-center justify-center flex-none"
        :class="iconTokens"
      >
        <UIcon
          :name="icon"
          class="size-[18px]"
        />
      </span>
    </div>
    <div class="text-[27px] font-bold tracking-tight mt-[11px] mb-[6px]">
      {{ value }}
    </div>
    <div
      class="flex items-center gap-[5px] text-xs font-medium"
      :class="trendTokens"
    >
      <UIcon
        :name="trendIcon"
        class="size-[13px]"
      />
      <span>{{ trendText }}</span>
    </div>
  </div>
</template>
