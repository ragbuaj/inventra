<script setup lang="ts">
import type { MockAsset } from '~/mock/assets'
// TODO(Task 6): rewire this page to useAssets()/the real /assets API; until then it
// reads the mock assetStore directly (useAssets.ts now targets the real backend).
import { assetStore } from '~/mock/assets'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const route = useRoute()
const localePath = useLocalePath()

const asset = ref<MockAsset | null>(null)
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  asset.value = assetStore.find(String(route.params.tag)) ?? null
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
