<script setup lang="ts">
definePageMeta({ middleware: 'can', permission: 'request.create' })
</script>

<template>
  <AssetForm mode="new" />
</template>
