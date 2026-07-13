<script setup lang="ts">
definePageMeta({ layout: 'auth' })
const { t } = useI18n()
const { login, refresh, fetchMe } = useAuthApi()
const config = useRuntimeConfig()
const route = useRoute()
const localePath = useLocalePath()

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

function startGoogle() {
  window.location.href = `${config.public.apiBase}/auth/google`
}

const GOOGLE_REASONS = ['not_registered', 'account_mismatch', 'inactive', 'disabled', 'server']

onMounted(async () => {
  if (route.query.oauth === 'success') {
    try {
      if (await refresh()) {
        await fetchMe()
        await navigateTo('/')
        return
      }
    } catch {
      // fall through to error message
    }
    errorMsg.value = t('auth.google.error.server')
  } else if (route.query.oauth === 'error') {
    const reason = String(route.query.reason ?? 'server')
    errorMsg.value = t(`auth.google.error.${GOOGLE_REASONS.includes(reason) ? reason : 'server'}`)
  }
})
</script>

<template>
  <div class="w-full max-w-[392px]">
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
          size="lg"
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
          size="lg"
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
        <NuxtLink
          :to="localePath('/forgot-password')"
          class="text-primary text-sm hover:underline"
        >
          {{ $t('auth.forgotPassword') }}
        </NuxtLink>
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
        @click="startGoogle"
      >
        {{ $t('auth.signInWithGoogle') }}
      </UButton>
    </UForm>
  </div>
</template>
