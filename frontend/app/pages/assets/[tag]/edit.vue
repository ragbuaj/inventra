<script setup lang="ts">
import type { Asset } from '~/types'

definePageMeta({ middleware: 'can', permission: 'asset.manage' })

const { t } = useI18n()
const route = useRoute()
const localePath = useLocalePath()
const assetsApi = useAssets()

const asset = ref<Asset | null>(null)
const loading = ref(true)
const notFound = ref(false)
const loadError = ref(false)

async function load() {
  loading.value = true
  notFound.value = false
  loadError.value = false
  try {
    asset.value = await assetsApi.getByTag(String(route.params.tag))
  } catch (err) {
    const status = (err as { statusCode?: number } | undefined)?.statusCode
    if (status === 404) notFound.value = true
    else loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<template>
  <div
    v-if="loading"
    class="flex items-center justify-center py-24"
  >
    <UIcon
      name="i-lucide-loader-circle"
      class="size-6 animate-spin text-muted"
    />
  </div>
  <div
    v-else-if="notFound"
    class="bg-default border border-default rounded-2xl shadow-sm py-16 px-6 text-center"
  >
    <div class="text-[17px] font-semibold mb-2">
      {{ t('assets.errNotFound') }}
    </div>
    <UButton
      :to="localePath('/assets')"
      color="neutral"
      variant="outline"
      icon="i-lucide-arrow-left"
      :label="t('assets.detail.backToCatalog')"
    />
  </div>
  <div
    v-else-if="loadError"
    class="bg-default border border-default rounded-[13px] shadow-sm flex flex-col items-center justify-center gap-3 py-16 text-muted"
  >
    <UIcon
      name="i-lucide-circle-alert"
      class="size-6"
    />
    <span class="text-sm">{{ t('common.loadError') }}</span>
    <UButton
      color="neutral"
      variant="subtle"
      @click="load"
    >
      {{ t('common.retry') }}
    </UButton>
  </div>
  <AssetForm
    v-else-if="asset"
    mode="edit"
    :initial="asset"
  />
</template>
