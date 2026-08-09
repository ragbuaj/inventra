<script setup lang="ts">
import type { GuideModule, GuideModuleInput, RowAction } from '~/types'

/**
 * Panduan Penggunaan CMS.
 *
 * The list is fetched whole (nine modules, steps and attachments included), so
 * search, the status filter, and the step/attachment counts are all resolved on
 * the client — no extra endpoint, no aggregate columns. Reordering swaps the
 * sort_order of two neighbours; publishing flips one field. Both go through the
 * same PATCH as the editor, because an update always replaces the whole module.
 */
definePageMeta({ middleware: 'can', permission: 'guide.manage' })

const ALL = '__all__'

const { t } = useI18n()
const toast = useToast()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()
const api = useGuide()

const modules = ref<GuideModule[]>([])
const loading = ref(true)
const loadFailed = ref(false)

const search = ref('')
const fStatus = ref<string>(ALL)

const formOpen = ref(false)
const saving = ref(false)
const editing = ref<GuideModule | null>(null)

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    // Drafts included: this screen is the authoring surface, and the API only
    // honours status=all for a caller who actually holds guide.manage.
    modules.value = await api.list(true)
  } catch {
    loadFailed.value = true
    modules.value = []
  } finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return modules.value.filter((m) => {
    if (fStatus.value !== ALL && m.status !== fStatus.value) return false
    if (!q) return true
    return `${m.title_id} ${m.title_en ?? ''}`.toLowerCase().includes(q)
  })
})

const statusTabs = computed(() => [
  { value: ALL, label: t('settings.guide.filter.all') },
  { value: 'published', label: t('settings.guide.filter.published') },
  { value: 'draft', label: t('settings.guide.filter.draft') }
])

const columns = computed(() => [
  { accessorKey: 'modul', header: t('settings.guide.columns.modul') },
  { accessorKey: 'urutan', header: t('settings.guide.columns.urutan') },
  { accessorKey: 'status', header: t('settings.guide.columns.status') },
  { accessorKey: 'langkah', header: t('settings.guide.columns.langkah') },
  { accessorKey: 'lampiran', header: t('settings.guide.columns.lampiran') },
  { accessorKey: 'diperbarui', header: t('settings.guide.columns.diperbarui') }
])

function countByKind(m: GuideModule, kind: 'video' | 'document'): number {
  return m.attachments.filter(a => a.kind === kind).length
}

/* ── row actions ───────────────────────────────────────────────────────── */

