<script setup lang="ts">
import type { GuideModule, GuideModuleInput } from '~/types'

/**
 * The guide module editor: a 720px slideover holding module information, the
 * numbered step list, and (for a module that already exists) its attachments.
 *
 * Module fields and steps are saved together by the parent — an update replaces
 * the whole module, steps included, because that is how the editor works.
 * Attachments are NOT part of that save: each one is created or deleted
 * immediately by GuideAttachmentManager, which is why it is a slot rather than
 * part of this form's state.
 */
const props = defineProps<{
  module: GuideModule | null
  loading?: boolean
}>()

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ submit: [GuideModuleInput] }>()

const { t } = useI18n()

interface StepState {
  text_id: string
  text_en: string
}

interface FormState {
  icon: string
  sort_order: string
  status: 'draft' | 'published'
  title_id: string
  title_en: string
  body_id: string
  body_en: string
  steps: StepState[]
}

function emptyForm(): FormState {
  return {
    icon: GUIDE_ICON_CHOICES[0]!,
    sort_order: '1',
    status: 'draft',
    title_id: '',
    title_en: '',
    body_id: '',
    body_en: '',
    steps: []
  }
}

const form = reactive<FormState>(emptyForm())
const titleError = ref(false)
const stepErrors = ref<Set<number>>(new Set())
const showStepEn = ref(true)
const iconPickerOpen = ref(false)

function hydrate() {
  titleError.value = false
  stepErrors.value = new Set()
  iconPickerOpen.value = false
  const m = props.module
  if (!m) {
    Object.assign(form, emptyForm())
    return
  }
  Object.assign(form, {
    icon: m.icon || GUIDE_ICON_CHOICES[0]!,
    sort_order: String(m.sort_order),
    status: m.status,
    title_id: m.title_id,
    title_en: m.title_en ?? '',
    body_id: m.body_id ?? '',
    body_en: m.body_en ?? '',
    steps: m.steps.map(s => ({ text_id: s.text_id, text_en: s.text_en ?? '' }))
  })
}

watch(open, (v) => {
  if (v) hydrate()
}, { immediate: true })

const statusChoices = computed(() => [
  { value: 'draft' as const, label: t('settings.guide.status.draft'), icon: 'i-lucide-file-text' },
  { value: 'published' as const, label: t('settings.guide.status.published'), icon: 'i-lucide-send' }
])

/**
 * A brand-new module is always a draft — the create endpoint has no status field
 * at all. The control stays visible so the form keeps its shape and the author
 * can see where publishing will live, but it is inert until the module exists.
 */
const isCreate = computed(() => props.module === null)

/* ── step list ─────────────────────────────────────────────────────────────
   Steps are reordered with up/down buttons rather than drag-and-drop: the list
   is short, the buttons are reachable by keyboard, and ordering is implicit in
   the array (there is no per-step sort column to keep in sync). */

function addStep() {
  form.steps.push({ text_id: '', text_en: '' })
}

function moveStep(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= form.steps.length) return
  const rows = form.steps
  const moved = rows[index]!
  rows[index] = rows[target]!
  rows[target] = moved
  stepErrors.value = new Set()
}

function removeStep(index: number) {
  form.steps.splice(index, 1)
  stepErrors.value = new Set()
}

function orderDelta(delta: number) {
  const next = Math.max(0, (Number(form.sort_order) || 0) + delta)
  form.sort_order = String(next)
}

function validate(): boolean {
  titleError.value = form.title_id.trim() === ''
  // A step with no Indonesian text is rejected by the API (Indonesian is the
  // mandatory language), so it is flagged here instead of arriving as a 422 on
  // a form the author can no longer connect to a specific row.
  const bad = new Set<number>()
  form.steps.forEach((s, i) => {
    if (s.text_id.trim() === '') bad.add(i)
  })
  stepErrors.value = bad
  return !titleError.value && bad.size === 0
}

