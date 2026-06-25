<script setup lang="ts">
defineProps<{
  view: boolean
  edit: boolean
  /** true when the field has no explicit rule (follows default) — rendered dimmed */
  dimmed?: boolean
}>()

defineEmits<{ toggleView: [], toggleEdit: [] }>()

const { t } = useI18n()

const onPill = 'border border-transparent'
const offPill = 'bg-transparent text-dimmed border border-dashed border-default'
</script>

<template>
  <div
    class="inline-flex gap-[5px]"
    :class="dimmed ? 'opacity-50' : ''"
  >
    <button
      type="button"
      class="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-semibold cursor-pointer transition"
      :class="view ? `bg-info/10 text-info ${onPill}` : offPill"
      :title="t('settings.fieldPermission.viewLabel')"
      @click="$emit('toggleView')"
    >
      <UIcon
        name="i-lucide-eye"
        class="size-3"
      />L
    </button>
    <button
      type="button"
      class="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-semibold cursor-pointer transition"
      :class="edit ? `bg-primary/10 text-primary ${onPill}` : offPill"
      :title="t('settings.fieldPermission.editLabel')"
      @click="$emit('toggleEdit')"
    >
      <UIcon
        name="i-lucide-pencil"
        class="size-3"
      />E
    </button>
  </div>
</template>
