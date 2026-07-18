<script setup lang="ts">
import { passwordStrength } from '~/utils/passwordStrength'

const props = defineProps<{ password: string }>()

const { t } = useI18n()

const strength = computed(() => passwordStrength(props.password))

// Index 0 is unreachable in the template (the meter is hidden on an empty
// password), but keeping it makes the array indexable by score directly.
const BAR_COLORS = ['bg-elevated', 'bg-error', 'bg-warning', 'bg-info', 'bg-success']
const TEXT_COLORS = ['text-muted', 'text-error', 'text-warning', 'text-info', 'text-success']

const barColor = computed(() => BAR_COLORS[strength.value.score]!)
const textColor = computed(() => TEXT_COLORS[strength.value.score]!)
</script>

<template>
  <div
    v-if="password"
    data-testid="password-strength"
  >
    <div
      class="flex gap-1"
      role="presentation"
    >
      <span
        v-for="i in 4"
        :key="i"
        class="h-1 flex-1 rounded-full transition-colors"
        :class="i <= strength.score ? barColor : 'bg-elevated'"
      />
    </div>
    <!-- flex + gap rather than a literal space: Vue's whitespace: 'condense'
         drops the whitespace-only text node between the two spans. -->
    <p class="mt-1.5 flex gap-1 text-xs">
      <span class="text-muted">{{ t('auth.strengthLabel') }}:</span>
      <span
        class="font-medium"
        :class="textColor"
        data-testid="password-strength-label"
      >
        {{ strength.labelKey ? t(strength.labelKey) : '—' }}
      </span>
    </p>
  </div>
</template>
