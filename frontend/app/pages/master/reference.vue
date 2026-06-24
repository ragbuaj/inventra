<script setup lang="ts">
import type { ReferenceRow } from '~/types'
import type { ReferenceKey, ReferenceDescriptor } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useReference()

const resourceKey = ref<ReferenceKey>(referenceResources[0]!.key)
const descriptor = computed<ReferenceDescriptor>(() => referenceResources.find(r => r.key === resourceKey.value) ?? referenceResources[0]!)

const resourceOptions = referenceResources.map(r => ({ value: r.key, label: t(r.labelKey) }))

const rows = ref<ReferenceRow[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Record<string, unknown>>({})

const columns = computed(() => descriptor.value.fields.map(f => ({
  accessorKey: f.key, header: t(f.labelKey)
})))

async function refresh() {
  loading.value = true
  const res = await api.list(resourceKey.value, { search: search.value, limit: limit.value, offset: offset.value })
  rows.value = res.data
  total.value = res.total
  loading.value = false
}

function resetForm() {
  const cleared = Object.fromEntries(Object.keys(form).map(k => [k, '']))
  const fresh = Object.fromEntries(descriptor.value.fields.map(f => [f.key, '']))
  Object.assign(form, cleared, fresh)
}

function openCreate() {
  editingId.value = undefined
  resetForm()
  formOpen.value = true
}

function openEdit(row: ReferenceRow) {
  editingId.value = row.id
  resetForm()
  for (const f of descriptor.value.fields) form[f.key] = row[f.key] ?? ''
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    if (editingId.value) await api.update(resourceKey.value, editingId.value, { ...form })
    else await api.create(resourceKey.value, { ...form })
    formOpen.value = false
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete(row: ReferenceRow) {
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.reference.deleteConfirm') })
  if (!ok) return
  await api.remove(resourceKey.value, row.id)
  await refresh()
}

watch(resourceKey, () => {
  offset.value = 0
  search.value = ''
  refresh()
})
watch([search, offset], refresh)
onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.reference.title')"
      :subtitle="t(descriptor.labelKey)"
    >
      <template #actions>
        <Can permission="masterdata.global.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.reference.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    >
      <template #filters>
        <USelect
          v-model="resourceKey"
          :items="resourceOptions"
          class="w-56"
          :aria-label="t('masterdata.reference.resourceLabel')"
        />
      </template>
    </DataToolbar>

    <ResourceTable
      :rows="rows"
      :columns="columns"
      :loading="loading"
      :total="total"
      :limit="limit"
      :offset="offset"
      :empty-title="t('masterdata.reference.empty')"
      @update:offset="offset = $event"
    >
      <template #row-actions="{ row }">
        <Can permission="masterdata.global.manage">
          <div class="flex gap-1">
            <UButton
              color="neutral"
              variant="ghost"
              icon="i-lucide-pencil"
              size="xs"
              @click="openEdit(row as ReferenceRow)"
            />
            <UButton
              color="error"
              variant="ghost"
              icon="i-lucide-trash-2"
              size="xs"
              @click="onDelete(row as ReferenceRow)"
            />
          </div>
        </Can>
      </template>
    </ResourceTable>

    <FormModal
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.reference.editTitle') : t('masterdata.reference.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField
          v-for="field in descriptor.fields"
          :key="field.key"
          :label="t(field.labelKey)"
        >
          <UInput
            :model-value="form[field.key] as string"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
        </UFormField>
      </div>
    </FormModal>
  </div>
</template>
