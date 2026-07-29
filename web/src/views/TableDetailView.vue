<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, reactive, ref, watch } from "vue";
import { RouterLink, useRouter } from "vue-router";
import {
  addColumn,
  addRows,
  deleteRow,
  deleteTable,
  deriveTable,
  exportUrl,
  filterRows,
  getDistinctValues,
  getSchema,
  getTable,
  listRows,
  renameTable,
  updateRow,
} from "../client/tables";
import type { ColumnSchema, FilterCondition, Metadata, Row, RowValue } from "../client/types";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import Modal from "../components/Modal.vue";
import Pagination from "../components/Pagination.vue";
import { useToast } from "../composables/useToast";
import { PencilLine, Plus, SlidersHorizontal } from "@lucide/vue";

// MapLibre GL is the largest dependency in the bundle — load it only when
// a geo table's map tab is actually opened.
const MapView = defineAsyncComponent(() => import("../components/MapView.vue"));

const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const table = ref<Metadata | null>(null);
const columns = ref<ColumnSchema[]>([]);
const rows = ref<Row[]>([]);
const total = ref(0);
const offset = ref(0);
const limit = ref(20);
const tab = ref<"rows" | "schema" | "map" | "filter">("rows");
const loading = ref(false);
const renaming = ref(false);
const editingName = ref(false);
const nameInput = ref("");

// Columns actually writable by AddRows/UpdateRow: everything but the
// server-generated pkey. The geometry column (if any) stays in this list —
// InsertRow/UpdateRow accept WKT for it (règle 22) — but it's excluded from
// GetRows/GetRowsWithFilter (règle 7), so it never has a value to display.
const editableColumns = computed(() => columns.value.map((c) => c.name).filter((name) => name !== "__pkey"));
const rowColumns = computed(() => editableColumns.value.filter((name) => name !== table.value?.geometryColumn));

function cellValue(row: Row, col: string): RowValue {
  // The physical column is named "__pkey", but Row objects key it as
  // "pkey" (router.rowToJSON pulls it from storage.Row's dedicated Pkey
  // field rather than its Values map) — map the two back together for
  // display rather than showing a blank cell.
  return col === "__pkey" ? row.pkey : row[col];
}

async function loadTable() {
  table.value = await getTable(props.tableId);
  if (table.value.isGeo) tab.value = "map";
}

async function loadSchema() {
  const res = await getSchema(props.tableId);
  columns.value = res.columns;
}

async function loadRows() {
  const page = await listRows(props.tableId, offset.value, limit.value);
  rows.value = page.rows;
  total.value = page.total;
}

async function loadAll() {
  loading.value = true;
  try {
    await Promise.all([loadTable(), loadSchema(), loadRows()]);
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger la table"));
  } finally {
    loading.value = false;
  }
}

function startRename() {
  nameInput.value = table.value?.tableName ?? "";
  editingName.value = true;
}

async function confirmRename() {
  if (!nameInput.value.trim()) return;
  renaming.value = true;
  try {
    table.value = await renameTable(props.tableId, nameInput.value.trim());
    editingName.value = false;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de renommer la table"));
  } finally {
    renaming.value = false;
  }
}

async function onDeleteTable() {
  try {
    await deleteTable(props.tableId);
    toast.success(`Table "${table.value?.tableName}" supprimée.`);
    router.push("/tables");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de supprimer la table"));
  }
}

function nextPage() {
  if (offset.value + limit.value >= total.value) return;
  offset.value += limit.value;
}
function prevPage() {
  offset.value = Math.max(0, offset.value - limit.value);
}
watch(offset, loadRows);

// --- Edit mode: toggling reveals a per-row "Modifier" action instead of
// switching straight to a spreadsheet-like grid, matching the inline
// edit/cancel pattern used everywhere else in this app (rename, EPSG). ---
const editMode = ref(false);
const editingPkey = ref<string | null>(null);
const editValues = reactive<Record<string, string>>({});
const savingRow = ref(false);
const deletingPkey = ref<string | null>(null);

function startEditRow(row: Row) {
  editingPkey.value = row.pkey;
  for (const col of rowColumns.value) {
    const v = cellValue(row, col);
    editValues[col] = v === null || v === undefined ? "" : String(v);
  }
}

function cancelEditRow() {
  editingPkey.value = null;
}

