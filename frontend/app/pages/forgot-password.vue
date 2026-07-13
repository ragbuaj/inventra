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
  } catch (err: unknown) {
    errorKey.value = (err as { statusCode?: number }).statusCode === 429 ? 'auth.forgotRateLimited' : 'common.error'
  } finally {
    loading.value = false
  }
}

async function submit() {
  await doRequest()
  if (sent.value) cooldown.start()
}

async function resend() {
  if (!cooldown.canResend.value) return
  await doRequest()
  cooldown.start()
}
</script>

<template>
  <div class="w-full max-w-sm mx-auto">
    <h1 class="text-xl font-semibold mb-1">
      {{ t('auth.forgotTitle') }}
    </h1>
    <p class="text-muted text-sm mb-6">
      {{ t('auth.forgotSubtitle') }}
    </p>

    <template v-if="sent">
      <UAlert
        color="success"
        variant="soft"
        :title="t('auth.forgotSent')"
        data-testid="forgot-sent"
      />
      <UButton
        data-testid="forgot-resend"
        variant="soft"
        block
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
      @submit="submit"
    >
      <UFormField
        :label="t('auth.email')"
        name="email"
      >
        <UInput
          v-model="email"
          type="email"
          required
          autocomplete="email"
          class="w-full"
          data-testid="forgot-email"
        />
      </UFormField>
      <p
        v-if="errorKey"
        class="text-error text-sm mt-2"
      >
        {{ t(errorKey) }}
      </p>
      <UButton
        type="submit"
        block
        class="mt-4"
        :loading="loading"
        data-testid="forgot-submit"
      >
        {{ t('auth.forgotSubmit') }}
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
