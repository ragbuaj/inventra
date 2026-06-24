<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const { login } = useAuthApi()
const toast = useToast()

const state = reactive({ email: '', password: '' })
const showPassword = ref(false)
const rememberMe = ref(true)
const loading = ref(false)
const errorMsg = ref('')

async function onSubmit() {
  loading.value = true
  errorMsg.value = ''
  try {
    await login(state.email, state.password)
    await navigateTo('/')
  } catch (err: unknown) {
    // Only a 401 means bad credentials; a missing/!=401 status is a
    // network/CORS/server failure and must not be mislabelled.
    const status = (err as { statusCode?: number }).statusCode
    errorMsg.value = status === 401 ? t('auth.invalidCredentials') : t('auth.connectionError')
  } finally {
    loading.value = false
  }
}

// Google sign-in and password reset are not wired to the backend yet.
function notAvailable() {
  toast.add({ title: t('auth.featureComingSoon'), color: 'info' })
}
</script>

<template>
  <div class="w-full max-w-sm">
    <h1 class="text-2xl font-bold tracking-tight">
      {{ $t('auth.signInTitle') }}
    </h1>
    <p class="mt-1 mb-6 text-muted">
      {{ $t('auth.signInSubtitle') }}
    </p>

    <UAlert
      v-if="errorMsg"
      icon="i-lucide-circle-x"
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
          icon="i-lucide-mail"
          :placeholder="$t('auth.emailPlaceholder')"
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
          :type="showPassword ? 'text' : 'password'"
          icon="i-lucide-lock"
          :placeholder="$t('auth.passwordPlaceholder')"
          class="w-full"
          autocomplete="current-password"
        >
          <template #trailing>
            <UButton
              type="button"
              color="neutral"
              variant="link"
              size="sm"
              :icon="showPassword ? 'i-lucide-eye-off' : 'i-lucide-eye'"
              :aria-label="$t('auth.togglePassword')"
              @click="showPassword = !showPassword"
            />
          </template>
        </UInput>
      </UFormField>

      <div class="flex items-center justify-between">
        <UCheckbox
          v-model="rememberMe"
          :label="$t('auth.rememberMe')"
        />
        <UButton
          type="button"
          variant="link"
          class="p-0"
          @click="notAvailable"
        >
          {{ $t('auth.forgotPassword') }}
        </UButton>
      </div>

      <UButton
        type="submit"
        block
        size="lg"
        :loading="loading"
      >
        {{ $t('auth.signIn') }}
      </UButton>

      <USeparator :label="$t('auth.or')" />

      <UButton
        type="button"
        block
        size="lg"
        color="neutral"
        variant="outline"
        icon="i-simple-icons-google"
        @click="notAvailable"
      >
        {{ $t('auth.signInWithGoogle') }}
      </UButton>
    </UForm>
  </div>
</template>
