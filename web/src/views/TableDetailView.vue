<script setup lang="ts">
import { ref, toRef, watch } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { exportUrl } from "../client/tables";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import ColumnsModal from "../components/ColumnsModal.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import TableDataPanel from "../components/TableDataPanel.vue";
import DeriveTableModal from "../components/DeriveTableModal.vue";
import AddColumnModal from "../components/AddColumnModal.vue";
import BaseButton from "../components/ui/BaseButton.vue";
import { useTableMetadata } from "../composables/useTableMetadata";
import { useToast } from "../composables/useToast";

const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const { table, columns, editableColumns, rowColumns, loadTable, rename, remove } = useTableMetadata(
  toRef(props, "tableId"),
);

const renaming = ref(false);
const editingName = ref(false);
const nameInput = ref("");

const visibleColumns = ref<string[]>([]);
watch(rowColumns, (cols) => (visibleColumns.value = cols), { immediate: true });

const showColumns = ref(false);
const showDerive = ref(false);
const showAddColumn = ref(false);

const deriveModalRef = ref<InstanceType<typeof DeriveTableModal>>();
const addColumnModalRef = ref<InstanceType<typeof AddColumnModal>>();

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

function openDerive() {
  showDerive.value = true;
  deriveModalRef.value?.openModal();
}

async function onDeriveSuccess(result: { tableId: string }) {
  await router.push(`/tables/${result.tableId}`);
}

function openAddColumn() {
  showAddColumn.value = true;
  addColumnModalRef.value?.openModal();
}

async function onAddColumnSuccess() {
  await loadTable();
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
          <BaseButton size="sm" variant="primary" :busy="renaming" :disabled="renaming || !nameInput.trim()" @click="confirmRename">
            {{ renaming ? "..." : "Enregistrer" }}
          </BaseButton>
          <BaseButton size="sm" variant="secondary" @click="editingName = false">Annuler</BaseButton>
        </div>
        <h1 v-else class="group flex items-center gap-2 text-lg font-semibold text-slate-900">
          {{ table.tableName }}
          <button class="text-xs font-medium text-slate-400 hover:text-indigo-600" @click="startRename">Renommer</button>
        </h1>
        <p class="flex items-center gap-2 text-sm text-slate-500">
          <Badge :tone="table.isGeo ? 'primary' : 'neutral'">{{ table.isGeo ? "Géo" : "Standard" }}</Badge>
          {{ table.rowCount.toLocaleString("fr-FR") }} lignes · mis à jour {{ new Date(table.updatedAt).toLocaleString("fr-FR") }}
        </p>
      </div>
      <div class="flex gap-2">
        <BaseButton variant="secondary" @click="showColumns = true">Colonnes</BaseButton>
        <BaseButton variant="secondary" @click="openAddColumn">Ajouter une colonne</BaseButton>
        <BaseButton variant="secondary" @click="openDerive">Dériver</BaseButton>
        <a
          :href="exportUrl(table.tableId, table.isGeo ? 'geojson' : 'csv')"
          class="inline-flex items-center rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
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

    <DeriveTableModal
      v-if="showDerive && table"
      ref="deriveModalRef"
      :table-id="table.tableId"
      :physical-name="table.physicalName"
      @derived="onDeriveSuccess"
      @close="showDerive = false"
    />

    <ColumnsModal
      v-if="showColumns && table"
      :table-id="table.tableId"
      :row-columns="rowColumns"
      :visible-columns="visibleColumns"
      @update:visible-columns="visibleColumns = $event"
      @close="showColumns = false"
    />

    <AddColumnModal
      v-if="showAddColumn && table"
      ref="addColumnModalRef"
      :table-id="table.tableId"
      @added="onAddColumnSuccess"
      @close="showAddColumn = false"
    />
  </div>
</template>
