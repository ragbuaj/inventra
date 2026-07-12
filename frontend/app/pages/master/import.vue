<script setup lang="ts">
// Generic master-data bulk-import entry point. The concrete target (employee,
// office, or a reference sub-resource) is chosen via the `?target=` query —
// only targets registered on the backend importer (see
// backend/internal/importer/service.go PermissionKey) are valid. Each target
// maps to the exact permission the backend enforces for that import, which is
// NOT always the same permission that gates the resource's plain CRUD page
// (e.g. employee CRUD is gated by masterdata.office.manage, but employee
// import is gated by masterdata.employee.manage) — so this mapping must track
// PermissionKey, not the CRUD pages' definePageMeta permission.
definePageMeta({ middleware: 'can' })

const { t } = useI18n()
const route = useRoute()
const can = useCan()

type MasterImportTarget = 'employee' | 'office' | 'reference:provinces' | 'reference:cities'
const VALID_TARGETS: MasterImportTarget[] = ['employee', 'office', 'reference:provinces', 'reference:cities']

const PERMISSION_BY_TARGET: Record<MasterImportTarget, string> = {
  'employee': 'masterdata.employee.manage',
  'office': 'masterdata.office.manage',
  'reference:provinces': 'masterdata.global.manage',
  'reference:cities': 'masterdata.global.manage'
}

const LABEL_KEY_BY_TARGET: Record<MasterImportTarget, string> = {
  'employee': 'masterdata.import.targets.employee',
  'office': 'masterdata.import.targets.office',
  'reference:provinces': 'masterdata.import.targets.provinces',
  'reference:cities': 'masterdata.import.targets.cities'
}

const target = computed<MasterImportTarget | null>(() => {
  const raw = route.query.target
  const val = Array.isArray(raw) ? raw[0] : raw
  return val && VALID_TARGETS.includes(val as MasterImportTarget) ? (val as MasterImportTarget) : null
})

const permission = computed(() => (target.value ? PERMISSION_BY_TARGET[target.value] : ''))
const targetLabel = computed(() => (target.value ? t(LABEL_KEY_BY_TARGET[target.value]) : ''))
const allowed = computed(() => (target.value ? can(permission.value) : false))
</script>

<template>
  <div>
    <div class="mb-5">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ target ? t('masterdata.import.title', { target: targetLabel }) : t('masterdata.import.titleGeneric') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('masterdata.import.subtitle') }}
      </p>
    </div>

    <EmptyState
      v-if="!target"
      icon="i-lucide-file-question"
      :title="t('masterdata.import.invalidTarget')"
      :description="t('masterdata.import.invalidTargetSub')"
    />
    <EmptyState
      v-else-if="!allowed"
      icon="i-lucide-lock"
      :title="t('masterdata.import.notAuthorized')"
    />
    <ImportWizard
      v-else
      :key="target"
      :target="target"
      :permission="permission"
    />
  </div>
</template>
