<script setup lang="ts">
import type { TableColumn, ContextMenuItem } from '@nuxt/ui'
import type { RowActions, TableSorting } from '~/types'

interface Column {
  accessorKey: string
  header: string
  sortable?: boolean
}

const props = withDefaults(defineProps<{
  rows: Record<string, unknown>[]
  columns: Column[]
  loading?: boolean
  total?: number
  limit?: number
  offset?: number
  emptyTitle?: string
  actions?: RowActions
}>(), { loading: false, total: 0, limit: 10, offset: 0, emptyTitle: '' })

const emit = defineEmits<{ 'update:offset': [number] }>()
const sorting = defineModel<TableSorting>('sorting', { default: () => [] })

const { t } = useI18n()

const slots = useSlots()

const hasActions = computed(() => !!props.actions || !!slots['row-actions'])
const sortableColumns = computed(() => props.columns.filter(c => c.sortable))

// Skeleton only on the *first* load. Once data has arrived once, later fetches
// (filter, search, refetch) reuse the table chrome with Nuxt UI's own inline
// loading bar instead of flashing a skeleton.
const everLoaded = ref(false)
watch(() => props.loading, (l) => {
  if (!l) everLoaded.value = true
}, { immediate: true })
const showSkeleton = computed(() => props.loading && !everLoaded.value)

// Client-side sort of the rows we were handed. Pages that paginate the full
// dataset sort it before slicing (via the shared sortRows util); sorting the
// already-ordered slice here is idempotent, so this stays correct everywhere.
const displayRows = computed(() => sortRows(props.rows, sorting.value))

// Build TanStack-compatible column definitions for UTable
const tableColumns = computed<TableColumn<Record<string, unknown>>[]>(() => {
  const cols: TableColumn<Record<string, unknown>>[] = props.columns.map(c => ({
    accessorKey: c.accessorKey,
    header: c.header
  }))
  if (hasActions.value) {
    cols.push({
      accessorKey: '__actions',
      header: t('common.actions'),
      meta: { class: { th: 'text-right pe-4', td: 'text-right pe-4' } }
    })
  }
  return cols
})

// Match the Component Library mockup: muted uppercase header, comfortable
// row padding, hover highlight; the card chrome lives on the wrapper below.
const tableUi = {
  th: 'px-4 py-3 text-left rtl:text-right text-xs font-semibold uppercase tracking-wide text-muted bg-muted',
  td: 'px-4 py-3 text-sm whitespace-nowrap',
  tr: 'border-default hover:bg-muted/50'
}

function sortIcon(id: string): string {
  const s = sorting.value[0]
  if (!s || s.id !== id) return 'i-lucide-chevrons-up-down'
  return s.desc ? 'i-lucide-chevron-down' : 'i-lucide-chevron-up'
}

// Single-column sort cycle: none → asc → desc → none.
function toggleSort(id: string) {
  const s = sorting.value[0]
  if (!s || s.id !== id) sorting.value = [{ id, desc: false }]
  else if (!s.desc) sorting.value = [{ id, desc: true }]
  else sorting.value = []
}

// Right-click handler: resolve which rendered row the cursor is over and load
// that row's actions into the context menu before Reka UI opens it. Grouping
// (a `separator` flag starts a new group so a divider is drawn before it) is
// shared with the row dropdown (RowActionsMenu) via `buildActionGroups`.
const contextItems = ref<ContextMenuItem[][]>([])
function onContextMenu(e: MouseEvent) {
  const tr = (e.target as HTMLElement | null)?.closest('tbody tr')
  if (!tr || !tr.parentElement) {
    contextItems.value = []
    return
  }
  const index = Array.from(tr.parentElement.children).indexOf(tr)
  const row = displayRows.value[index]
  contextItems.value = row && props.actions
    ? (buildActionGroups(props.actions(row)) as ContextMenuItem[][])
    : []
}
</script>

<template>
  <div>
    <TableSkeleton
      v-if="showSkeleton"
      :cols="columns.length + (hasActions ? 1 : 0)"
    />

    <EmptyState
      v-else-if="!loading && rows.length === 0"
      :title="emptyTitle || $t('common.noData')"
    />

    <div
      v-else
      class="bg-default border border-default rounded-2xl shadow overflow-hidden"
    >
      <UContextMenu
        :items="contextItems"
        :disabled="!props.actions"
      >
        <div
          class="overflow-x-auto"
          @contextmenu="onContextMenu"
        >
          <UTable
            :data="displayRows"
            :columns="tableColumns"
            :loading="loading"
            :ui="tableUi"
          >
            <template
              v-for="col in sortableColumns"
              :key="`header-${col.accessorKey}`"
              #[`${col.accessorKey}-header`]
            >
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-muted hover:text-default transition-colors cursor-pointer"
                @click="toggleSort(col.accessorKey)"
              >
                {{ col.header }}
                <UIcon
                  :name="sortIcon(col.accessorKey)"
                  class="size-3.5"
                />
              </button>
            </template>

            <template
              v-for="col in columns"
              :key="col.accessorKey"
              #[`${col.accessorKey}-cell`]="{ row }"
            >
              <slot
                :name="`${col.accessorKey}-cell`"
                :row="row.original"
              >
                {{ row.original[col.accessorKey] }}
              </slot>
            </template>

            <template
              v-if="hasActions"
              #__actions-cell="{ row }"
            >
              <div class="flex justify-end">
                <RowActionsMenu
                  v-if="props.actions && buildActionGroups(props.actions(row.original)).length"
                  :items="props.actions(row.original)"
                />
                <slot
                  v-else
                  name="row-actions"
                  :row="row.original"
                />
              </div>
            </template>
          </UTable>
        </div>
      </UContextMenu>

      <TablePagination
        v-if="total > 0"
        :total="total"
        :limit="limit"
        :offset="offset"
        @update:offset="emit('update:offset', $event)"
      />
    </div>
  </div>
</template>
