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
    <span
      v-if="node.kind === 'leaf'"
      class="flex items-center gap-1 rounded-full pr-1 pl-2.5 py-1 text-xs font-medium"
      :class="node.active ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-400'"
    >
      <button type="button" class="py-0.5" @click="toggleActive(node)">{{ node.column }} {{ node.operator }} {{ node.value }}</button>
      <button type="button" class="rounded-full p-0.5 hover:bg-black/10" aria-label="Supprimer le filtre" @click="removeNode(node.id)">
        <X :size="12" aria-hidden="true" />
      </button>
    </span>

    <span
      v-else
      class="flex items-center gap-1.5 rounded-lg border py-1 pr-1 pl-2"
      :class="node.active ? 'border-indigo-300 bg-indigo-50' : 'border-slate-200 bg-slate-50 opacity-50'"
    >
      <button type="button" class="shrink-0 text-xs font-semibold text-slate-400" @click="toggleActive(node)">
        {{ node.bool === "AND" ? "ET" : "OU" }}
      </button>
      <span class="flex flex-wrap items-center gap-1.5">
        <FilterPills :nodes="node.children" :root="false" @update:nodes="updateChildren(node, $event)" />
      </span>
      <button
        type="button"
        class="shrink-0 rounded-full p-0.5 text-slate-400 hover:bg-black/5 hover:text-red-600"
        aria-label="Supprimer le groupe de filtres"
        @click="removeNode(node.id)"
      >
        <X :size="12" aria-hidden="true" />
      </button>
    </span>
  </template>
</template>
