<script setup lang="ts">
import { ref } from "vue";
import { getDistinctValues } from "../client/tables";
import type { RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { useToast } from "../composables/useToast";
import Modal from "./Modal.vue";

const props = defineProps<{ tableId: string; rowColumns: string[]; visibleColumns: string[] }>();
const emit = defineEmits<{ "update:visibleColumns": [columns: string[]]; close: [] }>();
const toast = useToast();

function isVisible(col: string) {
  return props.visibleColumns.includes(col);
}

function toggleVisible(col: string) {
  emit("update:visibleColumns", isVisible(col) ? props.visibleColumns.filter((c) => c !== col) : [...props.visibleColumns, col]);
}

const expandedColumn = ref<string | null>(null);
const distinctValues = ref<RowValue[]>([]);
const distinctLoading = ref(false);
const distinctCache = new Map<string, RowValue[]>();

async function toggleDistinct(col: string) {
  if (expandedColumn.value === col) {
    expandedColumn.value = null;
    return;
  }
  expandedColumn.value = col;
  const cached = distinctCache.get(col);
  if (cached) {
    distinctValues.value = cached;
    return;
  }
  distinctLoading.value = true;
  try {
    const res = await getDistinctValues(props.tableId, col);
    distinctCache.set(col, res.values);
    distinctValues.value = res.values;
  } catch (e) {
    toast.error(errorMessage(e, `Impossible de lister les valeurs de ${col}`));
    expandedColumn.value = null;
  } finally {
    distinctLoading.value = false;
  }
}
</script>

<template>
  <Modal title="Colonnes" wide @close="emit('close')">
    <div class="flex max-h-96 flex-col gap-1 overflow-y-auto pr-1">
      <div v-for="col in rowColumns" :key="col" class="rounded-md border border-slate-200">
        <div class="flex items-center justify-between px-3 py-2">
          <label class="flex items-center gap-2 text-sm text-slate-700">
            <input type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" :checked="isVisible(col)" @change="toggleVisible(col)" />
            {{ col }}
          </label>
          <button class="text-xs font-medium text-indigo-600 hover:text-indigo-700" @click="toggleDistinct(col)">
            {{ expandedColumn === col ? "Masquer les valeurs" : "Voir les valeurs" }}
          </button>
        </div>
        <div v-if="expandedColumn === col" class="border-t border-slate-100 bg-slate-50 px-3 py-2">
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
        </div>
      </div>
    </div>
  </Modal>
</template>
