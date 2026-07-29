<script setup lang="ts">
import { computed, ref } from "vue";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import { filterRows } from "../client/tables";
import type { ColumnSchema, FilterCondition, Row, RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { coerceByType, columnType } from "../utils/columnTypes";
import { Plus, SlidersHorizontal } from "@lucide/vue";
import DataTable from "./DataTable.vue";

const props = defineProps<{ tableId: string; rowColumns: string[]; columns: ColumnSchema[] }>();

function cellValue(row: Row, col: string): RowValue {
  return col === "__pkey" ? row.pkey : row[col];
}

interface DraftCondition {
  column: string;
  operator: string;
  value: string;
}
const operators = ["=", "!=", ">", "<", ">=", "<=", "LIKE"];
const draftConditions = ref<DraftCondition[]>([]);
const filterLimit = ref(50);
const filterResults = ref<Row[] | null>(null);
const filtering = ref(false);
const filterError = ref<string | null>(null);

function addCondition() {
  draftConditions.value.push({ column: props.rowColumns[0] ?? "", operator: "=", value: "" });
}
function removeCondition(i: number) {
  draftConditions.value.splice(i, 1);
}

async function applyFilter() {
  filtering.value = true;
  filterError.value = null;
  try {
    const conditions: FilterCondition[] = draftConditions.value.map((c) => ({
      column: c.column,
      operator: c.operator,
      value: coerceByType(columnType(props.columns, c.column), c.value),
    }));
    const res = await filterRows(props.tableId, conditions, filterLimit.value);
    filterResults.value = res.rows;
  } catch (e) {
    filterError.value = errorMessage(e, "Impossible d'appliquer le filtre");
  } finally {
    filtering.value = false;
  }
}

const filterColumnHelper = createColumnHelper<Row>();
const filterColumnDefs = computed(() =>
  props.rowColumns.map((col) => filterColumnHelper.accessor((row) => cellValue(row, col), { id: col, header: col })),
);

const filterTable = useVueTable({
  get data() {
    return filterResults.value ?? [];
  },
  get columns() {
    return filterColumnDefs.value;
  },
  getCoreRowModel: getCoreRowModel(),
});
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div class="mb-4 shrink-0 rounded-lg border border-slate-200 bg-white p-4">
      <div v-for="(cond, i) in draftConditions" :key="i" class="mb-2 flex items-center gap-2">
        <select v-model="cond.column" class="rounded-md border border-slate-300 px-2 py-1.5 text-sm">
          <option v-for="col in rowColumns" :key="col" :value="col">{{ col }}</option>
        </select>
        <select v-model="cond.operator" class="rounded-md border border-slate-300 px-2 py-1.5 text-sm">
          <option v-for="op in operators" :key="op" :value="op">{{ op }}</option>
        </select>
        <input
          v-model="cond.value"
          type="text"
          placeholder="Valeur"
          class="flex-1 rounded-md border border-slate-300 px-2 py-1.5 text-sm"
        />
        <button class="text-slate-400 hover:text-red-600" @click="removeCondition(i)">✕</button>
      </div>
      <div class="mt-2 flex items-center gap-2">
        <button
          class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
          @click="addCondition"
        >
          <Plus :size="12" /> Condition
        </button>
        <div class="ml-auto flex items-center gap-2">
          <label class="text-xs text-slate-500">Limite</label>
          <input v-model.number="filterLimit" type="number" min="1" class="w-20 rounded-md border border-slate-300 px-2 py-1 text-xs" />
          <button
            class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
            :disabled="filtering || draftConditions.length === 0"
            @click="applyFilter"
          >
            <SlidersHorizontal :size="14" /> {{ filtering ? "…" : "Appliquer" }}
          </button>
        </div>
      </div>
      <p v-if="filterError" class="mt-2 text-xs text-red-600">{{ filterError }}</p>
    </div>

    <DataTable v-if="filterResults" :table="filterTable" empty-message="Aucun résultat." />
  </div>
</template>
