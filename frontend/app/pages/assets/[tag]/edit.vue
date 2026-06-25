<script setup lang="ts">
import type { Asset } from '~/types'
import { useAssets } from '~/composables/api/useAssets'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const route = useRoute()
const { get } = useAssets()
const localePath = useLocalePath()

const asset = ref<Asset | null>(null)
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  asset.value = (await get(String(route.params.tag))) ?? null
  loading.value = false
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
    v-else-if="!asset"
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
  <AssetForm
    v-else
    mode="edit"
    :initial="asset"
  />
</template>
