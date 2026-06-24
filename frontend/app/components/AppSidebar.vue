<script setup lang="ts">
const ui = useUiStore()
const can = useCan()

interface NavItem {
  labelKey: string
  icon: string
  to: string
  permission?: string
}
interface NavGroup {
  labelKey: string
  items: NavItem[]
}

const groups: NavGroup[] = [
  {
    labelKey: 'nav.group.main',
    items: [
      { labelKey: 'nav.dashboard', icon: 'i-lucide-layout-dashboard', to: '/' }
    ]
  },
  {
    labelKey: 'nav.group.asset',
    items: [
      { labelKey: 'nav.assets', icon: 'i-lucide-package', to: '/assets', permission: 'asset.read' },
      { labelKey: 'nav.assignment', icon: 'i-lucide-arrow-left-right', to: '/assignment', permission: 'asset.checkout' },
      { labelKey: 'nav.maintenance', icon: 'i-lucide-wrench', to: '/maintenance', permission: 'maintenance.manage' },
      { labelKey: 'nav.approval', icon: 'i-lucide-check-square', to: '/approval', permission: 'request.approve' }
    ]
  },
  {
    labelKey: 'nav.group.masterdata',
    items: [
      { labelKey: 'nav.offices', icon: 'i-lucide-building-2', to: '/master/offices', permission: 'masterdata.office.manage' },
      { labelKey: 'nav.employees', icon: 'i-lucide-users', to: '/master/employees', permission: 'masterdata.office.manage' },
      { labelKey: 'nav.reference', icon: 'i-lucide-list', to: '/master/reference', permission: 'masterdata.global.manage' }
    ]
  },
  {
    labelKey: 'nav.group.settings',
    items: [
      { labelKey: 'nav.users', icon: 'i-lucide-user-cog', to: '/settings/users', permission: 'user.manage' },
      { labelKey: 'nav.audit', icon: 'i-lucide-scroll-text', to: '/settings/audit', permission: 'audit.view' }
    ]
  }
]

function visibleItems(items: NavItem[]) {
  return items.filter(i => !i.permission || can(i.permission))
}

const visibleGroups = computed(() =>
  groups
    .map(g => ({ ...g, items: visibleItems(g.items) }))
    .filter(g => g.items.length)
)
</script>

<template>
  <aside
    class="flex flex-col border-e border-default bg-default transition-all"
    :class="ui.sidebarCollapsed ? 'w-16' : 'w-60'"
  >
    <div class="flex items-center gap-3 h-15 px-4 border-b border-default">
      <div class="size-9 rounded-lg bg-primary text-inverted flex items-center justify-center shrink-0">
        <UIcon
          name="i-lucide-package"
          class="size-5"
        />
      </div>
      <span
        v-if="!ui.sidebarCollapsed"
        class="font-bold text-lg"
      >{{ $t('app.name') }}</span>
    </div>

    <nav class="flex-1 overflow-y-auto p-3 space-y-4">
      <div
        v-for="group in visibleGroups"
        :key="group.labelKey"
      >
        <p
          v-if="!ui.sidebarCollapsed"
          class="px-3 pb-1 text-[10px] font-semibold uppercase tracking-wider text-dimmed font-mono"
        >
          {{ $t(group.labelKey) }}
        </p>
        <NuxtLink
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          :aria-label="$t(item.labelKey)"
          class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm hover:bg-elevated"
          active-class="bg-primary/10 text-primary font-medium"
        >
          <UIcon
            :name="item.icon"
            class="size-5 shrink-0"
          />
          <span v-if="!ui.sidebarCollapsed">{{ $t(item.labelKey) }}</span>
        </NuxtLink>
      </div>
    </nav>
  </aside>
</template>
