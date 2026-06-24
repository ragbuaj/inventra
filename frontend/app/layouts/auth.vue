<script setup lang="ts">
const { t } = useI18n()
const features = computed(() => [
  t('auth.brandFeature1'),
  t('auth.brandFeature2'),
  t('auth.brandFeature3')
])
</script>

<template>
  <div class="min-h-screen grid lg:grid-cols-[1.05fr_1fr]">
    <!-- Brand panel -->
    <aside class="relative hidden lg:flex flex-col justify-between overflow-hidden p-12 text-white bg-gradient-to-br from-green-900 via-green-700 to-green-600">
      <div class="absolute -top-24 -right-20 size-80 rounded-full bg-white/10 blur-lg" />
      <div class="absolute -bottom-28 -left-16 size-72 rounded-full bg-white/[0.07]" />

      <div class="relative flex items-center gap-3">
        <div class="flex items-center justify-center size-11 rounded-xl border border-white/25 bg-white/15 backdrop-blur">
          <UIcon
            name="i-lucide-package"
            class="size-6"
          />
        </div>
        <span class="text-2xl font-bold tracking-tight">Inventra</span>
      </div>

      <div class="relative max-w-md">
        <h1 class="mb-4 text-3xl font-bold leading-tight tracking-tight">
          {{ $t('auth.brandHeadline') }}
        </h1>
        <p class="mb-8 leading-relaxed text-white/80">
          {{ $t('auth.brandTagline') }}
        </p>
        <ul class="space-y-3.5">
          <li
            v-for="f in features"
            :key="f"
            class="flex items-center gap-3 font-medium text-white/95"
          >
            <span class="flex items-center justify-center size-6 shrink-0 rounded-full bg-white/15">
              <UIcon
                name="i-lucide-check"
                class="size-3.5"
              />
            </span>
            {{ f }}
          </li>
        </ul>
      </div>

      <p class="relative text-sm text-white/60">
        {{ $t('auth.brandCopyright') }}
      </p>
    </aside>

    <!-- Form panel -->
    <main class="relative flex items-center justify-center p-6">
      <div class="absolute top-5 right-5 flex items-center gap-2">
        <LangSwitcher />
        <ThemeToggle />
      </div>
      <slot />
    </main>
  </div>
</template>
