<script setup lang="ts">
import { computed, defineAsyncComponent, ref, toRef, watch } from "vue";
import type { ColumnSchema, Row } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useLayerEditing } from "../composables/useLayerEditing";
import { useRowFilter } from "../composables/useRowFilter";
import { useRowGeometry } from "../composables/useRowGeometry";
import { useTableRows } from "../composables/useTableRows";
import FilteredRowsPanel from "./filters/FilteredRowsPanel.vue";
import AddRowModal from "./AddRowModal.vue";
import GeometryModal from "./tables/GeometryModal.vue";
import TableEditBar from "./tables/TableEditBar.vue";
import TableRowsPanel from "./tables/TableRowsPanel.vue";
import TableToolbar from "./tables/TableToolbar.vue";

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
// mutated: after any row add/edit/delete, since that changes the table's
// rowCount — the parent owns table metadata and reloads it.
// openColumns: the visible-column choice is the view's state (it survives a
// schema reload), so the toolbar's button only asks for its modal.
const emit = defineEmits<{ mutated: []; openColumns: [] }>();

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
const rowsState = useTableRows(tableId, toRef(props, "rowColumns"), columns, geometryColumn, () => {
  dataVersion.value += 1;
  emit("mutated");
});
const rowGeometry = useRowGeometry(tableId);
const layerEditing = useLayerEditing(tableId, geometryColumn, async () => {
  dataVersion.value += 1;
  await rowsState.retry();
  emit("mutated");
});

// Editing is **one** mode of the table, not two: the rows and the geometries
// are two halves of the same object, and asking the user which half they mean
// before they have touched anything is asking a question they cannot answer
// yet. What the mode offers is what the table can carry — a table without
// geometry simply has no geometry tools (§8.8).
const editing = computed(() => rowsState.editMode.value || layerEditing.active.value);

function toggleEditing() {
  if (editing.value) {
    // Cancelling says what it drops: a session opened to give a new row its
    // geometry loses that row (useLayerEditing.cancel).
    layerEditing.cancel();
    rowsState.editMode.value = false;
    return;
  }
  rowsState.editMode.value = true;
  if (props.isGeo) layerEditing.start();
}

// The tools the map owns are rendered into the bar above both panels: they
// are part of the same mode as the row tools, and they have to stay reachable
// when the map is the hidden half (see mapMounted).
const MAP_TOOLS_ID = "darow-map-edit-tools";

// Table and map share the space 50/50 by default; either can be maximized to
// take the whole width, restored back to 'split' from the same toggle group.
// Neither edit mode touches it: the layout is the user's, and an editing
// session that reframes the screen under them takes away the only view they
// were working from (§8.3).
const focusPanel = ref<"split" | "table" | "map">("split");
watch(tableId, () => (focusPanel.value = "split"));

// The session's unsaved work lives inside the map, so the map stays mounted
// for as long as one lasts — maximizing the table hides it rather than
// unmounting it, which would drop every reshape without a word.
const mapMounted = computed(() => props.isGeo && (focusPanel.value !== "table" || layerEditing.active.value));
const mapHidden = computed(() => focusPanel.value === "table");

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
    // The session is already open — it is the same mode the button was
    // reached from — so its values are attached to it rather than starting it
    // over, which would drop everything loaded and every reshape made since.
    layerEditing.awaitGeometry(rowsState.coerceValues(values));
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
  if (!props.isGeo) return "";
  return selection.selectedPkeys.value.includes(row.original.pkey)
    ? "bg-warning/10 hover:bg-warning/20 cursor-pointer"
    : "cursor-pointer";
}

// One click on a row toggles it in and out of the selection, exactly as one
// click on its feature does — map selection and table selection are the same
// selection, so the gesture that reverses it has to be the same too (§8.8).
// A row open for editing is not a target: the click is aimed at the field
// under the pointer, and DataTable has already filtered out the controls.
function onRowClick(row: { original: Row }) {
  if (!props.isGeo || rowsState.editingPkey.value === row.original.pkey) return;
  selection.toggle(row.original.pkey);
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4">
    <TableToolbar
      :table-id="tableId"
      :columns="columns"
      :is-geo="isGeo"
      :editing="editing"
      :focus-panel="focusPanel"
      @update:nodes="filter.conditions.value = $event"
      @update:focus-panel="focusPanel = $event"
      @toggle-editing="toggleEditing"
      @open-columns="emit('openColumns')"
    />

    <TableEditBar v-if="editing" :is-geo="isGeo" :tools-id="MAP_TOOLS_ID" @add-row="openAddRow" />

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
          @update-cell="(column, value) => (rowsState.editValues[column] = value)"
          @geometry="rowGeometry.open"
          @clear-selection="selection.clear"
          @row-click="onRowClick"
        />
      </div>

      <!-- Kept in the DOM but hidden while a session holds unsaved reshapes
           (mapMounted): OpenLayers re-measures its container on its own when
           it comes back, so maximizing the table costs nothing but the view. -->
      <div
        v-if="mapMounted"
        class="card min-h-0 overflow-hidden rounded-lg border border-base-300"
        :class="mapHidden ? 'hidden' : focusPanel === 'map' ? 'w-full' : 'w-1/2'"
      >
        <MapView
          :table-id="tableId"
          :bbox="mapBbox"
          :conditions="filter.appliedConditions.value"
          :selected-pkeys="selection.selectedPkeys.value"
          :editing="layerEditing.active.value"
          :features="layerEditing.features.value"
          :loading-features="layerEditing.loading.value"
          :saving="layerEditing.saving.value"
          :toolbar-target="`#${MAP_TOOLS_ID}`"
          :data-version="dataVersion"
          @toggle-selection="selection.toggle"
          @request-feature="layerEditing.reach"
          @release-features="layerEditing.release"
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
