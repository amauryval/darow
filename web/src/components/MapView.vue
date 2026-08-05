<script setup lang="ts">
import { computed, onUnmounted, ref, toRef, watch } from "vue";
import "ol/ol.css";
import type { FilterCondition } from "../client/types";
import { useMapDraw, type DrawShape } from "../composables/useMapDraw";
import type { EditableFeature, LayerChanges } from "../composables/useEditLayer";
import { useOpenLayers } from "../composables/useOpenLayers";
import { useMvtLayer } from "../composables/useMvtLayer";
import { useWfsLayer } from "../composables/useWfsLayer";
import { viewBbox } from "../utils/projection";
import MapEditBar from "./map/MapEditBar.vue";
import BaseCheckbox from "./ui/BaseCheckbox.vue";
import FloatingPanel from "./ui/FloatingPanel.vue";

const props = defineProps<{
  tableId: string;
  bbox?: string;
  conditions: FilterCondition[];
  selectedPkeys: string[];
  // Editing is a mode of the whole layer, on only while the parent says so:
  // outside it the map stays a reading surface.
  editing?: boolean;
  editingPrompt?: string;
  // The rows of the current view at full resolution, loaded by the parent in
  // answer to `request-features`. What is already on the layer is left alone,
  // so a geometry the user moved is never overwritten by a pan.
  features?: EditableFeature[];
  loadingFeatures?: boolean;
  saving?: boolean;
  // Bumped by the parent after every write: a tile is cached by URL, so a
  // rendering only shows an edited geometry once the layers are told to ask
  // again.
  dataVersion?: number;
}>();

// Whoever owns selection (useFeatureSelection, in the parent) applies the
// toggle — this component only reports the click. The session's changes
// leave the same way: the parent decides what they mean and writes them.
const emit = defineEmits<{
  toggleSelection: [pkey: string];
  requestFeatures: [bbox: [number, number, number, number]];
  save: [changes: LayerChanges];
  cancel: [];
}>();

// MVT and WFS are two independent rendering modes for the same table/filter,
// toggled manually by the user — not tied to whether a filter is active.
const wfsEnabled = ref(false);

const container = ref<HTMLDivElement | null>(null);
const tableId = toRef(props, "tableId");
const bbox = toRef(props, "bbox");
const conditions = computed(() => props.conditions);
const selectedPkeys = computed(() => props.selectedPkeys);
const dataVersion = computed(() => props.dataVersion ?? 0);

const { map } = useOpenLayers(container, bbox, () => {
  showTable();
  if (props.editing) startEditing();
});

// Selection is suspended for the whole of an edit session: the click that
// places a vertex must not also toggle a row, and the hover cursor must not
// fight the one Modify sets. The layers stay drawn — only the listeners go.
const selection = {
  selectedPkeys,
  toggle: (pkey: string) => emit("toggleSelection", pkey),
  selectable: computed(() => !props.editing),
};
const mvt = useMvtLayer(map, tableId, bbox, conditions, selection, dataVersion);
const wfs = useWfsLayer(map, tableId, conditions, selection, dataVersion);
const draw = useMapDraw(map);

// MVT and WFS are mutually exclusive on the map — switching modes tears down
// the previous mode's layer entirely rather than stacking both, so there is
// only ever one rendering of the table at a time.
function showTable() {
  if (!map.value) return;
  if (wfsEnabled.value) {
    mvt.deactivate();
    wfs.activate();
  } else {
    wfs.deactivate();
    mvt.activate();
  }
}

watch(wfsEnabled, showTable);

// The table's own rendering stays on under an edit session: it is what shows
// the layer while the editable copies are being fetched, so entering the mode
// never leaves the map blank. The edit layer draws above it (see
// useEditLayer), and a row being reshaped shows its stored geometry
// underneath until the session is saved.

// The session works on what is in view, so what is in view has to be asked
// for: on entering, and on every pan or zoom that reveals more.
// A view that has not been measured yet has no extent to ask about — the
// panel is still being laid out when the mode is entered, and the map then
// reports a zero-sized viewport. Retrying on the next frame is what keeps
// the session from starting empty and staying that way; it cannot be hung on
// `postrender`, because a map with no area draws nothing and so never renders
// another frame to hang on.
let retry = 0;

function cancelRetry() {
  if (retry) cancelAnimationFrame(retry);
  retry = 0;
}

function requestFeatures() {
  const m = map.value;
  if (!m) return;
  cancelRetry();
  const extent = viewBbox(m);
  if (extent) emit("requestFeatures", extent);
  else retry = requestAnimationFrame(requestFeatures);
}

function startEditing() {
  draw.activate();
  requestFeatures();
  map.value?.on("moveend", requestFeatures);
  // Entering the mode gives the map the whole width: the view grows without
  // moving, which is a resize and not a pan, so moveend alone would miss
  // everything the widening revealed — and it is the event that arrives once
  // a viewport that was not measurable finally has an area.
  map.value?.on("change:size", requestFeatures);
}

function stopEditing() {
  cancelRetry();
  map.value?.un("moveend", requestFeatures);
  map.value?.un("change:size", requestFeatures);
  draw.deactivate();
  showTable();
}

onUnmounted(cancelRetry);

watch(
  () => props.features,
  (features) => {
    if (props.editing) draw.load(features ?? []);
  },
);

watch(
  () => props.editing,
  (editing) => {
    if (editing) startEditing();
    else stopEditing();
  },
);

function onSave() {
  emit("save", draw.commit());
}

// Picking a shape arms the drawing; picking the same one again disarms it and
// the map goes back to reshaping, which is what a session spends most of its
// time doing.
function pick(value: DrawShape) {
  draw.setShape(draw.shape.value === value ? null : value);
}
</script>

<template>
  <div class="relative h-full w-full">
    <!-- The render mode is a reading choice, and the table is still drawn
         under an edit session — so the toggle keeps acting on something. -->
    <FloatingPanel position="top-left">
      <label class="flex flex-row items-center gap-1.5 text-xs font-medium">
        <BaseCheckbox v-model="wfsEnabled" />
        Rendu WFS
      </label>
    </FloatingPanel>

    <!-- The editing bar sits over the map for as long as the mode lasts: a
         drag is not a submission, saving is a click. -->
    <FloatingPanel v-if="editing" position="bottom">
      <MapEditBar
        :prompt="editingPrompt"
        :loading-features="loadingFeatures"
        :saving="saving"
        :shape="draw.shape.value"
        :snapping="draw.snapping.value"
        :topological="draw.topological.value"
        :cutting-hole="draw.cuttingHole.value"
        :drawing="draw.drawing.value"
        :has-areal="draw.hasAreal.value"
        :changed-count="draw.changedCount.value"
        :created-count="draw.createdCount.value"
        @pick="pick"
        @toggle-hole="draw.toggleHole"
        @toggle-snapping="draw.toggleSnapping"
        @toggle-topology="draw.toggleTopology"
        @undo="draw.undo"
        @save="onSave"
        @cancel="emit('cancel')"
      />
    </FloatingPanel>

    <div ref="container" class="h-full w-full rounded-lg" />
  </div>
</template>
