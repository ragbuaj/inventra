<script setup lang="ts">
const ui = useUiStore()
</script>

<template>
  <header class="flex items-center gap-3 h-15 px-4 border-b border-default bg-default">
    <UButton
      color="neutral"
      variant="ghost"
      icon="i-lucide-panel-left"
      :aria-label="$t('nav.toggleSidebar')"
      @click="ui.toggleSidebar()"
    />
    <GlobalSearch />
    <div class="ms-auto flex items-center gap-1">
      <LangSwitcher />
      <ThemeToggle />
      <NotificationBell :count="0" />
      <UserMenu />
    </div>
  </header>
</template>
