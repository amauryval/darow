<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import type { FilterCondition } from "../../client/types";
import type { EditableFeature, LayerChanges } from "../../composables/useEditLayer";

// OpenLayers is the largest dependency in the bundle — load it only when a
// geo table's map half is actually rendered.
const MapView = defineAsyncComponent(() => import("../map/MapView.vue"));

const props = defineProps<{
  tableId: string;
  bbox?: string;
  isGeo: boolean;
  focusPanel: "split" | "table" | "map";
  conditions: FilterCondition[];
  selectedPkeys: string[];
  editing: boolean;
  features?: EditableFeature[];
  saving?: boolean;
  rowCount: number;
  toolbarTarget: string;
  dataVersion?: number;
  zoomBbox?: string;
  zoomToken?: number;
}>();

const emit = defineEmits<{
  toggleSelection: [pkey: string];
  requestFeature: [pkey: string];
  releaseFeatures: [pkeys: string[]];
  save: [changes: LayerChanges];
  cancel: [];
  drawnCount: [count: number];
}>();

// The session's unsaved work lives inside the map, so the map stays mounted
// for as long as one lasts — maximizing the table hides it rather than
// unmounting it, which would drop every reshape without a word.
const mapMounted = computed(() => props.isGeo && (props.focusPanel !== "table" || props.editing));
const mapHidden = computed(() => props.focusPanel === "table");
</script>

<template>
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
      :bbox="bbox"
      :conditions="conditions"
      :selected-pkeys="selectedPkeys"
      :editing="editing"
      :features="features"
      :saving="saving"
      :row-count="rowCount"
      :toolbar-target="toolbarTarget"
      :data-version="dataVersion"
      :zoom-bbox="zoomBbox"
      :zoom-token="zoomToken"
      @toggle-selection="emit('toggleSelection', $event)"
      @request-feature="emit('requestFeature', $event)"
      @release-features="emit('releaseFeatures', $event)"
      @drawn-count="emit('drawnCount', $event)"
      @save="emit('save', $event)"
      @cancel="emit('cancel')"
    />
  </div>
</template>
