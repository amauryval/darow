<script setup lang="ts">
import { ref } from "vue";
import { createColumnHelper, getCoreRowModel, useVueTable } from "@tanstack/vue-table";
import { addColumn, getDistinctValues } from "../client/tables";
import type { ColumnSchema, RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { useToast } from "../composables/useToast";
import { Plus } from "@lucide/vue";
import DataTable from "./DataTable.vue";
import Modal from "./Modal.vue";

const props = defineProps<{ tableId: string; columns: ColumnSchema[] }>();
const emit = defineEmits<{ "column-added": [] }>();
const toast = useToast();

// --- Add column modal ---
const showAddColumn = ref(false);
const newColumnName = ref("");
const newColumnType = ref("VARCHAR");
const addingColumn = ref(false);
const addColumnError = ref<string | null>(null);

async function confirmAddColumn() {
  if (!newColumnName.value.trim()) return;
  addingColumn.value = true;
  addColumnError.value = null;
  try {
    await addColumn(props.tableId, newColumnName.value.trim(), newColumnType.value.trim());
    showAddColumn.value = false;
    newColumnName.value = "";
    emit("column-added");
    toast.success("Colonne ajoutée.");
  } catch (e) {
    addColumnError.value = errorMessage(e, "Impossible d'ajouter la colonne");
  } finally {
    addingColumn.value = false;
  }
}

// --- Distinct values inspector, inline per schema row ---
const expandedColumn = ref<string | null>(null);
const distinctValues = ref<RowValue[]>([]);
const distinctLoading = ref(false);
const distinctCache = new Map<string, RowValue[]>();

async function toggleDistinct(col: ColumnSchema) {
  if (expandedColumn.value === col.name) {
    expandedColumn.value = null;
    return;
  }
  expandedColumn.value = col.name;
  const cached = distinctCache.get(col.name);
  if (cached) {
    distinctValues.value = cached;
    return;
  }
  distinctLoading.value = true;
  try {
    const res = await getDistinctValues(props.tableId, col.name);
    distinctCache.set(col.name, res.values);
    distinctValues.value = res.values;
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de lister les valeurs de ${col.name}`));
    expandedColumn.value = null;
  } finally {
    distinctLoading.value = false;
  }
}

const schemaColumnHelper = createColumnHelper<ColumnSchema>();
const schemaColumnDefs = [
  schemaColumnHelper.accessor("name", { header: "Colonne", meta: { class: "font-medium text-slate-800" } }),
  schemaColumnHelper.accessor("type", { header: "Type", meta: { class: "font-mono text-xs text-slate-500" } }),
];

const schemaTable = useVueTable({
  get data() {
    return props.columns;
  },
  columns: schemaColumnDefs,
  getCoreRowModel: getCoreRowModel(),
});
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div class="mb-3 flex shrink-0 justify-end">
      <button
        class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
        @click="showAddColumn = true"
      >
        <Plus :size="14" /> Ajouter une colonne
      </button>
    </div>
    <DataTable :table="schemaTable" :row-class="() => 'cursor-pointer hover:bg-slate-50'" @row-click="(row) => toggleDistinct(row.original)">
      <template #row-after="{ row }">
        <tr v-if="expandedColumn === row.original.name">
          <td :colspan="row.getVisibleCells().length" class="bg-slate-50 px-4 py-3">
            <p v-if="distinctLoading" class="text-xs text-slate-400">Chargement…</p>
            <div v-else class="flex flex-wrap gap-1.5">
              <span
                v-for="(v, i) in distinctValues"
                :key="i"
                class="rounded-full bg-white px-2 py-0.5 text-xs text-slate-600 ring-1 ring-slate-200"
              >
                {{ v === null ? "NULL" : v }}
              </span>
              <span v-if="distinctValues.length === 0" class="text-xs text-slate-400">Aucune valeur.</span>
            </div>
          </td>
        </tr>
      </template>
    </DataTable>
  </div>

  <Modal v-if="showAddColumn" title="Ajouter une colonne" @close="showAddColumn = false">
    <label class="mb-3 block">
      <span class="mb-1 block text-xs font-medium text-slate-500">Nom</span>
      <input
        v-model="newColumnName"
        type="text"
        class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </label>
    <label class="block">
      <span class="mb-1 block text-xs font-medium text-slate-500">Type DuckDB</span>
      <input
        v-model="newColumnType"
        type="text"
        placeholder="VARCHAR, INTEGER, DOUBLE…"
        class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 font-mono text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
    </label>
    <p v-if="addColumnError" class="mt-3 text-xs text-red-600">{{ addColumnError }}</p>
    <div class="mt-4 flex justify-end gap-2">
      <button
        class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        @click="showAddColumn = false"
      >
        Annuler
      </button>
      <button
        class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
        :disabled="addingColumn || !newColumnName.trim()"
        @click="confirmAddColumn"
      >
        {{ addingColumn ? "Ajout…" : "Ajouter" }}
      </button>
    </div>
  </Modal>
</template>
