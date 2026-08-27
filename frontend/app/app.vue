<script setup>
// Catatan: tag PWA (manifest, theme-color, apple-touch-icon) TIDAK di sini melainkan di
// `app.head` pada nuxt.config.ts. Aplikasi ini SPA (`ssr: false`), sehingga useHead baru
// berlaku setelah hidrasi — terlambat untuk kriteria pemasangan dan untuk Safari iOS yang
// membaca apple-touch-icon saat halaman dimuat.
useHead({
  htmlAttrs: { lang: 'id' },
  link: [
    { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
    { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
    { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap' },
    { rel: 'icon', href: '/favicon.ico' }
  ]
})
useSeoMeta({ title: 'Inventra', description: 'Manajemen aset & inventaris' })
</script>

<template>
  <UApp>
    <NuxtLoadingIndicator />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
  </UApp>
</template>
