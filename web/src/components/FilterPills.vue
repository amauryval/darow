<script setup lang="ts">
import { X } from "@lucide/vue";
import type { DraftNode } from "../utils/filterTree";

const props = withDefaults(defineProps<{ nodes: DraftNode[]; root?: boolean }>(), { root: true });
const emit = defineEmits<{ "update:nodes": [nodes: DraftNode[]] }>();

// Root-level nodes are independent filters — only one can be active at a
// time. Nested nodes (inside a group) combine via that group's AND/OR
// instead, so they toggle independently of their siblings.
function toggleActive(node: DraftNode) {
  if (!props.root) {
    emit(
      "update:nodes",
      props.nodes.map((n) => (n.id === node.id ? { ...n, active: !n.active } : n)),
    );
    return;
  }
  emit(
    "update:nodes",
    props.nodes.map((n) => ({ ...n, active: n.id === node.id ? !node.active : false })),
  );
}
function removeNode(id: number) {
  emit(
    "update:nodes",
    props.nodes.filter((n) => n.id !== id),
  );
}
function updateChildren(group: DraftNode, children: DraftNode[]) {
  emit(
    "update:nodes",
    props.nodes.map((n) => (n.id === group.id ? { ...n, children } : n)),
  );
}
</script>

<template>
  <template v-for="node in nodes" :key="node.id">
    <button
      v-if="node.kind === 'leaf'"
      class="flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
      :class="node.active ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-400'"
      @click="toggleActive(node)"
    >
      {{ node.column }} {{ node.operator }} {{ node.value }}
      <X :size="12" @click.stop="removeNode(node.id)" />
    </button>

    <div
      v-else
      class="flex items-center gap-1.5 rounded-lg border px-2 py-1"
      :class="node.active ? 'border-indigo-300 bg-indigo-50' : 'border-slate-200 bg-slate-50 opacity-50'"
      @click="toggleActive(node)"
    >
      <span class="shrink-0 text-xs font-semibold text-slate-400">{{ node.bool === "AND" ? "ET" : "OU" }}</span>
      <div class="flex flex-wrap items-center gap-1.5" @click.stop>
        <FilterPills :nodes="node.children" :root="false" @update:nodes="updateChildren(node, $event)" />
      </div>
      <X :size="12" class="shrink-0 text-slate-400 hover:text-red-600" @click.stop="removeNode(node.id)" />
    </div>
  </template>
</template>
