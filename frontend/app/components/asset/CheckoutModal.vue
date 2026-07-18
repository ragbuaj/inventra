<script setup lang="ts">
import type { AssetCondition } from '~/constants/assignmentMeta'
import { CONDITION_KEYS } from '~/constants/assignmentMeta'
import type { LockedAsset } from '~/components/assignment/AjukanPeminjamanModal.vue'

const props = defineProps<{
  open: boolean
  asset?: LockedAsset | null
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'submitted': []
}>()

const { t } = useI18n()
const toast = useToast()
const assignmentApi = useAssignment()
const employee = useEmployeePicker()

const employeeId = ref('')
const checkoutDate = ref('')
const dueDate = ref('')
const condition = ref<AssetCondition>('baik')
const notes = ref('')
const employeeError = ref(false)
const dateError = ref(false)
const submitting = ref(false)

const conditionItems = computed(() => CONDITION_KEYS.map(k => ({ value: k, label: t(`assignment.condition.${k}`) })))

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function reset() {
  employeeId.value = ''
  checkoutDate.value = todayISO()
  dueDate.value = ''
  condition.value = 'baik'
  notes.value = ''
  employeeError.value = false
  dateError.value = false
  submitting.value = false
}

watch(() => props.open, (isOpen) => {
  if (isOpen) reset()
}, { immediate: true })

function close() {
  emit('update:open', false)
}

async function submit() {
  employeeError.value = !employeeId.value
  dateError.value = !checkoutDate.value
  if (!props.asset || employeeError.value || dateError.value || submitting.value) return

  submitting.value = true
  try {
    // Resolve the picked employee's label for the success toast (best-effort;
    // the checkout itself does not depend on it).
    const employeeItem = await employee.resolveFn(employeeId.value).catch(() => null)
    await assignmentApi.checkout({
      asset_id: props.asset.id,
      employee_id: employeeId.value,
      checkout_date: checkoutDate.value,
      due_date: dueDate.value || null,
      condition_out: condition.value,
      notes: notes.value.trim() || null
    })
    toast.add({ title: t('assignment.checkout.ok', { name: props.asset.name, holder: employeeItem?.label ?? '—' }), color: 'success' })
    emit('submitted')
    close()
  } catch {
    // useApiClient already raised an error toast
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('assets.detail.checkoutModal.title')"
    :description="t('assets.detail.checkoutModal.subtitle')"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div
        class="flex flex-col gap-4"
        data-testid="checkout-modal"
      >
        <div
          v-if="asset"
          data-testid="checkout-modal-locked-asset"
          class="border border-default rounded-xl bg-muted p-3.5"
        >
          <div class="flex items-center gap-2.5">
            <span class="size-[38px] rounded-[9px] bg-default border border-default flex items-center justify-center flex-none text-primary">
              <UIcon
                name="i-lucide-monitor"
                class="size-[19px]"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm truncate">
                {{ asset.name }}
              </div>
              <div class="font-mono text-[11.5px] text-dimmed">
                {{ asset.asset_tag }}
              </div>
            </div>
            <UIcon
              name="i-lucide-lock"
              class="size-[15px] text-dimmed flex-none"
              :title="t('peminjaman.modal.lockedTip')"
            />
          </div>
        </div>

        <UFormField
          :label="t('assignment.checkout.recipient')"
          :error="employeeError ? t('assets.detail.checkoutModal.recipientErr') : undefined"
          required
        >
          <AsyncSearchPicker
            :model-value="employeeId || null"
            :search-fn="employee.searchFn"
            :resolve-fn="employee.resolveFn"
            :placeholder="t('assignment.checkout.recipientPlaceholder')"
            testid="checkout-employee"
            @update:model-value="(v) => { employeeId = v ?? ''; employeeError = false }"
          />
        </UFormField>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <UFormField
            :label="t('assignment.checkout.borrowDate')"
            :error="dateError ? t('assets.detail.checkoutModal.dateErr') : undefined"
            required
          >
            <DateField
              v-model="checkoutDate"
              testid="checkout-date"
              @update:model-value="dateError = false"
            />
          </UFormField>
          <UFormField :label="t('peminjaman.form.tempo')">
            <DateField
              v-model="dueDate"
              testid="checkout-due-date"
            />
          </UFormField>
        </div>

        <UFormField :label="t('assignment.checkout.condOut')">
          <USelect
            v-model="condition"
            value-key="value"
            :items="conditionItems"
            data-testid="checkout-condition"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('assignment.checkout.note')">
          <UTextarea
            v-model="notes"
            data-testid="checkout-notes"
            :rows="3"
            :placeholder="t('assignment.checkout.notePlaceholder')"
            class="w-full"
          />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="outline"
          data-testid="checkout-cancel"
          @click="close"
        >
          {{ t('common.cancel') }}
        </UButton>
        <UButton
          icon="i-lucide-clipboard-check"
          :loading="submitting"
          data-testid="checkout-submit"
          @click="submit"
        >
          {{ t('assignment.checkout.submit') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
