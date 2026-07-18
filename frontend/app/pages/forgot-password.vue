<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const localePath = useLocalePath()
const account = useAccount()
const email = ref('')
const sent = ref(false)
const loading = ref(false)
const errorKey = ref('')
const cooldown = useResendCooldown(30)

async function doRequest() {
  loading.value = true
  errorKey.value = ''
  try {
    await account.requestPasswordReset(email.value.trim())
    sent.value = true
    return true
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 429 ? 'auth.forgotRateLimited' : 'common.error'
    return false
  } finally {
    loading.value = false
  }
}

async function submit() {
  const ok = await doRequest()
  if (ok) cooldown.start()
}

async function resend() {
  if (!cooldown.canResend.value) return
  const ok = await doRequest()
  if (ok) cooldown.start()
}
</script>

<template>
  <div class="w-full max-w-[392px]">
    <h1 class="text-2xl font-bold tracking-tight">
      {{ t('auth.forgotTitle') }}
    </h1>
    <p class="mt-1 mb-6 text-muted">
      {{ t('auth.forgotSubtitle') }}
    </p>

    <template v-if="sent">
      <UAlert
        icon="i-lucide-mail-check"
        color="success"
        variant="subtle"
        :description="t('auth.forgotSent')"
        data-testid="forgot-sent"
      />
      <UAlert
        v-if="errorKey"
        icon="i-lucide-circle-x"
        color="error"
        variant="subtle"
        :description="t(errorKey)"
        class="mt-3"
      />
      <UButton
        data-testid="forgot-resend"
        variant="soft"
        block
        size="lg"
        class="mt-3"
        :disabled="!cooldown.canResend.value || loading"
        :loading="loading"
        @click="resend"
      >
        {{ cooldown.canResend.value ? t('auth.forgotResend') : t('auth.forgotResendWait', { s: cooldown.remaining.value }) }}
      </UButton>
    </template>

    <UForm
      v-else
      :state="{ email }"
      class="space-y-4"
      @submit="submit"
    >
      <UAlert
        v-if="errorKey"
        icon="i-lucide-circle-x"
        color="error"
        variant="subtle"
        :description="t(errorKey)"
      />

      <UFormField
        :label="t('auth.email')"
        name="email"
        required
      >
        <UInput
          v-model="email"
          type="email"
          icon="i-lucide-mail"
          size="lg"
          class="w-full"
          :placeholder="t('auth.emailPlaceholder')"
          required
          autocomplete="email"
          data-testid="forgot-email"
        />
      </UFormField>

      <UButton
        type="submit"
        block
        size="lg"
        :loading="loading"
        data-testid="forgot-submit"
      >
        {{ t('auth.forgotSubmit') }}
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
