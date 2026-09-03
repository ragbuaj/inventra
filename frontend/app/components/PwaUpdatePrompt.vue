<script setup lang="ts">
// Ajakan muat ulang saat build baru sudah ter-precache dan service worker-nya menunggu.
//
// Modul PWA dipasang dengan `registerType: 'prompt'` justru supaya keputusan memuat
// ulang ada di tangan pengguna: muat ulang otomatis bisa membuang isian formulir aset
// yang sedang diketik. Komponen ini karena itu tidak pernah memanggil muat ulang
// sendiri — hanya lewat tombol.
//
// `$pwa` bisa saja tidak ada (service worker mati saat `pnpm dev`, atau plugin klien
// modul dimatikan di lingkungan tes), jadi setiap aksesnya opsional.
const { $pwa } = useNuxtApp()
const { t } = useI18n()

const dismissed = ref(false)
const show = computed(() => !!$pwa?.needRefresh && !dismissed.value)

function reload() {
  // `true` = aktifkan service worker yang menunggu lalu muat ulang halamannya.
  $pwa?.updateServiceWorker(true)
}

function dismiss() {
  // Dua langkah, dan keduanya perlu. `dismissed` menjaga kartu ini tidak balik lagi
  // di sesi yang sama; `cancelPrompt` menurunkan `needRefresh` supaya kunci yang
  // menahan `PwaInstallPrompt` ikut terlepas — tanpa itu ajakan pasang mati sesesi
  // penuh padahal layarnya sudah bersih. Service worker yang menunggu tidak
  // terpengaruh: ia tetap menunggu sampai pengguna memuat ulang sendiri.
  dismissed.value = true
  $pwa?.cancelPrompt()
}
</script>

<template>
  <UCard
    v-if="show"
    data-testid="pwa-update-prompt"
    role="status"
    class="fixed inset-x-4 bottom-4 z-50 shadow-lg sm:left-auto sm:w-96"
  >
    <div class="flex items-start gap-3">
      <UIcon
        name="i-lucide-refresh-cw"
        class="size-5 text-primary shrink-0 mt-0.5"
      />
      <p class="text-sm">
        {{ t('pwa.update.message') }}
      </p>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          data-testid="pwa-update-later"
          @click="dismiss"
        >
          {{ t('pwa.update.later') }}
        </UButton>
        <UButton
          color="primary"
          data-testid="pwa-update-reload"
          @click="reload"
        >
          {{ t('pwa.update.reload') }}
        </UButton>
      </div>
    </template>
  </UCard>
</template>
