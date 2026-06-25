<script setup lang="ts">
import type { AuditRow } from '~/composables/api/useAudit'
import { useAudit } from '~/composables/api/useAudit'
import { AUDIT_ACTION_META, AUDIT_ENTITIES } from '~/mock/audit'

definePageMeta({ middleware: 'can', permission: 'user.manage' })

type Locale = 'id' | 'en'
const PAGE_SIZE = 8
const ALL = '__all__'

const { t, locale } = useI18n()
const toast = useToast()
const { list, actors } = useAudit()

const rows = ref<AuditRow[]>([])
const loading = ref(true)
const search = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const fActor = ref(ALL)
const fAction = ref(ALL)
const fEntity = ref(ALL)
const page = ref(1)
const openId = ref<number | null>(null)

const actorOptions = computed(() => [{ value: ALL, label: t('settings.audit.filter.allActors') }, ...actors().map(a => ({ value: a, label: a }))])
const actionOptions = computed(() => [
  { value: ALL, label: t('settings.audit.filter.allActions') },
  { value: 'create', label: t('settings.audit.action.create') },
  { value: 'update', label: t('settings.audit.action.update') },
  { value: 'delete', label: t('settings.audit.action.delete') }
])
const entityOptions = computed(() => [{ value: ALL, label: t('settings.audit.filter.allEntities') }, ...AUDIT_ENTITIES.map(e => ({ value: e, label: e }))])

const anyFilter = computed(() =>
  !!(search.value.trim() || dateFrom.value || dateTo.value || fActor.value !== ALL || fAction.value !== ALL || fEntity.value !== ALL)
)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter((r) => {
    if (q && !r.summary.toLowerCase().includes(q) && !r.actor.toLowerCase().includes(q) && !r.ref.toLowerCase().includes(q)) return false
    if (fActor.value !== ALL && r.actor !== fActor.value) return false
    if (fAction.value !== ALL && r.action !== fAction.value) return false
    if (fEntity.value !== ALL && r.entity !== fEntity.value) return false
    if (dateFrom.value && r.dateKey < dateFrom.value) return false
    if (dateTo.value && r.dateKey > dateTo.value) return false
    return true
  })
})

