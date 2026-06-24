<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <div>
    <PageHeader
      :title="t('nav.dashboard')"
      :subtitle="t('app.tagline')"
    />
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <StatCard
        :label="t('nav.assets')"
        value="0"
        icon="i-lucide-package"
      />
      <StatCard
        :label="t('nav.maintenance')"
        value="0"
        icon="i-lucide-wrench"
      />
      <StatCard
        :label="t('nav.approval')"
        value="0"
        icon="i-lucide-check-square"
      />
      <StatCard
        :label="t('nav.assignment')"
        value="0"
        icon="i-lucide-arrow-left-right"
      />
    </div>
  </div>
</template>
