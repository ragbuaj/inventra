<template>
  <div class="flex-1 flex justify-center min-w-0 hidden md:flex">
    <div class="relative w-full max-w-[420px]">
      <span class="absolute left-3 top-1/2 -translate-y-1/2 text-dimmed flex pointer-events-none">
        <UIcon
          name="i-lucide-search"
          class="size-4"
        />
      </span>
      <input
        :placeholder="$t('nav.searchPlaceholder')"
        class="w-full pl-9 pr-[52px] py-2 text-[13.5px] text-default bg-muted border border-transparent rounded-[9px] outline-none focus:bg-default focus:border-primary focus:ring-3 focus:ring-primary/25 transition-colors"
      >
      <span class="absolute right-[9px] top-1/2 -translate-y-1/2 px-[6px] py-[1px] font-mono text-[11px] font-semibold text-dimmed bg-default border border-default rounded-[5px] pointer-events-none">
        ⌘K
      </span>
    </div>
  </div>
</template>
