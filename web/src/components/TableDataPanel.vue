<script setup lang="ts">
import { computed, defineAsyncComponent, ref, toRef, watch } from "vue";
import type { ColumnSchema, Row } from "../client/types";
import { useFeatureSelection } from "../composables/useFeatureSelection";
import { useGeometryEditing } from "../composables/useGeometryEditing";
import { useRowFilter } from "../composables/useRowFilter";
import { useRowGeometry } from "../composables/useRowGeometry";
import { useTableRows } from "../composables/useTableRows";
import { LayoutPanelLeft, Map as MapIcon, Table2 } from "@lucide/vue";
import FilteredRowsPanel from "./filters/FilteredRowsPanel.vue";
import TableFilterBar from "./filters/TableFilterBar.vue";
import AddRowModal from "./AddRowModal.vue";
import GeometryModal from "./tables/GeometryModal.vue";
import TableRowsPanel from "./tables/TableRowsPanel.vue";
import BaseButton from "./ui/BaseButton.vue";

// MapLibre GL is the largest dependency in the bundle — load it only when
// a geo table's data panel is actually rendered.
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

// Every write bumps this: the map renders from tiles MapLibre caches by
// URL, so an edited geometry only appears once the layers are told the data
// changed (they would otherwise wait for the next zoom to refetch).
const dataVersion = ref(0);

const filter = useRowFilter(tableId, columns, isGeo);
const selection = useFeatureSelection(tableId);
const rowsState = useTableRows(tableId, toRef(props, "rowColumns"), columns, () => {
  dataVersion.value += 1;
  emit("mutated");
});
const rowGeometry = useRowGeometry(tableId);
const geometryEditing = useGeometryEditing(tableId, geometryColumn, async () => {
  dataVersion.value += 1;
  await rowsState.retry();
  emit("mutated");
});

// Table and map share the space 50/50 by default; either can be maximized to
// take the whole width, restored back to 'split' from the same toggle group.
const focusPanel = ref<"split" | "table" | "map">("split");
watch(tableId, () => (focusPanel.value = "split"));

// Entering a geometry needs the map: it takes the whole width for as long
// as the drawing lasts, and the previous layout comes back afterwards.
watch(
  () => geometryEditing.active.value,
  (active) => {
    if (active) focusPanel.value = "map";
    else focusPanel.value = "split";
  },
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
// second: the row is only posted once both are in hand, so a row without
// its geometry never lands in the table.
async function onAddRow(values: Record<string, string>) {
  if (props.isGeo) {
    geometryEditing.startCreate(rowsState.coerceValues(values));
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

      <!-- Table and map default to half the width each; either can be
           maximized when the data being read needs the room (§8.10). -->
      <div v-if="isGeo" class="join ml-auto">
        <BaseButton
          class="join-item"
          size="sm"
          :variant="focusPanel === 'table' ? 'primary' : 'secondary'"
          :icon="Table2"
          :aria-pressed="focusPanel === 'table'"
          @click="focusPanel = focusPanel === 'table' ? 'split' : 'table'"
        >
          Tableau
        </BaseButton>
        <BaseButton
          class="join-item"
          size="sm"
          :variant="focusPanel === 'split' ? 'primary' : 'secondary'"
          :icon="LayoutPanelLeft"
          icon-only
          aria-label="Partager la vue"
          title="Partager la vue"
          @click="focusPanel = 'split'"
        />
        <BaseButton
          class="join-item"
          size="sm"
          :variant="focusPanel === 'map' ? 'primary' : 'secondary'"
          :icon="MapIcon"
          :aria-pressed="focusPanel === 'map'"
          @click="focusPanel = focusPanel === 'map' ? 'split' : 'map'"
        >
          Carte
        </BaseButton>
      </div>
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
          @toggle-edit-mode="rowsState.editMode.value = !rowsState.editMode.value"
          @update-cell="(column, value) => (rowsState.editValues[column] = value)"
          @geometry="rowGeometry.open"
          @edit-geometry="geometryEditing.startEdit"
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
          v-model:topology="geometryEditing.topology.value"
          :table-id="tableId"
          :bbox="mapBbox"
          :conditions="filter.appliedConditions.value"
          :selected-pkeys="selection.selectedPkeys.value"
          :drawing="geometryEditing.active.value"
          :drawing-prompt="geometryEditing.prompt.value"
          :initial-geometry="geometryEditing.initialGeometry.value"
          :neighbours="geometryEditing.neighbours.value"
          :saving="geometryEditing.saving.value"
          :data-version="dataVersion"
          @toggle-selection="selection.toggle"
          @save="geometryEditing.save"
          @cancel="geometryEditing.cancel"
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
