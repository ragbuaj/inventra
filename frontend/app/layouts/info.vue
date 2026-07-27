<script setup lang="ts">
// Adaptive chrome for the public info pages (privacy / guide / faq).
// - Authenticated users get the full app shell (sidebar + topbar) so the pages
//   feel like a normal in-app screen reachable from the sidebar and user menu.
// - Guests get a minimal public shell (logo, language/theme switch, sign-in) so
//   the same pages stay reachable without a session — e.g. the privacy policy
//   URL required by the Play Store listing.
// The auth client plugin is awaited before the app mounts, so isAuthenticated is
// already resolved at first render (no chrome flash), and the getter stays
// reactive if the session changes afterwards.
const auth = useAuthStore()
const { t } = useI18n()
const localePath = useLocalePath()

const publicLinks = computed(() => [
  { to: '/guide', label: t('nav.guide') },
  { to: '/faq', label: t('nav.faq') },
  { to: '/privacy', label: t('nav.privacy') }
])
</script>

<template>
  <!-- Signed in: reuse the standard app shell so info pages match every other screen. -->
  <div
    v-if="auth.isAuthenticated"
    class="flex h-screen overflow-hidden bg-default"
  >
    <AppSidebar />
    <div class="flex-1 flex flex-col min-w-0">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto px-4 py-5 sm:px-6 sm:py-6 lg:px-8 lg:py-7">
        <slot />
      </main>
    </div>
    <ConfirmDialog />
    <CommandPalette />
  </div>

  <!-- Guest: standalone reading shell, no app navigation. -->
  <div
    v-else
    class="min-h-screen flex flex-col bg-default"
  >
    <header class="flex-none border-b border-default bg-default/80 backdrop-blur sticky top-0 z-30">
      <div class="mx-auto w-full max-w-4xl flex items-center gap-3 px-4 sm:px-6 h-[61px]">
        <NuxtLink
          :to="localePath('/login')"
          class="flex items-center gap-[10px] min-w-0"
        >
          <span class="w-[34px] h-[34px] rounded-[9px] bg-primary text-inverted flex items-center justify-center flex-none shrink-0">
            <UIcon
              name="i-lucide-package"
              class="size-[19px]"
            />
          </span>
          <span class="font-bold text-[18px] tracking-tight whitespace-nowrap">{{ $t('app.name') }}</span>
        </NuxtLink>

        <!-- Cross-links between the info pages (md+; collapses on small screens) -->
        <nav class="hidden md:flex items-center gap-1 ms-4">
          <NuxtLink
            v-for="link in publicLinks"
            :key="link.to"
            :to="localePath(link.to)"
            class="px-3 py-[7px] text-[13.5px] rounded-[8px] text-muted hover:text-default hover:bg-muted transition-colors"
            active-class="text-primary font-medium bg-primary/10"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>

        <div class="flex items-center gap-2 ms-auto flex-none">
          <LangSwitcher />
          <ThemeToggle />
          <UButton
            :to="localePath('/login')"
            color="primary"
            size="sm"
            icon="i-lucide-log-in"
          >
            {{ $t('nav.signIn') }}
          </UButton>
        </div>
      </div>
    </header>

    <main class="flex-1 mx-auto w-full max-w-4xl px-4 sm:px-6 py-8 sm:py-10">
      <slot />
    </main>

    <footer class="flex-none border-t border-default">
      <div class="mx-auto w-full max-w-4xl px-4 sm:px-6 py-6 flex flex-col sm:flex-row items-center justify-between gap-3">
        <p class="text-[12.5px] text-muted">
          {{ $t('auth.brandCopyright') }}
        </p>
        <nav class="flex items-center gap-1">
          <NuxtLink
            v-for="link in publicLinks"
            :key="link.to"
            :to="localePath(link.to)"
            class="px-2 py-1 text-[12.5px] rounded-[6px] text-muted hover:text-default hover:bg-muted transition-colors"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>
      </div>
    </footer>
  </div>
</template>
