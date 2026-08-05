<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import "ol/ol.css";
import type { FilterCondition } from "../client/types";
import { useMapDraw, type DrawShape } from "../composables/useMapDraw";
import type { EditableFeature, LayerChanges } from "../composables/useEditLayer";
import { useOpenLayers } from "../composables/useOpenLayers";
import { useMvtLayer } from "../composables/useMvtLayer";
import { useWfsLayer } from "../composables/useWfsLayer";
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
  // The geometries the parent has loaded at full resolution, in answer to
  // `request-feature`. What is already on the layer is left alone, so a
  // geometry the user moved is never overwritten.
  features?: EditableFeature[];
  loadingFeatures?: boolean;
  saving?: boolean;
  // CSS selector of the workspace bar the session's tools are rendered into.
  // The tools act on the map but belong above both panels: a session is a
  // mode of the table, and half of what it offers (adding a row) has nothing
  // to do with the canvas. They stay inside this component all the same —
  // OpenLayers must not be pulled into the initial bundle by a parent that
  // imports the session's state (§8.10).
  toolbarTarget: string;
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
  // The row the pointer rests over and the session does not hold yet: a
  // session grows by pointing, one geometry at a time (§8.8).
  requestFeature: [pkey: string];
  // The rows the map has stopped holding: untouched geometries the view has
  // left. They are what gives the session's budget back (§8.8).
  releaseFeatures: [pkeys: string[]];
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
// The session reads the rendering to know what the pointer is over: only the
// mode actually on the map can answer, and it is the one the user is reading.
const draw = useMapDraw(map, {
  pkeyAt: (pixel) => (wfsEnabled.value ? wfs.pkeyAt(pixel) : mvt.pkeyAt(pixel)),
  reach: (pkey) => emit("requestFeature", pkey),
});

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

// The session asks for nothing on entering and nothing on panning: what it
// holds is what the pointer has reached. What a move still does is give room
// back — the untouched geometries the view has left are released, so a
// session that walked across a region is not capped by ground it no longer
// shows. Nothing is asked in exchange, so there is no view to measure and no
// unmeasurable viewport to wait for.
function releaseLeftBehind() {
  const released = draw.prune();
  if (released.length > 0) emit("releaseFeatures", released);
}

function startEditing() {
  draw.activate();
  map.value?.on("moveend", releaseLeftBehind);
}

function stopEditing() {
  map.value?.un("moveend", releaseLeftBehind);
  draw.deactivate();
  showTable();
}

watch(
  () => props.features,
  (features) => {
    if (!props.editing) return;
    draw.load(features ?? []);
    // Reaching a geometry is a hover and hovering is cheap, so the count has
    // to be given back where it grows — a session worked in place never pans,
    // and moveend alone would never bound it.
    releaseLeftBehind();
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

    <!-- The session's tools are rendered above both panels rather than over
         the canvas: a drag is not a submission, saving is a click, and the
         bar has to stay reachable even when the map is the hidden half. -->
    <Teleport v-if="editing" :to="toolbarTarget">
      <MapEditBar
        :loading-features="loadingFeatures"
        :saving="saving"
        :shape="draw.shape.value"
        :snapping="draw.snapping.value"
        :topological="draw.topological.value"
        :moving="draw.moving.value"
        :cutting-hole="draw.cuttingHole.value"
        :drawing="draw.drawing.value"
        :has-areal="draw.hasAreal.value"
        :changed-count="draw.changedCount.value"
        :created-count="draw.createdCount.value"
        @pick="pick"
        @toggle-hole="draw.toggleHole"
        @toggle-moving="draw.toggleMoving"
        @toggle-snapping="draw.toggleSnapping"
        @toggle-topology="draw.toggleTopology"
        @undo="draw.undo"
        @save="onSave"
        @cancel="emit('cancel')"
      />
    </Teleport>

    <div ref="container" class="h-full w-full rounded-lg" />
  </div>
</template>
