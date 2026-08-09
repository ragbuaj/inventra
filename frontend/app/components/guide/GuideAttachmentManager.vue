<script setup lang="ts">
import type { GuideAttachment } from '~/types'

/**
 * Attachment editor for one guide module: the existing list (retitle, reorder,
 * delete) plus the two ways to add one — paste a YouTube link, or upload a PDF.
 *
 * Unlike the module fields, nothing here waits for the slideover's Save button.
 * Every action hits the API immediately and asks the parent to refetch, because
 * an upload cannot be staged: the file has to reach MinIO before the row exists.
 * The two adder paths are tabs precisely because they are never used together —
 * an attachment is a video or a document, never both (the DB has a CHECK for it).
 */
const props = defineProps<{
  moduleId: string
  attachments: GuideAttachment[]
}>()

const emit = defineEmits<{ changed: [] }>()

const { t, locale } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useGuide()

const maxSizeLabel = computed(() => formatFileSize(GUIDE_PDF_MAX_BYTES, locale.value))
const limitReached = computed(() => props.attachments.length >= GUIDE_MAX_ATTACHMENTS)

// New attachments land after the current last one. Computed from the rows we
// were handed rather than from the count, so a gap in the sequence (a deleted
// row) never produces a duplicate sort_order.
const nextOrder = computed(() =>
  props.attachments.reduce((max, a) => Math.max(max, a.sort_order), 0) + 1
)

function kindLabel(a: GuideAttachment): string {
  return a.kind === 'video'
    ? t('settings.guide.attachments.kindVideo')
    : t('settings.guide.attachments.kindDocument')
}

/** The mono meta line under an attachment title: video id, or filename + size. */
function attachmentMeta(a: GuideAttachment): string {
  if (a.kind === 'video') {
    return a.youtube_id ? t('settings.guide.attachments.extractedId', { id: a.youtube_id }) : ''
  }
  const size = formatFileSize(a.size_bytes, locale.value)
  return [a.original_filename, size].filter(Boolean).join(' · ')
}

/* ── retitle an existing row ───────────────────────────────────────────── */

const editingId = ref<string | null>(null)
const editTitleId = ref('')
const editTitleEn = ref('')
const editSaving = ref(false)

