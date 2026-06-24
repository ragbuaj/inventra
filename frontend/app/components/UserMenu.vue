<script setup lang="ts">
const auth = useAuthStore()
const { logout } = useAuthApi()
const { t } = useI18n()
const items = computed(() => [[
  { label: t('nav.profile'), icon: 'i-lucide-user' },
  { label: t('auth.signOut'), icon: 'i-lucide-log-out', onSelect: () => logout() }
]])
</script>

<template>
  <UDropdownMenu :items="items">
    <button class="flex items-center gap-2">
      <UAvatar
        :alt="auth.user?.name ?? ''"
        size="sm"
      />
      <div class="hidden md:block text-start">
        <p class="text-sm font-medium">
          {{ auth.user?.name }}
        </p>
        <p class="text-xs text-muted">
          {{ auth.user?.role_name }}
        </p>
      </div>
    </button>
  </UDropdownMenu>
</template>
