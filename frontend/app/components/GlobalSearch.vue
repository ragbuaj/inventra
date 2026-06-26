<script setup lang="ts">
const { open } = useCommandPalette()
</script>

<template>
  <div class="flex-1 flex justify-center min-w-0 hidden md:flex">
    <button
      class="w-full max-w-[420px] bg-muted rounded-[10px] py-2 px-3 flex items-center gap-2.5 cursor-text hover:border-strong border border-transparent transition-colors"
      @click="open"
    >
      <UIcon
        name="i-lucide-search"
        class="size-4 text-dimmed shrink-0"
      />
      <span class="flex-1 text-left text-dimmed text-[13.5px] truncate">{{ $t('search.topbarPlaceholder') }}</span>
      <span class="px-[6px] py-[1px] font-mono text-[11px] font-semibold text-dimmed bg-default border border-default rounded-[5px] shrink-0">
        ⌘K
      </span>
    </button>
  </div>
</template>
