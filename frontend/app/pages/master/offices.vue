<script setup lang="ts">
import type { Office, TreeNode } from '~/types'
import type { OfficeInput } from '~/composables/api/useOffices'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useOffices()

const nodes = ref<TreeNode[]>([])
const selectedId = ref<string>()
const selected = ref<Office>()
const search = ref('')
const loading = ref(true)

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<OfficeInput>({
  nama: '', kode: '', tipe: 'cabang', parent_id: null, provinsi: '', kota: '', alamat: ''
})

const tipeOptions = (['pusat', 'kanwil', 'cabang', 'unit'] as const).map(v => ({
  value: v, label: t(`masterdata.offices.tipe.${v}`)
}))

async function refresh() {
  loading.value = true
  nodes.value = await api.tree()
  loading.value = false
}

async function onSelect(id: string) {
  selectedId.value = id
  selected.value = await api.get(id)
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, { nama: '', kode: '', tipe: 'cabang', parent_id: selectedId.value ?? null, provinsi: '', kota: '', alamat: '' })
  formOpen.value = true
}

function openEdit() {
  if (!selected.value) return
  editingId.value = selected.value.id
  Object.assign(form, {
    nama: selected.value.nama, kode: selected.value.kode, tipe: selected.value.tipe,
    parent_id: selected.value.parent_id, provinsi: selected.value.provinsi,
    kota: selected.value.kota, alamat: selected.value.alamat
  })
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    const saved = editingId.value
      ? await api.update(editingId.value, { ...form })
      : await api.create({ ...form })
    formOpen.value = false
    await refresh()
    await onSelect(saved.id)
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  if (!selected.value) return
  const ok = await confirm({ title: t('common.delete'), description: t('masterdata.offices.deleteConfirm') })
  if (!ok) return
  await api.remove(selected.value.id)
  selected.value = undefined
  selectedId.value = undefined
  await refresh()
}

const detailRows = computed(() => {
  const o = selected.value
  if (!o) return []
  return [
    { label: t('masterdata.offices.fields.kode'), value: o.kode },
    { label: t('masterdata.offices.fields.tipe'), value: t(`masterdata.offices.tipe.${o.tipe}`) },
    { label: t('masterdata.offices.fields.provinsi'), value: o.provinsi },
    { label: t('masterdata.offices.fields.kota'), value: o.kota },
    { label: t('masterdata.offices.fields.alamat'), value: o.alamat }
  ]
})

onMounted(refresh)
</script>

<template>
  <div>
    <PageHeader
      :title="t('masterdata.offices.title')"
      :subtitle="t('masterdata.offices.subtitle')"
    >
      <template #actions>
        <Can permission="masterdata.office.manage">
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.offices.add') }}
          </UButton>
        </Can>
      </template>
    </PageHeader>

    <DataToolbar
      v-model:search="search"
      @reset="search = ''"
    />

    <div class="grid grid-cols-1 lg:grid-cols-[20rem_1fr] gap-4">
      <UCard>
        <TableSkeleton
          v-if="loading"
          :cols="1"
        />
        <EmptyState
          v-else-if="nodes.length === 0"
          :title="t('masterdata.offices.empty')"
        />
        <TreeView
          v-else
          :nodes="nodes"
          :selected-id="selectedId"
          @select="onSelect"
        />
      </UCard>

      <UCard>
        <EmptyState
          v-if="!selected"
          :title="t('masterdata.offices.selectHint')"
        />
        <div v-else>
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold">
              {{ selected.nama }}
            </h2>
            <Can permission="masterdata.office.manage">
              <div class="flex gap-2">
                <UButton
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-pencil"
                  @click="openEdit"
                >
                  {{ t('common.edit') }}
                </UButton>
                <UButton
                  color="error"
                  variant="ghost"
                  icon="i-lucide-trash-2"
                  @click="onDelete"
                >
                  {{ t('common.delete') }}
                </UButton>
              </div>
            </Can>
          </div>
          <dl class="grid grid-cols-2 gap-y-3 text-sm">
            <template
              v-for="row in detailRows"
              :key="row.label"
            >
              <dt class="text-muted">
                {{ row.label }}
              </dt>
              <dd>{{ row.value }}</dd>
            </template>
          </dl>
        </div>
      </UCard>
    </div>

    <FormSlideover
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.offices.editTitle') : t('masterdata.offices.createTitle')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <UFormField :label="t('masterdata.offices.fields.nama')">
          <UInput
            v-model="form.nama"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.kode')">
          <UInput
            v-model="form.kode"
            placeholder="JKT01"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.tipe')">
          <USelect
            v-model="form.tipe"
            :items="tipeOptions"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.provinsi')">
          <UInput
            v-model="form.provinsi"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.kota')">
          <UInput
            v-model="form.kota"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('masterdata.offices.fields.alamat')">
          <UTextarea
            v-model="form.alamat"
            class="w-full"
          />
        </UFormField>
      </div>
    </FormSlideover>
  </div>
</template>
