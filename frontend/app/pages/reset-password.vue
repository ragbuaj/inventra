<script setup lang="ts">
import { passwordStrength } from '~/utils/passwordStrength'

definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const account = useAccount()
const token = computed(() => (route.query.token as string) || '')
const newPass = ref('')
const confirmPass = ref('')
const loading = ref(false)
const errorKey = ref('')
const strength = computed(() => passwordStrength(newPass.value))

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
    await navigateTo({ path: localePath('/login'), query: { reset: 'success' } })
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 400 ? 'auth.resetInvalid' : 'common.error'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-sm mx-auto">
    <h1 class="text-xl font-semibold mb-6">
      {{ t('auth.resetTitle') }}
    </h1>

    <UAlert
      v-if="!token"
      color="error"
      variant="soft"
      :title="t('auth.resetInvalid')"
      data-testid="reset-notoken"
    />

    <UForm
      v-else
      :state="{ newPass, confirmPass }"
      @submit="submit"
    >
      <UFormField
        :label="t('auth.newPassword')"
        name="newPass"
      >
        <UInput
          v-model="newPass"
          type="password"
          required
          autocomplete="new-password"
          data-testid="reset-new"
        />
      </UFormField>
      <UFormField
        :label="t('auth.confirmPassword')"
        name="confirmPass"
        class="mt-3"
      >
        <UInput
          v-model="confirmPass"
          type="password"
          required
          autocomplete="new-password"
          data-testid="reset-confirm"
        />
      </UFormField>
      <p class="text-muted text-xs mt-2">
        {{ strength.labelKey ? t(strength.labelKey) : '' }}
      </p>
      <p
        v-if="errorKey"
        class="text-error text-sm mt-2"
        data-testid="reset-error"
      >
        {{ t(errorKey) }}
      </p>
      <UButton
        type="submit"
        block
        class="mt-4"
        :loading="loading"
        data-testid="reset-submit"
      >
        {{ t('auth.resetSubmit') }}
      </UButton>
    </UForm>

    <div class="mt-6 text-center">
      <NuxtLink
        :to="localePath('/login')"
        class="text-primary text-sm"
      >
        {{ t('auth.backToLogin') }}
      </NuxtLink>
    </div>
  </div>
</template>
