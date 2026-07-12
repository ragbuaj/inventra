<script setup lang="ts">
definePageMeta({ middleware: 'can', permission: 'asset.manage' })

const { t } = useI18n()
</script>

<template>
  <div>
    <div class="mb-5">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('assets.import.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('assets.import.subtitle') }}
      </p>
    </div>

    <ImportWizard
      target="asset"
      permission="asset.manage"
    />
  </div>
</template>