function coerceByType(type: string, raw: string): RowValue {
  const t = type.toUpperCase();
  if (t.includes("BOOL")) return raw === "true" || raw === "1";
  if (/INT|DOUBLE|FLOAT|DECIMAL|REAL|NUMERIC|HUGEINT/.test(t)) {
    if (raw === "") return null;
    const n = Number(raw);
    return Number.isNaN(n) ? raw : n;
  }
  return raw === "" ? null : raw;
}

function columnType(name: string): string {
  return columns.value.find((c) => c.name === name)?.type ?? "";
}

async function saveEditRow() {
  if (!editingPkey.value) return;
  savingRow.value = true;
  try {
    const updates: Record<string, unknown> = {};
    for (const col of rowColumns.value) {
      updates[col] = coerceByType(columnType(col), editValues[col] ?? "");
    }
    await updateRow(props.tableId, editingPkey.value, updates);
    editingPkey.value = null;
    await loadRows();
    toast.success("Ligne mise à jour.");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de mettre à jour la ligne"));
  } finally {
    savingRow.value = false;
  }
}

async function onDeleteRow(pkey: string) {
  deletingPkey.value = pkey;
  try {
    await deleteRow(props.tableId, pkey);
    await Promise.all([loadRows(), loadTable()]);
    toast.success("Ligne supprimée.");
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de supprimer la ligne"));
  } finally {
    deletingPkey.value = null;
  }
}

// --- Add row modal ---
const showAddRow = ref(false);
const newRowValues = reactive<Record<string, string>>({});
const addingRow = ref(false);
const addRowError = ref<string | null>(null);

function openAddRow() {
  for (const col of editableColumns.value) newRowValues[col] = "";
  addRowError.value = null;
  showAddRow.value = true;
}

async function confirmAddRow() {
  addingRow.value = true;
  addRowError.value = null;
  try {
    const values: Record<string, unknown> = {};
    for (const col of editableColumns.value) {
      values[col] = coerceByType(columnType(col), newRowValues[col] ?? "");
    }
    await addRows(props.tableId, [values]);
    showAddRow.value = false;
    await Promise.all([loadRows(), loadTable()]);
    toast.success("Ligne ajoutée.");
  } catch (e) {
    addRowError.value = errorMessage(e, "Impossible d'ajouter la ligne");
  } finally {
    addingRow.value = false;
  }
}

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
    await loadSchema();
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

// --- Filter tab ---
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
  draftConditions.value.push({ column: rowColumns.value[0] ?? "", operator: "=", value: "" });
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
      value: coerceByType(columnType(c.column), c.value),
    }));
    const res = await filterRows(props.tableId, conditions, filterLimit.value);
    filterResults.value = res.rows;
  } catch (e) {
    filterError.value = errorMessage(e, "Impossible d'appliquer le filtre");
  } finally {
    filtering.value = false;
  }
}

// --- Derive table modal ---
const showDerive = ref(false);
const deriveQuery = ref("");
const deriveName = ref("");
const deriving = ref(false);
const deriveError = ref<string | null>(null);

function openDerive() {
  deriveQuery.value = table.value ? `SELECT * FROM "${table.value.physicalName}"` : "SELECT * FROM ";
  deriveName.value = "";
  deriveError.value = null;
  showDerive.value = true;
}

async function confirmDerive() {
  if (!deriveQuery.value.trim() || !deriveName.value.trim()) return;
  deriving.value = true;
  deriveError.value = null;
  try {
    const result = await deriveTable(props.tableId, deriveQuery.value.trim(), deriveName.value.trim());
    showDerive.value = false;
    toast.success("Table dérivée créée.");
    router.push(`/tables/${result.tableId}`);
  } catch (e) {
    deriveError.value = errorMessage(e, "Impossible de dériver la table");
  } finally {
    deriving.value = false;
  }
}

onMounted(loadAll);
</script>

