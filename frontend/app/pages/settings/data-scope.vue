<script setup lang="ts">
import type { ScopeModuleView, ScopeRoleItem, RoleScopeView } from '~/composables/api/useDataScope'
import { useDataScope } from '~/composables/api/useDataScope'
import type { ScopeLevel, ScopeTone } from '~/constants/dataScope'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'

definePageMeta({ middleware: 'can', permission: 'scope.manage' })

const { t, te } = useI18n()
const toast = useToast()
const { getModules, listRoles, getRoleScope, saveRoleScope } = useDataScope()

const roles = ref<ScopeRoleItem[]>([])
const modules = ref<ScopeModuleView[]>([])
// Per-role scope drafts, cached on first selection. Edits mutate the draft in
// place, so switching roles never loses unsaved changes — Save flushes every
// dirty role at once.
const scopes = ref<Record<string, RoleScopeView>>({})
const dirtyIds = ref(new Set<string>())
const selectedId = ref('')
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)
const scopeLoading = ref(false)
const scopeFailed = ref(false)
const saving = ref(false)
// Mobile drill-down (below lg): false = role list full-width, true = editor
// pane full-width with a back button. On lg+ both panes are always visible.
// Stays false on the initial auto-select so mobile starts on the list.
const showDetailMobile = ref(false)

const dirty = computed(() => dirtyIds.value.size > 0)
const selectedRole = computed(() => roles.value.find(r => r.id === selectedId.value))
const selectedScope = computed(() => scopes.value[selectedId.value])

const filteredRoles = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return roles.value
  return roles.value.filter(r => r.name.toLowerCase().includes(q) || r.code.toLowerCase().includes(q) || r.sub.toLowerCase().includes(q))
})

