<script setup lang="ts">
import type { Category, FiscalGroup } from '~/types'
import type { CategoryInput } from '~/composables/api/useCategories'
import { FISCAL_GROUPS, isBuildingGroup } from '~/constants/categoryMeta'

const props = defineProps<{
  category: Category | null
  parentOptions: { value: string, label: string }[]
  loading?: boolean
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [CategoryInput] }>()

const { t } = useI18n()

interface FormState {
  name: string
  code: string
  parent_id: string
  asset_class: Category['asset_class']
  default_depreciation_method: 'straight_line' | 'declining_balance'
  default_useful_life_months: string
  default_salvage_rate: string
  default_fiscal_group: string
  default_fiscal_life_months: string
  gl_account_code: string
  capitalization_threshold: string
  is_active: boolean
}

function emptyForm(): FormState {
  return {
    name: '', code: '', parent_id: '__none__', asset_class: 'tangible',
    default_depreciation_method: 'straight_line', default_useful_life_months: '',
    default_salvage_rate: '', default_fiscal_group: '__none__', default_fiscal_life_months: '',
    gl_account_code: '', capitalization_threshold: '', is_active: true
  }
}

const form = reactive<FormState>(emptyForm())
const errors = reactive<{ name: boolean, code: boolean }>({ name: false, code: false })

function hydrate() {
  const c = props.category
  errors.name = false
  errors.code = false
  if (!c) {
    Object.assign(form, emptyForm())
    return
  }
  Object.assign(form, {
    name: c.name,
    code: c.code ?? '',
    parent_id: c.parent_id ?? '__none__',
    asset_class: c.asset_class,
    default_depreciation_method: c.default_depreciation_method ?? 'straight_line',
    default_useful_life_months: c.default_useful_life_months != null ? String(c.default_useful_life_months) : '',
    default_salvage_rate: c.default_salvage_rate ?? '',
    default_fiscal_group: c.default_fiscal_group ?? '__none__',
    default_fiscal_life_months: c.default_fiscal_life_months != null ? String(c.default_fiscal_life_months) : '',
    gl_account_code: c.gl_account_code ?? '',
    capitalization_threshold: c.capitalization_threshold ?? '',
    is_active: c.is_active
  })
}

watch(open, (v) => {
  if (v) hydrate()
}, { immediate: true })

const isIntangible = computed(() => form.asset_class === 'intangible')
const isBuilding = computed(() => isBuildingGroup(form.default_fiscal_group as FiscalGroup))
const metodeLocked = computed(() => isBuilding.value)

// Building assets must use straight line.
watch(isBuilding, (b) => {
  if (b) form.default_depreciation_method = 'straight_line'
})

// Intangible classes can't be buildings; drop the bangunan_* options.
const fiscalGroupOptions = computed(() =>
  FISCAL_GROUPS
    .filter(g => !(isIntangible.value && isBuildingGroup(g)))
    .map(g => ({ value: g as string, label: t(`masterdata.categories.fiscalGroup.${g}`) }))
)

const methodOptions = computed(() => [
  { value: 'straight_line', label: t('masterdata.categories.method.straight_line') },
  { value: 'declining_balance', label: t('masterdata.categories.method.declining_balance') }
])

const susutTitle = computed(() =>
  isIntangible.value ? t('masterdata.categories.section.amortCommercial') : t('masterdata.categories.section.deprCommercial')
)
const susutRef = computed(() =>
  isIntangible.value ? t('masterdata.categories.section.amortRef') : t('masterdata.categories.section.deprRef')
)

const formTitle = computed(() =>
  props.category ? t('masterdata.categories.editTitle') : t('masterdata.categories.createTitle')
)
const formSub = computed(() =>
  props.category ? t('masterdata.categories.editSub') : t('masterdata.categories.createSub')
)

function toInput(): CategoryInput {
  const numOrNull = (s: string): number | null => {
    const n = Number(s)
    return s.trim() !== '' && Number.isFinite(n) ? Math.trunc(n) : null
  }
  const strOrNull = (s: string): string | null => (s.trim() !== '' ? s.trim() : null)
  const cap = form.capitalization_threshold
  return {
    name: form.name.trim(),
    code: strOrNull(form.code),
    parent_id: (form.parent_id === '__none__' || form.parent_id === '') ? null : form.parent_id,
    default_depreciation_method: form.default_depreciation_method,
    default_useful_life_months: numOrNull(form.default_useful_life_months),
    default_salvage_rate: strOrNull(form.default_salvage_rate),
    asset_class: form.asset_class,
    default_fiscal_group: (form.default_fiscal_group === '__none__' || form.default_fiscal_group === '') ? null : (form.default_fiscal_group as Category['default_fiscal_group']),
    default_fiscal_life_months: numOrNull(form.default_fiscal_life_months),
    gl_account_code: strOrNull(form.gl_account_code),
    capitalization_threshold: cap !== '' ? cap : null,
    is_active: form.is_active
  }
}

function onSubmit() {
  errors.name = form.name.trim() === ''
  errors.code = form.code.trim() === ''
  if (errors.name || errors.code) return
  emit('submit', toInput())
}

defineExpose({ form, isIntangible, isBuilding, onSubmit })
</script>

