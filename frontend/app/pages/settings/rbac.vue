<script setup lang="ts">
import type { RoleView, ModuleView } from '~/composables/api/useRbac'
import { useRbac } from '~/composables/api/useRbac'

definePageMeta({ middleware: 'can', permission: 'role.manage' })

const { t } = useI18n()
const toast = useToast()
const { getCatalog, listRoles, getRolePermissions, createRole, updateRolePermissions } = useRbac()

const roles = ref<RoleView[]>([])
const modules = ref<ModuleView[]>([])
const selectedId = ref('')
const draft = ref<string[]>([])
const dirty = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
// Mobile drill-down (below lg): false = role list full-width, true = permission
// pane full-width with a back button. On lg+ both panes are always visible.
// Stays false on the initial auto-select so mobile starts on the list.
const showDetailMobile = ref(false)

const selectedRole = computed(() => roles.value.find(r => r.id === selectedId.value))
const saveDisabled = computed(() => !dirty.value)

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const [mods, roleList] = await Promise.all([getCatalog(), listRoles()])
    modules.value = mods
    // Eager-load each role's permissions (parallel) so the list count + matrix are populated.
    const permsList = await Promise.all(roleList.map(r => getRolePermissions(r.id)))
    roleList.forEach((r, i) => {
      r.perms = permsList[i] ?? []
    })
    roles.value = roleList
    if (!selectedId.value || !roles.value.some(r => r.id === selectedId.value)) {
      const mgr = roles.value.find(r => r.code === 'manager')
      selectedId.value = mgr?.id ?? roles.value[0]?.id ?? ''
    }
    draft.value = [...(selectedRole.value?.perms ?? [])]
    dirty.value = false
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function selectRole(id: string) {
  selectedId.value = id
  showDetailMobile.value = true
  draft.value = [...(roles.value.find(r => r.id === id)?.perms ?? [])]
  dirty.value = false
}

function togglePerm(code: string) {
  draft.value = draft.value.includes(code) ? draft.value.filter(c => c !== code) : [...draft.value, code]
  dirty.value = true
}

function toggleModule(modKey: string) {
  const mod = modules.value.find(m => m.key === modKey)
  if (!mod) return
  const ids = mod.perms.map(p => p.code)
  const allOn = ids.every(id => draft.value.includes(id))
  draft.value = allOn ? draft.value.filter(c => !ids.includes(c)) : [...new Set([...draft.value, ...ids])]
  dirty.value = true
}

