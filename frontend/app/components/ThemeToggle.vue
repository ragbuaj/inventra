<script setup lang="ts">
const colorMode = useColorMode()
const { t } = useI18n()

function toggle() {
  colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
}
</script>

<template>
  <button
    class="flex items-center justify-center w-9 h-9 rounded-[9px] border border-default bg-transparent text-muted cursor-pointer hover:bg-muted hover:text-default transition-colors"
    :title="colorMode.value === 'dark' ? t('theme.switchToLight') : t('theme.switchToDark')"
    @click="toggle"
  >
    <UIcon
      v-if="colorMode.value === 'dark'"
      name="i-lucide-sun"
      class="size-[17px]"
    />
    <UIcon
      v-else
      name="i-lucide-moon"
      class="size-[17px]"
    />
  </button>
</template>
