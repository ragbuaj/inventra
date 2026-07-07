<script setup lang="ts">
import type { OpnameSessionDetail } from '~/composables/api/useStockOpname'

const props = defineProps<{
  open: boolean
  session: OpnameSessionDetail | null
  damaged: number
  misplaced: number
  notFound: number
  submitting: boolean
  exporting: boolean
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'confirm': []
  'export': ['pdf' | 'xlsx']
}>()

const { t } = useI18n()

const baRows = computed(() => {
  if (!props.session) return []
  return [
    { label: t('stockOpname.finish.summaryTotal'), value: String(props.session.total), colorClass: 'text-default' },
    { label: t('stockOpname.finish.summaryFound'), value: String(props.session.found), colorClass: 'text-success' },
    { label: t('stockOpname.finish.summaryDamaged'), value: String(props.damaged), colorClass: 'text-warning' },
    { label: t('stockOpname.finish.summaryMisplaced'), value: String(props.misplaced), colorClass: 'text-primary' },
    { label: t('stockOpname.finish.summaryNotFound'), value: String(props.notFound), colorClass: 'text-error' }
  ]
})

function close() {
  emit('update:open', false)
}

function confirm() {
  if (props.submitting) return
  emit('confirm')
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('stockOpname.finish.title')"
    :description="t('stockOpname.finish.sub')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div
        v-if="session"
        class="border border-default rounded-xl overflow-hidden"
      >
        <div class="px-4 py-3.5 border-b border-default bg-muted text-center">
          <div class="text-[13px] font-bold tracking-wide">
            {{ t('stockOpname.finish.reportTitle') }}
          </div>
          <div class="text-[11px] text-muted mt-0.5">
            {{ session.name ?? '—' }} · {{ session.period }}
          </div>
        </div>
        <div class="px-4 py-3.5 flex flex-col gap-2.5">
          <div
            v-for="(row, i) in baRows"
            :key="i"
            class="flex items-center justify-between gap-3 text-[12.5px]"
          >
            <span class="text-muted">{{ row.label }}</span>
            <span
              class="font-semibold"
              :class="row.colorClass"
            >{{ row.value }}</span>
          </div>
          <div class="h-px bg-default my-0.5" />
          <div class="flex items-center justify-between gap-3 text-[11px] text-dimmed">
            <span>{{ t('stockOpname.finish.signedBy') }}</span>
          </div>
        </div>
      </div>
      <div class="flex gap-2.5 mt-4">
        <UButton
          block
          color="neutral"
          variant="outline"
          icon="i-lucide-file-text"
          :loading="exporting"
          data-testid="opname-finish-pdf"
          @click="emit('export', 'pdf')"
        >
          {{ t('stockOpname.finish.pdf') }}
        </UButton>
        <UButton
          block
          color="neutral"
          variant="outline"
          icon="i-lucide-file-spreadsheet"
          :loading="exporting"
          data-testid="opname-finish-excel"
          @click="emit('export', 'xlsx')"
        >
          {{ t('stockOpname.finish.excel') }}
        </UButton>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          @click="close"
        >
          {{ t('stockOpname.create.cancel') }}
        </UButton>
        <UButton
          :loading="submitting"
          data-testid="opname-finish-confirm"
          @click="confirm"
        >
          {{ t('stockOpname.finish.confirm') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
