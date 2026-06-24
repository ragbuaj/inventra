<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const { login } = useAuthApi()

const state = reactive({ email: '', password: '' })
const loading = ref(false)
const errorMsg = ref('')

async function onSubmit() {
  loading.value = true
  errorMsg.value = ''
  try {
    await login(state.email, state.password)
    await navigateTo('/')
  } catch {
    errorMsg.value = t('auth.invalidCredentials')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard class="w-full max-w-sm">
    <template #header>
      <h1 class="text-xl font-semibold">
        {{ $t('auth.signInTitle') }}
      </h1>
      <p class="text-sm text-muted">
        {{ $t('auth.signInSubtitle') }}
      </p>
    </template>

    <UAlert
      v-if="errorMsg"
      color="error"
      variant="subtle"
      :description="errorMsg"
      class="mb-4"
    />

    <UForm
      :state="state"
      class="space-y-4"
      @submit="onSubmit"
    >
      <UFormField
        :label="$t('auth.email')"
        name="email"
        required
      >
        <UInput
          v-model="state.email"
          type="email"
          class="w-full"
          autocomplete="email"
        />
      </UFormField>
      <UFormField
        :label="$t('auth.password')"
        name="password"
        required
      >
        <UInput
          v-model="state.password"
          type="password"
          class="w-full"
          autocomplete="current-password"
        />
      </UFormField>
      <UButton
        type="submit"
        block
        :loading="loading"
      >
        {{ $t('auth.signIn') }}
      </UButton>
    </UForm>
  </UCard>
</template>
