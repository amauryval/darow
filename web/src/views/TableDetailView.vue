<script setup lang="ts">
import { ref, toRef, watch } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { exportUrl } from "../client/tables";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import ColumnsModal from "../components/ColumnsModal.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import Modal from "../components/Modal.vue";
import TableDataPanel from "../components/TableDataPanel.vue";
import { useTableMetadata } from "../composables/useTableMetadata";
import { useToast } from "../composables/useToast";

const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const { table, columns, editableColumns, rowColumns, loadTable, rename, remove, derive, addNewColumn } = useTableMetadata(
  toRef(props, "tableId"),
);

const renaming = ref(false);
const editingName = ref(false);
const nameInput = ref("");

const visibleColumns = ref<string[]>([]);
watch(rowColumns, (cols) => (visibleColumns.value = cols), { immediate: true });

function startRename() {
  nameInput.value = table.value?.tableName ?? "";
  editingName.value = true;
}

async function confirmRename() {
  if (!nameInput.value.trim()) return;
  renaming.value = true;
  try {
    await rename(nameInput.value.trim());
    editingName.value = false;
  } catch (e) {
    toast.error(errorMessage(e, "Impossible de renommer la table"));
  } finally {
    renaming.value = false;
  }
}

async function onDeleteTable() {
  try {
    const name = table.value?.tableName;
    await remove();
    toast.success(`Table "${name}" supprimée.`);
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
    const result = await derive(deriveQuery.value.trim(), deriveName.value.trim());
    showDerive.value = false;
    toast.success("Table dérivée créée.");
    router.push(`/tables/${result.tableId}`);
  } catch (e) {
    deriveError.value = errorMessage(e, "Impossible de dériver la table");
  } finally {
    deriving.value = false;
  }
}

// --- Columns modal (visibility + distinct values) ---
const showColumns = ref(false);

// --- Add column modal ---
const showAddColumn = ref(false);
const newColumnName = ref("");
const newColumnType = ref("VARCHAR");
const addingColumn = ref(false);
const addColumnError = ref<string | null>(null);

function openAddColumn() {
  newColumnName.value = "";
  newColumnType.value = "VARCHAR";
  addColumnError.value = null;
  showAddColumn.value = true;
}

async function confirmAddColumn() {
  if (!newColumnName.value.trim()) return;
  addingColumn.value = true;
  addColumnError.value = null;
  try {
    await addNewColumn(newColumnName.value.trim(), newColumnType.value.trim());
    showAddColumn.value = false;
    toast.success("Colonne ajoutée.");
  } catch (e) {
    addColumnError.value = errorMessage(e, "Impossible d'ajouter la colonne");
  } finally {
    addingColumn.value = false;
  }
}
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
          @click="showColumns = true"
        >
          Colonnes
        </button>
        <button
          class="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          @click="openAddColumn"
        >
          Ajouter une colonne
        </button>
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

    <TableDataPanel
      v-if="table"
      :table-id="table.tableId"
      :bbox="table.bbox"
      :is-geo="table.isGeo"
      :row-columns="rowColumns"
      :visible-columns="visibleColumns"
      :editable-columns="editableColumns"
      :columns="columns"
      :geometry-column="table.geometryColumn"
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

    <ColumnsModal
      v-if="showColumns && table"
      :table-id="table.tableId"
      :row-columns="rowColumns"
      :visible-columns="visibleColumns"
      @update:visible-columns="visibleColumns = $event"
      @close="showColumns = false"
    />

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
  </div>
</template>
