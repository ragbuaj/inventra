<script setup lang="ts">
withDefaults(defineProps<{ rows?: number, cols?: number }>(), { rows: 5, cols: 4 })
</script>

<template>
  <div class="bg-default border border-default rounded-2xl shadow overflow-hidden">
    <!-- Header row -->
    <div class="bg-muted px-4 py-3 flex items-center gap-4">
      <USkeleton
        v-for="c in cols"
        :key="`h-${c}`"
        class="h-3.5 flex-1"
        :class="c === cols ? 'max-w-14' : ''"
      />
    </div>
    <!-- Body rows -->
    <div
      v-for="r in rows"
      :key="r"
      class="px-4 py-3 border-t border-default flex items-center gap-4"
    >
      <USkeleton
        v-for="c in cols"
        :key="c"
        class="h-4 flex-1"
        :class="c === cols ? 'max-w-14' : ''"
      />
    </div>
  </div>
</template>