const total = computed(() => filtered.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const pageRows = computed(() => {
  const p = Math.min(page.value, totalPages.value)
  const start = (p - 1) * PAGE_SIZE
  return filtered.value.slice(start, start + PAGE_SIZE)
})
const pageInfo = computed(() => {
  const p = Math.min(page.value, totalPages.value)
  const from = total.value === 0 ? 0 : (p - 1) * PAGE_SIZE + 1
  const to = Math.min(p * PAGE_SIZE, total.value)
  return t('settings.audit.showing', { from, to, total: total.value })
})

function actionMeta(action: AuditRow['action']) {
  return AUDIT_ACTION_META[action]
}
function toggle(id: number) {
  openId.value = openId.value === id ? null : id
}
function resetFilters() {
  search.value = ''
  dateFrom.value = ''
  dateTo.value = ''
  fActor.value = ALL
  fAction.value = ALL
  fEntity.value = ALL
  page.value = 1
}
function comingSoon() {
  toast.add({ title: t('settings.audit.exportComingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}

async function load() {
  loading.value = true
  rows.value = await list(locale.value as Locale)
  loading.value = false
}

watch([search, dateFrom, dateTo, fActor, fAction, fEntity], () => {
  page.value = 1
})
watch(locale, () => load())
onMounted(() => load())
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ t('settings.audit.title') }}
        </h1>
        <p class="text-sm text-muted">
          {{ t('settings.audit.subtitle') }}
        </p>
      </div>
      <UButton
        icon="i-lucide-download"
        color="neutral"
        variant="outline"
        :label="t('settings.audit.export')"
        @click="comingSoon"
      />
    </div>

    <!-- Filter bar -->
    <div class="bg-default border border-default rounded-[13px] shadow-sm p-[14px] mb-4 flex items-center gap-2.5 flex-wrap">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('settings.audit.searchPlaceholder')"
        class="flex-1 min-w-[180px]"
      />
      <div class="flex items-center gap-1.5">
        <UInput
          v-model="dateFrom"
          type="date"
          :aria-label="t('settings.audit.dateFrom')"
        />
        <span class="text-dimmed">–</span>
        <UInput
          v-model="dateTo"
          type="date"
          :aria-label="t('settings.audit.dateTo')"
        />
      </div>
      <USelect
        v-model="fActor"
        :items="actorOptions"
        class="min-w-[150px] max-w-[180px]"
      />
      <USelect
        v-model="fAction"
        :items="actionOptions"
        class="min-w-[130px]"
      />
      <USelect
        v-model="fEntity"
        :items="entityOptions"
        class="min-w-[140px]"
      />
      <UButton
        v-if="anyFilter"
        color="error"
        variant="ghost"
        icon="i-lucide-x"
        @click="resetFilters"
      >
        {{ t('settings.audit.reset') }}
      </UButton>
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

    <!-- Table -->
    <div
      v-else-if="pageRows.length > 0"
      class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full border-collapse text-[13.5px]">
          <thead>
            <tr class="bg-muted">
              <th class="w-[30px] px-2 py-[11px]" />
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted whitespace-nowrap">
                {{ t('settings.audit.columns.time') }}
              </th>
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted">
                {{ t('settings.audit.columns.actor') }}
              </th>
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted">
                {{ t('settings.audit.columns.action') }}
              </th>
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted">
                {{ t('settings.audit.columns.entity') }}
              </th>
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted">
                {{ t('settings.audit.columns.summary') }}
              </th>
              <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase text-muted whitespace-nowrap">
                {{ t('settings.audit.columns.office') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <template
              v-for="r in pageRows"
              :key="r.id"
            >
              <tr
                class="border-t border-default cursor-pointer hover:bg-muted transition-colors"
                :class="openId === r.id ? 'bg-muted' : ''"
                @click="toggle(r.id)"
              >
                <td class="px-2 py-3 ps-4">
                  <UIcon
                    name="i-lucide-chevron-right"
                    class="size-[15px] text-dimmed transition-transform"
                    :class="openId === r.id ? 'rotate-90' : ''"
                  />
                </td>
                <td class="px-3.5 py-3 whitespace-nowrap">
                  <div class="text-[13px] font-medium">
                    {{ r.date }}
                  </div>
                  <div class="text-[11.5px] font-mono text-dimmed">
                    {{ r.time }}
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <div class="flex items-center gap-2.5">
                    <span class="size-[30px] rounded-full bg-primary/10 text-primary flex items-center justify-center font-bold text-[11px] flex-none">
                      {{ r.initials }}
                    </span>
                    <div class="min-w-0">
                      <div class="text-[13px] font-medium whitespace-nowrap">
                        {{ r.actor }}
                      </div>
                      <div class="text-[11.5px] text-dimmed whitespace-nowrap">
                        {{ r.role }}
                      </div>
                    </div>
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <UBadge
                    :color="actionMeta(r.action).tone"
                    variant="subtle"
                    class="rounded-full gap-1"
                  >
                    <UIcon
                      :name="actionMeta(r.action).icon"
                      class="size-3"
                    />
                    {{ t(`settings.audit.action.${r.action}`) }}
                  </UBadge>
                </td>
                <td class="px-3.5 py-3 text-muted whitespace-nowrap">
                  {{ r.entity }}
                </td>
                <td class="px-3.5 py-3 max-w-[320px]">
                  <span class="block truncate">{{ r.summary }}</span>
                </td>
                <td class="px-4 py-3 whitespace-nowrap">
                  <div class="text-[13px] text-muted">
                    {{ r.office }}
                  </div>
                  <div class="text-[11.5px] font-mono text-dimmed">
                    {{ r.ip }}
                  </div>
                </td>
              </tr>
              <tr
                v-if="openId === r.id"
                class="bg-muted"
              >
                <td
                  colspan="7"
                  class="px-4 pb-4 ps-[47px]"
                >
                  <div class="bg-elevated border border-default rounded-[10px] overflow-hidden">
                    <div class="flex items-center gap-2 px-3.5 py-2.5 border-b border-default">
                      <UIcon
                        name="i-lucide-code"
                        class="size-3.5 text-muted"
                      />
                      <span class="text-xs font-semibold text-muted">{{ t('settings.audit.diffTitle') }}</span>
                      <span class="text-[11.5px] font-mono text-dimmed">{{ r.ref }}</span>
                    </div>
                    <div class="px-3.5 pt-1.5 pb-2.5">
                      <div
                        v-for="(df, i) in r.diff"
                        :key="i"
                        class="grid [grid-template-columns:170px_1fr] gap-3 items-start py-2 border-b border-default last:border-b-0"
                      >
                        <span class="text-[12.5px] font-mono text-muted pt-0.5">{{ df.field }}</span>
                        <div class="flex items-center gap-2.5 flex-wrap">
                          <span
                            v-if="df.hasBefore"
                            class="text-[12.5px] font-mono px-2 py-0.5 rounded-md bg-error/10 text-error line-through"
                          >{{ df.before }}</span>
                          <UIcon
                            v-if="df.hasArrow"
                            name="i-lucide-arrow-right"
                            class="size-3.5 text-dimmed"
                          />
                          <span
                            v-if="df.hasAfter"
                            class="text-[12.5px] font-mono px-2 py-0.5 rounded-md bg-success/10 text-success"
                          >{{ df.after }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div class="flex items-center justify-between flex-wrap gap-2.5 px-4 py-3 border-t border-default">
        <span class="text-[13px] text-muted">{{ pageInfo }}</span>
        <div class="flex items-center gap-1.5">
          <UButton
            icon="i-lucide-chevron-left"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page <= 1"
            :aria-label="t('common.actions')"
            @click="page = Math.max(1, page - 1)"
          />
          <UButton
            v-for="p in totalPages"
            :key="p"
            :color="p === Math.min(page, totalPages) ? 'primary' : 'neutral'"
            :variant="p === Math.min(page, totalPages) ? 'solid' : 'outline'"
            size="sm"
            class="min-w-[34px] justify-center"
            @click="page = p"
          >
            {{ p }}
          </UButton>
          <UButton
            icon="i-lucide-chevron-right"
            color="neutral"
            variant="outline"
            size="sm"
            square
            :disabled="page >= totalPages"
            :aria-label="t('common.actions')"
            @click="page = Math.min(totalPages, page + 1)"
          />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else
      class="bg-default border border-default rounded-[14px] shadow-sm py-14 px-6 text-center"
    >
      <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
        <UIcon
          name="i-lucide-history"
          class="size-6"
        />
      </div>
      <div class="text-base font-semibold mb-1.5">
        {{ t('settings.audit.emptyTitle') }}
      </div>
      <div class="text-sm text-muted max-w-[320px] mx-auto mb-[18px]">
        {{ t('settings.audit.emptySub') }}
      </div>
      <UButton
        v-if="anyFilter"
        color="neutral"
        variant="outline"
        @click="resetFilters"
      >
        {{ t('settings.audit.reset') }}
      </UButton>
    </div>
  </div>
</template>
