<script setup lang="ts">
import { Plus, X } from "@lucide/vue";
import { type DraftNode, nextNodeId } from "../utils/filterTree";

const operators = ["=", "!=", ">", "<", ">=", "<=", "LIKE"];

const props = withDefaults(defineProps<{ columns: string[]; nodes: DraftNode[]; root?: boolean }>(), { root: true });
const emit = defineEmits<{ "update:nodes": [nodes: DraftNode[]] }>();

function updateNode(id: number, patch: Partial<DraftNode>) {
  emit(
    "update:nodes",
    props.nodes.map((n) => (n.id === id ? ({ ...n, ...patch } as DraftNode) : n)),
  );
}
// Root-level nodes are independent filters — only one can be active at a
// time. Nested nodes (inside a group) combine via that group's AND/OR
// instead, so they toggle independently of their siblings.
function toggleActive(node: DraftNode) {
  if (!props.root) {
    updateNode(node.id, { active: !node.active });
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
  updateNode(group.id, { children } as Partial<DraftNode>);
}
// A newly added root filter becomes the selected one (only one root filter
// is ever active at a time) — nested group children stay independently
// active alongside their siblings instead.
function deactivateSiblings(): DraftNode[] {
  return props.root ? props.nodes.map((n) => ({ ...n, active: false })) : props.nodes;
}
function addLeaf() {
  emit("update:nodes", [
    ...deactivateSiblings(),
    { id: nextNodeId(), kind: "leaf", column: props.columns[0] ?? "", operator: operators[0], value: "", active: true },
  ]);
}
function addGroup() {
  emit("update:nodes", [...deactivateSiblings(), { id: nextNodeId(), kind: "group", bool: "AND", children: [], active: true }]);
}
function clearAll() {
  emit("update:nodes", []);
}
</script>

<template>
  <div>
    <div v-if="root" class="mb-1 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-slate-900">Gestionnaire de filtres</h3>
      <button v-if="nodes.length > 0" class="text-xs font-medium text-slate-400 hover:text-red-600" @click="clearAll">Vider</button>
    </div>
    <p v-if="root" class="mb-3 text-xs text-slate-400">Un seul filtre actif à la fois.</p>

    <div class="flex flex-col gap-2">
      <div v-for="node in nodes" :key="node.id">
        <div v-if="node.kind === 'leaf'" class="flex items-center gap-1.5 rounded-md border border-slate-200 p-2">
          <input
            type="checkbox"
            class="h-3.5 w-3.5 rounded border-slate-300"
            :checked="node.active"
            @change="toggleActive(node)"
          />
          <select
            :value="node.column"
            class="min-w-0 flex-1 rounded-md border border-slate-300 px-2 py-1 text-xs"
            @change="updateNode(node.id, { column: ($event.target as HTMLSelectElement).value })"
          >
            <option v-for="col in columns" :key="col" :value="col">{{ col }}</option>
          </select>
          <select
            :value="node.operator"
            class="rounded-md border border-slate-300 px-2 py-1 text-xs"
            @change="updateNode(node.id, { operator: ($event.target as HTMLSelectElement).value })"
          >
            <option v-for="op in operators" :key="op" :value="op">{{ op }}</option>
          </select>
          <input
            :value="node.value"
            type="text"
            placeholder="Valeur"
            class="min-w-0 flex-1 rounded-md border border-slate-300 px-2 py-1 text-xs"
            @input="updateNode(node.id, { value: ($event.target as HTMLInputElement).value })"
          />
          <button class="shrink-0 text-slate-400 hover:text-red-600" @click="removeNode(node.id)"><X :size="14" /></button>
        </div>

        <div v-else class="rounded-md border border-slate-300 p-2">
          <div class="mb-2 flex items-center gap-1.5">
            <input
              type="checkbox"
              class="h-3.5 w-3.5 rounded border-slate-300"
              :checked="node.active"
              @change="toggleActive(node)"
            />
            <select
              :value="node.bool"
              class="rounded-md border border-slate-300 px-2 py-1 text-xs font-medium"
              @change="updateNode(node.id, { bool: ($event.target as HTMLSelectElement).value as 'AND' | 'OR' })"
            >
              <option value="AND">ET</option>
              <option value="OR">OU</option>
            </select>
            <span class="text-xs text-slate-400">groupe</span>
            <button class="ml-auto shrink-0 text-slate-400 hover:text-red-600" @click="removeNode(node.id)"><X :size="14" /></button>
          </div>

          <div class="border-l-2 border-slate-200 pl-3">
            <FilterConditionsEditor
              :columns="columns"
              :nodes="node.children"
              :root="false"
              @update:nodes="updateChildren(node, $event)"
            />
          </div>
        </div>
      </div>
    </div>

    <div class="mt-2 flex gap-2">
      <button
        class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
        @click="addLeaf"
      >
        <Plus :size="12" /> Ajouter un filtre
      </button>
      <button
        class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
        @click="addGroup"
      >
        <Plus :size="12" /> Ajouter un groupe
      </button>
    </div>
  </div>
</template>
