<script setup lang="ts">
import { defineAsyncComponent, ref } from "vue";
import SelectedFeaturesPanel from "./SelectedFeaturesPanel.vue";
import { Layers } from "@lucide/vue";

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

const wfsEnabled = ref(false);
</script>

<template>
  <div class="flex h-[65vh] flex-col gap-4">
    <div class="flex shrink-0 items-center gap-4 rounded-lg border border-slate-200 bg-white p-3">
      <label class="flex items-center gap-1.5 text-sm font-medium text-slate-700">
        <input v-model="wfsEnabled" type="checkbox" />
        <Layers :size="14" /> Rendu WFS (au lieu du MVT)
      </label>
    </div>

    <div class="flex min-h-0 flex-1 gap-4">
      <SelectedFeaturesPanel
        :table-id="props.tableId"
        :columns="rowColumns"
        :selected-pkeys="selectedPkeys"
        @deselect="onDeselect"
        @clear="onClearSelection"
      />
      <div class="flex-1 overflow-hidden rounded-lg border border-slate-200">
        <MapView
          ref="mapViewRef"
          :table-id="props.tableId"
          :bbox="props.bbox"
          :wfs-enabled="wfsEnabled"
          @selection-change="selectedPkeys = $event"
        />
      </div>
    </div>
  </div>
</template>
