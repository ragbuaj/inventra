<script setup lang="ts">
const props = defineProps<{
  open: boolean
  submitting: boolean
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'confirm': [{ officeId: string, name: string, period: string }]
}>()

const { t } = useI18n()
const office = useOfficePicker()

const name = ref('')
const officeId = ref('')
const period = ref('')

function reset() {
  name.value = ''
  officeId.value = ''
  period.value = ''
}

watch(() => props.open, (isOpen) => {
  if (isOpen) reset()
})

const ready = computed(() => !!(officeId.value && period.value))

function close() {
  emit('update:open', false)
}

function confirm() {
  if (!ready.value || props.submitting) return
  emit('confirm', { officeId: officeId.value, name: name.value.trim(), period: period.value })
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('stockOpname.create.title')"
    :description="t('stockOpname.create.sub')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div class="space-y-4">
        <UFormField :label="t('stockOpname.create.name')">
          <UInput
            v-model="name"
            data-testid="opname-create-name"
            :placeholder="t('stockOpname.create.namePlaceholder')"
            class="w-full"
          />
        </UFormField>
        <UFormField
          :label="t('stockOpname.create.scope')"
          required
        >
          <AsyncSearchPicker
            :model-value="officeId || null"
            :search-fn="office.searchFn"
            :resolve-fn="office.resolveFn"
            :placeholder="t('common.searchOffice')"
            testid="office"
            @update:model-value="officeId = $event ?? ''"
          />
        </UFormField>
        <UFormField
          :label="t('stockOpname.create.period')"
          required
        >
          <UInput
            v-model="period"
            data-testid="opname-create-period"
            type="month"
            class="w-full"
          />
        </UFormField>
        <div class="flex gap-2.5 items-center px-3.5 py-2.5 rounded-[10px] bg-info/10 border border-info/25">
          <UIcon
            name="i-lucide-info"
            class="size-[15px] flex-none text-info"
          />
          <span class="text-xs leading-relaxed text-info">{{ t('stockOpname.create.snapshotNote') }}</span>
        </div>
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
          :disabled="!ready"
          data-testid="opname-create-confirm"
          @click="confirm"
        >
          {{ t('stockOpname.create.confirm') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
