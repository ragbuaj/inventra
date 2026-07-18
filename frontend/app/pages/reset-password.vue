<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const account = useAccount()
const auth = useAuthStore()
const token = computed(() => (route.query.token as string) || '')
const newPass = ref('')
const confirmPass = ref('')
const showPassword = ref(false)
const loading = ref(false)
const errorKey = ref('')

async function submit() {
  errorKey.value = ''
  if (newPass.value.length < 8) {
    errorKey.value = 'account.errWeak'
    return
  }
  if (newPass.value !== confirmPass.value) {
    errorKey.value = 'account.errConfirmMismatch'
    return
  }
  loading.value = true
  try {
    await account.resetPassword(token.value, newPass.value)
    // Resetting the password invalidates all sessions server-side (password_changed_at
    // epoch). Drop any stale local session so a still-"authenticated" client isn't
    // bounced /login -> / by auth.global.ts (happens when a logged-in user changes
    // their password via the emailed link).
    auth.clear()
    await navigateTo({ path: localePath('/login'), query: { reset: 'success' } })
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 400 ? 'auth.resetInvalid' : 'common.error'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-[392px]">
    <h1 class="text-2xl font-bold tracking-tight">
      {{ t('auth.resetTitle') }}
    </h1>
    <p class="mt-1 mb-6 text-muted">
      {{ t('auth.resetSubtitle') }}
    </p>

    <UAlert
      v-if="!token"
      icon="i-lucide-circle-x"
      color="error"
      variant="subtle"
      :description="t('auth.resetInvalid')"
      data-testid="reset-notoken"
    />

    <UForm
      v-else
      :state="{ newPass, confirmPass }"
      class="space-y-4"
      @submit="submit"
    >
      <UAlert
        v-if="errorKey"
        icon="i-lucide-circle-x"
        color="error"
        variant="subtle"
        :description="t(errorKey)"
        data-testid="reset-error"
      />

      <UFormField
        :label="t('auth.newPassword')"
        name="newPass"
        required
      >
        <UInput
          v-model="newPass"
          :type="showPassword ? 'text' : 'password'"
          icon="i-lucide-lock"
          size="lg"
          class="w-full"
          :placeholder="t('auth.newPasswordPlaceholder')"
          required
          autocomplete="new-password"
          data-testid="reset-new"
        >
          <template #trailing>
            <UButton
              type="button"
              color="neutral"
              variant="link"
              size="sm"
              :icon="showPassword ? 'i-lucide-eye-off' : 'i-lucide-eye'"
              :aria-label="t('auth.togglePassword')"
              @click="() => { showPassword = !showPassword }"
            />
          </template>
        </UInput>
        <PasswordStrengthMeter
          :password="newPass"
          class="mt-2"
        />
      </UFormField>

      <UFormField
        :label="t('auth.confirmPassword')"
        name="confirmPass"
        required
      >
        <UInput
          v-model="confirmPass"
          :type="showPassword ? 'text' : 'password'"
          icon="i-lucide-lock"
          size="lg"
          class="w-full"
          :placeholder="t('auth.confirmPasswordPlaceholder')"
          required
          autocomplete="new-password"
          data-testid="reset-confirm"
        />
      </UFormField>

      <UButton
        type="submit"
        block
        size="lg"
        :loading="loading"
        data-testid="reset-submit"
      >
        {{ t('auth.resetSubmit') }}
      </UButton>
    </UForm>

    <div class="mt-6 text-center">
      <NuxtLink
        :to="localePath('/login')"
        class="text-primary text-sm hover:underline"
      >
        {{ t('auth.backToLogin') }}
      </NuxtLink>
    </div>
  </div>
</template>
