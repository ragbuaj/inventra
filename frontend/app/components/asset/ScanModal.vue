<script setup lang="ts">
// Barcode/QR scan modal for the asset catalog. Opens the rear camera and runs
// a detect loop (native BarcodeDetector when it supports our formats,
// otherwise the zxing-wasm ponyfill from `barcode-detector/ponyfill`, loaded
// lazily on the client so SSR never touches it). Labels encode the raw
// asset_tag (see backend/internal/asset/barcode.go), so the decoded string is
// emitted as-is via `detected`. The manual input below the camera area is the
// always-available fallback: it works with no camera, denied permission, or
// an insecure (non-HTTPS) context.
interface DetectedBarcode {
  rawValue: string
}
interface DetectorLike {
  detect: (source: HTMLVideoElement) => Promise<DetectedBarcode[]>
}
interface DetectorCtor {
  new (options?: { formats?: string[] }): DetectorLike
  getSupportedFormats?: () => Promise<string[]>
}

const FORMATS = ['qr_code', 'code_128']

const props = defineProps<{
  open: boolean
  /** Parent's tag lookup is in flight — disables the manual submit. */
  submitting?: boolean
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  /** A tag was decoded from the camera or entered manually. */
  'detected': [string]
}>()

const { t } = useI18n()

const videoEl = ref<HTMLVideoElement | null>(null)
// idle → starting → scanning → paused (after a successful decode); `error`
// keeps the camera area replaced by a friendly message while the manual
// input stays usable.
const phase = ref<'idle' | 'starting' | 'scanning' | 'paused' | 'error'>('idle')
const errorKind = ref<'denied' | 'unavailable'>('unavailable')
const manualTag = ref('')

let stream: MediaStream | null = null
let timer: ReturnType<typeof setInterval> | undefined
let detector: DetectorLike | null = null
let detecting = false
// Invalidates in-flight start()/getUserMedia results after the modal closes
// or a newer start() begins (e.g. open → close → open quickly).
let session = 0

function stopCamera() {
  if (timer) {
    clearInterval(timer)
    timer = undefined
  }
  if (stream) {
    stream.getTracks().forEach(track => track.stop())
    stream = null
  }
  if (videoEl.value) videoEl.value.srcObject = null
}

async function createDetector(): Promise<DetectorLike> {
  // Prefer the native BarcodeDetector when it exists AND supports both of our
  // formats (some platforms ship it QR-only); otherwise fall back to the
  // zxing-wasm ponyfill. Dynamic import keeps the wasm bundle out of SSR and
  // out of the page until the user actually opens the scanner.
  const native = (globalThis as { BarcodeDetector?: DetectorCtor }).BarcodeDetector
  if (native?.getSupportedFormats) {
    try {
      const supported = await native.getSupportedFormats()
      if (FORMATS.every(f => supported.includes(f))) return new native({ formats: FORMATS })
    } catch {
      // Fall through to the ponyfill.
    }
  }
  const { BarcodeDetector } = await import('barcode-detector/ponyfill')
  return new BarcodeDetector({ formats: FORMATS as never }) as DetectorLike
}

async function detectFrame() {
  const video = videoEl.value
  if (!video || video.readyState < 2 || !detector || detecting) return
  detecting = true
  try {
    const codes = await detector.detect(video)
    const raw = codes[0]?.rawValue?.trim()
    if (raw && phase.value === 'scanning') {
      stopCamera()
      phase.value = 'paused'
      emit('detected', raw)
    }
  } catch {
    // Ignore per-frame decode errors and keep scanning.
  } finally {
    detecting = false
  }
}

async function startCamera() {
  if (import.meta.server) return
  const mine = ++session
  stopCamera()
  if (!globalThis.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
    errorKind.value = 'unavailable'
    phase.value = 'error'
    return
  }
  phase.value = 'starting'
  try {
    detector = detector ?? await createDetector()
    const media = await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' } })
    if (mine !== session || !props.open) {
      media.getTracks().forEach(track => track.stop())
      return
    }
    stream = media
    await nextTick()
    const video = videoEl.value
    if (!video) {
      stopCamera()
      return
    }
    video.srcObject = media
    await video.play().catch(() => {})
    if (mine !== session) return
    phase.value = 'scanning'
    timer = setInterval(detectFrame, 250)
  } catch (err) {
    if (mine !== session) return
    stopCamera()
    errorKind.value = (err as DOMException | undefined)?.name === 'NotAllowedError' ? 'denied' : 'unavailable'
    phase.value = 'error'
  }
}

function teardown() {
  session++
  stopCamera()
  phase.value = 'idle'
}

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    manualTag.value = ''
    startCamera()
  } else {
    teardown()
  }
})

onUnmounted(() => {
  teardown()
})

const manualReady = computed(() => manualTag.value.trim().length > 0)

function submitManual() {
  const tag = manualTag.value.trim()
  if (!tag || props.submitting) return
  emit('detected', tag)
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('assets.scanModal.title')"
    :description="t('assets.scanModal.instruction')"
    @update:open="(v: boolean) => emit('update:open', v)"
  >
    <template #body>
      <div
        data-testid="scan-modal"
        class="space-y-4"
      >
        <!-- Camera preview + scan-frame overlay -->
        <div
          v-if="phase !== 'error'"
          class="relative aspect-video overflow-hidden rounded-[10px] bg-neutral-950"
        >
          <video
            ref="videoEl"
            autoplay
            playsinline
            muted
            class="absolute inset-0 size-full object-cover"
          />
          <div class="absolute inset-0 flex items-center justify-center pointer-events-none">
            <div class="w-[62%] max-w-[240px] aspect-square rounded-xl border-2 border-white/80 shadow-[0_0_0_9999px_rgba(0,0,0,0.35)]" />
          </div>
          <div
            v-if="phase === 'idle' || phase === 'starting'"
            class="absolute inset-0 flex items-center justify-center"
          >
            <UIcon
              name="i-lucide-loader-circle"
              class="size-6 animate-spin text-white/90"
            />
          </div>
          <div
            v-else-if="phase === 'paused'"
            class="absolute inset-0 flex items-center justify-center bg-black/60"
          >
            <UButton
              icon="i-lucide-rotate-cw"
              color="neutral"
              variant="solid"
              :label="t('assets.scanModal.rescan')"
              @click="startCamera"
            />
          </div>
        </div>

        <!-- Camera unavailable / permission denied -->
        <div
          v-else
          class="flex items-start gap-2.5 px-3.5 py-3 rounded-[10px] bg-warning/10 border border-warning/25"
        >
          <UIcon
            name="i-lucide-camera-off"
            class="size-[17px] flex-none text-warning mt-px"
          />
          <span class="text-[13px] leading-relaxed text-warning">
            {{ errorKind === 'denied' ? t('assets.scanModal.cameraDenied') : t('assets.scanModal.cameraError') }}
          </span>
        </div>

        <!-- Manual fallback — always available -->
        <UFormField :label="t('assets.scanModal.manualLabel')">
          <div class="flex gap-2">
            <UInput
              v-model="manualTag"
              data-testid="scan-manual-input"
              :placeholder="t('assets.scanModal.manualPlaceholder')"
              icon="i-lucide-tag"
              class="flex-1"
              @keyup.enter="submitManual"
            />
            <UButton
              data-testid="scan-manual-submit"
              icon="i-lucide-search"
              :label="t('assets.scanModal.submit')"
              :loading="submitting"
              :disabled="!manualReady"
              @click="submitManual"
            />
          </div>
        </UFormField>
      </div>
    </template>
  </UModal>
</template>
