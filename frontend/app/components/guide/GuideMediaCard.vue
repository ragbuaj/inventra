<script setup lang="ts">
import type { GuideAttachment } from '~/types'

const props = defineProps<{ attachment: GuideAttachment }>()

const { t, locale } = useI18n()
const localePath = useLocalePath()
const guideApi = useGuide()

const a = computed(() => props.attachment)
const title = computed(() => guideText(a.value.title_id, a.value.title_en, locale.value))
const isVideo = computed(() => a.value.kind === 'video')

// A guest gets no video id and no file link from the API at all, so "locked" is
// not a UI-only decision — there is genuinely nothing here to play.
const locked = computed(() => a.value.locked)

/* ── video ─────────────────────────────────────────────────────────────── */

const playing = ref(false)
const thumbBroken = ref(false)

const embedUrl = computed(() => (a.value.youtube_id ? youtubeEmbedUrl(a.value.youtube_id) : ''))
const thumbUrl = computed(() => (a.value.youtube_id ? youtubeThumbnailUrl(a.value.youtube_id) : ''))

// A cross-origin iframe cannot tell us that a video is gone: YouTube's "video
// unavailable" page loads successfully as far as the browser is concerned. The
// thumbnail is the one honest signal we can read — it 404s for a deleted video —
// so a failed thumbnail is what surfaces the broken state. A merely *private*
// video still has a thumbnail and will report itself inside the player.
const broken = computed(() => isVideo.value && !locked.value && thumbBroken.value)

function retryVideo() {
  thumbBroken.value = false
  playing.value = false
}

/* ── document ──────────────────────────────────────────────────────────── */

const previewOpen = ref(false)
const previewUrl = ref<string | null>(null)
const previewLoading = ref(false)
const previewFailed = ref(false)

const fileMeta = computed(() => formatFileSize(a.value.size_bytes, locale.value))

function revokePreview() {
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
  }
}

async function togglePreview() {
  if (previewOpen.value) {
    previewOpen.value = false
    revokePreview()
    return
  }
  previewOpen.value = true
  previewFailed.value = false
  if (previewUrl.value) return

  previewLoading.value = true
  // The PDF endpoint is authenticated, so the bytes are fetched and handed to the
  // iframe as an object URL; the src is same-origin and stays sandboxed.
  previewUrl.value = await guideApi.attachmentObjectURL(a.value.id)
  previewFailed.value = previewUrl.value === null
  previewLoading.value = false
}

async function download() {
  const url = await guideApi.attachmentObjectURL(a.value.id)
  if (!url) {
    previewFailed.value = true
    return
  }
  const link = document.createElement('a')
  link.href = url
  link.download = a.value.original_filename || 'panduan.pdf'
  link.click()
  URL.revokeObjectURL(url)
}

onBeforeUnmount(revokePreview)
</script>

