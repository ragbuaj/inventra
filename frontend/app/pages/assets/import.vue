<script setup lang="ts">
import { IMPORT_SAMPLE_ROWS, IMPORT_COLUMNS } from '~/mock/assets'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()

const step = ref(1)
const processing = ref(false)
const progress = ref(0)
const progressKey = ref<'validate' | 'create'>('validate')
const file = ref<{ name: string, meta: string } | null>(null)
const onlyErrors = ref(false)

let timer: ReturnType<typeof setInterval> | undefined

const rows = IMPORT_SAMPLE_ROWS
const validCount = computed(() => rows.filter(r => r.errFields.length === 0).length)
const errorCount = computed(() => rows.length - validCount.value)
const previewRows = computed(() => onlyErrors.value ? rows.filter(r => r.errFields.length > 0) : rows)

const steps = computed(() => ([
  { n: 1, label: t('assets.import.steps.upload'), sub: t('assets.import.steps.uploadSub') },
  { n: 2, label: t('assets.import.steps.validate'), sub: t('assets.import.steps.validateSub') },
  { n: 3, label: t('assets.import.steps.result'), sub: t('assets.import.steps.resultSub') }
]))

function runProgress(key: 'validate' | 'create', onDone: () => void) {
  if (timer) clearInterval(timer)
  processing.value = true
  progress.value = 0
  progressKey.value = key
  timer = setInterval(() => {
    progress.value = Math.min(100, progress.value + 18)
    if (progress.value >= 100) {
      if (timer) clearInterval(timer)
      setTimeout(() => {
        processing.value = false
        onDone()
      }, 250)
    }
  }, 120)
}

function pickFile() {
  file.value = { name: 'aset-import-batch.xlsx', meta: t('assets.import.fileMeta', { rows: rows.length }) }
}
function removeFile() {
  file.value = null
}
function comingSoon() {
  toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })
}
function startValidate() {
  if (!file.value) return
  runProgress('validate', () => {
    step.value = 2
  })
}
function createAssets() {
  runProgress('create', () => {
    step.value = 3
  })
}
function backToUpload() {
  step.value = 1
  onlyErrors.value = false
}
function finish() {
  step.value = 1
  file.value = null
  onlyErrors.value = false
}

