<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import { useRouter } from "vue-router";
import { exportTable } from "../client/tables";
import { errorMessage } from "../client/http";
import ColumnsModal from "../components/ColumnsModal.vue";
import TableDataPanel from "../components/TableDataPanel.vue";
import TableHeader from "../components/tables/TableHeader.vue";
import DeriveTableModal from "../components/tables/DeriveTableModal.vue";
import AddColumnModal from "../components/AddColumnModal.vue";
import Breadcrumbs from "../components/ui/Breadcrumbs.vue";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useTableMetadata } from "../composables/useTableMetadata";
import { useToast } from "../composables/useToast";
import { useDocumentTitle } from "../composables/useDocumentTitle";

const props = defineProps<{ tableId: string }>();
const router = useRouter();
const toast = useToast();

const { table, columns, rowColumns, loadTable, rename, remove } = useTableMetadata(
  toRef(props, "tableId"),
);

useDocumentTitle(computed(() => table.value?.tableName ?? "Table"));

// Selection is owned here rather than in the data panel: what is selected is
// a count of the table's rows, and it is stated where the table's other
// counters are (TableHeader). The panel and the map act on it through this
// one instance — there is a single selection, map and grid alike (§8.8).
const selection = useFeatureSelection(toRef(props, "tableId"));

const renaming = ref(false);
const editingName = ref(false);

// The user's visible-column choice must survive a schema reload (§8.3): only
// the first population is a straight assignment, later ones diff against the
// previously known column list so a hidden column stays hidden, a removed
// column drops out, and a newly added one defaults to visible.
const visibleColumns = ref<string[]>([]);
let knownRowColumns: string[] = [];
// Another table's columns are not a schema reload: forgetting what was known
// makes the next population a straight assignment again, so the new table
// opens on all of its columns rather than on the previous one's choice (§9).
watch(toRef(props, "tableId"), () => {
  knownRowColumns = [];
  editingName.value = false;
});
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

// The rename form closes on success only: a rejected name has to stay in
// front of the user, in the field they typed it in (§8.4).
async function onRename(name: string) {
  renaming.value = true;
  try {
    await rename(name);
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

    <TableHeader
      v-if="table"
      :table="table"
      :editing-name="editingName"
      :selected-count="selection.selectedPkeys.value.length"
      :renaming="renaming"
      :exporting="exporting"
      @update:editing-name="editingName = $event"
      @clear-selection="selection.clear"
      @rename="onRename"
      @add-column="openAddColumn"
      @derive="showDerive = true"
      @export="onExportTable"
      @delete="onDeleteTable"
    />

    <TableDataPanel
      v-if="table"
      :table-id="table.tableId"
      :bbox="table.bbox"
      :is-geo="table.isGeo"
      :row-columns="rowColumns"
      :visible-columns="visibleColumns"
      :columns="columns"
      :geometry-column="table.geometryColumn"
      :selection="selection"
      @mutated="loadTable"
      @open-columns="showColumns = true"
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
