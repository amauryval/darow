<script setup lang="ts">
import { computed, defineAsyncComponent, ref, toRef, watch } from "vue";
import type { ColumnSchema, Row } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useLayerEditing } from "../composables/useLayerEditing";
import { useRowFilter } from "../composables/useRowFilter";
import { useRowGeometry } from "../composables/useRowGeometry";
import { useTableRows } from "../composables/useTableRows";
import { LayoutPanelLeft, Map as MapIcon, PencilLine, Spline, Table2 } from "@lucide/vue";
import FilteredRowsPanel from "./filters/FilteredRowsPanel.vue";
import TableFilterBar from "./filters/TableFilterBar.vue";
import AddRowModal from "./AddRowModal.vue";
import GeometryModal from "./tables/GeometryModal.vue";
import TableRowsPanel from "./tables/TableRowsPanel.vue";
import BaseButton from "./ui/BaseButton.vue";
import ButtonGroup from "./ui/ButtonGroup.vue";

// OpenLayers is the largest dependency in the bundle — load it only when a
// geo table's data panel is actually rendered.
const MapView = defineAsyncComponent(() => import("./MapView.vue"));

const props = defineProps<{
  tableId: string;
  bbox?: string;
  isGeo: boolean;
  rowColumns: string[];
  visibleColumns: string[];
  columns: ColumnSchema[];
  geometryColumn?: string;
}>();
// Emitted after any row add/edit/delete, since that changes the table's
// rowCount — the parent owns table metadata and reloads it.
const emit = defineEmits<{ mutated: [] }>();

const tableId = toRef(props, "tableId");
const columns = toRef(props, "columns");
const isGeo = toRef(props, "isGeo");
const geometryColumn = toRef(props, "geometryColumn");

// Every write bumps this: the map renders from tiles cached by URL, so an
// edited geometry only appears once the layers are told the data changed
// (they would otherwise wait for the next zoom to refetch).
const dataVersion = ref(0);

const filter = useRowFilter(tableId, columns, isGeo);
const selection = useFeatureSelection(tableId);
const rowsState = useTableRows(tableId, toRef(props, "rowColumns"), columns, () => {
  dataVersion.value += 1;
  emit("mutated");
});
const rowGeometry = useRowGeometry(tableId);
const layerEditing = useLayerEditing(tableId, geometryColumn, async () => {
  dataVersion.value += 1;
  await rowsState.retry();
  emit("mutated");
});

// The two halves of a table are edited separately and never at once: the
// geometries on the map, the attributes in the grid. Arming one disarms the
// other, so there is only ever one thing the screen is waiting for.
function toggleAttributeEditing() {
  if (layerEditing.active.value) layerEditing.cancel();
  rowsState.editMode.value = !rowsState.editMode.value;
}

function toggleGeometryEditing() {
  if (layerEditing.active.value) {
    layerEditing.cancel();
    return;
  }
  rowsState.editMode.value = false;
  layerEditing.start();
}

// Table and map share the space 50/50 by default; either can be maximized to
// take the whole width, restored back to 'split' from the same toggle group.
const focusPanel = ref<"split" | "table" | "map">("split");
watch(tableId, () => (focusPanel.value = "split"));

// Editing the geometries needs the map: it takes the whole width for as long
// as the session lasts, and the previous layout comes back afterwards.
watch(
  () => layerEditing.active.value,
  (active) => (focusPanel.value = active ? "map" : "split"),
);

// The map is mounted only while it is shown, and the session's unsaved work
// lives inside it — hiding the map would drop every reshape without a word.
// So the layout is frozen for as long as a session lasts: the way out is
// Enregistrer or Annuler, both of which are on the map itself (§8.3).
const layoutLocked = computed(() => layerEditing.active.value);
const layoutLockReason = computed(() =>
  layoutLocked.value ? "Édition des géométries en cours : enregistrez ou annulez pour changer la disposition." : undefined,
);

// Re-centers the map on the filter's own results rather than the whole
// table's extent, cleared back to the table's bbox once inactive.
const mapBbox = computed(() => filter.filteredBbox.value ?? props.bbox);

const showAddRow = ref(false);
const addRowModalRef = ref<InstanceType<typeof AddRowModal>>();

function openAddRow() {
  showAddRow.value = true;
  addRowModalRef.value?.openModal();
}

// On a geo table the attributes are collected first and the geometry drawn
// second: the row is only posted once both are in hand, so a row without its
// geometry never lands in the table. The session that opens is the ordinary
// layer-wide one — the shape drawn first is the one that takes these values.
async function onAddRow(values: Record<string, string>) {
  if (props.isGeo) {
    rowsState.editMode.value = false;
    layerEditing.start(rowsState.coerceValues(values));
    return;
  }
  await rowsState.addRow(values);
}

const displayRows = computed(() => {
  if (selection.selectedRows.value.length === 0) return rowsState.rows.value;
  const selectedSet = new Set(selection.selectedRows.value.map((row) => row.pkey));
  return [...selection.selectedRows.value, ...rowsState.rows.value.filter((row) => !selectedSet.has(row.pkey))];
});

function rowClassFor(row: { original: Row }) {
  return props.isGeo && selection.selectedPkeys.value.includes(row.original.pkey)
    ? "bg-warning/10 hover:bg-warning/20 cursor-pointer"
    : "";
}

