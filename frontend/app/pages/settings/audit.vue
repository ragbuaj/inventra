<script setup lang="ts">
import type { AuditRow, AuditAction } from '~/composables/api/useAudit'
import { useAudit, entityLabel } from '~/composables/api/useAudit'
import { AUDIT_ENTITY_TYPES } from '~/constants/auditCatalog'

definePageMeta({ middleware: 'can', permission: 'audit.view' })

const PAGE_SIZE = 20
const ALL = '__all__'

const { t, te } = useI18n()
const { list } = useAudit()
const actor = useUserPicker()
const can = useCan()
const canFilterByActor = computed(() => can('user.manage'))

const rows = ref<AuditRow[]>([])
const total = ref(0)
const loading = ref(true)
const loadFailed = ref(false)
const search = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const fAction = ref(ALL)
const fEntity = ref(ALL)
const fActorId = ref<string | null>(null)
const page = ref(1)
const openId = ref<string | null>(null)

// Action display metadata (tone + icon), inlined (was imported from the mock).
const ACTION_META: Record<AuditAction, { tone: 'success' | 'warning' | 'error', icon: string }> = {
  create: { tone: 'success', icon: 'i-lucide-plus' },
  update: { tone: 'warning', icon: 'i-lucide-pencil' },
  delete: { tone: 'error', icon: 'i-lucide-trash-2' }
}

function entityLabelFor(key: string): string {
  return entityLabel(key, t, te)
}

const actionOptions = computed(() => [
  { value: ALL, label: t('settings.audit.filter.allActions') },
  { value: 'create', label: t('settings.audit.action.create') },
  { value: 'update', label: t('settings.audit.action.update') },
  { value: 'delete', label: t('settings.audit.action.delete') }
])
const entityOptions = computed(() => [
  { value: ALL, label: t('settings.audit.filter.allEntities') },
  ...AUDIT_ENTITY_TYPES.map(e => ({ value: e, label: entityLabelFor(e) }))
])

const anyFilter = computed(() =>
  !!(search.value.trim() || dateFrom.value || dateTo.value || fAction.value !== ALL || fEntity.value !== ALL || fActorId.value)
)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const pageInfo = computed(() => {
  const from = total.value === 0 ? 0 : (page.value - 1) * PAGE_SIZE + 1
  const to = Math.min(page.value * PAGE_SIZE, total.value)
  return t('settings.audit.showing', { from, to, total: total.value })
})

// A 'YYYY-MM-DD' date input → an RFC3339 day bound for the backend from/to filter.
function toRfc(d: string, endOfDay: boolean): string | undefined {
  if (!d) return undefined
  return new Date(`${d}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).toISOString()
}

function actionMeta(action: AuditAction) {
  return ACTION_META[action]
}
function toggle(id: string) {
  openId.value = openId.value === id ? null : id
}
function resetFilters() {
  search.value = ''
  dateFrom.value = ''
  dateTo.value = ''
  fAction.value = ALL
  fEntity.value = ALL
  fActorId.value = null
  page.value = 1
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const res = await list({
      search: search.value.trim() || undefined,
      entity_type: fEntity.value !== ALL ? fEntity.value : undefined,
      action: fAction.value !== ALL ? (fAction.value as AuditAction) : undefined,
      actor_id: fActorId.value ?? undefined,
      from: toRfc(dateFrom.value, false),
      to: toRfc(dateTo.value, true),
      limit: PAGE_SIZE,
      offset: (page.value - 1) * PAGE_SIZE
    }, t, te)
    rows.value = res.rows
    total.value = res.total
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

watch([search, dateFrom, dateTo, fAction, fEntity, fActorId], () => {
  page.value = 1
  load()
})
watch(page, () => load())
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
        disabled
        :label="t('settings.audit.export')"
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
        <DateField
          v-model="dateFrom"
          :aria-label="t('settings.audit.dateFrom')"
        />
        <span class="text-dimmed">–</span>
        <DateField
          v-model="dateTo"
          :aria-label="t('settings.audit.dateTo')"
        />
      </div>
      <AsyncSearchPicker
        v-if="canFilterByActor"
        :model-value="fActorId"
        :search-fn="actor.searchFn"
        :resolve-fn="actor.resolveFn"
        :placeholder="t('settings.audit.filter.actor')"
        testid="audit-actor"
        clearable
        class="min-w-[190px]"
        @update:model-value="fActorId = $event"
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

    <div
      v-else-if="loadFailed"
      class="flex flex-col items-center justify-center gap-3 py-20 text-muted"
    >
      <UIcon
        name="i-lucide-circle-alert"
        class="size-6"
      />
      <span class="text-sm">{{ t('settings.audit.loadError') }}</span>
      <UButton
        color="neutral"
        variant="subtle"
        @click="load"
      >
        {{ t('settings.audit.retry') }}
      </UButton>
    </div>

    <!-- Table -->
    <div
      v-else-if="rows.length > 0"
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
              <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase text-muted whitespace-nowrap">
                {{ t('settings.audit.columns.office') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <template
              v-for="r in rows"
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
                      <div
                        v-if="r.role"
                        class="text-[11.5px] text-dimmed whitespace-nowrap"
                      >
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
                  {{ entityLabelFor(r.entity_type) }}
                </td>
                <td class="px-3.5 py-3 max-w-[320px]">
                  <span class="block overflow-hidden text-ellipsis whitespace-nowrap">{{ r.summary }}</span>
                </td>
                <td class="px-3.5 py-3 whitespace-nowrap">
                  <div class="text-[13px] text-muted">
                    {{ r.office_name || '—' }}
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
                      <span class="text-[11.5px] font-mono text-dimmed">{{ r.entity_id }}</span>
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
            data-testid="audit-next-page"
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
