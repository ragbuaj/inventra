<script setup lang="ts">
import type { TreeNode } from '~/components/TreeView.vue'
import type { RowAction } from '~/types'

const { open } = useConfirm()
const rows = ref([
  { id: '1', name: 'Laptop Dell', status: 'available' },
  { id: '2', name: 'Proyektor Epson', status: 'under_maintenance' }
])
const columns = [
  { accessorKey: 'name', header: 'Nama', sortable: true },
  { accessorKey: 'status', header: 'Status', sortable: true }
]
const tree: TreeNode[] = [
  {
    id: 'p',
    label: 'Kantor Pusat',
    icon: 'i-lucide-building-2',
    childCount: 1,
    children: [
      {
        id: 'w',
        label: 'Kanwil Jakarta',
        icon: 'i-lucide-building',
        children: [
          { id: 'c', label: 'Cabang Jakarta Selatan', icon: 'i-lucide-store' }
        ]
      }
    ]
  }
]
const offset = ref(0)

// FilterBar showcase state
const DEMO_ALL = '__all__'
const demoSearch = ref('')
const demoStatus = ref(DEMO_ALL)
const demoCategory = ref(DEMO_ALL)
const demoStatusOptions = [
  { value: DEMO_ALL, label: 'Semua Status' },
  { value: 'available', label: 'Tersedia' },
  { value: 'under_maintenance', label: 'Maintenance' }
]
const demoCategoryOptions = [
  { value: DEMO_ALL, label: 'Semua Kategori' },
  { value: 'it', label: 'Perangkat IT' },
  { value: 'furniture', label: 'Furnitur' }
]
const demoActiveCount = computed(
  () => [demoStatus.value, demoCategory.value].filter(v => v !== DEMO_ALL).length
)
function resetDemoFilters() {
  demoSearch.value = ''
  demoStatus.value = DEMO_ALL
  demoCategory.value = DEMO_ALL
}

async function askDelete() {
  await open({ title: 'Hapus data?', description: 'Tindakan ini tidak dapat dibatalkan.' })
}
const rowActions = (): RowAction[] => [
  { label: 'Lihat Detail', icon: 'i-lucide-eye', onSelect: () => {} },
  { label: 'Edit', icon: 'i-lucide-pencil', onSelect: () => {} },
  { label: 'Hapus', icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => askDelete() }
]
</script>

<template>
  <div class="space-y-8 max-w-4xl">
    <PageHeader
      title="Component Library"
      subtitle="Style guide & verifikasi"
    >
      <template #actions>
        <UButton @click="askDelete">
          Confirm dialog
        </UButton>
      </template>
    </PageHeader>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Status badges
      </h2>
      <div class="flex flex-wrap gap-2">
        <StatusBadge status="available" />
        <StatusBadge status="under_maintenance" />
        <StatusBadge status="lost" />
        <StatusBadge
          status="pending"
          kind="approval"
        />
        <StatusBadge
          status="approved"
          kind="approval"
        />
      </div>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Stat cards
      </h2>
      <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
        <StatCard
          label="Total Aset"
          value="1.248"
          icon="i-lucide-package"
          trend="+3,2%"
        />
        <CardSkeleton />
      </div>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Filter bar
      </h2>
      <p class="text-sm text-muted">
        Sempitkan jendela di bawah 768px: filter lanjutan pindah ke slideover bawah, dan
        tombol filter membawa badge jumlah filter aktif.
      </p>
      <FilterBar
        v-model:search="demoSearch"
        search-placeholder="Cari aset…"
        :active-count="demoActiveCount"
        :total="rows.length"
        testid="demo-filter-bar"
        @reset="resetDemoFilters"
      >
        <template #filters>
          <USelect
            v-model="demoStatus"
            :items="demoStatusOptions"
            class="min-w-[150px]"
          />
          <USelect
            v-model="demoCategory"
            :items="demoCategoryOptions"
            class="min-w-[150px]"
          />
        </template>
      </FilterBar>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Resource table
      </h2>
      <ResourceTable
        :rows="rows"
        :columns="columns"
        :total="2"
        :offset="offset"
        :actions="rowActions"
        @update:offset="offset = $event"
      >
        <template #name-cell="{ row }">
          <span class="font-bold">{{ row.name as string }}</span>
        </template>
        <template #status-cell="{ row }">
          <StatusBadge :status="row.status as string" />
        </template>
      </ResourceTable>
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Empty state
      </h2>
      <EmptyState
        title="Belum ada aset"
        description="Tambahkan aset pertama Anda."
      />
    </section>

    <section class="space-y-2">
      <h2 class="font-semibold">
        Tree view
      </h2>
      <TreeView
        :nodes="tree"
        selected-id="c"
      />
    </section>
  </div>
</template>
