<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { deleteTable, deriveTable, exportUrl, getSchema, getTable, renameTable } from "../client/tables";
import type { ColumnSchema, Metadata } from "../client/types";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import Modal from "../components/Modal.vue";
import TableFilterTab from "../components/TableFilterTab.vue";
import TableMapTab from "../components/TableMapTab.vue";
import TableRowsTab from "../components/TableRowsTab.vue";
import TableSchemaTab from "../components/TableSchemaTab.vue";
import { useToast } from "../composables/useToast";

const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const table = ref<Metadata | null>(null);
const columns = ref<ColumnSchema[]>([]);
const tab = ref<"rows" | "schema" | "map" | "filter">("rows");
const renaming = ref(false);
const editingName = ref(false);
const nameInput = ref("");

// Columns actually writable by AddRows/UpdateRow: everything but the
// server-generated pkey. The geometry column (if any) stays in this list —
// InsertRow/UpdateRow accept WKT for it (règle 22) — but it's excluded from
// GetRows/GetRowsWithFilter (règle 7), so it never has a value to display.
const editableColumns = computed(() => columns.value.map((c) => c.name).filter((name) => name !== "__pkey"));
const rowColumns = computed(() => editableColumns.value.filter((name) => name !== table.value?.geometryColumn));

async function loadTable() {
  table.value = await getTable(props.tableId);
  if (table.value.isGeo) tab.value = "map";
}

async function loadSchema() {
  const res = await getSchema(props.tableId);
  columns.value = res.columns;
}

async function loadAll() {
  try {
    await Promise.all([loadTable(), loadSchema()]);
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de charger la table"));
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
  <div class="mx-auto flex h-full max-w-5xl flex-col px-8 py-8">
    <RouterLink to="/tables" class="shrink-0 text-sm font-medium text-slate-500 hover:text-slate-700">&larr; Tables</RouterLink>

    <div v-if="table" class="mt-2 mb-6 flex shrink-0 items-start justify-between">
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

    <div class="mb-4 flex shrink-0 gap-1 border-b border-slate-200">
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

    <TableMapTab v-if="tab === 'map' && table?.isGeo" :table-id="table.tableId" :bbox="table.bbox" :row-columns="rowColumns" />
    <TableSchemaTab v-else-if="tab === 'schema'" :table-id="tableId" :columns="columns" @column-added="loadSchema" />
    <TableFilterTab v-else-if="tab === 'filter'" :table-id="tableId" :row-columns="rowColumns" :columns="columns" />
    <TableRowsTab
      v-else
      :table-id="tableId"
      :row-columns="rowColumns"
      :editable-columns="editableColumns"
      :columns="columns"
      :geometry-column="table?.geometryColumn"
      @mutated="loadTable"
    />

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