function onRowClick(row: { original: Row }) {
  if (props.isGeo && selection.selectedPkeys.value.includes(row.original.pkey)) selection.deselect(row.original.pkey);
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4">
    <div class="flex shrink-0 flex-wrap items-center gap-2">
      <TableFilterBar :table-id="tableId" :columns="columns" @update:nodes="filter.conditions.value = $event" />

      <!-- The two halves of a table are edited apart: the attributes in the
           grid, the geometries on the map. A table without geometry has only
           the first, and the button says just that. -->
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <BaseButton
          size="sm"
          :variant="rowsState.editMode.value ? 'primary' : 'secondary'"
          :icon="PencilLine"
          :aria-pressed="rowsState.editMode.value"
          @click="toggleAttributeEditing"
        >
          {{ isGeo ? "Éditer la table" : "Mode édition" }}
        </BaseButton>
        <BaseButton
          v-if="isGeo"
          size="sm"
          :variant="layerEditing.active.value ? 'primary' : 'secondary'"
          :icon="Spline"
          :aria-pressed="layerEditing.active.value"
          @click="toggleGeometryEditing"
        >
          Éditer la géométrie
        </BaseButton>
      </div>

      <!-- Table and map default to half the width each; either can be
           maximized when the data being read needs the room (§8.10). -->
      <ButtonGroup v-if="isGeo">
        <BaseButton
          joined
          size="sm"
          :variant="focusPanel === 'table' ? 'primary' : 'secondary'"
          :icon="Table2"
          :aria-pressed="focusPanel === 'table'"
          :disabled="layoutLocked"
          :title="layoutLockReason"
          @click="focusPanel = focusPanel === 'table' ? 'split' : 'table'"
        >
          Tableau
        </BaseButton>
        <BaseButton
          joined
          size="sm"
          :variant="focusPanel === 'split' ? 'primary' : 'secondary'"
          :icon="LayoutPanelLeft"
          icon-only
          aria-label="Partager la vue"
          :disabled="layoutLocked"
          :title="layoutLockReason ?? 'Partager la vue'"
          @click="focusPanel = 'split'"
        />
        <BaseButton
          joined
          size="sm"
          :variant="focusPanel === 'map' ? 'primary' : 'secondary'"
          :icon="MapIcon"
          :aria-pressed="focusPanel === 'map'"
          :disabled="layoutLocked"
          :title="layoutLockReason"
          @click="focusPanel = focusPanel === 'map' ? 'split' : 'map'"
        >
          Carte
        </BaseButton>
      </ButtonGroup>
    </div>

    <div class="flex min-h-0 flex-1 gap-4">
      <div
        v-if="!isGeo || focusPanel !== 'map'"
        class="flex min-h-0 flex-col"
        :class="isGeo ? (focusPanel === 'table' ? 'w-full' : 'w-1/2') : 'w-full flex-1'"
      >
        <FilteredRowsPanel
          v-if="filter.filterActive.value"
          key="filter"
          :visible-columns="visibleColumns"
          :rows="filter.filterResults.value"
          :loading="filter.filtering.value"
          :error="filter.filterError.value"
          :limit="filter.filterLimit.value"
          :is-geo="isGeo"
          :row-class="rowClassFor"
          @update:limit="filter.filterLimit.value = $event"
          @retry="filter.retry"
          @row-click="onRowClick"
          @geometry="rowGeometry.open"
        />
        <TableRowsPanel
          v-else
          key="rows"
          :state="rowsState"
          :display-rows="displayRows"
          :visible-columns="visibleColumns"
          :is-geo="isGeo"
          :selected-count="selection.selectedPkeys.value.length"
          :row-class="rowClassFor"
          @add-row="openAddRow"
          @update-cell="(column, value) => (rowsState.editValues[column] = value)"
          @geometry="rowGeometry.open"
          @clear-selection="selection.clear"
          @row-click="onRowClick"
        />
      </div>

      <div
        v-if="isGeo && focusPanel !== 'table'"
        class="card min-h-0 overflow-hidden rounded-lg border border-base-300"
        :class="focusPanel === 'map' ? 'w-full' : 'w-1/2'"
      >
        <MapView
          :table-id="tableId"
          :bbox="mapBbox"
          :conditions="filter.appliedConditions.value"
          :selected-pkeys="selection.selectedPkeys.value"
          :editing="layerEditing.active.value"
          :editing-prompt="layerEditing.prompt.value"
          :features="layerEditing.features.value"
          :loading-features="layerEditing.loading.value"
          :saving="layerEditing.saving.value"
          :data-version="dataVersion"
          @toggle-selection="selection.toggle"
          @request-features="layerEditing.loadIn"
          @save="layerEditing.save"
          @cancel="layerEditing.cancel"
        />
      </div>
    </div>
  </div>

  <AddRowModal
    v-if="showAddRow"
    ref="addRowModalRef"
    :editable-columns="rowColumns"
    :is-geo="isGeo"
    @add="onAddRow"
    @close="showAddRow = false"
  />

  <GeometryModal
    v-if="rowGeometry.pkey.value"
    :pkey="rowGeometry.pkey.value"
    :geometry="rowGeometry.geometry.value"
    :wkt="rowGeometry.wkt.value"
    :loading="rowGeometry.loading.value"
    :error="rowGeometry.error.value"
    @close="rowGeometry.close"
    @retry="rowGeometry.retry"
    @copy="rowGeometry.copy"
  />
</template>
