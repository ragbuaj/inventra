<script setup lang="ts">
import type { ModuleView } from '~/composables/api/useRbac'

const props = defineProps<{
  module: ModuleView
  granted: string[]
  readonly?: boolean
}>()

defineEmits<{ toggle: [code: string], toggleAll: [] }>()

const { t, te } = useI18n()
function permLabel(code: string, fallback: string) {
  const k = `settings.rbac.catalog.perm.${code}`
  return te(k) ? t(k) : fallback
}
function groupLabel(key: string, fallback: string) {
  const k = `settings.rbac.catalog.group.${key}`
  return te(k) ? t(k) : fallback
}

const grantedSet = computed(() => new Set(props.granted))
const grantedCount = computed(() => props.module.perms.filter(p => grantedSet.value.has(p.code)).length)
const allOn = computed(() => grantedCount.value === props.module.perms.length)
</script>

<template>
  <div class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden">
    <!-- Card header -->
    <div class="flex items-center gap-[10px] px-[15px] py-[13px] border-b border-default">
      <span class="size-[30px] rounded-lg bg-primary/10 text-primary flex items-center justify-center flex-none">
        <UIcon
          :name="module.icon"
          class="size-4"
        />
      </span>
      <div class="flex-1 min-w-0">
        <div class="font-semibold text-sm">
          {{ groupLabel(module.key, module.label) }}
        </div>
        <div class="text-[11.5px] text-dimmed">
          {{ t('settings.rbac.moduleCount', { granted: grantedCount, total: module.perms.length }) }}
        </div>
      </div>
      <button
        v-if="!readonly"
        type="button"
        class="text-[11.5px] font-semibold text-primary px-1.5 py-1 rounded-md cursor-pointer hover:bg-primary/10 transition-colors"
        @click="$emit('toggleAll')"
      >
        {{ allOn ? t('settings.rbac.clearAll') : t('settings.rbac.selectAll') }}
      </button>
    </div>

    <!-- Permission rows -->
    <div class="px-2 py-1.5">
      <button
        v-for="p in module.perms"
        :key="p.code"
        type="button"
        :disabled="readonly"
        class="flex items-center gap-[11px] w-full px-2 py-[9px] rounded-lg text-left transition-colors"
        :class="readonly ? 'cursor-not-allowed' : 'cursor-pointer hover:bg-muted'"
        @click="!readonly && $emit('toggle', p.code)"
      >
        <USwitch
          :model-value="grantedSet.has(p.code)"
          :disabled="readonly"
          class="pointer-events-none flex-none"
          :class="readonly ? 'opacity-55' : ''"
        />
        <span class="flex-1 min-w-0">
          <span
            class="block text-[13px] font-medium"
            :class="grantedSet.has(p.code) ? 'text-default' : 'text-muted'"
          >{{ permLabel(p.code, p.label) }}</span>
          <span class="block text-[11px] font-mono text-dimmed">{{ p.code }}</span>
        </span>
      </button>
    </div>
  </div>
</template>
