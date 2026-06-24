<script setup lang="ts">
import type { TreeNode } from '~/types'

export type { TreeNode }
const props = defineProps<{ nodes: TreeNode[], selectedId?: string }>()
const emit = defineEmits<{ select: [string] }>()

function collectIds(nodes: TreeNode[]): string[] {
  return nodes.flatMap(n => [n.id, ...(n.children ? collectIds(n.children) : [])])
}

const expanded = ref<Record<string, boolean>>(Object.fromEntries(collectIds(props.nodes).map(id => [id, true])))

watch(() => props.nodes, (nodes) => {
  const ids = collectIds(nodes)
  for (const id of ids) {
    if (!(id in expanded.value)) {
      expanded.value[id] = true
    }
  }
}, { deep: true })

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
        :class="[
          node.id === selectedId
            ? 'bg-elevated text-primary font-medium shadow-[inset_3px_0_0_var(--ui-primary)]'
            : ''
        ]"
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
        <!-- Colored type badge icon (when iconBg/iconColor provided) -->
        <span
          v-if="node.icon && (node.iconBg || node.iconColor)"
          class="flex items-center justify-center size-5 rounded flex-none"
          :class="[node.iconBg ?? '', node.iconColor ?? '']"
        >
          <UIcon
            :name="node.icon"
            class="size-3"
          />
        </span>
        <!-- Plain icon (no badge) -->
        <UIcon
          v-else-if="node.icon"
          :name="node.icon"
          class="size-4 text-muted"
        />
        <span
          class="text-sm truncate"
          :class="node.inactive ? 'text-muted' : ''"
        >{{ node.label }}</span>
        <!-- Inactive dot -->
        <span
          v-if="node.inactive"
          class="ms-1 size-1.5 rounded-full bg-muted flex-none"
          :title="$t('common.inactive')"
        />
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
