<script setup lang="ts">
import { Plus } from "@lucide/vue";

export interface DraftCondition {
  column: string;
  operator: string;
  value: string;
}
const operators = ["=", "!=", ">", "<", ">=", "<=", "LIKE"];

const props = defineProps<{ columns: string[]; conditions: DraftCondition[] }>();
const emit = defineEmits<{ "update:conditions": [conditions: DraftCondition[]] }>();

function addCondition() {
  emit("update:conditions", [...props.conditions, { column: props.columns[0] ?? "", operator: "=", value: "" }]);
}
function removeCondition(i: number) {
  emit(
    "update:conditions",
    props.conditions.filter((_, idx) => idx !== i),
  );
}
function updateCondition(i: number, patch: Partial<DraftCondition>) {
  emit(
    "update:conditions",
    props.conditions.map((c, idx) => (idx === i ? { ...c, ...patch } : c)),
  );
}
</script>

<template>
  <div>
    <div v-for="(cond, i) in conditions" :key="i" class="mb-2 flex items-center gap-2">
      <select
        :value="cond.column"
        class="rounded-md border border-slate-300 px-2 py-1.5 text-sm"
        @change="updateCondition(i, { column: ($event.target as HTMLSelectElement).value })"
      >
        <option v-for="col in columns" :key="col" :value="col">{{ col }}</option>
      </select>
      <select
        :value="cond.operator"
        class="rounded-md border border-slate-300 px-2 py-1.5 text-sm"
        @change="updateCondition(i, { operator: ($event.target as HTMLSelectElement).value })"
      >
        <option v-for="op in operators" :key="op" :value="op">{{ op }}</option>
      </select>
      <input
        :value="cond.value"
        type="text"
        placeholder="Valeur"
        class="flex-1 rounded-md border border-slate-300 px-2 py-1.5 text-sm"
        @input="updateCondition(i, { value: ($event.target as HTMLInputElement).value })"
      />
      <button class="text-slate-400 hover:text-red-600" @click="removeCondition(i)">✕</button>
    </div>
    <button
      class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
      @click="addCondition"
    >
      <Plus :size="12" /> Condition
    </button>
  </div>
</template>
