<script setup lang="ts">
import { defineAsyncComponent, ref } from "vue";
import SelectedFeaturesPanel from "./SelectedFeaturesPanel.vue";

// MapLibre GL is the largest dependency in the bundle — load it only when
// a geo table's map tab is actually opened.
const MapView = defineAsyncComponent(() => import("./MapView.vue"));

interface MapViewHandle {
  deselect: (pkey: string) => void;
  clearSelection: () => void;
}
const mapViewRef = ref<MapViewHandle | null>(null);
const selectedPkeys = ref<string[]>([]);

const props = defineProps<{ tableId: string; bbox?: string; rowColumns: string[] }>();

function onDeselect(pkey: string) {
  mapViewRef.value?.deselect(pkey);
}
function onClearSelection() {
  mapViewRef.value?.clearSelection();
}
</script>

<template>
  <div class="flex h-[65vh] gap-4">
    <SelectedFeaturesPanel
      :table-id="props.tableId"
      :columns="rowColumns"
      :selected-pkeys="selectedPkeys"
      @deselect="onDeselect"
      @clear="onClearSelection"
    />
    <div class="flex-1 overflow-hidden rounded-lg border border-slate-200">
      <MapView ref="mapViewRef" :table-id="props.tableId" :bbox="props.bbox" @selection-change="selectedPkeys = $event" />
    </div>
  </div>
</template>
