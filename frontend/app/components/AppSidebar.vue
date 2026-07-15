<script setup lang="ts">
import { appNav } from '~/utils/nav'
import type { NavItem } from '~/types'

const ui = useUiStore()
const can = useCan()
const auth = useAuthStore()
const inboxStore = useInboxStore()
const { t } = useI18n()
const localePath = useLocalePath()

// Single per-permission nav model: every role sees exactly the items its
// permissions unlock (see isVisible). No more binary superadmin/staff split.
const nav = appNav

// Track which parent groups are expanded; default all open
const expandedGroups = ref<Record<string, boolean>>({})

function isGroupExpanded(labelKey: string): boolean {
  if (expandedGroups.value[labelKey] === undefined) {
    return true
  }
  return expandedGroups.value[labelKey]
}

function toggleGroup(labelKey: string) {
  if (expandedGroups.value[labelKey] === undefined) {
    expandedGroups.value[labelKey] = false
  } else {
    expandedGroups.value[labelKey] = !expandedGroups.value[labelKey]
  }
}

// Clicking a parent group while the sidebar is collapsed can't reveal its
// children (they only render when expanded), so open the sidebar first and
// force this group open instead of silently toggling nothing.
function onParentClick(labelKey: string) {
  if (ui.sidebarCollapsed) {
    ui.sidebarCollapsed = false
    expandedGroups.value[labelKey] = true
    return
  }
  toggleGroup(labelKey)
}

// OR semantics: no permission means always visible; an array is satisfied when
// the caller holds any one of its keys.
function hasAny(permission?: string | string[]): boolean {
  if (!permission) return true
  return Array.isArray(permission) ? permission.some(p => can(p)) : can(permission)
}

function isVisible(item: NavItem): boolean {
  // A parent group is visible only when at least one of its children is; this
  // auto-hides an entire group whose items the caller cannot reach.
  if (item.children) return item.children.some(isVisible)
  return hasAny(item.permission)
}

// Drop a whole top-level section (e.g. Administrasi) when none of its items are
// visible, so the section header never renders over an empty list.
const visibleNav = computed(() => nav.filter(group => group.items.some(isVisible)))

// The approval leaf shows the live pending-approval count from the inbox store
// instead of any static badgeCount.
function badgeFor(item: NavItem): number | undefined {
  if (item.labelKey === 'nav.approval') {
    return inboxStore.pendingCount || undefined
  }
  return item.badgeCount
}

// Compute initials from the user's name (first letter of each word, max 2)
const userInitials = computed(() => {
  const name = auth.user?.name ?? ''
  return name
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map(w => w[0]?.toUpperCase() ?? '')
    .join('')
})

const userName = computed(() => auth.user?.name ?? '')
const userScope = computed(() => auth.user?.role_name ?? '')

// Lock the rail to an exact px width. A bare `width` is treated as a flex-basis
// the flex row can override (leaving the rail content-wide); pinning min/max as
// well makes the collapse deterministic. Transitions smoothly via `transition-all`.
const sidebarWidth = computed(() => ui.sidebarCollapsed ? '76px' : '264px')
</script>

