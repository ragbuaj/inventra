<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()
const items = computed(() => (locales.value as { code: string, name: string }[]).map(l => ({
  label: l.name,
  onSelect: () => setLocale(l.code as 'id' | 'en')
})))
</script>

<template>
  <UDropdownMenu :items="items">
    <UButton
      color="neutral"
      variant="ghost"
      icon="i-lucide-languages"
      :label="locale.toUpperCase()"
    />
  </UDropdownMenu>
</template>