const toneDot: Record<ScopeTone, string> = {
  info: 'bg-info',
  primary: 'bg-primary',
  warning: 'bg-warning',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const legend = computed(() => SCOPE_LEVEL_KEYS.map(k => ({
  key: k,
  dot: toneDot[SCOPE_LEVEL_TONE[k]],
  desc: t(`settings.dataScope.level.${k}`)
})))

function moduleLabel(key: string): string {
  const k = `settings.dataScope.module.${key}`
  return te(k) ? t(k) : key
}

async function fetchScope(id: string) {
  scopeLoading.value = true
  scopeFailed.value = false
  try {
    scopes.value = { ...scopes.value, [id]: await getRoleScope(id) }
  } catch {
    scopeFailed.value = true
  } finally {
    scopeLoading.value = false
  }
}

async function selectRole(id: string, opts?: { fromLoad?: boolean }) {
  selectedId.value = id
  if (!opts?.fromLoad) showDetailMobile.value = true
  if (!scopes.value[id]) await fetchScope(id)
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [mods, roleList] = await Promise.all([getModules(), listRoles()])
    modules.value = mods
    roles.value = roleList
    scopes.value = {}
    dirtyIds.value = new Set()
    if (roleList.length && roleList[0]) await selectRole(roleList[0].id, { fromLoad: true })
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function setDefault(level: ScopeLevel) {
  const s = selectedScope.value
  if (!s) return
  s.def = level
  dirtyIds.value.add(selectedId.value)
}
function setOverride(mod: string, level: ScopeLevel) {
  const s = selectedScope.value
  if (!s) return
  s.ov = { ...s.ov, [mod]: level }
  dirtyIds.value.add(selectedId.value)
}
function clearOverride(mod: string) {
  const s = selectedScope.value
  if (!s) return
  const { [mod]: _omit, ...rest } = s.ov
  s.ov = rest
  dirtyIds.value.add(selectedId.value)
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    const ids = [...dirtyIds.value]
    await Promise.all(ids.map((id) => {
      const s = scopes.value[id]
      return s ? saveRoleScope(id, s.def, s.ov) : Promise.resolve()
    }))
    dirtyIds.value = new Set()
    toast.add({ title: t('settings.dataScope.savedToast'), color: 'success', icon: 'i-lucide-save' })
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
      <span class="text-sm">{{ t('settings.dataScope.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('settings.dataScope.retry') }}
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
            {{ t('settings.dataScope.title') }}
          </h1>
          <p class="text-xs text-muted">
            {{ t('settings.dataScope.subtitle') }}
          </p>
          <UInput
            v-model="search"
            icon="i-lucide-search"
            size="sm"
            :placeholder="t('settings.dataScope.roleSearchPlaceholder')"
            class="w-full mt-3"
            data-testid="scope-role-search"
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
            :data-testid="`scope-role-item-${r.code}`"
            @click="selectRole(r.id)"
          >
            <span
              class="size-8 rounded-lg flex items-center justify-center flex-none"
              :class="r.id === selectedId ? 'bg-primary/20 text-primary' : 'bg-muted text-muted'"
            >
              <UIcon
                name="i-lucide-database"
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
              <div class="text-[11.5px] text-dimmed truncate">
                {{ r.sub }}
              </div>
            </div>
            <span
              v-if="dirtyIds.has(r.id)"
              class="size-[7px] rounded-full bg-warning flex-none"
              :title="t('settings.dataScope.unsavedChanges')"
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
            data-testid="scope-back"
            @click="() => { showDetailMobile = false }"
          />
        </div>

        <div class="flex-none flex items-center justify-between gap-3.5 flex-wrap px-4 py-4 lg:px-7 lg:py-[18px] border-b border-default bg-default">
          <div class="min-w-0">
            <h2 class="text-[19px] font-bold tracking-tight truncate">
              {{ selectedRole?.name }}
            </h2>
            <div class="text-[13px] text-muted mt-[3px] truncate">
              {{ selectedRole?.sub }}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <span
              v-if="dirty"
              class="inline-flex items-center gap-1.5 text-[12.5px] font-medium text-warning"
            >
              <span class="size-[7px] rounded-full bg-warning" />
              {{ t('settings.dataScope.unsavedChanges') }}
            </span>
            <UButton
              icon="i-lucide-save"
              :disabled="!dirty"
              :loading="saving"
              data-testid="scope-save"
              @click="save"
            >
              {{ t('settings.dataScope.save') }}
            </UButton>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto px-4 py-4 lg:px-7 lg:py-[22px]">
          <!-- Legend -->
          <div class="bg-default border border-default rounded-[13px] shadow-sm p-4 mb-4">
            <div class="flex items-center gap-2 mb-3">
              <UIcon
                name="i-lucide-info"
                class="size-[15px] text-muted"
              />
              <span class="text-[12.5px] font-semibold text-muted">{{ t('settings.dataScope.legendTitle') }}</span>
            </div>
            <div class="grid gap-2.5 [grid-template-columns:repeat(auto-fit,minmax(220px,1fr))]">
              <div
                v-for="l in legend"
                :key="l.key"
                class="flex items-start gap-2.5"
              >
                <span class="size-[22px] rounded-md bg-muted flex items-center justify-center flex-none mt-0.5">
                  <span
                    class="size-2 rounded-full"
                    :class="l.dot"
                  />
                </span>
                <div>
                  <div class="text-[12.5px] font-semibold font-mono">
                    {{ l.key }}
                  </div>
                  <div class="text-xs text-muted">
                    {{ l.desc }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="scopeLoading"
            class="flex items-center justify-center py-16"
          >
            <UIcon
              name="i-lucide-loader-circle"
              class="size-6 animate-spin text-muted"
            />
          </div>

          <div
            v-else-if="scopeFailed"
            class="flex flex-col items-center justify-center gap-3 py-16 text-muted"
          >
            <UIcon
              name="i-lucide-circle-alert"
              class="size-6"
            />
            <span class="text-sm">{{ t('settings.dataScope.loadError') }}</span>
            <UButton
              color="neutral"
              variant="subtle"
              @click="fetchScope(selectedId)"
            >
              {{ t('settings.dataScope.retry') }}
            </UButton>
          </div>

          <template v-else-if="selectedScope">
            <!-- Default scope -->
            <div class="bg-default border border-default rounded-[13px] shadow-sm p-4 mb-4">
              <div class="flex items-center justify-between gap-3 flex-wrap">
                <div class="min-w-0">
                  <div class="text-[13.5px] font-semibold">
                    {{ t('settings.dataScope.defaultColumn') }}
                  </div>
                  <div class="text-xs text-muted mt-0.5">
                    {{ t('settings.dataScope.defaultDesc') }}
                  </div>
                </div>
                <div data-testid="scope-default-cell">
                  <ScopeCell
                    :effective="selectedScope.def"
                    :selected="selectedScope.def"
                    :is-module="false"
                    :role-default="selectedScope.def"
                    @select="setDefault($event)"
                  />
                </div>
              </div>
            </div>

            <!-- Per-module overrides -->
            <div class="text-[12.5px] font-semibold text-muted mb-2">
              {{ t('settings.dataScope.modulesTitle') }}
            </div>
            <div class="grid grid-cols-1 xl:grid-cols-2 gap-2.5">
              <div
                v-for="m in modules"
                :key="m.key"
                class="bg-default border border-default rounded-[11px] px-3.5 py-2.5 flex items-center justify-between gap-3"
                :data-testid="`scope-module-row-${m.key}`"
              >
                <span class="text-[13px] font-medium truncate">{{ moduleLabel(m.key) }}</span>
                <ScopeCell
                  :effective="selectedScope.ov[m.key] ?? selectedScope.def"
                  :selected="selectedScope.ov[m.key] ?? null"
                  :is-module="true"
                  :role-default="selectedScope.def"
                  @select="setOverride(m.key, $event)"
                  @clear="clearOverride(m.key)"
                />
              </div>
            </div>

            <div class="mt-4 flex items-start gap-2 max-w-[760px] text-[12.5px] leading-relaxed text-dimmed">
              <UIcon
                name="i-lucide-info"
                class="size-3.5 mt-0.5 flex-none"
              />
              <span>{{ t('settings.dataScope.footNote') }}</span>
            </div>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>
