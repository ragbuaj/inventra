<script setup lang="ts">
const { locale, setLocale } = useI18n()
</script>

<template>
  <div class="flex gap-0.5 p-[3px] bg-muted rounded-[9px]">
    <button
      :class="[
        'px-[9px] py-[5px] text-[12px] font-semibold rounded-[6px] border-0 cursor-pointer transition-colors',
        locale === 'id'
          ? 'bg-default text-default shadow-sm'
          : 'bg-transparent text-muted hover:text-default'
      ]"
      @click="setLocale('id')"
    >
      ID
    </button>
    <button
      :class="[
        'px-[9px] py-[5px] text-[12px] font-semibold rounded-[6px] border-0 cursor-pointer transition-colors',
        locale === 'en'
          ? 'bg-default text-default shadow-sm'
          : 'bg-transparent text-muted hover:text-default'
      ]"
      @click="setLocale('en')"
    >
      EN
    </button>
  </div>
</template>