function onSubmit() {
  if (!validate()) return
  const titleID = form.title_id.trim()
  emit('submit', {
    // Renaming a module keeps its original slug so existing deep links and the
    // seed's ON CONFLICT key stay stable; only a brand-new module derives one.
    slug: props.module?.slug || guideSlug(titleID),
    icon: form.icon,
    sort_order: Number(form.sort_order) || 0,
    status: form.status,
    title_id: titleID,
    title_en: form.title_en.trim() || null,
    body_id: form.body_id.trim() || null,
    body_en: form.body_en.trim() || null,
    steps: form.steps.map(s => ({
      text_id: s.text_id.trim(),
      text_en: s.text_en.trim() || null
    }))
  })
}
</script>

<template>
  <USlideover
    v-model:open="open"
    :title="props.module ? t('settings.guide.form.editTitle') : t('settings.guide.form.createTitle')"
    :description="props.module ? props.module.title_id : t('settings.guide.form.createSub')"
    :ui="{ content: 'w-full max-w-[720px]' }"
  >
    <template #body>
      <div class="flex flex-col gap-[22px]">
        <!-- ── Section 1: module information ───────────────────────────── -->
        <section>
          <div class="flex items-center gap-2 mb-3">
            <span class="text-[11px] font-semibold font-mono tracking-[0.12em] uppercase text-dimmed">
              {{ t('settings.guide.form.secInfo') }}
            </span>
            <div class="flex-1 border-t border-default" />
          </div>

          <div class="flex flex-col gap-3.5">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
              <UFormField
                required
                :error="titleError ? t('settings.guide.form.required') : undefined"
              >
                <template #label>
                  <span class="inline-flex items-center gap-1.5">
                    {{ t('settings.guide.form.fieldTitle') }}
                    <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-primary/10 text-primary">ID</span>
                  </span>
                </template>
                <UInput
                  v-model="form.title_id"
                  data-testid="guide-title-id"
                  class="w-full"
                />
              </UFormField>

              <UFormField>
                <template #label>
                  <span class="inline-flex items-center gap-1.5 text-muted">
                    {{ t('settings.guide.form.fieldTitle') }}
                    <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-elevated text-muted">EN</span>
                    <span class="text-[11.5px] text-dimmed">{{ t('settings.guide.form.optional') }}</span>
                  </span>
                </template>
                <UInput
                  v-model="form.title_en"
                  data-testid="guide-title-en"
                  :placeholder="t('settings.guide.form.fallbackPlaceholder')"
                  class="w-full"
                />
              </UFormField>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
              <UFormField>
                <template #label>
                  <span class="inline-flex items-center gap-1.5">
                    {{ t('settings.guide.form.fieldBody') }}
                    <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-primary/10 text-primary">ID</span>
                  </span>
                </template>
                <UTextarea
                  v-model="form.body_id"
                  :rows="3"
                  autoresize
                  class="w-full"
                />
              </UFormField>

              <UFormField>
                <template #label>
                  <span class="inline-flex items-center gap-1.5 text-muted">
                    {{ t('settings.guide.form.fieldBody') }}
                    <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-elevated text-muted">EN</span>
                  </span>
                </template>
                <UTextarea
                  v-model="form.body_en"
                  :rows="3"
                  autoresize
                  :placeholder="t('settings.guide.form.fallbackPlaceholder')"
                  class="w-full"
                />
              </UFormField>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5 items-start">
              <!-- Icon: closed list, never a free-text field -->
              <UFormField :label="t('settings.guide.form.fieldIcon')">
                <UButton
                  color="neutral"
                  variant="outline"
                  class="w-full justify-start"
                  data-testid="guide-icon-toggle"
                  trailing-icon="i-lucide-chevron-down"
                  @click="iconPickerOpen = !iconPickerOpen"
                >
                  <span class="size-[30px] rounded-lg bg-primary/10 text-primary flex items-center justify-center flex-none">
                    <UIcon
                      :name="form.icon"
                      class="size-[18px]"
                    />
                  </span>
                  <span class="flex-1 min-w-0 truncate text-left text-[12.5px] font-mono text-muted">
                    {{ form.icon }}
                  </span>
                </UButton>
                <div
                  v-if="iconPickerOpen"
                  class="mt-2 p-2.5 border border-default rounded-[11px] bg-default shadow"
                >
                  <p class="text-[11.5px] text-muted mb-2">
                    {{ t('settings.guide.form.iconHint') }}
                  </p>
                  <div class="grid grid-cols-6 gap-1.5">
                    <UButton
                      v-for="choice in GUIDE_ICON_CHOICES"
                      :key="choice"
                      :color="choice === form.icon ? 'primary' : 'neutral'"
                      :variant="choice === form.icon ? 'subtle' : 'outline'"
                      :icon="choice"
                      :title="choice"
                      :aria-label="choice"
                      class="justify-center h-9"
                      @click="form.icon = choice; iconPickerOpen = false"
                    />
                  </div>
                </div>
              </UFormField>

              <UFormField :label="t('settings.guide.form.fieldOrder')">
                <div class="flex items-center gap-1.5">
                  <UButton
                    color="neutral"
                    variant="outline"
                    icon="i-lucide-minus"
                    :aria-label="t('settings.guide.form.orderDecrease')"
                    @click="orderDelta(-1)"
                  />
                  <NumberInput
                    v-model="form.sort_order"
                    data-testid="guide-order"
                    class="flex-1 min-w-0"
                  />
                  <UButton
                    color="neutral"
                    variant="outline"
                    icon="i-lucide-plus"
                    :aria-label="t('settings.guide.form.orderIncrease')"
                    @click="orderDelta(1)"
                  />
                </div>
                <p class="mt-1.5 text-[11.5px] leading-snug text-muted">
                  {{ t('settings.guide.form.orderHint') }}
                </p>
              </UFormField>

              <UFormField :label="t('settings.guide.form.fieldStatus')">
                <div class="flex gap-0.5 p-[3px] bg-muted rounded-[10px]">
                  <UButton
                    v-for="choice in statusChoices"
                    :key="choice.value"
                    :color="form.status === choice.value ? 'primary' : 'neutral'"
                    :variant="form.status === choice.value ? 'solid' : 'ghost'"
                    :icon="choice.icon"
                    :disabled="isCreate"
                    :data-testid="`guide-status-${choice.value}`"
                    size="sm"
                    class="flex-1 justify-center"
                    @click="form.status = choice.value"
                  >
                    {{ choice.label }}
                  </UButton>
                </div>
                <p class="mt-1.5 text-[11.5px] leading-snug text-muted">
                  {{ isCreate
                    ? t('settings.guide.form.statusHintCreate')
                    : t('settings.guide.form.statusHint') }}
                </p>
              </UFormField>
            </div>
          </div>
        </section>

        <!-- ── Section 2: numbered steps ───────────────────────────────── -->
        <section>
          <div class="flex items-center gap-2.5 mb-3 flex-wrap">
            <span class="text-[11px] font-semibold font-mono tracking-[0.12em] uppercase text-dimmed">
              {{ t('settings.guide.form.secSteps') }}
            </span>
            <div class="flex-1 border-t border-default min-w-[20px]" />
            <USwitch
              v-model="showStepEn"
              size="sm"
              :label="t('settings.guide.form.showEn')"
              data-testid="guide-step-en-toggle"
            />
          </div>

          <div class="flex flex-col gap-2">
            <div
              v-for="(step, idx) in form.steps"
              :key="idx"
              class="flex items-start gap-2.5 p-2.5 border border-default rounded-[11px] bg-default"
              data-testid="guide-step-row"
            >
              <span class="size-6 mt-1.5 rounded-full bg-muted text-default text-[12px] font-mono font-semibold flex items-center justify-center flex-none">
                {{ idx + 1 }}
              </span>
              <div class="flex-1 min-w-0 flex flex-col gap-1.5">
                <UTextarea
                  v-model="step.text_id"
                  :rows="1"
                  autoresize
                  :placeholder="t('settings.guide.form.stepPlaceholder')"
                  :color="stepErrors.has(idx) ? 'error' : undefined"
                  class="w-full"
                />
                <p
                  v-if="stepErrors.has(idx)"
                  class="text-[11.5px] text-error"
                >
                  {{ t('settings.guide.form.required') }}
                </p>
                <div
                  v-if="showStepEn"
                  class="flex items-start gap-1.5"
                >
                  <span class="mt-2 px-1.5 py-px text-[9.5px] font-mono font-semibold rounded bg-elevated text-muted flex-none">EN</span>
                  <UTextarea
                    v-model="step.text_en"
                    :rows="1"
                    autoresize
                    :placeholder="t('settings.guide.form.fallbackPlaceholder')"
                    class="flex-1 min-w-0"
                  />
                </div>
              </div>
              <div class="flex flex-col gap-0.5 flex-none">
                <UButton
                  color="neutral"
                  variant="outline"
                  size="xs"
                  icon="i-lucide-arrow-up"
                  :disabled="idx === 0"
                  :aria-label="t('settings.guide.actions.moveUp')"
                  @click="moveStep(idx, -1)"
                />
                <UButton
                  color="neutral"
                  variant="outline"
                  size="xs"
                  icon="i-lucide-arrow-down"
                  :disabled="idx === form.steps.length - 1"
                  :aria-label="t('settings.guide.actions.moveDown')"
                  @click="moveStep(idx, 1)"
                />
                <UButton
                  color="error"
                  variant="outline"
                  size="xs"
                  icon="i-lucide-trash-2"
                  :aria-label="t('settings.guide.actions.delete')"
                  @click="removeStep(idx)"
                />
              </div>
            </div>

            <p
              v-if="!form.steps.length"
              class="p-4 border border-dashed border-accented rounded-[11px] text-center text-[12.5px] leading-relaxed text-muted"
            >
              {{ t('settings.guide.form.noSteps') }}
            </p>

            <UButton
              color="neutral"
              variant="ghost"
              icon="i-lucide-plus"
              data-testid="guide-add-step"
              class="w-full justify-center border-[1.5px] border-dashed border-accented rounded-[11px] py-2.5"
              @click="addStep"
            >
              {{ t('settings.guide.form.addStep') }}
            </UButton>
          </div>
        </section>

        <!-- ── Section 3: attachments (owned by the parent) ────────────── -->
        <section>
          <div class="flex items-center gap-2.5 mb-3">
            <span class="text-[11px] font-semibold font-mono tracking-[0.12em] uppercase text-dimmed">
              {{ t('settings.guide.form.secAttachments') }}
            </span>
            <div class="flex-1 border-t border-default" />
            <span class="text-[11.5px] text-muted flex-none">{{ t('settings.guide.attachments.limitHint') }}</span>
          </div>
          <slot name="attachments">
            <p class="p-4 border border-dashed border-accented rounded-[11px] text-center text-[12.5px] leading-relaxed text-muted">
              {{ t('settings.guide.form.attachmentsAfterSave') }}
            </p>
          </slot>
        </section>
      </div>
    </template>

    <template #footer>
      <div class="flex items-center justify-between gap-3 w-full flex-wrap">
        <p class="flex items-start gap-1.5 text-[12px] leading-snug text-muted flex-1 min-w-0">
          <UIcon
            name="i-lucide-info"
            class="size-3.5 mt-px flex-none"
          />
          <span>{{ t('settings.guide.form.saveNote') }}</span>
        </p>
        <div class="flex gap-2 flex-none">
          <UButton
            color="neutral"
            variant="ghost"
            @click="open = false"
          >
            {{ t('common.cancel') }}
          </UButton>
          <UButton
            :loading="props.loading"
            data-testid="guide-save"
            @click="onSubmit"
          >
            {{ t('common.save') }}
          </UButton>
        </div>
      </div>
    </template>
  </USlideover>
</template>
