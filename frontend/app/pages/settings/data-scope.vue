<script setup lang="ts">
import type { ScopeLevel, ScopeTone } from '~/mock/dataScope'
import type { ScopeRoleView, ScopeModuleView } from '~/composables/api/useDataScope'
import { useDataScope } from '~/composables/api/useDataScope'
import { SCOPE_LEVELS, SCOPE_LEVEL_KEYS } from '~/mock/dataScope'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

type Locale = 'id' | 'en'

const { t, locale } = useI18n()
const toast = useToast()
const { getModules, listRoles, saveScopes } = useDataScope()

const roles = ref<ScopeRoleView[]>([])
const modules = ref<ScopeModuleView[]>([])
const loading = ref(true)
const saving = ref(false)
const dirty = ref(false)

const toneDot: Record<ScopeTone, string> = {
  info: 'bg-info',
  primary: 'bg-primary',
  warning: 'bg-warning',
  neutral: 'bg-[var(--ui-text-dimmed)]'
}

const legend = computed(() => SCOPE_LEVEL_KEYS.map((k) => {
  const def = SCOPE_LEVELS[k]
  return { key: k, dot: toneDot[def.tone], desc: def.desc[locale.value as Locale] ?? def.desc.id }
}))

async function load() {
  loading.value = true
  modules.value = getModules(locale.value as Locale)
  roles.value = await listRoles(locale.value as Locale)
  dirty.value = false
  loading.value = false
}

function findRole(key: string) {
  return roles.value.find(r => r.key === key)
}
function setDefault(key: string, level: ScopeLevel) {
  const r = findRole(key)
  if (!r) return
  r.def = level
  dirty.value = true
}
function setOverride(key: string, mod: string, level: ScopeLevel) {
  const r = findRole(key)
  if (!r) return
  r.ov = { ...r.ov, [mod]: level }
  dirty.value = true
}
function clearOverride(key: string, mod: string) {
  const r = findRole(key)
  if (!r) return
  const { [mod]: _omit, ...rest } = r.ov
  r.ov = rest
  dirty.value = true
}

async function save() {
  if (!dirty.value) return
  saving.value = true
  try {
    await saveScopes(roles.value)
    dirty.value = false
    toast.add({ title: t('settings.dataScope.savedToast'), color: 'success', icon: 'i-lucide-save' })
  } finally {
    saving.value = false
  }
}

watch(locale, () => load())
onMounted(() => load())
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('settings.dataScope.title') }}
        </h1>
        <p class="max-w-[620px] text-sm leading-relaxed text-muted">
          {{ t('settings.dataScope.subtitle') }}
        </p>
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
          @click="save"
        >
          {{ t('settings.dataScope.save') }}
        </UButton>
      </div>
    </div>

    <div
      v-if="loading"
      class="flex items-center justify-center py-20"
    >
      <UIcon
        name="i-lucide-loader-circle"
        class="size-6 animate-spin text-muted"
      />
    </div>

    <template v-else>
      <!-- Legend -->
      <div class="bg-default border border-default rounded-[13px] shadow-sm p-4 mb-2">
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
        <div class="flex flex-wrap gap-[18px] mt-3 pt-3 border-t border-default">
          <div class="flex items-center gap-[7px]">
            <span class="inline-flex items-center gap-1.5 px-2 py-0.5 text-[11px] font-semibold rounded-md bg-info/10 text-info">
              <span class="size-1 rounded-full bg-info" />office
            </span>
            <span class="text-xs text-muted">{{ t('settings.dataScope.overrideHint') }}</span>
          </div>
          <div class="flex items-center gap-[7px]">
            <span class="inline-flex items-center gap-1 px-2 py-0.5 text-[11px] font-semibold rounded-md border border-dashed border-default text-muted">
              <UIcon
                name="i-lucide-chevron-right"
                class="size-2.5"
              />own
            </span>
            <span class="text-xs text-muted">{{ t('settings.dataScope.inheritHint') }}</span>
          </div>
        </div>
      </div>

      <!-- Matrix table -->
      <div class="bg-default border border-default rounded-[13px] shadow-sm mt-2 overflow-visible">
        <div class="overflow-x-auto rounded-[13px]">
          <table class="w-full border-collapse whitespace-nowrap">
            <thead>
              <tr class="bg-muted">
                <th class="text-left px-4 py-3 text-xs font-semibold uppercase text-muted sticky left-0 bg-muted z-[2]">
                  {{ t('settings.dataScope.roleColumn') }}
                </th>
                <th class="text-left px-3.5 py-3 text-xs font-semibold uppercase text-primary bg-primary/10 border-x border-default">
                  {{ t('settings.dataScope.defaultColumn') }}
                </th>
                <th
                  v-for="m in modules"
                  :key="m.key"
                  class="text-left px-3.5 py-3 text-xs font-semibold uppercase text-muted"
                >
                  {{ m.label }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="r in roles"
                :key="r.key"
                class="border-t border-default"
              >
                <td class="px-4 py-3 sticky left-0 bg-default z-[1]">
                  <div class="text-[13.5px] font-semibold">
                    {{ r.nama }}
                  </div>
                  <div class="text-[11.5px] text-dimmed">
                    {{ r.sub }}
                  </div>
                </td>
                <td class="px-3 py-2.5 bg-primary/10 border-x border-default">
                  <ScopeCell
                    :effective="r.def"
                    :selected="r.def"
                    :is-module="false"
                    :role-default="r.def"
                    @select="setDefault(r.key, $event)"
                  />
                </td>
                <td
                  v-for="m in modules"
                  :key="m.key"
                  class="px-3 py-2.5"
                >
                  <ScopeCell
                    :effective="r.ov[m.key] ?? r.def"
                    :selected="r.ov[m.key] ?? null"
                    :is-module="true"
                    :role-default="r.def"
                    @select="setOverride(r.key, m.key, $event)"
                    @clear="clearOverride(r.key, m.key)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="mt-3.5 flex items-start gap-2 max-w-[760px] text-[12.5px] leading-relaxed text-dimmed">
        <UIcon
          name="i-lucide-info"
          class="size-3.5 mt-0.5 flex-none"
        />
        <span>{{ t('settings.dataScope.footNote') }}</span>
      </div>
    </template>
  </div>
</template>
