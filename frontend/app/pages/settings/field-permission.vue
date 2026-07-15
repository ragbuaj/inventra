<script setup lang="ts">
import type { RoleColumn, EntityRules } from '~/composables/api/useFieldPermission'
import { useFieldPermission } from '~/composables/api/useFieldPermission'
import type { CellRule } from '~/constants/fieldCatalog'

definePageMeta({ middleware: 'can', permission: 'fieldperm.manage' })

const { t, te } = useI18n()
const toast = useToast()
const { getEntities, load, getRules, saveRules } = useFieldPermission()

const entities = getEntities() // [{ key, fields }]
const roleCols = ref<RoleColumn[]>([])
const entityKey = ref(entities[0]?.key ?? 'assets')
const rules = ref<EntityRules>({})
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const dirty = ref(false)

function entityLabel(key: string): string {
  const k = `settings.fieldPermission.entity.${key}`
  return te(k) ? t(k) : key
}
function fieldLabel(field: string): string {
  const k = `settings.fieldPermission.field.${field}`
  return te(k) ? t(k) : field
}

const entityOptions = computed(() => entities.map(e => ({ value: e.key, label: entityLabel(e.key) })))
const currentEntity = computed(() => entities.find(e => e.key === entityKey.value))

const filteredFields = computed(() => {
  const q = search.value.trim().toLowerCase()
  return (currentEntity.value?.fields ?? []).filter(f => !q || f.toLowerCase().includes(q) || fieldLabel(f).toLowerCase().includes(q))
})

function isExplicit(field: string): boolean {
  return !!rules.value[field]
}
// Default-allow: a cell with no explicit restriction is view+edit.
function cell(field: string, roleId: string): CellRule {
  const fr = rules.value[field]
  if (fr && fr[roleId]) return fr[roleId]
  return { view: true, edit: true }
}
function ensure(field: string) {
  if (rules.value[field]) return
  const fr: Record<string, CellRule> = {}
  for (const c of roleCols.value) fr[c.key] = { view: true, edit: true }
  rules.value = { ...rules.value, [field]: fr }
}
function toggleView(field: string, roleId: string) {
  ensure(field)
  const fr = rules.value[field]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleId] ?? { view: true, edit: true }) }
  cur.view = !cur.view
  if (!cur.view) cur.edit = false
  fr[roleId] = cur
  dirty.value = true
}
function toggleEdit(field: string, roleId: string) {
  ensure(field)
  const fr = rules.value[field]
  if (!fr) return
  const cur: CellRule = { ...(fr[roleId] ?? { view: true, edit: true }) }
  cur.edit = !cur.edit
  if (cur.edit) cur.view = true
  fr[roleId] = cur
  dirty.value = true
}
function resetField(field: string) {
  const { [field]: _omit, ...rest } = rules.value
  rules.value = rest
  dirty.value = true
}

function refreshRules() {
  rules.value = getRules(entityKey.value)
  dirty.value = false
}

async function load_() {
  loading.value = true
  loadFailed.value = false
  try {
    roleCols.value = await load()
    refreshRules()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function onEntityChange() {
  search.value = ''
  refreshRules()
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    await saveRules(entityKey.value, rules.value, roleCols.value.map(c => c.key))
    refreshRules()
    toast.add({ title: t('settings.fieldPermission.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

onMounted(() => load_())
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

    <!-- Loading state -->
    <div
      v-if="loading"
      class="flex items-center justify-center py-20"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-6 animate-spin text-muted"
      />
    </div>

    <!-- Load error state -->
    <div
      v-else-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('settings.fieldPermission.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load_"
      >
        {{ t('settings.fieldPermission.retry') }}
      </UButton>
    </div>

    <!-- Controls + Matrix -->
    <template v-else>
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
                :key="fl"
                class="border-t border-default hover:bg-muted/50"
              >
                <td class="px-4 py-[11px] sticky left-0 bg-default z-[1]">
                  <div class="flex items-center gap-2.5">
                    <div class="min-w-0">
                      <div class="text-[12.5px] font-semibold font-mono">
                        {{ fl }}
                      </div>
                      <div class="text-[11.5px] text-dimmed">
                        {{ fieldLabel(fl) }}
                      </div>
                    </div>
                    <UBadge
                      v-if="!isExplicit(fl)"
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
                      @click="resetField(fl)"
                    />
                  </div>
                </td>
                <td
                  v-for="c in roleCols"
                  :key="c.key"
                  class="px-3 py-2.5 text-center border-s border-default"
                >
                  <FieldpermFieldPermToggle
                    :view="cell(fl, c.key).view"
                    :edit="cell(fl, c.key).edit"
                    :dimmed="!isExplicit(fl)"
                    @toggle-view="toggleView(fl, c.key)"
                    @toggle-edit="toggleEdit(fl, c.key)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div
          v-if="filteredFields.length === 0"
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
    </template>
  </div>
</template>