function cellClass(r: typeof rows[number], field: string): string {
  return r.errFields.includes(field) ? 'bg-error/10 text-error' : ''
}

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <div class="mb-5">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('assets.import.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('assets.import.subtitle') }}
      </p>
    </div>

    <!-- Stepper -->
    <div class="flex items-center gap-2 mb-6 max-w-[640px]">
      <template
        v-for="(st, i) in steps"
        :key="st.n"
      >
        <div class="flex items-center gap-3">
          <div
            class="size-9 rounded-full border flex items-center justify-center flex-none text-sm font-semibold"
            :class="step > st.n ? 'bg-primary text-inverted border-primary'
              : step === st.n ? 'bg-primary/10 text-primary border-primary'
                : 'bg-muted text-dimmed border-default'"
          >
            <UIcon
              v-if="step > st.n"
              name="i-lucide-check"
              class="size-4"
            />
            <span v-else>{{ st.n }}</span>
          </div>
          <div class="hidden sm:block">
            <div
              class="text-[13px] font-semibold"
              :class="step >= st.n ? 'text-default' : 'text-muted'"
            >
              {{ st.label }}
            </div>
            <div class="text-[11.5px] text-dimmed">
              {{ st.sub }}
            </div>
          </div>
        </div>
        <div
          v-if="i < 2"
          class="flex-1 h-px"
          :class="step > st.n ? 'bg-primary' : 'bg-default'"
        />
      </template>
    </div>

    <!-- Processing -->
    <div
      v-if="processing"
      class="bg-default border border-default rounded-[14px] shadow-sm p-10 text-center"
    >
      <div class="max-w-[420px] mx-auto">
        <UIcon
          name="i-lucide-loader-circle"
          class="size-7 animate-spin text-primary mb-3"
        />
        <div class="text-base font-semibold mb-1">
          {{ progressKey === 'create' ? t('assets.import.processing.create') : t('assets.import.processing.validate') }}
        </div>
        <div class="text-sm text-muted mb-4">
          {{ progressKey === 'create' ? t('assets.import.processing.createSub') : t('assets.import.processing.validateSub') }}
        </div>
        <div class="h-2 rounded-full bg-muted overflow-hidden">
          <div
            class="h-full bg-primary transition-[width] duration-150"
            :style="{ width: `${progress}%` }"
          />
        </div>
        <div class="text-xs text-dimmed mt-2">
          {{ progress }}%
        </div>
      </div>
    </div>

    <!-- Step 1: Upload -->
    <div
      v-else-if="step === 1"
      class="space-y-4"
    >
      <div class="bg-default border border-default rounded-[14px] shadow-sm p-5 flex items-center justify-between gap-4 flex-wrap">
        <div class="flex items-center gap-3">
          <span class="size-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-file-spreadsheet"
              class="size-5"
            />
          </span>
          <div>
            <div class="text-sm font-semibold">
              {{ t('assets.import.templateTitle') }}
            </div>
            <div class="text-[12.5px] text-muted">
              {{ t('assets.import.templateSub') }}
            </div>
          </div>
        </div>
        <UButton
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :label="t('assets.import.templateTitle')"
          @click="comingSoon"
        />
      </div>

      <div class="bg-default border border-default rounded-[14px] shadow-sm p-5">
        <button
          v-if="!file"
          type="button"
          class="w-full flex flex-col items-center justify-center gap-2 py-10 px-4 rounded-[12px] border-2 border-dashed border-default text-center cursor-pointer hover:border-primary transition-colors"
          @click="pickFile"
        >
          <UIcon
            name="i-lucide-upload-cloud"
            class="size-8 text-dimmed"
          />
          <span class="text-sm font-medium">{{ t('assets.import.dropTitle') }}</span>
          <span class="text-xs text-dimmed">{{ t('assets.import.dropSub') }}</span>
        </button>
        <div
          v-else
          class="flex items-center gap-3 p-3.5 rounded-[11px] border border-default bg-muted"
        >
          <span class="size-9 rounded-lg bg-success/10 text-success flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-file-check"
              class="size-4"
            />
          </span>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium truncate">
              {{ file.name }}
            </div>
            <div class="text-xs text-muted">
              {{ file.meta }}
            </div>
          </div>
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            size="sm"
            square
            :aria-label="t('common.delete')"
            @click="removeFile"
          />
        </div>

        <div class="mt-5">
          <div class="text-[13px] font-semibold mb-2">
            {{ t('assets.import.columnsTitle') }}
          </div>
          <div class="flex flex-wrap gap-2">
            <UBadge
              v-for="([name, req]) in IMPORT_COLUMNS"
              :key="name"
              :color="req ? 'primary' : 'neutral'"
              variant="subtle"
              class="rounded-md font-mono"
            >
              {{ name }}{{ req ? ' *' : '' }}
            </UBadge>
          </div>
          <div class="text-xs text-dimmed mt-2.5">
            {{ t('assets.import.columnsNote') }}
          </div>
        </div>
      </div>

      <div class="flex justify-end">
        <UButton
          icon="i-lucide-circle-check-big"
          :disabled="!file"
          :label="t('assets.import.validate')"
          @click="startValidate"
        />
      </div>
    </div>

    <!-- Step 2: Validate -->
    <div
      v-else-if="step === 2"
      class="space-y-4"
    >
      <div class="bg-default border border-default rounded-[14px] shadow-sm p-4 flex items-center gap-4 flex-wrap">
        <span class="text-[13px] text-muted">{{ t('assets.import.totalRows') }}: <span class="font-semibold text-default">{{ rows.length }}</span></span>
        <UBadge
          color="success"
          variant="subtle"
          class="rounded-full"
        >
          {{ validCount }} {{ t('assets.import.valid') }}
        </UBadge>
        <UBadge
          color="error"
          variant="subtle"
          class="rounded-full"
        >
          {{ errorCount }} {{ t('assets.import.error') }}
        </UBadge>
        <div class="flex-1" />
        <label class="flex items-center gap-2 text-[13px] cursor-pointer">
          <UCheckbox v-model="onlyErrors" />
          {{ t('assets.import.onlyErrors') }}
        </label>
      </div>

      <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase w-[40px]">
                  #
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  {{ t('assets.import.status') }}
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  asset_tag
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  nama
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  kategori
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  kantor
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  tgl_beli
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  harga
                </th>
                <th class="text-left px-3 py-2.5 text-xs font-semibold uppercase">
                  {{ t('assets.import.note') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(r, i) in previewRows"
                :key="i"
                class="border-t border-default"
              >
                <td class="px-3 py-2.5 text-dimmed">
                  {{ i + 1 }}
                </td>
                <td class="px-3 py-2.5">
                  <UBadge
                    :color="r.errFields.length === 0 ? 'success' : 'error'"
                    variant="subtle"
                    class="rounded-full"
                  >
                    {{ r.errFields.length === 0 ? t('assets.import.valid') : t('assets.import.error') }}
                  </UBadge>
                </td>
                <td
                  class="px-3 py-2.5 font-mono text-[12px]"
                  :class="cellClass(r, 'tag')"
                >
                  {{ r.tag }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="cellClass(r, 'nama')"
                >
                  {{ r.nama || '(kosong)' }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="cellClass(r, 'kategori')"
                >
                  {{ r.kategori }}
                </td>
                <td class="px-3 py-2.5 text-muted">
                  {{ r.kantor }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="cellClass(r, 'tgl')"
                >
                  {{ r.tgl }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="cellClass(r, 'harga')"
                >
                  {{ r.harga === 'dua juta' ? r.harga : `Rp ${r.harga}` }}
                </td>
                <td
                  class="px-3 py-2.5"
                  :class="r.errKey ? 'text-error' : 'text-dimmed'"
                >
                  {{ r.errKey ? t(`assets.import.errors.${r.errKey}`) : '—' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex justify-between">
        <UButton
          icon="i-lucide-arrow-left"
          color="neutral"
          variant="outline"
          :label="t('assets.import.back')"
          @click="backToUpload"
        />
        <UButton
          icon="i-lucide-circle-check-big"
          :label="`${t('assets.import.create')} (${validCount})`"
          @click="createAssets"
        />
      </div>
    </div>

    <!-- Step 3: Result -->
    <div
      v-else
      class="bg-default border border-default rounded-[14px] shadow-sm p-8 text-center max-w-[560px] mx-auto"
    >
      <div class="size-[54px] mx-auto mb-4 rounded-2xl bg-success/10 text-success flex items-center justify-center">
        <UIcon
          name="i-lucide-circle-check-big"
          class="size-7"
        />
      </div>
      <div class="text-[17px] font-semibold mb-1.5">
        {{ t('assets.import.resultTitle') }}
      </div>
      <div class="text-sm text-muted mb-5">
        {{ t('assets.import.resultSub') }}
      </div>
      <div class="grid grid-cols-2 gap-3 mb-5">
        <div class="rounded-[12px] border border-default p-4">
          <div class="text-2xl font-bold text-success">
            {{ validCount }}
          </div>
          <div class="text-[13px] text-muted">
            {{ t('assets.import.createdLabel') }}
          </div>
        </div>
        <div class="rounded-[12px] border border-default p-4">
          <div class="text-2xl font-bold text-error">
            {{ errorCount }}
          </div>
          <div class="text-[13px] text-muted">
            {{ t('assets.import.failedLabel') }}
          </div>
        </div>
      </div>
      <div class="flex items-center justify-center gap-2.5 flex-wrap">
        <UButton
          icon="i-lucide-download"
          color="neutral"
          variant="outline"
          :label="t('assets.import.downloadFailed')"
          @click="comingSoon"
        />
        <UButton
          :label="t('assets.import.finish')"
          @click="finish"
        />
      </div>
    </div>
  </div>
</template>