<template>
  <aside
    class="flex flex-col border-e border-default bg-default transition-colors duration-200 overflow-hidden shrink-0"
    :style="{ width: sidebarWidth, minWidth: sidebarWidth, maxWidth: sidebarWidth }"
  >
    <!-- Logo row -->
    <div class="flex items-center gap-[11px] h-[61px] px-[18px] border-b border-default flex-none">
      <div class="w-[34px] h-[34px] rounded-[9px] bg-primary text-inverted flex items-center justify-center flex-none shrink-0">
        <!-- Archive/box icon from mockup -->
        <svg
          width="19"
          height="19"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M21 8V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v2" />
          <path d="M3 8h18v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
          <path d="M9 13h6" />
        </svg>
      </div>
      <span
        v-if="!ui.sidebarCollapsed"
        data-wordmark
        class="font-bold text-[18px] tracking-tight whitespace-nowrap"
      >{{ $t('app.name') }}</span>
    </div>

    <!-- Nav -->
    <nav class="flex-1 overflow-y-auto overflow-x-hidden px-3 pt-3 pb-[18px]">
      <div
        v-for="group in visibleNav"
        :key="group.labelKey"
        class="mb-[6px]"
      >
        <!-- Section label (expanded) or divider (collapsed) -->
        <div
          v-if="!ui.sidebarCollapsed"
          class="px-3 pt-[14px] pb-[6px] text-[10px] font-semibold uppercase tracking-[.14em] text-dimmed font-mono whitespace-nowrap"
        >
          {{ $t(group.labelKey) }}
        </div>
        <div
          v-else
          class="h-px bg-[var(--ui-border)] mx-2 my-[10px]"
        />

        <template
          v-for="item in group.items"
          :key="item.labelKey"
        >
          <template v-if="isVisible(item)">
            <!-- Parent group with children -->
            <template v-if="item.children">
              <button
                type="button"
                :title="ui.sidebarCollapsed ? t(item.labelKey) : undefined"
                class="relative flex w-full mb-[2px] rounded-[9px] text-sm font-normal text-default hover:bg-muted transition-colors cursor-pointer border-0"
                :class="ui.sidebarCollapsed ? 'flex-col items-center justify-center gap-[3px] px-1 py-[8px]' : 'items-center gap-[11px] px-3 py-[9px]'"
                :style="{ boxShadow: 'inset 3px 0 0 transparent' }"
                @click="onParentClick(item.labelKey)"
              >
                <UIcon
                  v-if="item.icon"
                  :name="item.icon"
                  class="size-[19px] shrink-0"
                />
                <span
                  v-if="ui.sidebarCollapsed"
                  class="w-full text-[10px] leading-tight text-center truncate"
                >{{ $t(item.labelKey) }}</span>
                <template v-if="!ui.sidebarCollapsed">
                  <span class="flex-1 overflow-hidden text-ellipsis text-left">{{ $t(item.labelKey) }}</span>
                  <!-- Chevron -->
                  <span
                    class="flex-none flex text-dimmed transition-transform duration-150"
                    :style="{ transform: isGroupExpanded(item.labelKey) ? 'rotate(90deg)' : 'rotate(0deg)' }"
                  >
                    <svg
                      width="15"
                      height="15"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      aria-hidden="true"
                    >
                      <polyline points="9 18 15 12 9 6" />
                    </svg>
                  </span>
                </template>
              </button>

              <!-- Children (expanded only) -->
              <div
                v-if="!ui.sidebarCollapsed && isGroupExpanded(item.labelKey)"
                class="ms-[23px] ps-[24px] border-s border-default flex flex-col gap-[1px] mb-[4px] mt-[2px]"
              >
                <template
                  v-for="child in item.children"
                  :key="child.labelKey"
                >
                  <template v-if="isVisible(child)">
                    <!-- Built child (has `to`) -->
                    <NuxtLink
                      v-if="!child.disabled && child.to"
                      :to="localePath(child.to)"
                      class="flex items-center w-full px-3 py-[8px] text-[13.5px] rounded-[8px] text-default hover:bg-muted transition-colors"
                      active-class="bg-primary/10 text-primary font-medium shadow-[inset_3px_0_0_var(--ui-primary)]"
                      :style="{ boxShadow: 'inset 3px 0 0 transparent' }"
                    >
                      {{ $t(child.labelKey) }}
                    </NuxtLink>
                    <!-- Disabled child -->
                    <span
                      v-else
                      :title="t('nav.comingSoon')"
                      class="flex items-center w-full px-3 py-[8px] text-[13.5px] rounded-[8px] text-dimmed cursor-not-allowed select-none"
                    >
                      {{ $t(child.labelKey) }}
                    </span>
                  </template>
                </template>
              </div>
            </template>

            <!-- Leaf item -->
            <template v-else>
              <!-- Built leaf (has `to`) -->
              <NuxtLink
                v-if="!item.disabled && item.to"
                :to="localePath(item.to)"
                :aria-label="t(item.labelKey)"
                :title="ui.sidebarCollapsed ? t(item.labelKey) : undefined"
                class="relative flex w-full mb-[2px] rounded-[9px] text-sm text-default hover:bg-muted transition-colors"
                :class="ui.sidebarCollapsed ? 'flex-col items-center justify-center gap-[3px] px-1 py-[8px]' : 'items-center gap-[11px] px-3 py-[9px]'"
                active-class="text-primary font-medium bg-primary/10 shadow-[inset_3px_0_0_var(--ui-primary)]"
                :style="{ boxShadow: 'inset 3px 0 0 transparent' }"
              >
                <UIcon
                  v-if="item.icon"
                  :name="item.icon"
                  class="size-[19px] shrink-0"
                />
                <span
                  v-if="!ui.sidebarCollapsed"
                  class="flex-1 overflow-hidden text-ellipsis"
                >{{ $t(item.labelKey) }}</span>
                <span
                  v-else
                  class="w-full text-[10px] leading-tight text-center truncate"
                >{{ $t(item.labelKey) }}</span>
                <!-- Badge expanded -->
                <span
                  v-if="!ui.sidebarCollapsed && badgeFor(item)"
                  class="flex-none min-w-[20px] h-[20px] px-[6px] inline-flex items-center justify-center text-[11px] font-bold text-inverted bg-error rounded-full"
                >{{ badgeFor(item) }}</span>
                <!-- Badge collapsed -->
                <span
                  v-if="ui.sidebarCollapsed && badgeFor(item)"
                  class="absolute top-[6px] right-[10px] min-w-[16px] h-[16px] px-[4px] inline-flex items-center justify-center text-[9px] font-bold text-inverted bg-error rounded-full"
                >{{ badgeFor(item) }}</span>
              </NuxtLink>

              <!-- Disabled leaf -->
              <span
                v-else
                :aria-label="t(item.labelKey)"
                :title="ui.sidebarCollapsed ? t(item.labelKey) : t('nav.comingSoon')"
                tabindex="-1"
                aria-disabled="true"
                class="relative flex w-full mb-[2px] rounded-[9px] text-sm text-dimmed cursor-not-allowed select-none"
                :class="ui.sidebarCollapsed ? 'flex-col items-center justify-center gap-[3px] px-1 py-[8px]' : 'items-center gap-[11px] px-3 py-[9px]'"
              >
                <UIcon
                  v-if="item.icon"
                  :name="item.icon"
                  class="size-[19px] shrink-0"
                />
                <span
                  v-if="!ui.sidebarCollapsed"
                  class="flex-1 overflow-hidden text-ellipsis"
                >{{ $t(item.labelKey) }}</span>
                <span
                  v-else
                  class="w-full text-[10px] leading-tight text-center truncate"
                >{{ $t(item.labelKey) }}</span>
                <!-- Badge expanded -->
                <span
                  v-if="!ui.sidebarCollapsed && badgeFor(item)"
                  class="flex-none min-w-[20px] h-[20px] px-[6px] inline-flex items-center justify-center text-[11px] font-bold text-inverted bg-error rounded-full"
                >{{ badgeFor(item) }}</span>
                <!-- Badge collapsed -->
                <span
                  v-if="ui.sidebarCollapsed && badgeFor(item)"
                  class="absolute top-[6px] right-[10px] min-w-[16px] h-[16px] px-[4px] inline-flex items-center justify-center text-[9px] font-bold text-inverted bg-error rounded-full"
                >{{ badgeFor(item) }}</span>
              </span>
            </template>
          </template>
        </template>
      </div>
    </nav>

    <!-- Bottom user strip -->
    <div class="flex-none px-3 py-3 border-t border-default">
      <div
        class="flex items-center gap-[10px] px-2 py-[7px] rounded-[10px]"
        :class="{ 'justify-center': ui.sidebarCollapsed }"
      >
        <!-- Avatar with initials -->
        <div class="w-[34px] h-[34px] rounded-full bg-primary/10 text-primary flex items-center justify-center text-[13px] font-bold flex-none shrink-0">
          {{ userInitials }}
        </div>
        <div
          v-if="!ui.sidebarCollapsed"
          class="flex-1 min-w-0"
        >
          <div class="text-[13px] font-semibold whitespace-nowrap overflow-hidden text-ellipsis">
            {{ userName }}
          </div>
          <div class="text-[12px] text-muted whitespace-nowrap overflow-hidden text-ellipsis">
            {{ userScope }}
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>
