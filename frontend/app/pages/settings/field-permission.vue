<script setup lang="ts">
import type { EntityRules, CellRule } from '~/mock/fieldPermission'
import type { EntityView, RoleColumn } from '~/composables/api/useFieldPermission'
import { useFieldPermission } from '~/composables/api/useFieldPermission'
import { FIELD_ROLE_KEYS } from '~/mock/fieldPermission'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

type Locale = 'id' | 'en'

const { t, locale } = useI18n()
const toast = useToast()
const { getEntities, getRoleColumns, getRules, saveRules } = useFieldPermission()

const entities = ref<EntityView[]>([])
const roleCols = ref<RoleColumn[]>([])
const entityKey = ref('aset')
const rules = ref<EntityRules>({})
const search = ref('')
const loading = ref(true)
const saving = ref(false)
const dirty = ref(false)

const entityOptions = computed(() => entities.value.map(e => ({ value: e.key, label: e.label })))
const currentEntity = computed(() => entities.value.find(e => e.key === entityKey.value))

const filteredFields = computed(() => {
  const q = search.value.trim().toLowerCase()
  return (currentEntity.value?.fields ?? []).filter(fl => !q || fl.code.toLowerCase().includes(q) || fl.label.toLowerCase().includes(q))
})

function isExplicit(code: string): boolean {
  return !!rules.value[code]
}
function cell(code: string, roleKey: string): CellRule {
  const fr = rules.value[code]
  if (fr) return fr[roleKey] ?? { view: false, edit: false }
  return { view: true, edit: true }
}
function ensure(code: string) {
  if (rules.value[code]) return
  const fr: Record<string, CellRule> = {}
  for (const k of FIELD_ROLE_KEYS) fr[k] = { view: true, edit: true }
  rules.value = { ...rules.value, [code]: fr }
}
function toggleView(code: string, roleKey: string) {
  ensure(code)
  const fr = rules.value[code]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleKey] ?? { view: false, edit: false }) }
  cur.view = !cur.view
  if (!cur.view) cur.edit = false
  fr[roleKey] = cur
  dirty.value = true
}
function toggleEdit(code: string, roleKey: string) {
  ensure(code)
  const fr = rules.value[code]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleKey] ?? { view: false, edit: false }) }
  cur.edit = !cur.edit
  if (cur.edit) cur.view = true
  fr[roleKey] = cur
  dirty.value = true
}
function resetField(code: string) {
  const { [code]: _omit, ...rest } = rules.value
  rules.value = rest
  dirty.value = true
}

async function loadEntityRules() {
  loading.value = true
  rules.value = await getRules(entityKey.value)
  dirty.value = false
  loading.value = false
}

