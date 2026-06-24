<script setup lang="ts">
import type { TreeNode } from '~/components/TreeView.vue'

const { open } = useConfirm()
const rows = ref([
  { id: '1', name: 'Laptop Dell', status: 'available' },
  { id: '2', name: 'Proyektor Epson', status: 'under_maintenance' }
])
const columns = [
  { accessorKey: 'name', header: 'Nama' },
  { accessorKey: 'status', header: 'Status' }
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
async function askDelete() {
  await open({ title: 'Hapus data?', description: 'Tindakan ini tidak dapat dibatalkan.' })
}
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
        Resource table
      </h2>
      <ResourceTable
        :rows="rows"
        :columns="columns"
        :total="2"
        :offset="offset"
        @update:offset="offset = $event"
      >
        <template #name-cell="{ row }">
          <span class="font-bold">{{ row.name as string }}</span>
        </template>
        <template #status-cell="{ row }">
          <StatusBadge :status="row.status as string" />
        </template>
        <template #row-actions>
          <UButton
            size="xs"
            color="neutral"
            variant="ghost"
            icon="i-lucide-pencil"
          />
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
