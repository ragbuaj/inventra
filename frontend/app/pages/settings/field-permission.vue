<script setup lang="ts">
import type { FieldRoleItem, RoleRules } from '~/composables/api/useFieldPermission'
import { useFieldPermission } from '~/composables/api/useFieldPermission'
import type { CellRule } from '~/constants/fieldCatalog'

definePageMeta({ middleware: 'can', permission: 'fieldperm.manage' })

const { t, te } = useI18n()
const toast = useToast()
const { getEntities, listRoles, getRoleRules, saveRoleRules } = useFieldPermission()

const entities = getEntities() // [{ key, fields }]
const roles = ref<FieldRoleItem[]>([])
// Per-role rule drafts, cached on first selection. Edits mutate the draft in
// place, so switching roles or entities never loses unsaved changes — Save
// flushes every dirty role at once.
const rulesMap = ref<Record<string, RoleRules>>({})
const dirtyIds = ref(new Set<string>())
const selectedId = ref('')
const roleSearch = ref('')
const entityKey = ref(entities[0]?.key ?? 'assets')
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)
const rulesLoading = ref(false)
const rulesFailed = ref(false)
const saving = ref(false)
// Mobile drill-down (below lg): false = role list full-width, true = editor
// pane full-width with a back button. On lg+ both panes are always visible.
// Stays false on the initial auto-select so mobile starts on the list.
const showDetailMobile = ref(false)

const dirty = computed(() => dirtyIds.value.size > 0)
const selectedRole = computed(() => roles.value.find(r => r.id === selectedId.value))
const selectedRules = computed(() => rulesMap.value[selectedId.value])

const filteredRoles = computed(() => {
  const q = roleSearch.value.trim().toLowerCase()
  if (!q) return roles.value
  return roles.value.filter(r => r.name.toLowerCase().includes(q) || r.code.toLowerCase().includes(q))
})

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
  return !!selectedRules.value?.[entityKey.value]?.[field]
}
// Default-allow: a cell with no explicit restriction is view+edit.
function cell(field: string): CellRule {
  return selectedRules.value?.[entityKey.value]?.[field] ?? { view: true, edit: true }
}
function setCell(field: string, cr: CellRule) {
  const rr = selectedRules.value
  if (!rr) return
  ;(rr[entityKey.value] ??= {})[field] = cr
  dirtyIds.value.add(selectedId.value)
}
function toggleView(field: string) {
  const cur: CellRule = { ...cell(field) }
  cur.view = !cur.view
  if (!cur.view) cur.edit = false
  setCell(field, cur)
}
function toggleEdit(field: string) {
  const cur: CellRule = { ...cell(field) }
  cur.edit = !cur.edit
  if (cur.edit) cur.view = true
  setCell(field, cur)
}
function resetField(field: string) {
  const ent = selectedRules.value?.[entityKey.value]
  if (!ent || !(field in ent)) return
  const { [field]: _omit, ...rest } = ent
  selectedRules.value![entityKey.value] = rest
  dirtyIds.value.add(selectedId.value)
}

async function fetchRules(id: string) {
  rulesLoading.value = true
  rulesFailed.value = false
  try {
    rulesMap.value = { ...rulesMap.value, [id]: await getRoleRules(id) }
  } catch {
    rulesFailed.value = true
  } finally {
    rulesLoading.value = false
  }
}

