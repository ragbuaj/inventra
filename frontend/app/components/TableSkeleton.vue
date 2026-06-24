<script setup lang="ts">
withDefaults(defineProps<{ rows?: number, cols?: number }>(), { rows: 5, cols: 4 })
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="r in rows"
      :key="r"
      class="flex gap-3"
    >
      <USkeleton
        v-for="c in cols"
        :key="c"
        class="h-8 flex-1"
      />
    </div>
  </div>
</template>