<template>
  <div>
    <!-- Title row: shown whether the attachment is locked or not, so a guest can
         see that something exists here rather than facing a blank gap. -->
    <div class="flex items-center gap-2.5 flex-wrap mb-2">
      <span
        class="inline-flex items-center gap-1.5 px-2.5 py-[3px] text-[11.5px] font-semibold rounded-md flex-none"
        :class="isVideo ? 'bg-primary/10 text-primary' : 'bg-error/10 text-error'"
      >
        <UIcon
          :name="isVideo ? 'i-lucide-play-circle' : 'i-lucide-file-text'"
          class="size-3.5"
        />
        {{ isVideo ? t('guidePage.media.kindVideo') : t('guidePage.media.kindDocument') }}
      </span>
      <span class="text-[13.5px] font-semibold text-default min-w-0">{{ title }}</span>
    </div>

    <!-- Locked video -->
    <div
      v-if="isVideo && locked"
      class="w-full aspect-video border-[1.5px] border-dashed border-accented rounded-xl bg-muted flex flex-col items-center justify-center gap-2 p-4 text-center"
    >
      <span class="size-10 rounded-[11px] bg-primary/10 text-primary flex items-center justify-center flex-none">
        <UIcon
          name="i-lucide-lock"
          class="size-5"
        />
      </span>
      <p class="text-[13.5px] font-semibold">
        {{ t('guidePage.media.lockedVideoTitle') }}
      </p>
      <p class="text-[12.5px] leading-relaxed text-muted max-w-[330px]">
        {{ t('guidePage.media.lockedHint') }}
      </p>
      <UButton
        :to="localePath('/login')"
        icon="i-lucide-log-in"
        size="sm"
        class="mt-0.5"
      >
        {{ t('guidePage.media.signIn') }}
      </UButton>
    </div>

    <!-- Video that cannot be played: the failure stays inside this 16:9 box so
         the rest of the guide keeps rendering (AC17). -->
    <div
      v-else-if="broken"
      class="w-full aspect-video border border-default rounded-xl bg-muted flex flex-col items-center justify-center gap-2 p-4 text-center"
    >
      <span class="size-10 rounded-[11px] bg-elevated text-muted flex items-center justify-center flex-none">
        <UIcon
          name="i-lucide-video-off"
          class="size-5"
        />
      </span>
      <p class="text-[13.5px] font-semibold">
        {{ t('guidePage.media.brokenTitle') }}
      </p>
      <p class="text-[12.5px] leading-relaxed text-muted max-w-[360px]">
        {{ t('guidePage.media.brokenBody') }}
      </p>
      <UButton
        color="neutral"
        variant="outline"
        size="sm"
        icon="i-lucide-rotate-ccw"
        class="mt-0.5"
        @click="retryVideo"
      >
        {{ t('guidePage.media.brokenRetry') }}
      </UButton>
    </div>

    <!-- Video facade: nothing is requested from YouTube until the reader presses
         play, so opening the guide sets no third-party cookies (AC16). -->
    <template v-else-if="isVideo">
      <button
        v-if="!playing"
        type="button"
        class="block w-full p-0 border border-default rounded-xl overflow-hidden bg-transparent cursor-pointer text-left hover:border-primary transition-colors"
        :aria-label="t('guidePage.media.play', { title })"
        @click="playing = true"
      >
        <div class="relative w-full aspect-video bg-gradient-to-br from-[#0b1220] via-[#12234d] to-[#02194f] flex items-center justify-center">
          <img
            v-if="thumbUrl"
            :src="thumbUrl"
            alt=""
            loading="lazy"
            class="absolute inset-0 size-full object-cover opacity-70"
            @error="thumbBroken = true"
          >
          <span class="relative size-[62px] rounded-full bg-white/95 text-[#0f172a] flex items-center justify-center shadow-lg">
            <UIcon
              name="i-lucide-play"
              class="size-7"
            />
          </span>
          <span class="absolute left-3 bottom-3 inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-black/60 text-[#e2e8f0] text-[11.5px] font-mono">
            <UIcon
              name="i-lucide-shield-check"
              class="size-3.5"
            />
            youtube-nocookie.com
          </span>
        </div>
      </button>
      <div
        v-else
        class="w-full aspect-video border border-default rounded-xl overflow-hidden bg-black"
      >
        <iframe
          :src="embedUrl"
          :title="title"
          class="size-full"
          allow="accelerometer; encrypted-media; picture-in-picture"
          allowfullscreen
          referrerpolicy="strict-origin-when-cross-origin"
        />
      </div>
      <p
        v-if="!playing"
        class="mt-1.5 text-[12.5px] leading-relaxed text-muted"
      >
        {{ t('guidePage.media.lazyNote') }}
      </p>
    </template>

    <!-- Locked document: no filename and no size either, since the public
         response withholds both (AC26). -->
    <div
      v-else-if="locked"
      class="flex items-center gap-3 p-3.5 border-[1.5px] border-dashed border-accented rounded-xl bg-muted flex-wrap"
    >
      <span class="size-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
        <UIcon
          name="i-lucide-lock"
          class="size-5"
        />
      </span>
      <div class="flex-1 min-w-0">
        <p class="text-[13.5px] font-semibold">
          {{ t('guidePage.media.lockedDocTitle') }}
        </p>
        <p class="text-[12.5px] leading-relaxed text-muted">
          {{ t('guidePage.media.lockedHint') }}
        </p>
      </div>
      <UButton
        :to="localePath('/login')"
        icon="i-lucide-log-in"
        size="sm"
        class="flex-none"
      >
        {{ t('guidePage.media.signIn') }}
      </UButton>
    </div>

    <!-- Document, unlocked -->
    <div
      v-else
      class="border border-default rounded-xl bg-muted overflow-hidden"
    >
      <div class="flex items-center gap-3 p-3.5 flex-wrap">
        <span class="size-10 rounded-[10px] bg-error/10 text-error flex items-center justify-center flex-none">
          <UIcon
            name="i-lucide-file-text"
            class="size-5"
          />
        </span>
        <div class="flex-1 min-w-0">
          <p class="text-[13.5px] font-medium truncate">
            {{ a.original_filename }}
          </p>
          <p class="text-[12px] text-muted">
            {{ fileMeta }}
          </p>
        </div>
        <div class="flex items-center gap-2 flex-none">
          <UButton
            color="neutral"
            variant="outline"
            size="sm"
            :icon="previewOpen ? 'i-lucide-eye-off' : 'i-lucide-eye'"
            :loading="previewLoading"
            @click="togglePreview"
          >
            {{ previewOpen ? t('guidePage.media.hidePreview') : t('guidePage.media.preview') }}
          </UButton>
          <UButton
            size="sm"
            icon="i-lucide-download"
            @click="download"
          >
            {{ t('guidePage.media.download') }}
          </UButton>
        </div>
      </div>

      <div
        v-if="previewOpen"
        class="border-t border-default p-3.5 bg-elevated"
      >
        <UAlert
          v-if="previewFailed"
          color="error"
          variant="subtle"
          icon="i-lucide-alert-triangle"
          :title="t('guidePage.media.previewFailed')"
        />
        <iframe
          v-else-if="previewUrl"
          :src="previewUrl"
          :title="title"
          sandbox=""
          class="w-full h-[420px] border border-default rounded-lg bg-white"
        />
        <USkeleton
          v-else
          class="w-full h-[420px] rounded-lg"
        />
        <p class="mt-2 flex items-center gap-1.5 text-[12px] text-muted">
          <UIcon
            name="i-lucide-shield-check"
            class="size-3.5"
          />
          {{ t('guidePage.media.sandboxNote') }}
        </p>
      </div>
    </div>
  </div>
</template>
