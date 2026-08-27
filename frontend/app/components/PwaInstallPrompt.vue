<script setup lang="ts">
// Ajakan memasang aplikasi ke perangkat.
//
// Dua jalur, karena peramban memang terbelah dua: yang memicu
// `beforeinstallprompt` (Chrome Android, Edge, Chrome desktop) memberi dialog
// pasang bawaan yang boleh dipanggil aplikasi, sedangkan Safari iOS tidak
// menyediakan API apa pun — di sana satu-satunya jalan adalah petunjuk manual
// Bagikan lalu Tambahkan ke Layar Utama.
//
// Ajakan ini sengaja tidak muncul di layar masuk: mengganggu alur login demi
// pemasangan bukan pertukaran yang sepadan.
//
// Kuncinya diimpor, bukan diketik ulang: plugin klien modul membaca kunci yang sama
// saat menentukan apakah ajakan pernah ditutup (lihat pwa/client.ts), dan dua literal
// yang menyimpang membuat penutupan berhenti bertahan lintas pemuatan halaman.
import { PWA_INSTALL_DISMISS_KEY } from '~~/pwa/client'

const { $pwa } = useNuxtApp()
const { t } = useI18n()
const route = useRoute()

const dismissed = ref(false)
const iosSafari = ref(false)
const standalone = ref(false)

onMounted(() => {
  try {
    dismissed.value = localStorage.getItem(PWA_INSTALL_DISMISS_KEY) === 'true'
  } catch {
    // Mode privat sebagian peramban melempar error saat localStorage dibaca;
    // ajakan tetap boleh muncul, hanya penutupannya yang tidak bertahan.
  }
  iosSafari.value = isIosSafari(navigator.userAgent, navigator.maxTouchPoints)
  standalone.value = isStandaloneDisplay(window)
})

const installed = computed(() => standalone.value || !!$pwa?.isPWAInstalled)
// Ajakan perbarui memakai sudut layar yang sama dan lebih mendesak; satu ajakan
// pada satu waktu.
const blocked = computed(() =>
  dismissed.value || installed.value || route.meta.layout === 'auth' || !!$pwa?.needRefresh
)

const showAction = computed(() => !blocked.value && !!$pwa?.showInstallPrompt)
const showIosHint = computed(() => !blocked.value && iosSafari.value)
const show = computed(() => showAction.value || showIosHint.value)

function install() {
  $pwa?.install()
}

function dismiss() {
  dismissed.value = true
  try {
    localStorage.setItem(PWA_INSTALL_DISMISS_KEY, 'true')
  } catch { /* lihat catatan di onMounted */ }
  $pwa?.cancelInstall()
}
</script>

<template>
  <UCard
    v-if="show"
    data-testid="pwa-install-prompt"
    role="status"
    class="fixed inset-x-4 bottom-4 z-50 shadow-lg sm:left-auto sm:w-96"
  >
    <div class="flex items-start gap-3">
      <UIcon
        name="i-lucide-download"
        class="size-5 text-primary shrink-0 mt-0.5"
      />
      <div class="space-y-1">
        <p class="text-sm">
          {{ t('pwa.install.message') }}
        </p>
        <p
          v-if="showIosHint"
          data-testid="pwa-install-ios-hint"
          class="text-sm text-muted"
        >
          {{ t('pwa.install.iosHint') }}
        </p>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          data-testid="pwa-install-later"
          @click="dismiss"
        >
          {{ t('pwa.install.later') }}
        </UButton>
        <UButton
          v-if="showAction"
          color="primary"
          data-testid="pwa-install-action"
          @click="install"
        >
          {{ t('pwa.install.action') }}
        </UButton>
      </div>
    </template>
  </UCard>
</template>