async function save() {
  if (saveDisabled.value) return
  saving.value = true
  try {
    await updateRolePermissions(selectedId.value, draft.value)
    const r = roles.value.find(x => x.id === selectedId.value)
    if (r) r.perms = [...draft.value]
    dirty.value = false
    toast.add({ title: t('settings.rbac.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

// Add Role modal. NO_COPY sentinel — Nuxt UI Select rejects empty-string values.
const NO_COPY = '__none__'
const addOpen = ref(false)
const addForm = reactive({ name: '', copyFromId: NO_COPY, desc: '' })
const addError = ref('')
const creating = ref(false)

const copyOptions = computed(() => [
  { value: NO_COPY, label: t('settings.rbac.add.copyNone') },
  ...roles.value.map(r => ({ value: r.id, label: r.name }))
])

function openAdd() {
  addForm.name = ''
  addForm.copyFromId = NO_COPY
  addForm.desc = ''
  addError.value = ''
  addOpen.value = true
}

async function submitAdd() {
  if (!addForm.name.trim()) {
    addError.value = t('settings.rbac.add.required')
    return
  }
  creating.value = true
  try {
    const created = await createRole({
      name: addForm.name,
      copyFromId: addForm.copyFromId !== NO_COPY ? addForm.copyFromId : undefined,
      description: addForm.desc
    })
    roles.value.push(created)
    selectRole(created.id)
    addOpen.value = false
    toast.add({ title: t('settings.rbac.add.createdToast'), color: 'success', icon: 'i-lucide-plus' })
  } catch (err: unknown) {
    // 409 = duplicate code/name -> inline form error instead of a generic toast.
    if ((err as { statusCode?: number }).statusCode === 409) addError.value = t('settings.rbac.add.conflict')
    else addError.value = t('settings.rbac.loadError')
  } finally {
    creating.value = false
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
      <span class="text-sm">{{ t('settings.rbac.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('settings.rbac.retry') }}
      </UButton>
    </div>

    <template v-else>
      <RbacRoleList
        :roles="roles"
        :selected-id="selectedId"
        :mobile-hidden="showDetailMobile"
        @select="selectRole"
        @add="openAdd"
      />

      <!-- Right pane: header + matrix (mobile: only visible after selecting a role) -->
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
            data-testid="rbac-back"
            @click="() => { showDetailMobile = false }"
          />
        </div>
        <div class="flex-none flex items-center justify-between gap-3.5 flex-wrap px-4 py-4 lg:px-7 lg:py-[18px] border-b border-default bg-default">
          <div class="min-w-0">
            <div class="flex items-center gap-[9px] flex-wrap">
              <h1 class="text-[19px] font-bold tracking-tight">
                {{ selectedRole?.name }}
              </h1>
              <UBadge
                v-if="selectedRole?.is_system"
                color="neutral"
                variant="subtle"
                class="rounded-full gap-1"
              >
                <UIcon
                  name="i-lucide-lock"
                  class="size-3"
                />
                {{ t('settings.rbac.systemBadge') }}
              </UBadge>
              <UBadge
                v-else
                color="primary"
                variant="subtle"
                class="rounded-full"
              >
                {{ t('settings.rbac.customBadge') }}
              </UBadge>
            </div>
            <div class="text-[13px] text-muted mt-[3px]">
              {{ selectedRole?.description }}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <span
              v-if="dirty"
              class="inline-flex items-center gap-1.5 text-[12.5px] font-medium text-warning"
            >
              <span class="size-[7px] rounded-full bg-warning" />
              {{ t('settings.rbac.unsavedChanges') }}
            </span>
            <UButton
              icon="i-lucide-save"
              :disabled="saveDisabled"
              :loading="saving"
              @click="save"
            >
              {{ t('settings.rbac.saveChanges') }}
            </UButton>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto px-4 py-4 lg:px-7 lg:py-[22px]">
          <div
            v-if="selectedRole?.is_system"
            class="flex gap-[11px] items-center px-3.5 py-3 mb-[18px] rounded-[11px] bg-muted border border-default"
          >
            <UIcon
              name="i-lucide-lock"
              class="size-[17px] text-muted flex-none"
            />
            <span class="text-[13px] leading-snug text-muted">{{ t('settings.rbac.lockNote') }}</span>
          </div>

          <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
            <RbacPermissionCard
              v-for="m in modules"
              :key="m.key"
              :module="m"
              :granted="draft"
              @toggle="togglePerm"
              @toggle-all="toggleModule(m.key)"
            />
          </div>
        </div>
      </div>
    </template>

    <!-- Add Role modal -->
    <UModal
      v-model:open="addOpen"
      :title="t('settings.rbac.add.title')"
      :description="t('settings.rbac.add.subtitle')"
    >
      <template #body>
        <div class="space-y-4">
          <UFormField
            :label="t('settings.rbac.add.roleName')"
            required
            :error="addError"
          >
            <UInput
              v-model="addForm.name"
              :placeholder="t('settings.rbac.add.namePlaceholder')"
              class="w-full"
            />
          </UFormField>

          <UFormField :label="t('settings.rbac.add.copyFrom')">
            <USelect
              v-model="addForm.copyFromId"
              :items="copyOptions"
              class="w-full"
            />
            <template #hint>
              <span class="text-xs text-dimmed mt-1">{{ t('settings.rbac.add.copyNote') }}</span>
            </template>
          </UFormField>

          <UFormField :label="t('settings.rbac.add.description')">
            <UInput
              v-model="addForm.desc"
              :placeholder="t('settings.rbac.add.descPlaceholder')"
              class="w-full"
            />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex justify-end gap-2 w-full">
          <UButton
            color="neutral"
            variant="ghost"
            @click="() => { addOpen = false }"
          >
            {{ t('common.cancel') }}
          </UButton>
          <UButton
            :loading="creating"
            @click="submitAdd"
          >
            {{ t('settings.rbac.add.create') }}
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>