function startEdit(a: GuideAttachment) {
  editingId.value = a.id
  editTitleId.value = a.title_id
  editTitleEn.value = a.title_en ?? ''
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(a: GuideAttachment) {
  if (!editTitleId.value.trim()) return
  editSaving.value = true
  try {
    await api.updateAttachment(a.id, {
      title_id: editTitleId.value.trim(),
      title_en: editTitleEn.value.trim() || null,
      sort_order: a.sort_order
    })
    editingId.value = null
    toast.add({ title: t('settings.guide.attachments.toastSaved'), color: 'success', icon: 'i-lucide-check' })
    emit('changed')
  } catch {
    // useApiClient already toasted the failure.
  } finally {
    editSaving.value = false
  }
}

/* ── reorder / delete ──────────────────────────────────────────────────── */

const busyId = ref<string | null>(null)

async function move(index: number, delta: number) {
  const rows = props.attachments
  const a = rows[index]
  const b = rows[index + delta]
  if (!a || !b) return
  busyId.value = a.id
  try {
    // Two PATCHes, no transaction: the pair is swapped one row at a time. A
    // failure between them leaves both rows with their old order (the first
    // write is a no-op reorder on its own), and the refetch below shows the
    // real state rather than an optimistic guess.
    await api.updateAttachment(a.id, { title_id: a.title_id, title_en: a.title_en, sort_order: b.sort_order })
    await api.updateAttachment(b.id, { title_id: b.title_id, title_en: b.title_en, sort_order: a.sort_order })
    emit('changed')
  } catch {
    emit('changed')
  } finally {
    busyId.value = null
  }
}

async function remove(a: GuideAttachment) {
  const ok = await confirm({
    title: t('settings.guide.attachments.deleteTitle'),
    description: t('settings.guide.attachments.deleteBody', { title: a.title_id })
  })
  if (!ok) return
  busyId.value = a.id
  try {
    await api.removeAttachment(a.id)
    toast.add({ title: t('settings.guide.attachments.toastDeleted'), color: 'success', icon: 'i-lucide-check' })
    emit('changed')
  } catch {
    // useApiClient already toasted the failure.
  } finally {
    busyId.value = null
  }
}

/* ── adder: YouTube link ───────────────────────────────────────────────── */

const tab = ref<'video' | 'pdf'>('video')

const videoUrl = ref('')
const videoTitleId = ref('')
const videoTitleEn = ref('')
const videoSaving = ref(false)

// Parsed client-side purely for feedback — the server parses the URL again and
// stores only the id it extracts itself.
const videoId = computed(() => youtubeVideoId(videoUrl.value))
const videoUrlTouched = computed(() => videoUrl.value.trim() !== '')
const videoUrlInvalid = computed(() => videoUrlTouched.value && videoId.value === null)
const canAddVideo = computed(() => !!videoId.value && videoTitleId.value.trim() !== '')

function resetVideo() {
  videoUrl.value = ''
  videoTitleId.value = ''
  videoTitleEn.value = ''
}

async function addVideo() {
  if (!canAddVideo.value || videoSaving.value) return
  videoSaving.value = true
  try {
    await api.addVideo(props.moduleId, {
      url: videoUrl.value.trim(),
      title_id: videoTitleId.value.trim(),
      title_en: videoTitleEn.value.trim() || null,
      sort_order: nextOrder.value
    })
    resetVideo()
    toast.add({ title: t('settings.guide.attachments.toastAdded'), color: 'success', icon: 'i-lucide-check' })
    emit('changed')
  } catch {
    // useApiClient already toasted the failure.
  } finally {
    videoSaving.value = false
  }
}

/* ── adder: PDF upload ─────────────────────────────────────────────────── */

const fileInput = ref<HTMLInputElement | null>(null)
// shallowRef, not ref: a File must reach FormData as the real object. Vue would
// hand a reactive proxy to FormData.append, which has no File internal slots and
// would be coerced to the string "[object File]" instead of the bytes.
const pdfFile = shallowRef<File | null>(null)
const pdfTitleId = ref('')
const pdfTitleEn = ref('')
const pdfUploading = ref(false)
const pdfError = ref<'type' | 'size' | 'upload' | null>(null)
const rejectedName = ref('')

const canAddPdf = computed(() => !!pdfFile.value && pdfTitleId.value.trim() !== '')

function openPicker() {
  fileInput.value?.click()
}

/**
 * Pre-flight the file before it leaves the browser. The server checks the same
 * two things (magic bytes, then size) and is the real gate; doing it here saves
 * an author from watching a 10 MB upload finish only to be told no.
 */
function acceptFile(f: File | undefined | null) {
  if (!f) return
  rejectedName.value = f.name
  if (f.type !== 'application/pdf' && !/\.pdf$/i.test(f.name)) {
    pdfError.value = 'type'
    pdfFile.value = null
    return
  }
  if (f.size > GUIDE_PDF_MAX_BYTES) {
    pdfError.value = 'size'
    pdfFile.value = null
    return
  }
  pdfError.value = null
  pdfFile.value = f
  if (!pdfTitleId.value.trim()) pdfTitleId.value = f.name.replace(/\.pdf$/i, '')
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  acceptFile(input.files?.[0])
  input.value = ''
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  acceptFile(e.dataTransfer?.files?.[0])
}

const dragOver = ref(false)

function clearFile() {
  pdfFile.value = null
  pdfError.value = null
  rejectedName.value = ''
}

async function addPdf() {
  if (!canAddPdf.value || pdfUploading.value || !pdfFile.value) return
  pdfUploading.value = true
  pdfError.value = null
  try {
    await api.addDocument(props.moduleId, pdfFile.value, {
      title_id: pdfTitleId.value.trim(),
      title_en: pdfTitleEn.value.trim() || null,
      sort_order: nextOrder.value
    })
    pdfFile.value = null
    pdfTitleId.value = ''
    pdfTitleEn.value = ''
    rejectedName.value = ''
    toast.add({ title: t('settings.guide.attachments.toastAdded'), color: 'success', icon: 'i-lucide-check' })
    emit('changed')
  } catch (err: unknown) {
    // The server re-checks content and size, so its verdict wins over the
    // pre-flight above: a .pdf whose bytes are not a PDF only fails here.
    const status = (err as { statusCode?: number }).statusCode
    if (status === 415) pdfError.value = 'type'
    else if (status === 413) pdfError.value = 'size'
    else pdfError.value = 'upload'
  } finally {
    pdfUploading.value = false
  }
}
</script>

<template>
  <div>
    <!-- existing attachments -->
    <div class="flex flex-col gap-2 mb-3.5">
      <template
        v-for="(a, idx) in props.attachments"
        :key="a.id"
      >
        <!-- editing the title of one row -->
        <div
          v-if="editingId === a.id"
          class="p-3 border border-primary rounded-[11px] bg-default ring-2 ring-primary/20"
        >
          <div class="flex items-center gap-2.5 mb-2.5">
            <span
              class="size-[30px] rounded-lg flex items-center justify-center flex-none"
              :class="a.kind === 'video' ? 'bg-primary/10 text-primary' : 'bg-elevated text-muted'"
            >
              <UIcon
                :name="a.kind === 'video' ? 'i-lucide-video' : 'i-lucide-file-text'"
                class="size-4"
              />
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-[12.5px] font-semibold">
                {{ t('settings.guide.attachments.edit') }}
              </p>
              <p class="text-[11.5px] font-mono text-muted truncate">
                {{ attachmentMeta(a) }}
              </p>
            </div>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <UFormField required>
              <template #label>
                <span class="inline-flex items-center gap-1.5">
                  {{ t('settings.guide.attachments.fieldTitle') }}
                  <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-primary/10 text-primary">ID</span>
                </span>
              </template>
              <UInput
                v-model="editTitleId"
                data-testid="guide-att-edit-title"
                class="w-full"
              />
            </UFormField>
            <UFormField>
              <template #label>
                <span class="inline-flex items-center gap-1.5 text-muted">
                  {{ t('settings.guide.attachments.fieldTitle') }}
                  <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-elevated text-muted">EN</span>
                </span>
              </template>
              <UInput
                v-model="editTitleEn"
                :placeholder="t('settings.guide.form.fallbackPlaceholder')"
                class="w-full"
              />
            </UFormField>
          </div>
          <div class="flex items-center justify-between gap-3 mt-2.5 flex-wrap">
            <p class="flex items-start gap-1.5 text-[11.5px] leading-snug text-muted flex-1 min-w-[180px]">
              <UIcon
                name="i-lucide-info"
                class="size-3.5 mt-px flex-none"
              />
              <span>
                {{ a.kind === 'video'
                  ? t('settings.guide.attachments.editNoteVideo')
                  : t('settings.guide.attachments.editNoteDoc') }}
              </span>
            </p>
            <div class="flex gap-2 flex-none">
              <UButton
                color="neutral"
                variant="outline"
                size="sm"
                @click="cancelEdit"
              >
                {{ t('common.cancel') }}
              </UButton>
              <UButton
                size="sm"
                :loading="editSaving"
                :disabled="!editTitleId.trim()"
                data-testid="guide-att-edit-save"
                @click="saveEdit(a)"
              >
                {{ t('common.save') }}
              </UButton>
            </div>
          </div>
        </div>

        <!-- normal row -->
        <div
          v-else
          class="flex items-center gap-2.5 p-2.5 border border-default rounded-[11px] bg-default flex-wrap"
          data-testid="guide-att-row"
        >
          <span
            class="size-9 rounded-[9px] flex items-center justify-center flex-none"
            :class="a.kind === 'video' ? 'bg-primary/10 text-primary' : 'bg-elevated text-muted'"
          >
            <UIcon
              :name="a.kind === 'video' ? 'i-lucide-video' : 'i-lucide-file-text'"
              class="size-[17px]"
            />
          </span>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-1.5 flex-wrap">
              <span class="text-[13px] font-semibold">{{ a.title_id }}</span>
              <UBadge
                :color="a.kind === 'video' ? 'primary' : 'neutral'"
                variant="subtle"
                size="sm"
              >
                {{ kindLabel(a) }}
              </UBadge>
              <UBadge
                v-if="guideNeedsTranslation([a.title_en])"
                color="warning"
                variant="subtle"
                size="sm"
              >
                {{ t('settings.guide.noTranslation') }}
              </UBadge>
            </div>
            <p class="text-[11.5px] font-mono text-dimmed truncate mt-0.5">
              {{ attachmentMeta(a) }}
            </p>
          </div>
          <div class="flex items-center gap-1 flex-none">
            <UButton
              color="neutral"
              variant="outline"
              size="xs"
              icon="i-lucide-pencil"
              :aria-label="t('settings.guide.attachments.edit')"
              data-testid="guide-att-edit"
              @click="startEdit(a)"
            />
            <UButton
              color="neutral"
              variant="outline"
              size="xs"
              icon="i-lucide-arrow-up"
              :disabled="idx === 0 || busyId === a.id"
              :aria-label="t('settings.guide.attachments.moveUp')"
              @click="move(idx, -1)"
            />
            <UButton
              color="neutral"
              variant="outline"
              size="xs"
              icon="i-lucide-arrow-down"
              :disabled="idx === props.attachments.length - 1 || busyId === a.id"
              :aria-label="t('settings.guide.attachments.moveDown')"
              @click="move(idx, 1)"
            />
            <UButton
              color="error"
              variant="outline"
              size="xs"
              icon="i-lucide-trash-2"
              :aria-label="t('settings.guide.actions.delete')"
              data-testid="guide-att-delete"
              @click="remove(a)"
            />
          </div>
        </div>
      </template>

      <p
        v-if="!props.attachments.length"
        class="p-4 border border-dashed border-accented rounded-[11px] text-center text-[12.5px] leading-relaxed text-muted"
      >
        {{ t('settings.guide.attachments.none') }}
      </p>
    </div>

    <!-- limit reached: the adder is replaced by the reason it is gone -->
    <div
      v-if="limitReached"
      class="flex items-start gap-2.5 p-3.5 border border-dashed border-accented rounded-xl bg-muted"
      data-testid="guide-att-limit"
    >
      <span class="size-9 rounded-[10px] bg-default text-muted flex items-center justify-center flex-none">
        <UIcon
          name="i-lucide-layers"
          class="size-[18px]"
        />
      </span>
      <div class="flex-1 min-w-0">
        <p class="text-[13px] font-semibold">
          {{ t('settings.guide.attachments.limitTitle') }}
        </p>
        <p class="mt-0.5 text-[12.5px] leading-relaxed text-muted">
          {{ t('settings.guide.attachments.limitBody') }}
        </p>
      </div>
    </div>

    <!-- two ways to add one -->
    <div
      v-else
      class="border border-default rounded-xl bg-default overflow-hidden"
    >
      <div class="flex border-b border-default bg-muted">
        <UButton
          v-for="choice in (['video', 'pdf'] as const)"
          :key="choice"
          :color="tab === choice ? 'primary' : 'neutral'"
          variant="ghost"
          :icon="choice === 'video' ? 'i-lucide-video' : 'i-lucide-upload'"
          :data-testid="`guide-att-tab-${choice}`"
          class="rounded-none px-4 py-2.5"
          :class="tab === choice ? 'bg-default border-b-2 border-primary' : ''"
          @click="tab = choice"
        >
          {{ choice === 'video'
            ? t('settings.guide.attachments.tabVideo')
            : t('settings.guide.attachments.tabPdf') }}
        </UButton>
      </div>

      <!-- path A: paste a YouTube link -->
      <div
        v-if="tab === 'video'"
        class="p-3.5 flex flex-col gap-3"
      >
        <UFormField
          :label="t('settings.guide.attachments.urlLabel')"
          required
          :error="videoUrlInvalid ? t('settings.guide.attachments.urlError') : undefined"
        >
          <UInput
            v-model="videoUrl"
            data-testid="guide-att-url"
            placeholder="https://www.youtube.com/watch?v=..."
            class="w-full font-mono text-[13px]"
          />
          <p
            v-if="!videoUrlTouched"
            class="mt-1.5 text-[12px] leading-snug text-muted"
          >
            {{ t('settings.guide.attachments.urlHint') }}
          </p>
        </UFormField>

        <div
          v-if="videoId"
          class="flex items-center gap-3 p-2.5 border border-success/40 bg-success/10 rounded-[11px] flex-wrap"
          data-testid="guide-att-url-ok"
        >
          <div class="w-[104px] aspect-video rounded-lg flex-none bg-gradient-to-br from-[#0b1220] via-[#12234d] to-[#02194f] flex items-center justify-center text-white">
            <UIcon
              name="i-lucide-play"
              class="size-5"
            />
          </div>
          <div class="flex-1 min-w-0">
            <p class="flex items-center gap-1.5 text-[12.5px] font-semibold text-success">
              <UIcon
                name="i-lucide-check"
                class="size-3.5"
              />
              {{ t('settings.guide.attachments.urlOk') }}
            </p>
            <p class="mt-1 text-[12px] font-mono text-muted break-all">
              {{ t('settings.guide.attachments.extractedId', { id: videoId }) }}
            </p>
            <p class="mt-0.5 text-[11.5px] text-muted">
              {{ t('settings.guide.attachments.storedNote') }}
            </p>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <UFormField required>
            <template #label>
              <span class="inline-flex items-center gap-1.5">
                {{ t('settings.guide.attachments.fieldTitle') }}
                <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-primary/10 text-primary">ID</span>
              </span>
            </template>
            <UInput
              v-model="videoTitleId"
              data-testid="guide-att-video-title"
              class="w-full"
            />
          </UFormField>
          <UFormField>
            <template #label>
              <span class="inline-flex items-center gap-1.5 text-muted">
                {{ t('settings.guide.attachments.fieldTitle') }}
                <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-elevated text-muted">EN</span>
              </span>
            </template>
            <UInput
              v-model="videoTitleEn"
              :placeholder="t('settings.guide.form.fallbackPlaceholder')"
              class="w-full"
            />
          </UFormField>
        </div>

        <div class="flex justify-end">
          <UButton
            icon="i-lucide-plus"
            :disabled="!canAddVideo"
            :loading="videoSaving"
            data-testid="guide-att-add-video"
            @click="addVideo"
          >
            {{ t('settings.guide.attachments.addVideo') }}
          </UButton>
        </div>
      </div>

      <!-- path B: upload a PDF -->
      <div
        v-else
        class="p-3.5 flex flex-col gap-3"
      >
        <input
          ref="fileInput"
          type="file"
          accept="application/pdf"
          class="hidden"
          data-testid="guide-att-file-input"
          @change="onFileChange"
        >

        <UAlert
          color="warning"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          :description="t('settings.guide.attachments.uploadWarning')"
          :ui="{ description: 'text-[12.5px] leading-relaxed' }"
        />

        <!-- uploading -->
        <div
          v-if="pdfUploading"
          class="p-3.5 border border-default rounded-xl bg-muted"
          data-testid="guide-att-uploading"
        >
          <div class="flex items-center gap-3">
            <span class="size-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
              <UIcon
                name="i-lucide-file-text"
                class="size-5"
              />
            </span>
            <div class="flex-1 min-w-0">
              <p class="text-[13.5px] font-medium truncate">
                {{ pdfFile?.name }}
              </p>
              <p class="text-[12px] text-muted">
                {{ t('settings.guide.attachments.uploading', { size: formatFileSize(pdfFile?.size, locale) }) }}
              </p>
            </div>
          </div>
          <UProgress
            :model-value="null"
            class="mt-2.5"
          />
        </div>

        <!-- rejected: wrong type / too large / upload failed -->
        <UAlert
          v-else-if="pdfError"
          color="error"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          close
          data-testid="guide-att-error"
          :title="pdfError === 'type'
            ? t('settings.guide.attachments.errTypeTitle')
            : pdfError === 'size'
              ? t('settings.guide.attachments.errSizeTitle', { size: maxSizeLabel })
              : t('settings.guide.attachments.errUpload')"
          :description="pdfError === 'type'
            ? t('settings.guide.attachments.errTypeBody')
            : pdfError === 'size'
              ? t('settings.guide.attachments.errSizeBody', { size: maxSizeLabel })
              : rejectedName"
          @update:open="clearFile"
        />

        <!-- picked file -->
        <div
          v-else-if="pdfFile"
          class="flex items-center gap-3 p-3.5 border border-default rounded-[11px] bg-muted"
          data-testid="guide-att-file"
        >
          <span class="size-9 rounded-lg bg-success/10 text-success flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-file-check"
              class="size-5"
            />
          </span>
          <div class="flex-1 min-w-0">
            <p class="text-[13.5px] font-medium truncate">
              {{ pdfFile.name }}
            </p>
            <p class="text-[12px] text-muted">
              {{ formatFileSize(pdfFile.size, locale) }}
            </p>
          </div>
          <UButton
            color="neutral"
            variant="ghost"
            size="xs"
            icon="i-lucide-x"
            :aria-label="t('common.cancel')"
            @click="clearFile"
          />
        </div>

        <!-- drop zone; stays available after a rejection so the retry path is
             the same control that failed -->
        <button
          v-if="!pdfFile && !pdfUploading"
          type="button"
          class="w-full flex flex-col items-center justify-center gap-1 py-7 px-4 rounded-xl border-[1.5px] border-dashed text-center cursor-pointer transition-colors"
          :class="dragOver ? 'border-primary bg-primary/5' : 'border-accented hover:border-primary'"
          data-testid="guide-att-dropzone"
          @click="openPicker"
          @dragover.prevent="dragOver = true"
          @dragleave="dragOver = false"
          @drop.prevent="onDrop"
        >
          <span class="size-[46px] mb-1.5 rounded-xl bg-muted text-muted flex items-center justify-center">
            <UIcon
              name="i-lucide-upload"
              class="size-5"
            />
          </span>
          <span class="text-sm font-semibold">{{ t('settings.guide.attachments.dropTitle') }}</span>
          <span class="text-[12.5px] text-muted">{{ t('settings.guide.attachments.dropSub', { size: maxSizeLabel }) }}</span>
        </button>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <UFormField required>
            <template #label>
              <span class="inline-flex items-center gap-1.5">
                {{ t('settings.guide.attachments.fieldTitle') }}
                <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-primary/10 text-primary">ID</span>
              </span>
            </template>
            <UInput
              v-model="pdfTitleId"
              data-testid="guide-att-pdf-title"
              class="w-full"
            />
          </UFormField>
          <UFormField>
            <template #label>
              <span class="inline-flex items-center gap-1.5 text-muted">
                {{ t('settings.guide.attachments.fieldTitle') }}
                <span class="px-1.5 py-px text-[10px] font-mono font-semibold rounded bg-elevated text-muted">EN</span>
              </span>
            </template>
            <UInput
              v-model="pdfTitleEn"
              :placeholder="t('settings.guide.form.fallbackPlaceholder')"
              class="w-full"
            />
          </UFormField>
        </div>

        <div class="flex justify-end">
          <UButton
            icon="i-lucide-upload"
            :disabled="!canAddPdf"
            :loading="pdfUploading"
            data-testid="guide-att-add-pdf"
            @click="addPdf"
          >
            {{ t('settings.guide.attachments.addPdf') }}
          </UButton>
        </div>
      </div>
    </div>
  </div>
</template>
