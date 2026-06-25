<script setup lang="ts">
const props = defineProps<{ total: number, limit: number, offset: number }>()
const emit = defineEmits<{ 'update:offset': [number] }>()

const page = computed({
  get: () => Math.floor(props.offset / props.limit) + 1,
  set: (p: number) => emit('update:offset', (p - 1) * props.limit)
})
const from = computed(() => props.total === 0 ? 0 : props.offset + 1)
const to = computed(() => Math.min(props.offset + props.limit, props.total))
</script>

<template>
  <div class="flex items-center justify-between gap-4 px-4 py-3 border-t border-default">
    <p class="text-sm text-muted">
      {{ $t('common.showingRange', { from, to, total }) }}
    </p>
    <UPagination
      v-model:page="page"
      :total="total"
      :items-per-page="limit"
    />
  </div>
</template>
