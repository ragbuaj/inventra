<template>
  <div class="min-h-screen grid lg:grid-cols-2">
    <div
      class="hidden lg:flex flex-col justify-between p-12 bg-primary text-inverted"
    >
      <div class="flex items-center gap-3">
        <UIcon
          name="i-lucide-package"
          class="size-8"
        />
        <span class="text-2xl font-bold">{{ $t('app.name') }}</span>
      </div>
      <p class="text-lg opacity-90">
        {{ $t('app.tagline') }}
      </p>
      <span class="text-sm opacity-70">© {{ year }} Inventra</span>
    </div>
    <div class="flex items-center justify-center p-6">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
const year = new Date().getFullYear()
</script>