<template>
  <div class="mx-auto max-w-5xl px-8 py-8">
    <RouterLink to="/tables" class="text-sm font-medium text-slate-500 hover:text-slate-700">&larr; Tables</RouterLink>

    <div v-if="table" class="mt-2 mb-6 flex items-start justify-between">
      <div>
        <div v-if="editingName" class="flex items-center gap-2">
          <input
            v-model="nameInput"
            type="text"
            class="rounded-md border border-slate-300 px-2 py-1 text-lg font-semibold text-slate-900"
            @keyup.enter="confirmRename"
            @keyup.esc="editingName = false"
          />
          <button
            class="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
            :disabled="renaming || !nameInput.trim()"
            @click="confirmRename"
          >
            {{ renaming ? "..." : "Enregistrer" }}
          </button>
          <button
            class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
            @click="editingName = false"
          >
            Annuler
          </button>
        </div>
        <h1 v-else class="group flex items-center gap-2 text-lg font-semibold text-slate-900">
          {{ table.tableName }}
          <button class="text-xs font-medium text-slate-400 hover:text-indigo-600" @click="startRename">Renommer</button>
        </h1>
        <p class="flex items-center gap-2 text-sm text-slate-500">
          <Badge :tone="table.isGeo ? 'indigo' : 'slate'">{{ table.isGeo ? "Géo" : "Standard" }}</Badge>
          {{ table.rowCount.toLocaleString("fr-FR") }} lignes · mis à jour {{ new Date(table.updatedAt).toLocaleString("fr-FR") }}
        </p>
      </div>
      <div class="flex gap-2">
        <button
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          @click="openDerive"
        >
          Dériver
        </button>
        <a
          :href="exportUrl(table.tableId, table.isGeo ? 'geojson' : 'csv')"
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
        >
          Exporter
        </a>
        <ConfirmButton label="Supprimer" confirm-label="Supprimer la table" @confirm="onDeleteTable" />
      </div>
    </div>

    <div class="mb-4 flex gap-1 border-b border-slate-200">
      <button
        v-if="table?.isGeo"
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'map' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'map'"
      >
        Carte
      </button>
      <button
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'rows' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'rows'"
      >
        Données
      </button>
      <button
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'filter' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'filter'"
      >
        Filtrer
      </button>
      <button
        class="border-b-2 px-3 py-2 text-sm font-medium"
        :class="tab === 'schema' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-slate-500 hover:text-slate-700'"
        @click="tab = 'schema'"
      >
        Schéma
      </button>
    </div>

    <div v-if="tab === 'map' && table?.isGeo" class="h-[65vh] overflow-hidden rounded-lg border border-slate-200">
      <MapView :table-id="table.tableId" :bbox="table.bbox" />
    </div>

    <div v-else-if="tab === 'schema'">
      <div class="mb-3 flex justify-end">
        <button
          class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
          @click="showAddColumn = true"
        >
          <Plus :size="14" /> Ajouter une colonne
        </button>
      </div>
      <div class="overflow-hidden rounded-lg border border-slate-200 bg-white">
        <table class="w-full text-sm">
          <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
            <tr>
              <th class="px-4 py-2.5">Colonne</th>
              <th class="px-4 py-2.5">Type</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <template v-for="col in columns" :key="col.name">
              <tr class="cursor-pointer hover:bg-slate-50" @click="toggleDistinct(col)">
                <td class="px-4 py-2.5 font-medium text-slate-800">{{ col.name }}</td>
                <td class="px-4 py-2.5 font-mono text-xs text-slate-500">{{ col.type }}</td>
              </tr>
              <tr v-if="expandedColumn === col.name">
                <td colspan="2" class="bg-slate-50 px-4 py-3">
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
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="tab === 'filter'">
      <div class="mb-4 rounded-lg border border-slate-200 bg-white p-4">
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

      <div v-if="filterResults" class="overflow-x-auto rounded-lg border border-slate-200 bg-white">
        <table class="w-full text-sm">
          <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
            <tr>
              <th v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5">{{ col }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="row in filterResults" :key="row.pkey">
              <td v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5 text-slate-700">
                {{ cellValue(row, col) }}
              </td>
            </tr>
            <tr v-if="filterResults.length === 0">
              <td :colspan="rowColumns.length || 1" class="px-4 py-8 text-center text-sm text-slate-400">Aucun résultat.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else>
      <div class="mb-3 flex items-center justify-between">
        <label class="flex items-center gap-2 text-sm text-slate-600">
          <input v-model="editMode" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-300" />
          <PencilLine :size="14" /> Mode édition
        </label>
        <button
          v-if="editMode"
          class="flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500"
          @click="openAddRow"
        >
          <Plus :size="14" /> Ajouter une ligne
        </button>
      </div>

      <div class="overflow-x-auto rounded-lg border border-slate-200 bg-white">
        <table class="w-full text-sm">
          <thead class="border-b border-slate-200 bg-slate-50 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
            <tr>
              <th v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5">{{ col }}</th>
              <th v-if="editMode" class="px-4 py-2.5"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="row in rows" :key="row.pkey">
              <template v-if="editingPkey === row.pkey">
                <td v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2">
                  <input v-model="editValues[col]" type="text" class="w-full rounded-md border border-slate-300 px-2 py-1 text-sm" />
                </td>
                <td class="whitespace-nowrap px-4 py-2 text-right">
                  <div class="flex justify-end gap-2">
                    <button
                      class="rounded-md bg-indigo-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                      :disabled="savingRow"
                      @click="saveEditRow"
                    >
                      {{ savingRow ? "…" : "Enregistrer" }}
                    </button>
                    <button
                      class="rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                      @click="cancelEditRow"
                    >
                      Annuler
                    </button>
                  </div>
                </td>
              </template>
              <template v-else>
                <td v-for="col in rowColumns" :key="col" class="whitespace-nowrap px-4 py-2.5 text-slate-700">
                  {{ cellValue(row, col) }}
                </td>
                <td v-if="editMode" class="whitespace-nowrap px-4 py-2.5 text-right">
                  <div class="flex justify-end gap-2">
                    <button class="text-xs font-medium text-indigo-600 hover:text-indigo-700" @click="startEditRow(row)">
                      Modifier
                    </button>
                    <ConfirmButton label="Supprimer" :busy="deletingPkey === row.pkey" @confirm="onDeleteRow(row.pkey)" />
                  </div>
                </td>
              </template>
            </tr>
            <tr v-if="!loading && rows.length === 0">
              <td :colspan="(rowColumns.length || 1) + (editMode ? 1 : 0)" class="px-4 py-8 text-center text-sm text-slate-400">
                Aucune ligne.
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-3">
        <Pagination :offset="offset" :limit="limit" :total="total" @prev="prevPage" @next="nextPage" />
      </div>
    </div>

    <Modal v-if="showAddRow" title="Ajouter une ligne" @close="showAddRow = false">
      <div class="flex max-h-96 flex-col gap-3 overflow-y-auto pr-1">
        <label v-for="col in editableColumns" :key="col" class="block">
          <span class="mb-1 block text-xs font-medium text-slate-500">
            {{ col }} <span v-if="col === table?.geometryColumn">(WKT)</span>
          </span>
          <input
            v-model="newRowValues[col]"
            type="text"
            :placeholder="col === table?.geometryColumn ? 'POINT(2.35 48.85)' : ''"
            class="w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
          />
        </label>
      </div>
      <p v-if="addRowError" class="mt-3 text-xs text-red-600">{{ addRowError }}</p>
      <div class="mt-4 flex justify-end gap-2">
        <button
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          @click="showAddRow = false"
        >
          Annuler
        </button>
        <button
          class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
          :disabled="addingRow"
          @click="confirmAddRow"
        >
          {{ addingRow ? "Ajout…" : "Ajouter" }}
        </button>
      </div>
    </Modal>

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

    <Modal v-if="showDerive" title="Dériver une table" wide @close="showDerive = false">
      <p class="mb-3 text-xs text-slate-500">
        Requête <code class="rounded bg-slate-100 px-1 py-0.5">SELECT</code> exécutée sur les tables existantes ; le résultat est
        matérialisé dans une nouvelle table.
      </p>
      <textarea
        v-model="deriveQuery"
        rows="4"
        spellcheck="false"
        class="w-full resize-y rounded-md border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
      <input
        v-model="deriveName"
        type="text"
        placeholder="Nom de la nouvelle table"
        class="mt-3 w-full rounded-md border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-400 focus:outline-none focus:ring-1 focus:ring-indigo-400"
      />
      <p v-if="deriveError" class="mt-2 text-xs text-red-600">{{ deriveError }}</p>
      <div class="mt-4 flex justify-end gap-2">
        <button
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          @click="showDerive = false"
        >
          Annuler
        </button>
        <button
          class="rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
          :disabled="deriving || !deriveQuery.trim() || !deriveName.trim()"
          @click="confirmDerive"
        >
          {{ deriving ? "Création…" : "Créer" }}
        </button>
      </div>
    </Modal>
  </div>
</template>
