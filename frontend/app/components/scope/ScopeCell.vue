<script setup lang="ts">
import type { ScopeLevel, ScopeTone } from '~/constants/dataScope'
import { SCOPE_LEVEL_KEYS, SCOPE_LEVEL_TONE } from '~/constants/dataScope'

const props = defineProps<{
  /** level shown on the pill (override || role default) */
  effective: ScopeLevel
  /** explicitly-selected level — null means a module cell inheriting the default */
  selected: ScopeLevel | null
  isModule: boolean
  roleDefault: ScopeLevel
}>()

const emit = defineEmits<{ select: [level: ScopeLevel], clear: [] }>()

const { t } = useI18n()
const open = ref(false)

const toneClasses: Record<ScopeTone, { pill: string, dot: string }> = {
  info: { pill: 'bg-info/10 text-info', dot: 'bg-info' },
  primary: { pill: 'bg-primary/10 text-primary', dot: 'bg-primary' },
  warning: { pill: 'bg-warning/10 text-warning', dot: 'bg-warning' },
  neutral: { pill: 'bg-elevated text-dimmed', dot: 'bg-[var(--ui-text-dimmed)]' }
}

const isOverride = computed(() => props.isModule && props.selected !== null)
const isInheriting = computed(() => props.isModule && props.selected === null)
const effTone = computed(() => SCOPE_LEVEL_TONE[props.effective])
const effDot = computed(() => toneClasses[effTone.value].dot)

function levelDesc(level: ScopeLevel): string {
  return t(`settings.dataScope.level.${level}`)
}

function pick(level: ScopeLevel) {
  emit('select', level)
  open.value = false
}
function follow() {
  emit('clear')
  open.value = false
}
</script>

<template>
  <UPopover v-model:open="open">
    <button
      type="button"
      class="inline-flex items-center gap-[7px] pl-[10px] pr-[9px] py-[5px] rounded-lg cursor-pointer max-w-full transition hover:brightness-95"
      :class="isInheriting
        ? 'bg-transparent text-muted border border-dashed border-default'
        : `${toneClasses[effTone].pill} border border-transparent`"
    >
      <span
        class="size-[7px] rounded-full flex-none"
        :class="effDot"
      />
      <span class="font-mono text-xs font-semibold whitespace-nowrap">{{ effective }}</span>
      <span
        v-if="isOverride"
        class="size-1.5 rounded-full bg-warning flex-none"
        :title="t('settings.dataScope.overrideTag')"
      />
      <UIcon
        name="i-lucide-chevron-down"
        class="size-3 opacity-70 flex-none"
      />
    </button>

    <template #content>
      <div class="min-w-[230px] p-1.5">
        <button
          v-if="isModule"
          type="button"
          class="flex items-start gap-[9px] w-full px-2.5 py-2 rounded-lg text-left cursor-pointer hover:bg-muted transition-colors"
          :class="isInheriting ? 'bg-muted' : ''"
          @click="follow"
        >
          <UIcon
            name="i-lucide-corner-down-left"
            class="size-3.5 text-muted mt-0.5 flex-none"
          />
          <span class="flex-1 min-w-0">
            <span class="block text-[12.5px] font-semibold">{{ t('settings.dataScope.followDefault') }}</span>
            <span class="block text-[11.5px] text-muted">{{ t('settings.dataScope.inherits', { level: roleDefault }) }}</span>
          </span>
          <UIcon
            v-if="isInheriting"
            name="i-lucide-check"
            class="size-3.5 text-primary mt-0.5 flex-none"
          />
        </button>

        <button
          v-for="lvl in SCOPE_LEVEL_KEYS"
          :key="lvl"
          type="button"
          class="flex items-start gap-[9px] w-full px-2.5 py-2 rounded-lg text-left cursor-pointer hover:bg-muted transition-colors"
          :class="selected === lvl ? 'bg-primary/10' : ''"
          @click="pick(lvl)"
        >
          <span
            class="size-2 rounded-full mt-[5px] flex-none"
            :class="toneClasses[SCOPE_LEVEL_TONE[lvl]].dot"
          />
          <span class="flex-1 min-w-0">
            <span class="block text-[12.5px] font-semibold font-mono">{{ lvl }}</span>
            <span class="block text-[11.5px] text-muted">{{ levelDesc(lvl) }}</span>
          </span>
          <UIcon
            v-if="selected === lvl"
            name="i-lucide-check"
            class="size-3.5 text-primary mt-0.5 flex-none"
          />
        </button>
      </div>
    </template>
  </UPopover>
</template>