async function selectRole(id: string, opts?: { fromLoad?: boolean }) {
  selectedId.value = id
  if (!opts?.fromLoad) showDetailMobile.value = true
  if (!rulesMap.value[id]) await fetchRules(id)
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    roles.value = await listRoles()
    rulesMap.value = {}
    dirtyIds.value = new Set()
    if (roles.value.length && roles.value[0]) await selectRole(roles.value[0].id, { fromLoad: true })
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function onEntityChange() {
  search.value = ''
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    const ids = [...dirtyIds.value]
    await Promise.all(ids.map((id) => {
      const rr = rulesMap.value[id]
      return rr ? saveRoleRules(id, rr) : Promise.resolve()
    }))
    dirtyIds.value = new Set()
    toast.add({ title: t('settings.fieldPermission.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

onMounted(() => load())
</script>

<template>
  <div class="-mx-4 -my-5 sm:-mx-6 sm:-my-6 lg:-mx-8 lg:-my-7 h-[calc(100%+2.5rem)] sm:h-[calc(100%+3rem)] lg:h-[calc(100%+3.5rem)] flex overflow-hidden">
    <div
      v-if="loading"
      class="flex-1 flex items-center justify-center"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-6 animate-spin text-muted"
      />
    </div>

    <div
      v-else-if="loadFailed"
      class="flex-1 flex flex-col items-center justify-center gap-3 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('settings.fieldPermission.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('settings.fieldPermission.retry') }}
      </UButton>
    </div>

    <template v-else>
      <!-- Role list pane -->
      <div
        class="w-full lg:w-[300px] flex-none lg:border-e border-default bg-default flex-col overflow-hidden"
        :class="showDetailMobile ? 'hidden lg:flex' : 'flex'"
      >
        <div class="flex-none px-4 pt-4 pb-3">
          <h1 class="font-bold text-[15px] mb-0.5">
            {{ t('settings.fieldPermission.title') }}
          </h1>
          <p class="text-xs text-muted">
            {{ t('settings.fieldPermission.subtitle') }}
          </p>
          <UInput
            v-model="roleSearch"
            icon="i-lucide-search"
            size="sm"
            :placeholder="t('settings.fieldPermission.roleSearchPlaceholder')"
            class="w-full mt-3"
            data-testid="fieldperm-role-search"
          />
        </div>

        <div class="flex-1 overflow-y-auto px-[10px] pb-3">
          <button
            v-for="r in filteredRoles"
            :key="r.id"
            type="button"
            class="flex items-center gap-[10px] w-full px-[11px] py-[10px] mb-[3px] rounded-[9px] border text-left transition-colors cursor-pointer hover:border-primary"
            :class="r.id === selectedId
              ? 'border-primary bg-primary/10'
              : 'border-default bg-default'"
            :data-testid="`fieldperm-role-item-${r.code}`"
            @click="selectRole(r.id)"
          >
            <span
              class="size-8 rounded-lg flex items-center justify-center flex-none"
              :class="r.id === selectedId ? 'bg-primary/20 text-primary' : 'bg-muted text-muted'"
            >
              <UIcon
                name="i-lucide-eye-off"
                class="size-4"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div
                class="font-semibold text-[13.5px] truncate"
                :class="r.id === selectedId ? 'text-primary' : 'text-default'"
              >
                {{ r.name }}
              </div>
            </div>
            <span
              v-if="dirtyIds.has(r.id)"
              class="size-[7px] rounded-full bg-warning flex-none"
              :title="t('settings.fieldPermission.unsavedChanges')"
            />
          </button>
        </div>
      </div>

      <!-- Editor pane (mobile: only visible after selecting a role) -->
      <div
        class="flex-1 flex-col min-w-0 bg-muted/40"
        :class="showDetailMobile ? 'flex' : 'hidden lg:flex'"
      >
        <!-- mobile back bar -->
        <div class="lg:hidden flex-none border-b border-default bg-default px-3 py-2">
          <UButton
            icon="i-lucide-arrow-left"
            color="neutral"
            variant="ghost"
            size="sm"
            :label="t('common.back')"
            data-testid="fieldperm-back"
            @click="() => { showDetailMobile = false }"
          />
        </div>

        <div class="flex-none flex items-center justify-between gap-3.5 flex-wrap px-4 py-4 lg:px-7 lg:py-[18px] border-b border-default bg-default">
          <div class="min-w-0">
            <h2 class="text-[19px] font-bold tracking-tight truncate">
              {{ selectedRole?.name }}
            </h2>
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
              data-testid="fieldperm-save"
              @click="save"
            >
              {{ t('settings.fieldPermission.save') }}
            </UButton>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto px-4 py-4 lg:px-7 lg:py-[22px]">
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
              class="flex-1 min-w-[180px] max-w-[320px]"
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

          <div
            v-if="rulesLoading"
            class="flex items-center justify-center py-16"
          >
            <UIcon
              name="i-lucide-loader-circle"
              class="size-6 animate-spin text-muted"
            />
          </div>

          <div
            v-else-if="rulesFailed"
            class="flex flex-col items-center justify-center gap-3 py-16 text-muted"
          >
            <UIcon
              name="i-lucide-circle-alert"
              class="size-6"
            />
            <span class="text-sm">{{ t('settings.fieldPermission.loadError') }}</span>
            <UButton
              color="neutral"
              variant="subtle"
              @click="fetchRules(selectedId)"
            >
              {{ t('settings.fieldPermission.retry') }}
            </UButton>
          </div>

          <template v-else-if="selectedRules">
            <!-- Field rows for the selected role -->
            <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
              <div
                v-for="(fl, i) in filteredFields"
                :key="fl"
                class="flex items-center justify-between gap-3 px-4 py-[11px] hover:bg-muted/50"
                :class="i > 0 ? 'border-t border-default' : ''"
                :data-testid="`fieldperm-row-${fl}`"
              >
                <div class="flex items-center gap-2.5 min-w-0">
                  <div class="min-w-0">
                    <div class="text-[12.5px] font-semibold font-mono truncate">
                      {{ fl }}
                    </div>
                    <div class="text-[11.5px] text-dimmed truncate">
                      {{ fieldLabel(fl) }}
                    </div>
                  </div>
                  <UBadge
                    v-if="!isExplicit(fl)"
                    color="neutral"
                    variant="subtle"
                    size="sm"
                    class="rounded-full flex-none"
                  >
                    {{ t('settings.fieldPermission.defaultTag') }}
                  </UBadge>
                  <UButton
                    v-else
                    icon="i-lucide-rotate-ccw"
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    class="flex-none"
                    :title="t('settings.fieldPermission.resetField')"
                    @click="resetField(fl)"
                  />
                </div>
                <FieldpermFieldPermToggle
                  :view="cell(fl).view"
                  :edit="cell(fl).edit"
                  :dimmed="!isExplicit(fl)"
                  @toggle-view="toggleView(fl)"
                  @toggle-edit="toggleEdit(fl)"
                />
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
      </div>
    </template>
  </div>
</template>
