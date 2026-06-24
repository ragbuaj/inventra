<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'

interface Column {
  accessorKey: string
  header: string
}

const props = withDefaults(defineProps<{
  rows: Record<string, unknown>[]
  columns: Column[]
  loading?: boolean
  total?: number
  limit?: number
  offset?: number
  emptyTitle?: string
}>(), { loading: false, total: 0, limit: 20, offset: 0, emptyTitle: '' })

const emit = defineEmits<{ 'update:offset': [number] }>()
const { t } = useI18n()

const slots = useSlots()

// Build TanStack-compatible column definitions for UTable
const tableColumns = computed<TableColumn<Record<string, unknown>>[]>(() => {
  const cols: TableColumn<Record<string, unknown>>[] = props.columns.map(c => ({
    accessorKey: c.accessorKey,
    header: c.header
  }))
  if (slots['row-actions']) {
    cols.push({ accessorKey: '__actions', header: t('common.actions') })
  }
  return cols
})
</script>

<template>
  <div>
    <TableSkeleton
      v-if="loading"
      :cols="columns.length"
    />

    <EmptyState
      v-else-if="rows.length === 0"
      :title="emptyTitle || $t('common.noData')"
    />

    <template v-else>
      <UTable
        :data="rows"
        :columns="tableColumns"
      >
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
        <template #__actions-cell="{ row }">
          <slot
            name="row-actions"
            :row="row.original"
          />
        </template>
      </UTable>

      <TablePagination
        v-if="total > 0"
        :total="total"
        :limit="limit"
        :offset="offset"
        @update:offset="emit('update:offset', $event)"
      />
    </template>
  </div>
</template>
