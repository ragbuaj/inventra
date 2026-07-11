<script setup lang="ts">
import type { ApprovalItem } from '~/composables/api/useDashboard'

const props = defineProps<{
  title: string
  items: ApprovalItem[]
  emptyTitle: string
  emptySub: string
  /** Total pending count for the badge; defaults to the rendered item count. */
  count?: number
}>()

const badgeCount = computed(() => props.count ?? props.items.length)

defineEmits<{ approve: [id: string], reject: [id: string] }>()

type Tone = ApprovalItem['tone']
const iconClass: Record<Tone, string> = {
  info: 'bg-info/10 text-info',
  primary: 'bg-primary/10 text-primary',
  neutral: 'bg-elevated text-dimmed'
}

const isEmpty = computed(() => props.items.length === 0)
</script>

<template>
  <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between px-[18px] py-[15px] border-b border-default">
      <div class="flex items-center gap-[9px]">
        <span class="size-[30px] rounded-lg bg-primary/10 text-primary flex items-center justify-center">
          <UIcon
            name="i-lucide-check-square"
            class="size-4"
          />
        </span>
        <span class="text-sm font-semibold">{{ title }}</span>
      </div>
      <span class="px-[9px] py-[2px] text-[11.5px] font-bold rounded-full bg-warning/10 text-warning">
        {{ badgeCount }}
      </span>
    </div>

    <!-- Rows -->
    <div v-if="!isEmpty">
      <div
        v-for="a in items"
        :key="a.id"
        class="flex items-center gap-3 px-[18px] py-3 border-b border-default last:border-b-0"
      >
        <span
          class="size-[34px] rounded-[9px] flex items-center justify-center flex-none"
          :class="iconClass[a.tone]"
        >
          <UIcon
            :name="a.icon"
            class="size-4"
          />
        </span>
        <div class="flex-1 min-w-0">
          <div class="text-[13.5px] font-semibold truncate">
            {{ a.title }}
          </div>
          <div class="text-xs text-muted truncate">
            {{ a.meta }}
          </div>
        </div>
        <div class="flex gap-[6px] flex-none">
          <UButton
            icon="i-lucide-check"
            color="primary"
            size="sm"
            square
            :aria-label="`approve-${a.id}`"
            @click="$emit('approve', a.id)"
          />
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :aria-label="`reject-${a.id}`"
            @click="$emit('reject', a.id)"
          />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else
      class="flex flex-col items-center gap-2 px-5 py-[38px] text-center"
    >
      <span class="size-[42px] rounded-[11px] bg-primary/10 text-primary flex items-center justify-center">
        <UIcon
          name="i-lucide-circle-check-big"
          class="size-5"
        />
      </span>
      <div class="text-sm font-semibold">
        {{ emptyTitle }}
      </div>
      <div class="text-[13px] text-muted">
        {{ emptySub }}
      </div>
    </div>
  </div>
</template>
