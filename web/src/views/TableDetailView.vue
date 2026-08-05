<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import { useRouter } from "vue-router";
import { Columns3, Download, GitBranch, Pencil, Plus, Trash2 } from "@lucide/vue";
import { exportTable } from "../client/tables";
import { errorMessage } from "../client/http";
import Badge from "../components/Badge.vue";
import ColumnsModal from "../components/ColumnsModal.vue";
import ConfirmButton from "../components/ConfirmButton.vue";
import TableDataPanel from "../components/TableDataPanel.vue";
import DeriveTableModal from "../components/tables/DeriveTableModal.vue";
import AddColumnModal from "../components/AddColumnModal.vue";
import BaseButton from "../components/ui/BaseButton.vue";
import BaseInput from "../components/ui/BaseInput.vue";
import Breadcrumbs from "../components/ui/Breadcrumbs.vue";
import { useTableMetadata } from "../composables/useTableMetadata";
import { useToast } from "../composables/useToast";
import { useDocumentTitle } from "../composables/useDocumentTitle";
import Typography from "../components/ui/Typography.vue";


const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const { table, columns, rowColumns, loadTable, rename, remove } = useTableMetadata(
  toRef(props, "tableId"),
);

useDocumentTitle(computed(() => table.value?.tableName ?? "Table"));

const renaming = ref(false);
const editingName = ref(false);
const nameInput = ref("");

// The user's visible-column choice must survive a schema reload (§8.3): only
// the first population is a straight assignment, later ones diff against the
// previously known column list so a hidden column stays hidden, a removed
// column drops out, and a newly added one defaults to visible.
const visibleColumns = ref<string[]>([]);
let knownRowColumns: string[] = [];
// Another table's columns are not a schema reload: forgetting what was known
// makes the next population a straight assignment again, so the new table
// opens on all of its columns rather than on the previous one's choice (§9).
watch(toRef(props, "tableId"), () => (knownRowColumns = []));
watch(
  rowColumns,
  (cols) => {
    if (knownRowColumns.length === 0) {
      visibleColumns.value = cols;
    } else {
      const added = cols.filter((c) => !knownRowColumns.includes(c));
      visibleColumns.value = [...visibleColumns.value.filter((c) => cols.includes(c)), ...added];
    }
    knownRowColumns = cols;
  },
  { immediate: true },
);

const showColumns = ref(false);
const showDerive = ref(false);
const showAddColumn = ref(false);

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

const exporting = ref(false);

async function onExportTable() {
  if (!table.value) return;
  exporting.value = true;
  try {
    await exportTable(table.value.tableId, table.value.isGeo ? "geojson" : "csv", table.value.tableName);
  } catch (e) {
    toast.error(errorMessage(e, "Impossible d'exporter la table"));
  } finally {
    exporting.value = false;
  }
}

function openDerive() {
  showDerive.value = true;
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
  <div class="flex min-h-0 flex-1 flex-col gap-3 p-6">
    <Breadcrumbs :items="[{ label: 'Tables', to: '/tables' }, { label: table?.tableName ?? '…' }]" />

    <div v-if="table" class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-2">
        <template v-if="editingName">
          <BaseInput v-model="nameInput" size="sm" class="w-64" @keyup.enter="confirmRename" @keyup.esc="editingName = false" />
          <BaseButton size="xs" variant="primary" :busy="renaming" :disabled="renaming || !nameInput.trim()" @click="confirmRename">
            Enregistrer
          </BaseButton>
          <BaseButton size="xs" variant="secondary" @click="editingName = false">Annuler</BaseButton>
        </template>
        <template v-else>
          <BaseButton variant="ghost" size="xs" square :icon="Pencil" icon-only aria-label="Renommer la table" title="Renommer la table" @click="startRename" />
          <Badge :tone="table.isGeo ? 'primary' : 'neutral'">{{ table.isGeo ? "Géo" : "Standard" }}</Badge>
          <Typography variant="body-sm">
            {{ table.rowCount.toLocaleString("fr-FR") }} lignes · mis à jour {{ new Date(table.updatedAt).toLocaleString("fr-FR") }}
          </Typography>
        </template>
      </div>
      <div class="flex flex-wrap gap-2">
        <BaseButton variant="secondary" size="sm" :icon="Columns3" @click="showColumns = true">Colonnes</BaseButton>
        <BaseButton variant="secondary" size="sm" :icon="Plus" @click="openAddColumn">Ajouter une colonne</BaseButton>
        <BaseButton variant="secondary" size="sm" :icon="GitBranch" @click="openDerive">Dériver</BaseButton>
        <BaseButton variant="secondary" size="sm" :icon="Download" :busy="exporting" :disabled="exporting" @click="onExportTable">
          {{ exporting ? "Export en cours…" : "Exporter" }}
        </BaseButton>
        <ConfirmButton
          label="Supprimer"
          size="sm"
          :icon="Trash2"
          confirm-label="Supprimer la table"
          :message="`Supprimer « ${table.tableName} » ? Les tables dérivées de celle-ci seront également supprimées.`"
          @confirm="onDeleteTable"
        />
      </div>
    </div>

    <TableDataPanel
      v-if="table"
      :table-id="table.tableId"
      :bbox="table.bbox"
      :is-geo="table.isGeo"
      :row-columns="rowColumns"
      :visible-columns="visibleColumns"
      :columns="columns"
      :geometry-column="table.geometryColumn"
      @mutated="loadTable"
    />

    <DeriveTableModal
      v-if="showDerive && table"
      :table-id="table.tableId"
      :physical-name="table.physicalName"
      :columns="columns"
      :geometry-column="table.geometryColumn"
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
