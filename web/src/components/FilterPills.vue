<script setup lang="ts">
import { X } from "@lucide/vue";
import { type DraftNode, toggleNode, removeNode as removeNodeUtil, updateChildren as updateChildrenUtil } from "../utils/filterTree";

const props = withDefaults(defineProps<{ nodes: DraftNode[]; root?: boolean }>(), { root: true });
const emit = defineEmits<{ "update:nodes": [nodes: DraftNode[]] }>();

function toggleActive(node: DraftNode) {
  emit("update:nodes", toggleNode(props.nodes, node.id, props.root));
}

function removeNode(id: number) {
  emit("update:nodes", removeNodeUtil(props.nodes, id));
}

function updateChildren(group: DraftNode, children: DraftNode[]) {
  emit("update:nodes", updateChildrenUtil(props.nodes, group.id, children));
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