<template>
  <FormSlideover
    v-model:open="open"
    :title="formTitle"
    :subtitle="formSub"
    :loading="props.loading"
    @submit="onSubmit"
  >
    <div class="space-y-6">
      <!-- Section 1: Umum -->
      <section>
        <div class="flex items-center gap-2 mb-3">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">1</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.general') }}</span>
        </div>
        <div class="space-y-3">
          <div class="grid grid-cols-[1fr_140px] gap-3">
            <UFormField
              :label="t('masterdata.categories.fields.name')"
              required
              :error="errors.name ? t('masterdata.categories.req') : undefined"
            >
              <UInput
                v-model="form.name"
                :placeholder="t('masterdata.categories.placeholders.name')"
                class="w-full"
              />
            </UFormField>
            <UFormField
              :label="t('masterdata.categories.fields.code')"
              required
              :error="errors.code ? t('masterdata.categories.req') : undefined"
            >
              <UInput
                v-model="form.code"
                :placeholder="t('masterdata.categories.placeholders.code')"
                class="w-full font-mono"
              />
            </UFormField>
          </div>

          <UFormField
            :label="t('masterdata.categories.fields.parent')"
            :hint="t('masterdata.categories.hint.parent')"
          >
            <USelect
              v-model="form.parent_id"
              :items="[{ value: '__none__', label: t('masterdata.categories.placeholders.parentNone') }, ...props.parentOptions]"
              class="w-full"
              data-testid="category-parent-select"
            />
          </UFormField>

          <UFormField :label="t('masterdata.categories.fields.class')">
            <div class="flex gap-2">
              <UButton
                :color="form.asset_class === 'tangible' ? 'primary' : 'neutral'"
                :variant="form.asset_class === 'tangible' ? 'solid' : 'outline'"
                icon="i-lucide-box"
                class="flex-1 justify-center"
                @click="() => { form.asset_class = 'tangible' }"
              >
                {{ t('masterdata.categories.class.tangible') }}
              </UButton>
              <UButton
                :color="form.asset_class === 'intangible' ? 'primary' : 'neutral'"
                :variant="form.asset_class === 'intangible' ? 'solid' : 'outline'"
                icon="i-lucide-sparkles"
                class="flex-1 justify-center"
                @click="() => { form.asset_class = 'intangible' }"
              >
                {{ t('masterdata.categories.class.intangible') }}
              </UButton>
            </div>
          </UFormField>

          <label class="flex items-center justify-between gap-2 rounded-[10px] bg-muted px-3 h-11 cursor-pointer">
            <span class="text-sm font-semibold">{{ t('masterdata.categories.fields.active') }}</span>
            <USwitch v-model="form.is_active" />
          </label>
        </div>
      </section>

      <!-- Section 2: Penyusutan / Amortisasi -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-1">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">2</span>
          <span class="font-semibold text-sm">{{ susutTitle }}</span>
        </div>
        <div class="text-[11.5px] text-dimmed mb-3 ms-8">
          {{ susutRef }}
        </div>
        <div class="space-y-3">
          <UFormField :label="t('masterdata.categories.fields.method')">
            <USelect
              v-model="form.default_depreciation_method"
              :items="methodOptions"
              :disabled="metodeLocked"
              class="w-full"
            />
            <template
              v-if="metodeLocked"
              #hint
            >
              <span class="flex items-center gap-1 text-xs text-warning mt-1">
                <UIcon
                  name="i-lucide-lock"
                  class="size-3"
                />
                {{ t('masterdata.categories.hint.buildingLock') }}
              </span>
            </template>
          </UFormField>
          <div class="grid grid-cols-2 gap-3">
            <UFormField :label="t('masterdata.categories.fields.life')">
              <NumberInput
                v-model="form.default_useful_life_months"
                placeholder="48"
                class="w-full"
              >
                <template #trailing>
                  <span class="text-xs text-dimmed">{{ t('masterdata.categories.months') }}</span>
                </template>
              </NumberInput>
            </UFormField>
            <UFormField :label="t('masterdata.categories.fields.salvage')">
              <NumberInput
                v-model="form.default_salvage_rate"
                :max="100"
                placeholder="0"
                class="w-full"
              >
                <template #trailing>
                  <span class="text-xs text-dimmed">%</span>
                </template>
              </NumberInput>
            </UFormField>
          </div>
        </div>
      </section>

      <!-- Section 3: Pajak / Fiskal -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-1">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">3</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.tax') }}</span>
        </div>
        <div class="text-[11.5px] text-dimmed mb-3 ms-8">
          {{ t('masterdata.categories.section.taxRef') }}
        </div>
        <div class="space-y-3">
          <UFormField :label="t('masterdata.categories.fields.fiscalGroup')">
            <USelect
              v-model="form.default_fiscal_group"
              :items="[{ value: '__none__', label: t('masterdata.categories.placeholders.select') }, ...fiscalGroupOptions]"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('masterdata.categories.fields.fiscalLife')">
            <NumberInput
              v-model="form.default_fiscal_life_months"
              placeholder="48"
              class="w-full"
            >
              <template #trailing>
                <span class="text-xs text-dimmed">{{ t('masterdata.categories.months') }}</span>
              </template>
            </NumberInput>
          </UFormField>
        </div>
      </section>

      <!-- Section 4: Akuntansi -->
      <section class="border-t border-default pt-5">
        <div class="flex items-center gap-2 mb-3">
          <span class="w-6 h-6 rounded-md bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px]">4</span>
          <span class="font-semibold text-sm">{{ t('masterdata.categories.section.accounting') }}</span>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <UFormField
            :label="t('masterdata.categories.fields.gl')"
            :hint="t('masterdata.categories.hint.gl')"
          >
            <UInput
              v-model="form.gl_account_code"
              placeholder="1.2.3.01"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField
            :label="t('masterdata.categories.fields.capitalization')"
            :hint="t('masterdata.categories.hint.capitalization')"
          >
            <NumberInput
              v-model="form.capitalization_threshold"
              money
              placeholder="1.000.000"
              class="w-full"
            />
          </UFormField>
        </div>
      </section>
    </div>
  </FormSlideover>
</template>