function rowActions(row: Record<string, unknown>): RowAction[] {
  const m = row as unknown as GuideModule
  const index = filtered.value.findIndex(x => x.id === m.id)
  return [
    { label: t('settings.guide.actions.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(m) },
    { label: t('settings.guide.actions.attachments'), icon: 'i-lucide-paperclip', onSelect: () => openEdit(m) },
    m.status === 'published'
      ? { label: t('settings.guide.actions.unpublish'), icon: 'i-lucide-file-text', onSelect: () => togglePublish(m) }
      : { label: t('settings.guide.actions.publish'), icon: 'i-lucide-send', onSelect: () => togglePublish(m) },
    {
      label: t('settings.guide.actions.moveUp'),
      icon: 'i-lucide-arrow-up',
      separator: true,
      disabled: index <= 0,
      onSelect: () => move(m, -1)
    },
    {
      label: t('settings.guide.actions.moveDown'),
      icon: 'i-lucide-arrow-down',
      disabled: index < 0 || index >= filtered.value.length - 1,
      onSelect: () => move(m, 1)
    },
    {
      label: t('settings.guide.actions.delete'),
      icon: 'i-lucide-trash-2',
      color: 'error',
      separator: true,
      onSelect: () => onDelete(m)
    }
  ]
}

/** Everything a PATCH needs to leave a module unchanged except the overrides. */
function toInput(m: GuideModule, patch: Partial<GuideModuleInput> = {}): GuideModuleInput {
  return {
    slug: m.slug,
    icon: m.icon,
    sort_order: m.sort_order,
    status: m.status,
    title_id: m.title_id,
    title_en: m.title_en,
    body_id: m.body_id,
    body_en: m.body_en,
    steps: m.steps,
    ...patch
  }
}

function openCreate() {
  editing.value = null
  formOpen.value = true
}

function openEdit(m: GuideModule) {
  editing.value = m
  formOpen.value = true
}

async function onSubmit(input: GuideModuleInput) {
  saving.value = true
  try {
    if (editing.value) {
      await api.update(editing.value.id, input)
    } else {
      // status is dropped rather than passed along: creation always yields a
      // draft, and sending a field the endpoint does not have would only look
      // like it works.
      const { status: _status, ...create } = input
      await api.create(create)
    }
    formOpen.value = false
    toast.add({ title: t('settings.guide.toast.saved'), color: 'success', icon: 'i-lucide-check' })
    await load()
  } catch (err: unknown) {
    // A duplicate slug is the one failure the author can act on, and it is not
    // obvious from the generic toast — the slug is derived from the title.
    if ((err as { statusCode?: number }).statusCode === 409) {
      toast.add({ title: t('settings.guide.conflict'), color: 'error', icon: 'i-lucide-triangle-alert' })
    }
  } finally {
    saving.value = false
  }
}

async function togglePublish(m: GuideModule) {
  const next = m.status === 'published' ? 'draft' : 'published'
  try {
    await api.update(m.id, toInput(m, { status: next }))
    toast.add({
      title: t(next === 'published' ? 'settings.guide.toast.published' : 'settings.guide.toast.unpublished'),
      color: 'success',
      icon: 'i-lucide-check'
    })
    await load()
  } catch {
    // useApiClient already toasted the failure.
  }
}

async function move(m: GuideModule, delta: number) {
  // Order is swapped against the neighbour in the *filtered* view, which is what
  // the author is looking at. Two PATCHes, then a refetch: no optimistic
  // reordering, so a half-applied swap shows up as itself instead of a lie.
  const rows = filtered.value
  const index = rows.findIndex(x => x.id === m.id)
  const other = rows[index + delta]
  if (!other) return
  try {
    await api.update(m.id, toInput(m, { sort_order: other.sort_order }))
    await api.update(other.id, toInput(other, { sort_order: m.sort_order }))
    toast.add({ title: t('settings.guide.toast.reordered'), color: 'success', icon: 'i-lucide-check' })
  } catch {
    // useApiClient already toasted the failure.
  } finally {
    await load()
  }
}

async function onDelete(m: GuideModule) {
  const ok = await confirm({
    title: t('settings.guide.delete.title'),
    description: t('settings.guide.delete.body', { title: m.title_id }),
    confirmLabel: t('settings.guide.delete.confirm')
  })
  if (!ok) return
  try {
    await api.remove(m.id)
    toast.add({ title: t('settings.guide.toast.deleted'), color: 'success', icon: 'i-lucide-check' })
    await load()
  } catch {
    // useApiClient already toasted the failure.
  }
}

/**
 * The editor keeps showing whichever module it was opened with, so after an
 * attachment is added or removed it has to be re-read from the refreshed list —
 * otherwise the slideover would still be holding the pre-change copy.
 */
async function onAttachmentsChanged() {
  const id = editing.value?.id
  await load()
  if (id) editing.value = modules.value.find(m => m.id === id) ?? null
}

onMounted(load)

useSeoMeta({ title: () => `${t('settings.guide.title')} - ${t('app.name')}` })
</script>

<template>
  <div>
    <PageHeader
      :title="t('settings.guide.title')"
      :subtitle="t('settings.guide.subtitle')"
    >
      <template #actions>
        <UButton
          :to="localePath('/guide')"
          color="neutral"
          variant="outline"
          icon="i-lucide-eye"
          data-testid="guide-preview-page"
        >
          {{ t('settings.guide.previewPage') }}
        </UButton>
        <UButton
          icon="i-lucide-plus"
          data-testid="guide-add"
          @click="openCreate"
        >
          {{ t('settings.guide.add') }}
        </UButton>
      </template>
    </PageHeader>

    <!-- Toolbar: search, status tabs, count -->
    <div class="flex flex-wrap items-center gap-2.5 mb-3.5">
      <UInput
        v-model="search"
        icon="i-lucide-search"
        :placeholder="t('settings.guide.searchPlaceholder')"
        data-testid="guide-search"
        class="flex-1 min-w-[200px] max-w-[340px]"
      />
      <div class="flex gap-0.5 p-[3px] bg-muted rounded-[9px] flex-none">
        <UButton
          v-for="tab in statusTabs"
          :key="tab.value"
          :color="fStatus === tab.value ? 'primary' : 'neutral'"
          :variant="fStatus === tab.value ? 'solid' : 'ghost'"
          size="xs"
          :data-testid="`guide-filter-${tab.value === ALL ? 'all' : tab.value}`"
          @click="fStatus = tab.value"
        >
          {{ tab.label }}
        </UButton>
      </div>
      <span class="text-[12.5px] text-muted">
        {{ t('settings.guide.count', { shown: filtered.length, total: modules.length }) }}
      </span>
    </div>

    <!-- loading -->
    <TableSkeleton
      v-if="loading"
      :cols="7"
    />

    <!-- error -->
    <UCard v-else-if="loadFailed">
      <EmptyState
        icon="i-lucide-circle-alert"
        :title="t('settings.guide.error.title')"
        :description="t('settings.guide.error.description')"
      >
        <template #action>
          <UButton
            icon="i-lucide-rotate-ccw"
            data-testid="guide-retry"
            @click="load"
          >
            {{ t('common.retry') }}
          </UButton>
        </template>
      </EmptyState>
    </UCard>

    <!-- empty (no modules at all) -->
    <UCard v-else-if="!modules.length">
      <EmptyState
        icon="i-lucide-book-open"
        :title="t('settings.guide.empty.title')"
        :description="t('settings.guide.empty.description')"
      >
        <template #action>
          <UButton
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('settings.guide.add') }}
          </UButton>
        </template>
      </EmptyState>
    </UCard>

    <template v-else>
      <!-- wide screens: the table -->
      <div class="hidden md:block">
        <ResourceTable
          :rows="(filtered as unknown as Record<string, unknown>[])"
          :columns="columns"
          :empty-title="t('settings.guide.emptyFilter')"
          :actions="rowActions"
        >
          <template #modul-cell="{ row }">
            <div class="flex items-start gap-[11px] whitespace-normal">
              <span class="size-[34px] rounded-[9px] bg-primary/10 text-primary flex items-center justify-center flex-none">
                <UIcon
                  :name="(row as unknown as GuideModule).icon || 'i-lucide-circle-dot'"
                  class="size-[18px]"
                />
              </span>
              <div class="min-w-0">
                <div class="text-[13.5px] font-semibold">
                  {{ (row as unknown as GuideModule).title_id }}
                </div>
                <div
                  v-if="!guideNeedsTranslation([(row as unknown as GuideModule).title_en])"
                  class="text-xs text-dimmed mt-0.5"
                >
                  {{ (row as unknown as GuideModule).title_en }}
                </div>
                <UBadge
                  v-else
                  color="warning"
                  variant="subtle"
                  size="sm"
                  class="mt-1 rounded-full"
                  icon="i-lucide-languages"
                >
                  {{ t('settings.guide.noTranslation') }}
                </UBadge>
              </div>
            </div>
          </template>

          <template #urutan-cell="{ row }">
            <span class="font-mono text-[13px] text-muted">{{ (row as unknown as GuideModule).sort_order }}</span>
          </template>

          <template #status-cell="{ row }">
            <UBadge
              :color="(row as unknown as GuideModule).status === 'published' ? 'success' : 'warning'"
              variant="subtle"
              class="rounded-full gap-1.5"
            >
              <span
                class="size-1.5 rounded-full"
                :class="(row as unknown as GuideModule).status === 'published' ? 'bg-success' : 'bg-warning'"
              />
              {{ t(`settings.guide.status.${(row as unknown as GuideModule).status}`) }}
            </UBadge>
          </template>

          <template #langkah-cell="{ row }">
            <span
              class="font-mono text-[13px]"
              :class="(row as unknown as GuideModule).steps.length ? 'text-muted' : 'text-dimmed'"
            >
              {{ (row as unknown as GuideModule).steps.length }}
            </span>
          </template>

          <template #lampiran-cell="{ row }">
            <div
              v-if="(row as unknown as GuideModule).attachments.length"
              class="flex items-center gap-1.5"
            >
              <UBadge
                v-if="countByKind(row as unknown as GuideModule, 'video')"
                color="primary"
                variant="subtle"
                size="sm"
                icon="i-lucide-video"
              >
                {{ countByKind(row as unknown as GuideModule, 'video') }}
              </UBadge>
              <UBadge
                v-if="countByKind(row as unknown as GuideModule, 'document')"
                color="neutral"
                variant="subtle"
                size="sm"
                icon="i-lucide-file-text"
              >
                {{ countByKind(row as unknown as GuideModule, 'document') }}
              </UBadge>
            </div>
            <span
              v-else
              class="text-dimmed"
            >—</span>
          </template>

          <template #diperbarui-cell="{ row }">
            <span class="text-[13px] text-muted">{{ formatDate((row as unknown as GuideModule).updated_at) }}</span>
          </template>
        </ResourceTable>
      </div>

      <!-- phone width: the same rows as cards -->
      <div class="md:hidden flex flex-col gap-[11px]">
        <EmptyState
          v-if="!filtered.length"
          :title="t('settings.guide.emptyFilter')"
        />
        <div
          v-for="m in filtered"
          :key="m.id"
          class="bg-default border border-default rounded-[13px] shadow p-[13px]"
          data-testid="guide-card"
        >
          <div class="flex items-start gap-[11px]">
            <span class="size-[34px] rounded-[9px] bg-primary/10 text-primary flex items-center justify-center flex-none">
              <UIcon
                :name="m.icon || 'i-lucide-circle-dot'"
                class="size-[18px]"
              />
            </span>
            <div class="flex-1 min-w-0">
              <div class="text-[13.5px] font-semibold">
                {{ m.title_id }}
              </div>
              <div
                v-if="!guideNeedsTranslation([m.title_en])"
                class="text-xs text-dimmed mt-0.5"
              >
                {{ m.title_en }}
              </div>
              <UBadge
                v-else
                color="warning"
                variant="subtle"
                size="sm"
                class="mt-1 rounded-full"
                icon="i-lucide-languages"
              >
                {{ t('settings.guide.noTranslation') }}
              </UBadge>
            </div>
            <RowActionsMenu :items="rowActions(m as unknown as Record<string, unknown>)" />
          </div>
          <div class="flex items-center gap-2 flex-wrap mt-[11px] pt-2.5 border-t border-default">
            <UBadge
              :color="m.status === 'published' ? 'success' : 'warning'"
              variant="subtle"
              class="rounded-full gap-1.5"
            >
              <span
                class="size-1.5 rounded-full"
                :class="m.status === 'published' ? 'bg-success' : 'bg-warning'"
              />
              {{ t(`settings.guide.status.${m.status}`) }}
            </UBadge>
            <span class="font-mono text-[11.5px] text-dimmed">#{{ m.sort_order }}</span>
            <span class="text-xs text-muted">{{ t('settings.guide.stepsCount', { n: m.steps.length }) }}</span>
            <UBadge
              v-if="countByKind(m, 'video')"
              color="primary"
              variant="subtle"
              size="sm"
              icon="i-lucide-video"
            >
              {{ countByKind(m, 'video') }}
            </UBadge>
            <UBadge
              v-if="countByKind(m, 'document')"
              color="neutral"
              variant="subtle"
              size="sm"
              icon="i-lucide-file-text"
            >
              {{ countByKind(m, 'document') }}
            </UBadge>
            <span class="text-xs text-muted ms-auto">{{ formatDate(m.updated_at) }}</span>
          </div>
        </div>
      </div>
    </template>

    <GuideModuleForm
      v-model:open="formOpen"
      :module="editing"
      :loading="saving"
      @submit="onSubmit"
    >
      <template
        v-if="editing"
        #attachments
      >
        <GuideAttachmentManager
          :module-id="editing.id"
          :attachments="editing.attachments"
          @changed="onAttachmentsChanged"
        />
      </template>
    </GuideModuleForm>
  </div>
</template>