function onEntityChange() {
  search.value = ''
  loadEntityRules()
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    await saveRules(entityKey.value, rules.value)
    dirty.value = false
    toast.add({ title: t('settings.fieldPermission.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

function loadMeta() {
  entities.value = getEntities(locale.value as Locale)
  roleCols.value = getRoleColumns(locale.value as Locale)
}

watch(locale, () => {
  loadMeta()
})

onMounted(() => {
  loadMeta()
  loadEntityRules()
})
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('settings.fieldPermission.title') }}
        </h1>
        <p class="max-w-[620px] text-sm leading-relaxed text-muted">
          {{ t('settings.fieldPermission.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <span
          v-if="dirty"
          class="inline-flex items-center gap-1.5 text-[12.5px] font-medium text-warning"
        >
          <span class="size-[7px] rounded-full bg-warning" />
          {{ t('settings.fieldPermission.unsavedChanges') }}
        </span>
        <UButton
          icon="i-lucide-save"
          :disabled="!dirty"
          :loading="saving"
          @click="save"
        >
          {{ t('settings.fieldPermission.save') }}
        </UButton>
      </div>
    </div>

    <!-- Controls -->
    <div class="flex items-center gap-3 flex-wrap mb-3.5">
      <div class="flex items-center gap-2.5">
        <span class="text-[13px] font-medium text-muted">{{ t('settings.fieldPermission.entityLabel') }}</span>
        <USelect
          v-model="entityKey"
          :items="entityOptions"
          class="min-w-[160px]"
          @update:model-value="onEntityChange"
        />
      </div>
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('settings.fieldPermission.searchPlaceholder')"
        class="flex-1 min-w-[200px] max-w-[320px]"
      />
      <div class="flex items-center gap-3.5 ms-auto">
        <span class="inline-flex items-center gap-1.5 text-xs font-medium text-muted">
          <span class="size-[18px] rounded bg-info/10 text-info flex items-center justify-center">
            <UIcon
              name="i-lucide-eye"
              class="size-[11px]"
            />
          </span>
          {{ t('settings.fieldPermission.viewLabel') }}
        </span>
        <span class="inline-flex items-center gap-1.5 text-xs font-medium text-muted">
          <span class="size-[18px] rounded bg-primary/10 text-primary flex items-center justify-center">
            <UIcon
              name="i-lucide-pencil"
              class="size-[11px]"
            />
          </span>
          {{ t('settings.fieldPermission.editLabel') }}
        </span>
      </div>
    </div>

    <!-- Matrix -->
    <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
      <div class="overflow-x-auto rounded-[13px]">
        <table class="w-full border-collapse whitespace-nowrap">
          <thead>
            <tr class="bg-muted">
              <th class="text-left px-4 py-3 text-xs font-semibold uppercase text-muted sticky left-0 bg-muted z-[2] min-w-[230px]">
                {{ t('settings.fieldPermission.fieldColumn') }}
              </th>
              <th
                v-for="c in roleCols"
                :key="c.key"
                class="text-center px-3.5 py-3 text-[11.5px] font-semibold uppercase text-muted border-s border-default"
              >
                {{ c.label }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="fl in filteredFields"
              :key="fl.code"
              class="border-t border-default hover:bg-muted/50"
            >
              <td class="px-4 py-[11px] sticky left-0 bg-default z-[1]">
                <div class="flex items-center gap-2.5">
                  <div class="min-w-0">
                    <div class="text-[12.5px] font-semibold font-mono">
                      {{ fl.code }}
                    </div>
                    <div class="text-[11.5px] text-dimmed">
                      {{ fl.label }}
                    </div>
                  </div>
                  <UBadge
                    v-if="!isExplicit(fl.code)"
                    color="neutral"
                    variant="subtle"
                    size="sm"
                    class="rounded-full"
                  >
                    {{ t('settings.fieldPermission.defaultTag') }}
                  </UBadge>
                  <UButton
                    v-else
                    icon="i-lucide-rotate-ccw"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    :title="t('settings.fieldPermission.resetField')"
                    @click="resetField(fl.code)"
                  />
                </div>
              </td>
              <td
                v-for="c in roleCols"
                :key="c.key"
                class="px-3 py-2.5 text-center border-s border-default"
              >
                <FieldpermFieldPermToggle
                  :view="cell(fl.code, c.key).view"
                  :edit="cell(fl.code, c.key).edit"
                  :dimmed="!isExplicit(fl.code)"
                  @toggle-view="toggleView(fl.code, c.key)"
                  @toggle-edit="toggleEdit(fl.code, c.key)"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="!loading && filteredFields.length === 0"
        class="py-11 text-center text-[13.5px] text-dimmed"
      >
        {{ t('settings.fieldPermission.empty') }}
      </div>
    </div>

    <div class="mt-3.5 flex items-start gap-2 max-w-[780px] text-[12.5px] leading-relaxed text-dimmed">
      <UIcon
        name="i-lucide-info"
        class="size-3.5 mt-0.5 flex-none"
      />
      <span>{{ t('settings.fieldPermission.footNote') }}</span>
    </div>
  </div>
</template>
