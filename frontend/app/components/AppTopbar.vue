<script setup lang="ts">
import { appNav } from '~/utils/nav'

const ui = useUiStore()
const { t } = useI18n()
const route = useRoute()

// Derive breadcrumb parent + current page label from the nav model + route path
const routeLabel = computed(() => {
  const path = route.path

  // Flatten all nav items + their children to find a match
  for (const group of appNav) {
    for (const item of group.items) {
      if (item.to && item.to === path) {
        return { parent: null, label: t(item.labelKey) }
      }
      if (item.children) {
        for (const child of item.children) {
          if (child.to && child.to === path) {
            return { parent: t(item.labelKey), label: t(child.labelKey) }
          }
        }
      }
    }
  }
  // Fallback: use app name as label
  return { parent: null, label: t('app.name') }
})

const pageTitle = computed(() => routeLabel.value.label)
const breadcrumbParent = computed(() => routeLabel.value.parent)
</script>

<template>
  <header
    class="relative z-30 flex items-center gap-[14px] h-[61px] flex-none px-5 bg-default border-b border-default"
  >
    <!-- Sidebar toggle -->
    <button
      class="flex items-center justify-center w-9 h-9 flex-none rounded-[9px] border border-default bg-transparent text-muted cursor-pointer hover:bg-muted hover:text-default transition-colors"
      :title="$t('nav.toggleSidebar')"
      @click="ui.toggleSidebar()"
    >
      <UIcon
        name="i-lucide-panel-left"
        class="size-[18px]"
      />
    </button>

    <!-- Breadcrumb + page title two-line block -->
    <div class="flex flex-col gap-[1px] min-w-0">
      <div class="flex items-center gap-[6px] text-[11.5px] text-muted whitespace-nowrap">
        <span>{{ $t('app.name') }}</span>
        <template v-if="breadcrumbParent">
          <UIcon
            name="i-lucide-chevron-right"
            class="size-3 text-dimmed flex-none"
          />
          <span>{{ breadcrumbParent }}</span>
        </template>
      </div>
      <span class="text-[16px] font-semibold tracking-tight whitespace-nowrap overflow-hidden text-ellipsis">
        {{ pageTitle }}
      </span>
    </div>

    <!-- Centered search -->
    <GlobalSearch />

    <!-- Right cluster -->
    <div class="flex items-center gap-2 flex-none">
      <LangSwitcher />
      <ThemeToggle />
      <NotificationBell />
      <UserMenu />
    </div>
  </header>
</template>
