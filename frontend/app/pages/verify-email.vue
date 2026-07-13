<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const account = useAccount()
const auth = useAuthStore()

const token = computed(() => (route.query.token as string) || '')
const status = ref<'loading' | 'success' | 'error'>(token.value ? 'loading' : 'error')

onMounted(async () => {
  if (!token.value) {
    status.value = 'error'
    return
  }
  try {
    await account.confirmEmailChange(token.value)
    status.value = 'success'
    // Best-effort: if the user is already logged in, refresh their cached
    // email so the shell/topbar don't keep showing the stale address. Never
    // block the success state on this — the confirm itself already succeeded.
    if (auth.isAuthenticated && auth.user && auth.accessToken) {
      try {
        const profile = await account.getProfile()
        auth.setSession(auth.accessToken, { ...auth.user, email: profile.email }, auth.permissions)
      } catch {
        // ignore — the email change is already confirmed server-side
      }
    }
  } catch {
    status.value = 'error'
  }
})
</script>

<template>
  <div class="w-full max-w-sm mx-auto">
    <h1 class="text-xl font-semibold mb-6">
      {{ t('auth.verifyEmailTitle') }}
    </h1>

    <div
      v-if="status === 'loading'"
      class="flex flex-col items-center gap-3 py-6 text-center"
      data-testid="verify-email-loading"
    >
      <UIcon
        name="i-lucide-loader-2"
        class="size-6 animate-spin text-primary"
      />
      <p class="text-muted text-sm">
        {{ t('auth.verifyEmailLoading') }}
      </p>
    </div>

    <template v-else-if="status === 'success'">
      <UAlert
        color="success"
        variant="soft"
        :title="t('auth.verifyEmailSuccess')"
        data-testid="verify-email-success"
      />
      <div class="mt-6 text-center">
        <NuxtLink
          :to="localePath(auth.isAuthenticated ? '/account' : '/login')"
          class="text-primary text-sm"
        >
          {{ auth.isAuthenticated ? t('auth.verifyEmailToAccount') : t('auth.backToLogin') }}
        </NuxtLink>
      </div>
    </template>

    <template v-else>
      <UAlert
        color="error"
        variant="soft"
        :title="t('auth.verifyEmailError')"
        data-testid="verify-email-error"
      />
      <div class="mt-6 text-center">
        <NuxtLink
          :to="localePath('/login')"
          class="text-primary text-sm"
        >
          {{ t('auth.backToLogin') }}
        </NuxtLink>
      </div>
    </template>
  </div>
</template>
