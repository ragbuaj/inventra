<script setup lang="ts">
import type { MaintenanceItem } from '~/composables/api/useDashboard'

defineProps<{
  title: string
  seeAllLabel: string
  items: MaintenanceItem[]
}>()

defineEmits<{ seeAll: [] }>()
</script>

<template>
  <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between px-[18px] py-[15px] border-b border-default">
      <div class="flex items-center gap-[9px]">
        <span class="size-[30px] rounded-lg bg-warning/10 text-warning flex items-center justify-center">
          <UIcon
            name="i-lucide-wrench"
            class="size-4"
          />
        </span>
        <span class="text-sm font-semibold">{{ title }}</span>
      </div>
      <UButton
        variant="link"
        color="primary"
        size="sm"
        :label="seeAllLabel"
        class="px-0"
        @click="$emit('seeAll')"
      />
    </div>

    <!-- Rows -->
    <div>
      <div
        v-for="(m, i) in items"
        :key="i"
        class="flex items-center gap-3 px-[18px] py-[13px] border-b border-default last:border-b-0 hover:bg-muted transition-colors"
      >
        <span class="size-[34px] rounded-[9px] bg-elevated text-muted flex items-center justify-center flex-none">
          <UIcon
            :name="m.icon"
            class="size-4"
          />
        </span>
        <div class="flex-1 min-w-0">
          <div class="text-[13.5px] font-semibold truncate">
            {{ m.asset }}
          </div>
          <div class="text-[12.5px] text-muted">
            {{ m.task }}
          </div>
        </div>
        <span
          class="flex-none px-[10px] py-[3px] text-[11.5px] font-semibold rounded-full"
          :class="m.urg ? 'bg-warning/10 text-warning' : 'bg-elevated text-dimmed'"
        >{{ m.due }}</span>
      </div>
    </div>
  </div>
</template>
