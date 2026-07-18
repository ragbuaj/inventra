<script setup lang="ts">
import type { BadgeColor } from '~/types'
import type { OpnameSession } from '~/composables/api/useStockOpname'
import { SESSION_STATUS_TONE, type SessionStatus } from '~/constants/stockOpnameMeta'

const DOT_CLASS: Record<BadgeColor, string> = {
  primary: 'bg-primary',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-error',
  info: 'bg-info',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const props = defineProps<{
  session: OpnameSession
  found: number
  total: number
}>()

defineEmits<{ open: [] }>()

const { t } = useI18n()

const statusKey = computed<SessionStatus>(() => props.session.status as SessionStatus)
const pct = computed(() => (props.total > 0 ? Math.round((props.found / props.total) * 100) : 0))
</script>

<template>
  <button
    :id="`opname-session-row-${session.id}`"
    type="button"
    data-testid="opname-session-row"
    class="flex flex-wrap sm:flex-nowrap items-center gap-x-4 gap-y-3 w-full px-[18px] py-4 bg-default border border-default rounded-[13px] shadow-sm text-left transition-colors hover:border-primary"
    @click="$emit('open')"
  >
    <span class="size-11 rounded-[11px] bg-info/15 text-info flex items-center justify-center flex-none">
      <UIcon
        name="i-lucide-clipboard-check"
        class="size-[21px]"
      />
    </span>
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2.5 flex-wrap">
        <span class="font-semibold text-[15px]">{{ session.name ?? '—' }}</span>
        <UBadge
          :color="SESSION_STATUS_TONE[statusKey]"
          variant="subtle"
          class="rounded-full gap-1.5"
        >
          <span
            class="size-1.5 rounded-full"
            :class="DOT_CLASS[SESSION_STATUS_TONE[statusKey]]"
          />
          {{ t(`stockOpname.status.${statusKey}`) }}
        </UBadge>
      </div>
      <div class="flex items-center gap-3.5 mt-1.5 flex-wrap text-[12.5px] text-muted">
        <span class="inline-flex items-center gap-1.5">
          <UIcon
            name="i-lucide-building-2"
            class="size-[13px]"
          />
          {{ session.office_name ?? '—' }}
        </span>
        <span class="inline-flex items-center gap-1.5">
          <UIcon
            name="i-lucide-calendar"
            class="size-[13px]"
          />
          {{ session.period }}
        </span>
      </div>
    </div>
    <UIcon
      name="i-lucide-chevron-right"
      class="size-[18px] text-dimmed flex-none sm:order-last"
    />
    <div class="w-full sm:w-[180px] flex-none">
      <div class="flex justify-between text-xs font-medium mb-1.5">
        <span class="text-muted">{{ t('stockOpname.progress', { a: found, b: total }) }}</span>
        <span class="font-semibold">{{ pct }}%</span>
      </div>
      <div class="h-[7px] rounded-full bg-muted overflow-hidden">
        <div
          class="h-full rounded-full"
          :class="session.status === 'closed' ? 'bg-success' : 'bg-info'"
          :style="{ width: `${pct}%` }"
        />
      </div>
    </div>
  </button>
</template>
