<script setup lang="ts">
const auth = useAuthStore()
const { logout } = useAuthApi()
const { t } = useI18n()

const open = ref(false)

const userName = computed(() => auth.user?.name ?? '')
const userEmail = computed(() => auth.user?.email ?? '')
const userRole = computed(() => auth.user?.role_name || 'Superadmin')
const userScope = computed(() => t('nav.scopeGlobal'))

const initials = computed(() => {
  const name = userName.value.trim()
  if (!name) return '?'
  const parts = name.split(/\s+/)
  if (parts.length >= 2) {
    return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase()
  }
  return name.slice(0, 2).toUpperCase()
})

function handleLogout() {
  open.value = false
  logout()
}
</script>

<template>
  <UPopover v-model:open="open">
    <!-- Pill trigger: avatar initials + chevron, no name text -->
    <button
      class="flex items-center gap-2 px-1 py-1 pr-2 border border-default rounded-full bg-transparent cursor-pointer hover:bg-muted transition-colors"
      @click="open = !open"
    >
      <span class="w-[30px] h-[30px] rounded-full bg-primary text-white flex items-center justify-center text-[12px] font-bold flex-none select-none">
        {{ initials }}
      </span>
      <UIcon
        name="i-lucide-chevron-down"
        class="size-[15px] text-muted"
      />
    </button>

    <template #content>
      <div class="w-[264px] overflow-hidden rounded-[13px]">
        <!-- Header: larger avatar + name + email -->
        <div class="flex gap-[11px] items-center px-[15px] py-[15px] border-b border-default">
          <span class="w-[42px] h-[42px] rounded-full bg-primary text-white flex items-center justify-center text-[15px] font-bold flex-none select-none">
            {{ initials }}
          </span>
          <div class="min-w-0">
            <div class="text-[14px] font-semibold truncate">
              {{ userName }}
            </div>
            <div class="text-[12px] text-muted truncate">
              {{ userEmail }}
            </div>
          </div>
        </div>

        <!-- Role / scope section -->
        <div class="flex items-center gap-2 px-[15px] py-[11px] border-b border-default">
          <span class="inline-flex items-center gap-[6px] px-[10px] py-[3px] text-[12px] font-semibold rounded-full bg-primary/10 text-primary">
            <UIcon
              name="i-lucide-shield"
              class="size-3"
            />
            {{ userRole }}
          </span>
          <span class="text-[12px] text-muted">{{ userScope }}</span>
        </div>

        <!-- Menu items -->
        <div class="p-[6px]">
          <button
            class="flex items-center gap-[10px] w-full px-[10px] py-[9px] text-[14px] text-default bg-transparent border-0 rounded-[8px] cursor-pointer text-left hover:bg-muted transition-colors"
            @click="open = false"
          >
            <UIcon
              name="i-lucide-user"
              class="size-4 flex-none"
            />
            {{ t('nav.profile') }}
          </button>
          <button
            class="flex items-center gap-[10px] w-full px-[10px] py-[9px] text-[14px] text-default bg-transparent border-0 rounded-[8px] cursor-pointer text-left hover:bg-muted transition-colors"
            @click="open = false"
          >
            <UIcon
              name="i-lucide-settings"
              class="size-4 flex-none"
            />
            {{ t('nav.accountSettings') }}
          </button>
          <div class="h-px bg-border my-[5px] mx-1" />
          <button
            class="flex items-center gap-[10px] w-full px-[10px] py-[9px] text-[14px] font-medium text-error bg-transparent border-0 rounded-[8px] cursor-pointer text-left hover:bg-error/10 transition-colors"
            @click="handleLogout"
          >
            <UIcon
              name="i-lucide-log-out"
              class="size-4 flex-none"
            />
            {{ t('nav.signOut') }}
          </button>
        </div>
      </div>
    </template>
  </UPopover>
</template>
