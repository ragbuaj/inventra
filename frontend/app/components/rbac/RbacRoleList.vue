<script setup lang="ts">
import type { RoleView } from '~/composables/api/useRbac'

defineProps<{
  roles: RoleView[]
  selectedId: string
}>()

defineEmits<{ select: [id: string], add: [] }>()

const { t } = useI18n()
</script>

<template>
  <div class="w-[280px] flex-none border-e border-default bg-default flex flex-col overflow-hidden">
    <div class="flex-none px-4 pt-4 pb-3">
      <div class="font-bold text-[15px] mb-0.5">
        {{ t('settings.rbac.rolesTitle') }}
      </div>
      <div class="text-xs text-muted">
        {{ t('settings.rbac.rolesSub') }}
      </div>
    </div>

    <div class="flex-1 overflow-y-auto px-[10px]">
      <button
        v-for="r in roles"
        :key="r.id"
        type="button"
        class="flex items-center gap-[10px] w-full px-[11px] py-[10px] mb-[3px] rounded-[9px] border text-left transition-colors cursor-pointer hover:border-primary"
        :class="r.id === selectedId
          ? 'border-primary bg-primary/10'
          : 'border-default bg-default'"
        @click="$emit('select', r.id)"
      >
        <span
          class="size-8 rounded-lg flex items-center justify-center flex-none"
          :class="r.id === selectedId ? 'bg-primary/20 text-primary' : 'bg-muted text-muted'"
        >
          <UIcon
            name="i-lucide-shield"
            class="size-4"
          />
        </span>
        <div class="flex-1 min-w-0">
          <div
            class="font-semibold text-[13.5px] truncate"
            :class="r.id === selectedId ? 'text-primary' : 'text-default'"
          >
            {{ r.name }}
          </div>
          <div class="text-[11.5px] text-dimmed">
            {{ t('settings.rbac.permCount', { n: r.perms.length }) }}
          </div>
        </div>
        <UIcon
          v-if="r.is_system"
          name="i-lucide-lock"
          class="size-[13px] text-dimmed flex-none"
          :title="t('settings.rbac.systemBadge')"
        />
      </button>

      <button
        type="button"
        class="flex items-center justify-center gap-[7px] w-full p-[10px] my-[6px] mb-[14px] rounded-[9px] border-[1.5px] border-dashed border-default text-muted font-semibold text-[13px] cursor-pointer transition-colors hover:border-primary hover:text-primary"
        @click="$emit('add')"
      >
        <UIcon
          name="i-lucide-plus"
          class="size-3.5"
        />
        {{ t('settings.rbac.addRole') }}
      </button>
    </div>
  </div>
</template>
