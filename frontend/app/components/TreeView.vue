<script setup lang="ts">
import type { TreeNode } from '~/types'

export type { TreeNode }
defineProps<{ nodes: TreeNode[], selectedId?: string }>()
const emit = defineEmits<{ select: [string] }>()
const expanded = ref<Record<string, boolean>>({})
function toggle(id: string) {
  expanded.value[id] = !expanded.value[id]
}
</script>

<template>
  <ul class="space-y-0.5">
    <li
      v-for="node in nodes"
      :key="node.id"
    >
      <div
        class="flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer hover:bg-elevated"
        :class="node.id === selectedId ? 'bg-elevated text-primary font-medium' : ''"
        @click="emit('select', node.id)"
      >
        <UButton
          v-if="node.children?.length"
          color="neutral"
          variant="ghost"
          size="xs"
          :icon="expanded[node.id] ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
          @click.stop="toggle(node.id)"
        />
        <span
          v-else
          class="w-5"
        />
        <UIcon
          v-if="node.icon"
          :name="node.icon"
          class="size-4 text-muted"
        />
        <span class="text-sm truncate">{{ node.label }}</span>
        <UBadge
          v-if="node.childCount"
          color="neutral"
          variant="subtle"
          size="sm"
          class="ms-auto"
        >
          {{ node.childCount }}
        </UBadge>
      </div>
      <div
        v-if="node.children?.length && expanded[node.id]"
        class="ms-4 border-s border-default ps-1"
      >
        <TreeView
          :nodes="node.children"
          :selected-id="selectedId"
          @select="emit('select', $event)"
        />
      </div>
    </li>
  </ul>
</template>
