<script setup lang="ts">
import { ref } from "vue";
import { Eye, EyeOff } from "@lucide/vue";
import { getDistinctValues } from "../client/tables";
import type { RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import { useToast } from "../composables/useToast";
import Modal from "./Modal.vue";
import BaseButton from "./ui/BaseButton.vue";
import BaseCheckbox from "./ui/BaseCheckbox.vue";

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
      <div v-for="col in rowColumns" :key="col" class="rounded-md border border-base-300">
        <div class="flex items-center justify-between px-3 py-2">
          <label class="flex items-center gap-2 text-sm">
            <BaseCheckbox :model-value="isVisible(col)" @update:model-value="toggleVisible(col)" />
            {{ col }}
          </label>
          <BaseButton variant="ghost" size="xs" :icon="expandedColumn === col ? EyeOff : Eye" @click="toggleDistinct(col)">
            {{ expandedColumn === col ? "Masquer les valeurs" : "Voir les valeurs" }}
          </BaseButton>
        </div>
        <div v-if="expandedColumn === col" class="border-t border-base-200 bg-base-200 px-3 py-2">
          <p v-if="distinctLoading" class="text-xs text-base-content/40">Chargement…</p>
          <div v-else class="flex flex-wrap gap-1.5">
            <span v-for="(v, i) in distinctValues" :key="i" class="badge badge-ghost badge-sm">
              {{ v === null ? "NULL" : v }}
            </span>
            <span v-if="distinctValues.length === 0" class="text-xs text-base-content/40">Aucune valeur.</span>
          </div>
        </div>
      </div>
    </div>
  </Modal>
</template>
